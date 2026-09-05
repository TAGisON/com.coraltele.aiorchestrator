package control

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/observe"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// enqueuePostcall creates a Queued postcall_job if none is open for the session (idempotent).
func (s *Server) enqueuePostcall(ctx context.Context, sess store.Session) {
	id, err := newID()
	if err != nil {
		log.Printf("postcall id generate failed: %v", err)
		return
	}
	job := store.PostcallJob{
		ID:             id,
		SessionID:      sess.ID,
		ProfileID:      sess.ProfileID,
		ProfileVersion: sess.ProfileVersion,
		State:          store.JobQueued,
	}
	if err := s.repo.CreatePostcallJob(ctx, job); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return
		}
		log.Printf("postcall enqueue fail-open session=%s err=%v", sess.ID, err)
	}
}

// PostcallWorker leases jobs and runs disposition template Think (OPERATIONS.md §2).
type PostcallWorker struct {
	Repo   store.Repository
	Reg    port.Registry
	Owner  string
	Cancel context.CancelFunc
}

// StartPostcallWorker runs a background poller.
func (s *Server) StartPostcallWorker(parent context.Context) *PostcallWorker {
	ctx, cancel := context.WithCancel(parent)
	w := &PostcallWorker{
		Repo:   s.repo,
		Reg:    s.reg,
		Owner:  s.cfg.OwnerInstance,
		Cancel: cancel,
	}
	go w.loop(ctx)
	return w
}

func (w *PostcallWorker) loop(ctx context.Context) {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			job, err := w.Repo.LeaseNextPostcallJob(ctx, w.Owner)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				continue
			}
			w.runJob(ctx, job)
		}
	}
}

func (w *PostcallWorker) runJob(ctx context.Context, job store.PostcallJob) {
	sess, err := w.Repo.GetSession(ctx, job.SessionID)
	if err != nil {
		job.State = store.JobFailed
		job.ErrorMessage = "session missing"
		_ = w.Repo.UpdatePostcallJob(ctx, job)
		return
	}
	pv, err := w.Repo.GetVersion(ctx, job.ProfileID, job.ProfileVersion)
	if err != nil {
		job.State = store.JobFailed
		job.ErrorMessage = "profile missing"
		_ = w.Repo.UpdatePostcallJob(ctx, job)
		return
	}
	doc, err := profile.Parse(pv.Document)
	if err != nil {
		job.State = store.JobFailed
		job.ErrorMessage = "profile corrupt"
		_ = w.Repo.UpdatePostcallJob(ctx, job)
		return
	}

	obs := &observe.Observer{
		Repo: w.Repo,
		Meta: observe.SessionMeta{
			SessionID:      sess.ID,
			TenantID:       sess.TenantID,
			ProfileID:      sess.ProfileID,
			ProfileVersion: sess.ProfileVersion,
			Clock:          sess.Clock,
			RecordingRef:   sess.RecordingRef,
		},
	}

	suggestion := "unresolved"
	if doc.Templates.Disposition != nil && doc.Templates.Disposition.ID != "" {
		suggestion, err = w.runDisposition(ctx, doc, sess)
		if err != nil {
			job.State = store.JobFailed
			job.ErrorMessage = err.Error()
			_ = w.Repo.UpdatePostcallJob(ctx, job)
			obs.Audit(ctx, store.AuditError, map[string]any{
				"stage": "disposition", "message": err.Error(),
			})
			return
		}
	}

	tplID := dispositionTemplateID(doc)
	turns, _ := w.Repo.ListTranscriptTurns(ctx, sess.ID)
	_, _ = w.Repo.UpsertSessionDisposition(ctx, store.SessionDisposition{
		SessionID:  sess.ID,
		Suggestion: suggestion,
		TemplateID: tplID,
		Source:     "postcall_worker",
	})

	obs.Audit(ctx, store.AuditDisposition, map[string]any{
		"suggestion":            suggestion,
		"template_id":           tplID,
		"recording_ref":         sess.RecordingRef,
		"source":                "postcall_worker",
		"transcript_turn_count": len(turns),
		"transcript_link":       "/v1/sessions/" + sess.ID + "/transcript",
	})
	obs.Metric(ctx, "disposition_suggestion", 1, map[string]any{"suggestion": suggestion})

	// Optional coral-crm push: prefer push_disposition when allowed, else create_ticket.
	w.maybePushCRM(ctx, doc, sess, suggestion, tplID)

	job.State = store.JobCompleted
	job.ErrorMessage = ""
	_ = w.Repo.UpdatePostcallJob(ctx, job)
	log.Printf("postcall job %s session=%s disposition=%s", job.ID, job.SessionID, suggestion)
}

func dispositionTemplateID(doc profile.Document) string {
	if doc.Templates.Disposition != nil {
		return doc.Templates.Disposition.ID
	}
	return ""
}

func (w *PostcallWorker) runDisposition(ctx context.Context, doc profile.Document, sess store.Session) (string, error) {
	if len(doc.Routers.Think.Providers) == 0 {
		return "unresolved", nil
	}
	rec, err := router.Select(w.Reg, toGatewayIDs(doc.Routers.Think.Providers), port.PortThink, router.SelectOptions{Clock: "playback"})
	if err != nil {
		return "", err
	}
	th, ok := rec.Instance.(port.Think)
	if !ok {
		return "", errors.New("think type assert failed")
	}
	tpl := dispositionTemplateID(doc)
	excerpt := transcriptExcerpt(ctx, w.Repo, sess.ID)
	if excerpt == "" {
		excerpt = "session " + sess.ID
		if sess.RecordingRef != "" {
			excerpt += " recording_ref=" + sess.RecordingRef
		}
	}
	prompt := "Disposition template " + tpl + ". Classify as resolved, unresolved, or escalated. Transcript excerpt: " + excerpt
	tr, err := th.Complete(ctx, port.ThinkRequest{
		SessionID: port.SessionID(sess.ID),
		Messages: []port.ChatMessage{
			{Role: "system", Content: "You output one disposition tag: resolved | unresolved | escalated."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", err
	}
	return parseDisposition(tr.Text), nil
}

const (
	dispositionExcerptMaxTurns = 20
	dispositionExcerptMaxChars = 4000
)

func transcriptExcerpt(ctx context.Context, repo store.Repository, sessionID string) string {
	if repo == nil {
		return ""
	}
	turns, err := repo.ListTranscriptTurns(ctx, sessionID)
	if err != nil || len(turns) == 0 {
		return ""
	}
	if len(turns) > dispositionExcerptMaxTurns {
		turns = turns[len(turns)-dispositionExcerptMaxTurns:]
	}
	var b strings.Builder
	for _, t := range turns {
		line := t.Role + ": " + t.Text + "\n"
		if b.Len()+len(line) > dispositionExcerptMaxChars {
			break
		}
		b.WriteString(line)
	}
	return strings.TrimSpace(b.String())
}

func parseDisposition(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "escalat"):
		return "escalated"
	case strings.Contains(lower, "resolved"):
		return "resolved"
	case strings.Contains(lower, "unresolved"):
		return "unresolved"
	default:
		return "unresolved"
	}
}

func (w *PostcallWorker) maybePushCRM(ctx context.Context, doc profile.Document, sess store.Session, suggestion, templateID string) {
	if w.tryPushDisposition(ctx, doc, sess, suggestion, templateID) {
		return
	}
	w.tryPushCreateTicket(ctx, doc, sess, suggestion)
}

func skillAllowed(doc profile.Document, name string) bool {
	for _, a := range doc.Skills.Allowed {
		if a == name {
			return true
		}
	}
	return false
}

func (w *PostcallWorker) tryPushDisposition(ctx context.Context, doc profile.Document, sess store.Session, suggestion, templateID string) bool {
	def, ok := doc.Skills.Definitions["push_disposition"]
	if !ok || def.Gateway == "" || !skillAllowed(doc, "push_disposition") {
		return false
	}
	rec, err := router.Select(w.Reg, []port.GatewayID{port.GatewayID(def.Gateway)}, port.PortSkill, router.SelectOptions{})
	if err != nil {
		return false
	}
	sk, ok := rec.Instance.(port.Skill)
	if !ok {
		return false
	}
	excerpt := transcriptExcerpt(ctx, w.Repo, sess.ID)
	args, _ := json.Marshal(map[string]any{
		"action":             "push_disposition",
		"session_id":         sess.ID,
		"suggestion":         suggestion,
		"template_id":        templateID,
		"transcript_excerpt": excerpt,
		"recording_ref":      sess.RecordingRef,
	})
	_, _ = sk.Execute(ctx, port.SkillRequest{
		SessionID: port.SessionID(sess.ID),
		Name:      "push_disposition",
		Args:      args,
		TenantID:  sess.TenantID,
	})
	return true
}

func (w *PostcallWorker) tryPushCreateTicket(ctx context.Context, doc profile.Document, sess store.Session, suggestion string) {
	def, ok := doc.Skills.Definitions["create_ticket"]
	if !ok || def.Gateway == "" || !skillAllowed(doc, "create_ticket") {
		return
	}
	rec, err := router.Select(w.Reg, []port.GatewayID{port.GatewayID(def.Gateway)}, port.PortSkill, router.SelectOptions{})
	if err != nil {
		return
	}
	sk, ok := rec.Instance.(port.Skill)
	if !ok {
		return
	}
	args, _ := json.Marshal(map[string]any{
		"disposition": suggestion,
		"session_id":  sess.ID,
	})
	_, _ = sk.Execute(ctx, port.SkillRequest{
		SessionID: port.SessionID(sess.ID),
		Name:      "create_ticket",
		Args:      args,
		TenantID:  sess.TenantID,
	})
}

func toGatewayIDs(ss []string) []port.GatewayID {
	out := make([]port.GatewayID, len(ss))
	for i, s := range ss {
		out[i] = port.GatewayID(s)
	}
	return out
}

// DetectHandoffFromAudit returns true if warm_transfer skill succeeded in audit.
func DetectHandoffFromAudit(evs []store.AuditEvent) bool {
	for _, ev := range evs {
		if ev.EventType != store.AuditToolExecuted && ev.EventType != "skill.executed" {
			continue
		}
		var p map[string]any
		if json.Unmarshal(ev.Payload, &p) != nil {
			continue
		}
		name, _ := p["name"].(string)
		if name == "" {
			name, _ = p["tool"].(string)
		}
		ok, _ := p["ok"].(bool)
		if name == "warm_transfer" && ok {
			return true
		}
	}
	return false
}

// analyticsEmitSet returns whether profile analytics.emit includes name.
func analyticsEmitSet(doc profile.Document, name string) bool {
	if len(doc.Analytics.Emit) == 0 {
		// Default: emit contained/handoff when analytics block empty (lab profiles).
		return name == "contained" || name == "handoff"
	}
	for _, e := range doc.Analytics.Emit {
		if e == name {
			return true
		}
		if e == "containment" && name == "contained" {
			return true
		}
	}
	return false
}

// ensurePlaybackSession persists a session row for playback so postcall FK works.
func ensurePlaybackSession(ctx context.Context, repo store.Repository, job store.PlaybackJob, sid string, rate int) {
	_, err := repo.GetSession(ctx, sid)
	if err == nil {
		return
	}
	_ = repo.CreateSession(ctx, store.Session{
		ID:                    sid,
		TenantID:              job.TenantID,
		ProfileID:             job.ProfileID,
		ProfileVersion:        job.ProfileVersion,
		Clock:                 "playback",
		State:                 store.StateRunning,
		CanonicalSampleRateHz: rate,
	})
}

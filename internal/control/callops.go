package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/audio"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/fallback"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/record"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// hangupCauseFor maps a failure scenario to the SIP cause we release with, so
// the upstream switch and CDRs can tell a capacity problem from a bug.
func hangupCauseFor(sc fallback.Scenario) string {
	switch sc {
	case fallback.ScenarioCreditsExhausted, fallback.ScenarioSystemBusy:
		return "NORMAL_TEMPORARY_FAILURE"
	case fallback.ScenarioAIUnavailable, fallback.ScenarioTimeout:
		return "NETWORK_OUT_OF_ORDER"
	default:
		return "NORMAL_UNSPECIFIED"
	}
}

// dispositionFor maps FailCall scenarios to the P2.6 system_failure final.
// FailCall is the unrecoverable pipeline exit — not hangup_silence/abuse tools.
func dispositionFor(sc fallback.Scenario) string {
	_ = sc
	return store.DispositionFinalSystemFailure
}

// ---------------------------------------------------------------- recording --

// sessionCallMeta pulls the identifiers the recorder needs out of the durable
// session row. Missing fields are fine — only the session id is required.
func (r *SessionRuntime) sessionCallMeta(sessionID string, a *session.Actor) record.Meta {
	m := record.Meta{
		SessionID:  sessionID,
		SampleRate: int(a.SampleRate),
		FrameMs:    a.FrameMs,
		TenantID:   a.TenantID,
		ProfileID:  a.Profile.ID,
	}
	if r.Repo == nil {
		return m
	}
	sess, err := r.Repo.GetSession(context.Background(), sessionID)
	if err != nil {
		return m
	}
	if len(sess.Metadata) > 0 {
		var meta map[string]any
		if json.Unmarshal(sess.Metadata, &meta) == nil {
			m.CallID = stringField(meta, "call_uuid")
			m.SIPCallID = stringField(meta, "sip_call_id")
			m.Dest = stringField(meta, "destination")
		}
	}
	if len(sess.Caller) > 0 {
		var caller map[string]any
		if json.Unmarshal(sess.Caller, &caller) == nil {
			m.CallerANI = stringField(caller, "ani")
		}
	}
	return m
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// startRecorder begins recording a session. Failures are logged and ignored:
// a call must never fail because it could not be recorded.
func (r *SessionRuntime) startRecorder(sessionID string, a *session.Actor) *record.Recorder {
	if !r.RecordCfg.Enabled {
		return nil
	}
	r.mu.Lock()
	if r.recorders == nil {
		r.recorders = make(map[string]*record.Recorder)
	}
	if existing, ok := r.recorders[sessionID]; ok {
		r.mu.Unlock()
		return existing
	}
	r.mu.Unlock()

	rec, err := record.New(r.RecordCfg, r.sessionCallMeta(sessionID, a))
	if err != nil {
		applog.Error("session recording could not start", "session", sessionID, "err", err)
		return nil
	}
	if rec == nil {
		return nil
	}

	r.mu.Lock()
	// Another attach may have raced us; keep the first and discard ours.
	if existing, ok := r.recorders[sessionID]; ok {
		r.mu.Unlock()
		rec.Close(record.Summary{EndReason: "duplicate"})
		return existing
	}
	r.recorders[sessionID] = rec
	r.mu.Unlock()

	if r.Repo != nil {
		if _, err := r.Repo.MarkSessionRecordingStarted(context.Background(), sessionID, rec.Path()); err != nil {
			applog.Warn("persist recording start", "session", sessionID, "err", err)
		} else {
			r.auditRecording(sessionID, store.AuditRecordingStarted, map[string]any{
				"recording_ref": rec.Path(),
			})
		}
	}
	return rec
}

// recorderFor returns the session recorder, or nil. All Recorder methods are
// nil-safe, so callers do not need to check.
func (r *SessionRuntime) recorderFor(sessionID string) *record.Recorder {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recorders[sessionID]
}

// stopRecorder finalises and detaches the session recording.
func (r *SessionRuntime) stopRecorder(sessionID, endReason string) {
	r.mu.Lock()
	rec := r.recorders[sessionID]
	delete(r.recorders, sessionID)
	fs := r.failScenario[sessionID]
	delete(r.failScenario, sessionID)
	r.mu.Unlock()
	if rec == nil {
		return
	}
	sum := record.Summary{EndReason: endReason}
	if fs != "" {
		sum.Disposition = dispositionFor(fallback.Scenario(fs))
		sum.Extra = map[string]any{"failure_scenario": fs}
	}
	rec.Close(sum)
	reason := store.MapRecordingStopReason(endReason)
	var nbytes *int64
	if n := rec.FileBytes(); n > 0 {
		nbytes = &n
	}
	if r.Repo != nil {
		if _, err := r.Repo.MarkSessionRecordingStopped(context.Background(), sessionID, reason, nbytes); err != nil {
			applog.Warn("persist recording stop", "session", sessionID, "err", err)
		}
	}
	payload := map[string]any{
		"recording_ref": rec.Path(),
		"reason":        reason,
	}
	if nbytes != nil {
		payload["bytes"] = *nbytes
	}
	r.auditRecording(sessionID, store.AuditRecordingStopped, payload)
}

func (r *SessionRuntime) auditRecording(sessionID, eventType string, payload map[string]any) {
	if r == nil || r.Repo == nil {
		return
	}
	r.mu.Lock()
	talk := r.talks[sessionID]
	r.mu.Unlock()
	if talk != nil && talk.Obs != nil {
		talk.Obs.Audit(context.Background(), eventType, payload)
		return
	}
	raw, _ := json.Marshal(payload)
	tenant := ""
	if sess, err := r.Repo.GetSession(context.Background(), sessionID); err == nil {
		tenant = sess.TenantID
	}
	if _, err := r.Repo.AppendAuditEvent(context.Background(), store.AuditEvent{
		SessionID: sessionID,
		TenantID:  tenant,
		EventType: eventType,
		Payload:   raw,
	}); err != nil {
		applog.Warn("recording audit fail-open", "session", sessionID, "type", eventType, "err", err)
	}
}

// ------------------------------------------------------------- call control --

// callControl returns the telephony control surface for a session. Sinks that
// are not a real call leg (file, browser, tests) do not implement it.
func (r *SessionRuntime) callControl(sessionID string) (port.CallControl, *session.Actor, bool) {
	a, ok := r.Mgr.Get(sessionID)
	if !ok {
		return nil, nil, false
	}
	for _, s := range a.Sinks() {
		if cc, ok := s.(port.CallControl); ok {
			return cc, a, true
		}
	}
	return nil, a, false
}

// Transfer hands the caller leg to another extension. Any prompt already queued
// (for example "connecting you now") is played first: the edge holds the
// transfer until its playout buffer drains.
//
// This is the in-process equivalent of
//
//	uuid_transfer <call-uuid> <destination> <dialplan> <context>
func (r *SessionRuntime) Transfer(ctx context.Context, sessionID string, req port.TransferRequest) error {
	dest := strings.TrimSpace(req.Destination)
	if dest == "" {
		return errors.New("transfer: destination required")
	}
	req.Destination = dest

	cc, _, ok := r.callControl(sessionID)
	if !ok || cc == nil {
		return fmt.Errorf("transfer: session %s has no telephony leg", sessionID)
	}

	toolPayload := map[string]any{
		"tool":        "transfer",
		"destination": dest,
		"dialplan":    req.Dialplan,
		"context":     req.Context,
		"reason":      req.Reason,
	}
	r.auditRecording(sessionID, store.AuditToolArmed, toolPayload)
	// Let queued speech finish, but never let a wedged sink block the transfer.
	r.waitPlayout(ctx, sessionID, 10*time.Second)
	r.auditRecording(sessionID, store.AuditToolExecuting, toolPayload)

	if err := cc.Transfer(ctx, req); err != nil {
		fail := map[string]any{"tool": "transfer", "destination": dest, "error": err.Error()}
		r.auditRecording(sessionID, store.AuditToolFailed, fail)
		r.recordLiveDisposition(ctx, sessionID, store.DispositionFinalSystemFailure, err.Error())
		return fmt.Errorf("transfer: %w", err)
	}

	applog.Info("session transferred", "session", sessionID, "dest", dest,
		"dialplan", req.Dialplan, "context", req.Context, "reason", req.Reason)
	okPayload := map[string]any{"tool": "transfer", "destination": dest, "ok": true, "name": "warm_transfer"}
	r.auditRecording(sessionID, store.AuditToolExecuted, okPayload)
	r.recordLiveDisposition(ctx, sessionID, transferDispositionFinal(req), req.Reason)
	return nil
}

// FailCall is the single exit used for every unrecoverable pipeline failure:
// it plays the operator's prompt for the scenario, then releases the call.
//
// It is idempotent per session — the first failure wins, later ones are logged
// and dropped, so a cascade of engine errors produces one announcement.
func (r *SessionRuntime) FailCall(ctx context.Context, sessionID string, sc fallback.Scenario, cause error) {
	r.mu.Lock()
	if r.failScenario == nil {
		r.failScenario = make(map[string]string)
	}
	if prev, exists := r.failScenario[sessionID]; exists {
		r.mu.Unlock()
		applog.Warn("additional failure after call already failing",
			"session", sessionID, "first", prev, "scenario", string(sc), "err", cause)
		return
	}
	r.failScenario[sessionID] = string(sc)
	r.mu.Unlock()

	applog.Error("call failed; playing fallback and releasing",
		"session", sessionID, "scenario", string(sc), "err", cause)

	a, ok := r.Mgr.Get(sessionID)
	if !ok {
		return
	}

	// Stop whatever is mid-flight so the prompt is not mixed with broken TTS.
	r.mu.Lock()
	talk := r.talks[sessionID]
	r.mu.Unlock()
	if talk != nil {
		talk.Interrupt()
	}
	for _, s := range a.Sinks() {
		_ = s.Flush(ctx)
	}

	played := r.playFallback(ctx, sessionID, a, sc)

	cc, _, hasCC := r.callControl(sessionID)
	if hasCC && cc != nil {
		hangPayload := map[string]any{
			"tool":     "hangup",
			"scenario": string(sc),
			"cause":    hangupCauseFor(sc),
		}
		r.auditRecording(sessionID, store.AuditToolArmed, hangPayload)
		if played {
			// Give the prompt time to reach the caller; the edge also drains
			// before acting, this is just our own upper bound.
			r.waitPlayout(ctx, sessionID, 20*time.Second)
		}
		r.auditRecording(sessionID, store.AuditToolExecuting, hangPayload)
		if err := cc.Hangup(ctx, hangupCauseFor(sc)); err != nil {
			applog.Warn("fallback hangup", "session", sessionID, "err", err)
			fail := map[string]any{"tool": "hangup", "scenario": string(sc), "error": err.Error()}
			r.auditRecording(sessionID, store.AuditToolFailed, fail)
		} else {
			okPayload := map[string]any{"tool": "hangup", "scenario": string(sc), "ok": true}
			r.auditRecording(sessionID, store.AuditToolExecuted, okPayload)
			r.waitCallControlGone(sessionID, 16*time.Second)
		}
	}

	r.recordLiveDisposition(ctx, sessionID, dispositionFor(sc), errString(cause))
	if r.OnSessionEnd != nil {
		r.OnSessionEnd(context.WithoutCancel(ctx), sessionID, dispositionFor(sc))
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// playFallback streams the operator prompt for sc to the session's sinks at the
// session sample rate. Returns whether anything was actually played.
func (r *SessionRuntime) playFallback(ctx context.Context, sessionID string, a *session.Actor, sc fallback.Scenario) bool {
	if r.Fallback == nil {
		applog.Warn("no fallback store configured; releasing call silently",
			"session", sessionID, "scenario", string(sc))
		return false
	}
	asset, ok := r.Fallback.Resolve(a.TenantID, sc)
	if !ok {
		applog.Warn("no fallback prompt uploaded; releasing call silently",
			"session", sessionID, "scenario", string(sc), "tenant", a.TenantID)
		return false
	}

	rate := int(a.SampleRate)
	pcm := audio.Resample(asset.PCM, asset.SampleRate, rate)
	frameMs := a.FrameMs
	if frameMs <= 0 {
		frameMs = 20
	}
	n := audio.FrameBytes(rate, frameMs)
	if n <= 0 || len(pcm) == 0 {
		return false
	}

	rec := r.recorderFor(sessionID)
	sinks := a.Sinks()
	var seq uint64
	for off := 0; off < len(pcm); off += n {
		end := off + n
		if end > len(pcm) {
			end = len(pcm)
		}
		chunk := make([]byte, n) // pad the tail with silence, never a short frame
		copy(chunk, pcm[off:end])
		seq++
		frame := port.PCMFrame{
			Data:       chunk,
			SampleRate: port.SampleRateHz(rate),
			Seq:        seq,
			At:         time.Now(),
		}
		rec.WriteAgent(chunk)
		for _, s := range sinks {
			if err := s.WritePCM(ctx, frame); err != nil {
				applog.Warn("fallback prompt write", "session", sessionID, "err", err)
				return seq > 1
			}
		}
	}

	applog.Info("fallback prompt played", "session", sessionID,
		"scenario", string(sc), "file", asset.Path, "duration_ms", asset.DurationMs)
	return true
}

// waitPlayout blocks until every sink reports its queue drained, or timeout.
func (r *SessionRuntime) waitPlayout(ctx context.Context, sessionID string, timeout time.Duration) {
	a, ok := r.Mgr.Get(sessionID)
	if !ok {
		return
	}
	waitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	for _, s := range a.Sinks() {
		if err := s.WaitMark(waitCtx); err != nil {
			applog.Warn("playout wait ended early", "session", sessionID, "err", err)
			return
		}
	}
}

// recordLiveDisposition persists a P2.6 allowlisted final after tool settle.
// Best effort: never blocks the exit path.
func (r *SessionRuntime) recordLiveDisposition(ctx context.Context, sessionID, final, note string) {
	if r.Repo == nil {
		return
	}
	final = strings.TrimSpace(final)
	if !store.ValidDispositionFinal(final) {
		applog.Warn("reject non-allowlist disposition; using system_failure",
			"session", sessionID, "got", final, "note", note)
		final = store.DispositionFinalSystemFailure
	}
	d := store.SessionDisposition{
		SessionID: sessionID,
		Final:     final,
		Source:    store.DispositionSourceLiveTool,
	}
	if cur, err := r.Repo.GetSessionDisposition(context.WithoutCancel(ctx), sessionID); err == nil {
		d.Suggestion = cur.Suggestion
		d.TemplateID = cur.TemplateID
	}
	if _, err := r.Repo.UpsertSessionDisposition(context.WithoutCancel(ctx), d); err != nil {
		applog.Warn("persist disposition", "session", sessionID,
			"disposition", final, "note", note, "err", err)
	}
}

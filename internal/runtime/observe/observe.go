// Package observe provides best-effort audit and analytics writers for the Talk hot path.
// Fail-open: store errors are logged; media path continues.
package observe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// SessionMeta identifies the pinned session for correlation.
type SessionMeta struct {
	SessionID      string
	TenantID       string
	ProfileID      string
	ProfileVersion int
	Clock          string
	RecordingRef   string
	Caller         json.RawMessage
	Metadata       json.RawMessage
}

// Observer appends audit/analytics rows (best-effort).
type Observer struct {
	Repo store.Repository
	Meta SessionMeta
}

// storeCtx ignores caller cancel so hangup/stop does not drop transcript/audit rows.
func storeCtx(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

// Audit appends an immutable audit_event. Errors are logged only.
func (o *Observer) Audit(ctx context.Context, eventType string, payload map[string]any) {
	if o == nil || o.Repo == nil {
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["profile_id"] = o.Meta.ProfileID
	payload["profile_version"] = o.Meta.ProfileVersion
	if o.Meta.Clock != "" {
		payload["clock"] = o.Meta.Clock
	}
	if o.Meta.RecordingRef != "" {
		payload["recording_ref"] = o.Meta.RecordingRef
	}
	raw, _ := json.Marshal(payload)
	_, err := o.Repo.AppendAuditEvent(storeCtx(ctx), store.AuditEvent{
		SessionID: o.Meta.SessionID,
		TenantID:  o.Meta.TenantID,
		EventType: eventType,
		Payload:   raw,
	})
	if err != nil {
		applog.Warn("observe audit fail-open", "session", o.Meta.SessionID, "type", eventType, "err", err)
	}
}

// Metric appends an analytics_event. Errors are logged only.
func (o *Observer) Metric(ctx context.Context, metric string, value float64, dims map[string]any) {
	if o == nil || o.Repo == nil {
		return
	}
	if value == 0 {
		value = 1
	}
	var raw json.RawMessage
	if dims != nil {
		raw, _ = json.Marshal(dims)
	}
	_, err := o.Repo.AppendAnalyticsEvent(storeCtx(ctx), store.AnalyticsEvent{
		TenantID:   o.Meta.TenantID,
		ProfileID:  o.Meta.ProfileID,
		SessionID:  o.Meta.SessionID,
		Metric:     metric,
		Value:      value,
		Dimensions: raw,
	})
	if err != nil {
		applog.Warn("observe analytics fail-open", "session", o.Meta.SessionID, "metric", metric, "err", err)
	}
}

// TurnCompleted is the durable Talk turn completion hook (once per think+speak cycle).
type TurnCompleted struct {
	UserText      string
	ResponseText  string
	BargeIn       bool
	SkillName     string
	SkillOK       bool
	KnowledgeHit  bool
	GroundingReq  bool
	ListenGateway string
	ThinkGateway  string
	SpeakGateway  string
	VoiceID       string
	ResponseTier string // clip | template | llm | refuse | escalate
	Outcome       string
	LatencyMs     int64
	TurnID        string // optional; generated when empty
}

// OnTurnCompleted writes turn.completed audit + turn_completed analytics (and related metrics).
// Also appends durable transcript turns (user then assistant) with shared turn_id (fail-open).
func (o *Observer) OnTurnCompleted(ctx context.Context, t TurnCompleted) {
	if o == nil {
		return
	}
	turnID := t.TurnID
	if turnID == "" {
		turnID = newTurnID()
	}
	o.appendTranscript(ctx, turnID, t.UserText, t.ResponseText)

	payload := map[string]any{
		"turn_id":            turnID,
		"user_text_redacted": truncate(t.UserText, 256),
		"response_redacted":  truncate(t.ResponseText, 256),
		"barge_in":           t.BargeIn,
		"outcome":            t.Outcome,
		"gateways": map[string]string{
			"listen": t.ListenGateway,
			"think":  t.ThinkGateway,
			"speak":  t.SpeakGateway,
		},
	}
	if t.VoiceID != "" {
		payload["voice_id"] = t.VoiceID
	}
	if t.ResponseTier != "" {
		payload["response_tier"] = t.ResponseTier
	}
	if t.SkillName != "" {
		payload["skill_name"] = t.SkillName
		payload["skill_ok"] = t.SkillOK
	}
	o.Audit(ctx, store.AuditTurnCompleted, payload)
	dims := map[string]any{}
	if t.ResponseTier != "" {
		dims["response_tier"] = t.ResponseTier
	}
	if len(dims) == 0 {
		dims = nil
	}
	o.Metric(ctx, store.MetricTurnCompleted, 1, dims)
	if t.SkillName != "" {
		o.Audit(ctx, store.AuditSkillExecuted, map[string]any{
			"name": t.SkillName,
			"ok":   t.SkillOK,
		})
	}
	if t.GroundingReq && !t.KnowledgeHit {
		o.Metric(ctx, store.MetricNoGroundingHit, 1, nil)
	}
	if t.LatencyMs > 0 {
		o.Metric(ctx, store.MetricHopLatencyMs, float64(t.LatencyMs), map[string]any{"hop": "turn"})
	}
}

func (o *Observer) appendTranscript(ctx context.Context, turnID, userText, responseText string) {
	if o == nil || o.Repo == nil || o.Meta.SessionID == "" {
		return
	}
	writeCtx := storeCtx(ctx)
	for _, row := range []store.TranscriptTurn{
		{SessionID: o.Meta.SessionID, Role: store.RoleUser, Text: userText, TurnID: turnID},
		{SessionID: o.Meta.SessionID, Role: store.RoleAssistant, Text: responseText, TurnID: turnID},
	} {
		if _, err := o.Repo.AppendTranscriptTurn(writeCtx, row); err != nil {
			applog.Warn("observe transcript fail-open", "session", o.Meta.SessionID, "role", row.Role, "err", err)
		}
	}
}

// AppendAssistantOnly writes a single assistant transcript turn (call answer / opening).
func (o *Observer) AppendAssistantOnly(ctx context.Context, responseText string) {
	if o == nil || o.Repo == nil || o.Meta.SessionID == "" {
		return
	}
	if strings.TrimSpace(responseText) == "" {
		return
	}
	turnID := newTurnID()
	_, err := o.Repo.AppendTranscriptTurn(storeCtx(ctx), store.TranscriptTurn{
		SessionID: o.Meta.SessionID,
		Role:      store.RoleAssistant,
		Text:      responseText,
		TurnID:    turnID,
	})
	if err != nil {
		applog.Warn("observe transcript fail-open", "session", o.Meta.SessionID, "role", store.RoleAssistant, "err", err)
	}
}

func newTurnID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
	}
	return hex.EncodeToString(b[:])
}

// OnBargeIn records barge_in audit + analytics.
func (o *Observer) OnBargeIn(ctx context.Context) {
	if o == nil {
		return
	}
	o.Audit(ctx, store.AuditBargeIn, map[string]any{"ts_ms": time.Now().UnixMilli()})
	o.Metric(ctx, store.MetricBargeIn, 1, nil)
}

// OnSessionStarted records session start.
func (o *Observer) OnSessionStarted(ctx context.Context) {
	if o == nil {
		return
	}
	o.Audit(ctx, store.AuditSessionStarted, map[string]any{
		"state":    store.StateRunning,
		"caller":   jsonOrNil(o.Meta.Caller),
		"metadata": jsonOrNil(o.Meta.Metadata),
	})
	o.Metric(ctx, store.MetricSessionStarted, 1, nil)
}

// OnSessionTerminal records terminal state, containment/handoff, and optional analytics.
func (o *Observer) OnSessionTerminal(ctx context.Context, terminal string, handoff bool, emitContained, emitHandoff bool) {
	if o == nil {
		return
	}
	o.Audit(ctx, store.AuditSessionTerminal, map[string]any{"state": terminal, "handoff": handoff})
	switch terminal {
	case store.StateFailed:
		o.Metric(ctx, store.MetricSessionFailed, 1, nil)
	case store.StateCompleted, store.StateCancelled:
		o.Metric(ctx, store.MetricSessionCompleted, 1, map[string]any{"state": terminal})
	}
	if handoff && emitHandoff {
		o.Metric(ctx, store.MetricHandoff, 1, nil)
	} else if !handoff && terminal == store.StateCompleted && emitContained {
		o.Metric(ctx, store.MetricContained, 1, nil)
	}
}

func jsonOrNil(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return json.RawMessage(raw)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

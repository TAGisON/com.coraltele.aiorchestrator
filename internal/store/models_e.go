package store

import (
	"encoding/json"
	"time"
)

// P2.4 V1 audit event_type allowlist (docs/phases/P2.4_audit_events.md).
const (
	AuditSessionCreated   = "session.created"
	AuditSessionLive      = "session.live"
	AuditSessionEnding    = "session.ending"
	AuditSessionCompleted = "session.completed"
	AuditSessionCancelled = "session.cancelled"
	AuditSessionFailed    = "session.failed"
	AuditTurnState        = "turn.state"
	AuditSTTFinal         = "stt.final"
	AuditGraphEdge        = "graph.edge"
	AuditToolArmed        = "tool.armed"
	AuditToolExecuting    = "tool.executing"
	AuditToolExecuted     = "tool.executed"
	AuditToolFailed       = "tool.failed"
	AuditRecordingStarted = "recording.started"
	AuditRecordingStopped = "recording.stopped"
	AuditLanguageChanged  = "language.changed"

	// Kept until E.5 revisits disposition evidence (P2.4 inventory).
	AuditDisposition = "disposition.suggestion"

	// Legacy aliases — prefer allowlist names above. Values match transitioned emitters.
	AuditSessionStarted  = AuditSessionLive
	AuditSessionTerminal = "session.terminal" // do not emit; OnSessionTerminal maps to completed/cancelled/failed
	AuditTurnCompleted   = AuditTurnState
	AuditSkillExecuted   = AuditToolExecuted
	AuditBargeIn         = AuditTurnState
	AuditError           = AuditSessionFailed
)

// AuditEventTypes returns the P2.4 V1 emitter allowlist (no legacy aliases).
func AuditEventTypes() []string {
	return []string{
		AuditSessionCreated, AuditSessionLive, AuditSessionEnding,
		AuditSessionCompleted, AuditSessionCancelled, AuditSessionFailed,
		AuditTurnState, AuditSTTFinal, AuditGraphEdge,
		AuditToolArmed, AuditToolExecuting, AuditToolExecuted, AuditToolFailed,
		AuditRecordingStarted, AuditRecordingStopped, AuditLanguageChanged,
		AuditDisposition,
	}
}

// AuditTerminalType maps a durable session terminal state to a P2.4 event_type.
func AuditTerminalType(state string) string {
	switch state {
	case StateCompleted:
		return AuditSessionCompleted
	case StateCancelled:
		return AuditSessionCancelled
	case StateFailed:
		return AuditSessionFailed
	default:
		return AuditSessionCompleted
	}
}

// Locked analytics metric names (ANALYTICS_AND_POSTCALL.md §2).
const (
	MetricSessionStarted   = "session_started"
	MetricSessionCompleted = "session_completed"
	MetricSessionFailed    = "session_failed"
	MetricTurnCompleted    = "turn_completed"
	MetricNoGroundingHit   = "no_grounding_hit"
	MetricHandoff          = "handoff"
	MetricContained        = "contained"
	MetricBargeIn          = "barge_in"
	MetricHopLatencyMs     = "hop_latency_ms"
	// Live-talk barge (LIVE_TALK_CX_AND_INDIA_LANGUAGE.md §3.5).
	MetricBargeCandidateTotal    = "cd_barge_candidate_total"
	MetricBargeCommitTotal       = "cd_barge_commit_total"
	MetricBargeSuppressEchoTotal = "cd_barge_suppress_echo_total"
	MetricWelcomeFirstAudioMs    = "cd_welcome_first_audio_ms"
)

// AuditEvent is an append-only compliance row.
type AuditEvent struct {
	ID        int64
	SessionID string
	TenantID  string
	EventType string
	Payload   json.RawMessage
	CreatedAt time.Time
}

// AnalyticsEvent is an append-only product metric row.
type AnalyticsEvent struct {
	ID         int64
	TenantID   string
	ProfileID  string
	SessionID  string
	Metric     string
	Value      float64
	Dimensions json.RawMessage
	CreatedAt  time.Time
}

// PostcallJob is a disposition/summary work item (OPERATIONS.md §2).
type PostcallJob struct {
	ID             string
	SessionID      string
	ProfileID      string
	ProfileVersion int
	State          string
	LeaseOwner     string
	ErrorMessage   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

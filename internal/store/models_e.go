package store

import (
	"encoding/json"
	"time"
)

// Closed audit event_type set for Phase E (INTEGRATION.md §5 / turn correlation).
const (
	AuditSessionStarted  = "session.started"
	AuditSessionTerminal = "session.terminal"
	AuditTurnCompleted   = "turn.completed"
	AuditSkillExecuted   = "skill.executed"
	AuditBargeIn         = "barge_in"
	AuditError           = "error"
	AuditDisposition     = "disposition.suggestion"
)

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

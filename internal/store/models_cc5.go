package store

import (
	"encoding/json"
	"time"
)

// Closed transcript roles (CONTROL_API transcript turns).
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	RoleTool      = "tool"
)

// V1 transcript event_kind (P2.3 / docs 09). Emitters must not write utterance for new rows.
const (
	EventKindUserFinal    = "user_final"
	EventKindBotUtterance = "bot_utterance"
	EventKindEdgeTaken    = "edge_taken"
	EventKindToolLine     = "tool_line"
	EventKindNote         = "note"
	EventKindUtterance    = "utterance" // legacy placeholder only
)

// Closed actionable_reason starter set (docs/09 B1).
const (
	ActionableReasonEchoSuspect    = "echo_suspect"
	ActionableReasonToolLocked     = "tool_locked"
	ActionableReasonBargeForbidden = "barge_forbidden"
	ActionableReasonTooShort       = "too_short"
	ActionableReasonEmpty          = "empty"
	ActionableReasonThinkingBusy   = "thinking_busy"
	ActionableReasonEnding         = "ending"
	ActionableReasonLegacyImport   = "legacy_import"
)

// Closed disposition suggestion tags (ANALYTICS_AND_POSTCALL.md §3).
const (
	DispositionResolved   = "resolved"
	DispositionUnresolved = "unresolved"
	DispositionEscalated  = "escalated"
)

// TranscriptTurn is one ordered durable transcript event (table name kept from turn-pair era).
type TranscriptTurn struct {
	ID               int64
	SessionID        string
	Seq              int
	Role             string
	Text             string
	TurnID           string // correlation; may be empty
	EventKind        string
	Actionable       *bool // nil = N/A
	ActionableReason string
	NodeID           string
	EdgeID           string
	Language         string
	Payload          json.RawMessage
	CreatedAt        time.Time
}

// BoolPtr is a helper for Actionable fields.
func BoolPtr(v bool) *bool { return &v }

// SessionDisposition is the AI disposition suggestion (and optional final) for a session.
type SessionDisposition struct {
	SessionID  string
	Suggestion string
	TemplateID string
	Source     string
	Final      string // empty = null / unset
	UpdatedAt  time.Time
}

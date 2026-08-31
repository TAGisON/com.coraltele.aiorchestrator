package store

import "time"

// Closed transcript roles (CONTROL_API transcript turns).
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// Closed disposition suggestion tags (ANALYTICS_AND_POSTCALL.md §3).
const (
	DispositionResolved   = "resolved"
	DispositionUnresolved = "unresolved"
	DispositionEscalated  = "escalated"
)

// TranscriptTurn is one ordered durable turn row keyed by session_id.
// Shared turn_id correlates user+assistant rows for one Talk cycle (CONTROL_API.md).
type TranscriptTurn struct {
	ID        int64
	SessionID string
	Seq       int
	Role      string
	Text      string
	TurnID    string
	CreatedAt time.Time
}

// SessionDisposition is the AI disposition suggestion (and optional final) for a session.
type SessionDisposition struct {
	SessionID  string
	Suggestion string
	TemplateID string
	Source     string
	Final      string // empty = null / unset
	UpdatedAt  time.Time
}

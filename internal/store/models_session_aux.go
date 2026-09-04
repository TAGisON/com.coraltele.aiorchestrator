package store

import (
	"time"
)

// SessionAttribute is one durable contact attribute for a session.
type SessionAttribute struct {
	SessionID string
	Key       string
	Value     string
	Class     string
	UpdatedAt time.Time
}

// CallerPreference remembers per-ANI product prefs across calls (language first).
// Keyed by tenant + normalised ANI.
type CallerPreference struct {
	TenantID          string
	ANI               string
	PreferredLanguage string
	Source            string // stt_lock | operator | default
	UpdatedAt         time.Time
}

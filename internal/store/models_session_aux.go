package store

import (
	"encoding/json"
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

// SkillInvocation is the idempotency + audit ledger row for one connector call.
type SkillInvocation struct {
	ID             int64
	SessionID      string
	TenantID       string
	Skill          string
	IdempotencyKey string
	Status         string
	Args           json.RawMessage
	Output         json.RawMessage
	Error          string
	CreatedAt      time.Time
}

// PIIAccess records a read of a confidential attribute.
type PIIAccess struct {
	ID        int64
	TenantID  string
	SessionID string
	Actor     string
	Keys      string
	Reason    string
	CreatedAt time.Time
}

// ErasureRequest is a subject erasure ticket.
type ErasureRequest struct {
	ID          string
	TenantID    string
	SubjectRef  string
	Scope       string
	Status      string
	RequestedBy string
	RequestedAt time.Time
	CompletedAt *time.Time
}

// ConsentRecord is an outbound consent decision cached per number.
type ConsentRecord struct {
	TenantID  string
	Phone     string
	State     string
	Source    string
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

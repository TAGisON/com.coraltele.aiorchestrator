package store

import (
	"encoding/json"
	"time"
)

// Desk status values (CONTACT_DESK_POC_SOLUTION.md §6.1).
const (
	DeskStatusDraft       = "draft"
	DeskStatusPublished   = "published"
	DeskStatusUnpublished = "unpublished"
)

// Desk is the desk registry row; the authored body lives in DeskDraft / DeskVersion.
type Desk struct {
	ID             string
	TenantID       string
	Name           string
	Direction      string
	Purpose        string
	Status         string
	CurrentVersion int
	ProfileID      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// DeskDraft is the mutable working copy edited in the Configurator GUI.
type DeskDraft struct {
	DeskID    string
	Doc       json.RawMessage
	UpdatedAt time.Time
}

// DeskVersion is an immutable published desk snapshot bound to a profile version.
type DeskVersion struct {
	DeskID         string
	Version        int
	Doc            json.RawMessage
	ProfileID      string
	ProfileVersion int
	ContentHash    string
	PublishedBy    string
	PublishedAt    time.Time
}

// SessionAttribute is one durable contact attribute for a session (§6.5).
type SessionAttribute struct {
	SessionID string
	Key       string
	Value     string
	Class     string
	UpdatedAt time.Time
}

// SkillInvocation is the idempotency + audit ledger row for one connector call (§9).
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

// PIIAccess records a read of a confidential attribute (§11).
type PIIAccess struct {
	ID        int64
	TenantID  string
	SessionID string
	Actor     string
	Keys      string
	Reason    string
	CreatedAt time.Time
}

// ErasureRequest is a subject erasure ticket (§11).
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

// ConsentRecord is an outbound consent decision cached per number (§19.1).
type ConsentRecord struct {
	TenantID  string
	Phone     string
	State     string
	Source    string
	UpdatedAt time.Time
}

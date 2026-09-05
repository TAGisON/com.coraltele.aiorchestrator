package store

import (
	"encoding/json"
	"time"
)

// Flow status / direction (P2.7 Locked).
const (
	FlowDirectionInbound  = "inbound"
	FlowDirectionOutbound = "outbound"

	FlowStatusDraft       = "draft"
	FlowStatusPublished   = "published"
	FlowStatusUnpublished = "unpublished"

	BindingKindKnowledge  = "knowledge"
	BindingKindCRM        = "crm"
	BindingStatusActive   = "active"
	BindingStatusDisabled = "disabled"
)

// Flow is the registry row for a conversation graph.
type Flow struct {
	ID             string
	TenantID       string
	Name           string
	Direction      string
	Status         string
	CurrentVersion int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// FlowDraft is the mutable working document.
type FlowDraft struct {
	FlowID    string
	Doc       json.RawMessage
	UpdatedAt time.Time
}

// FlowVersion is an immutable published document.
type FlowVersion struct {
	FlowID      string
	Version     int
	Doc         json.RawMessage
	ContentHash string
	PublishedBy string
	PublishedAt time.Time
}

// Binding is a tenant-scoped knowledge/CRM capability (P2.10).
type Binding struct {
	ID        string
	TenantID  string
	Kind      string
	Name      string
	Config    json.RawMessage
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

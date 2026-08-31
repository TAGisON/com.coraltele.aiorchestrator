package store

import (
	"encoding/json"
	"time"
)

// GatewayCredential holds a vendor/gateway secret for a tenant (runtime DB config).
type GatewayCredential struct {
	TenantID  string
	GatewayID string
	APIKey    string
	Extra     json.RawMessage
	UpdatedAt time.Time
}

// SystemSetting is a tenant-scoped string setting (e.g. coral.base_url).
type SystemSetting struct {
	TenantID  string
	Key       string
	Value     string
	UpdatedAt time.Time
}

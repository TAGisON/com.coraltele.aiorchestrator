package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// GatewayBinding is the session-pinned Listen/Think/Speak snapshot.
type GatewayBinding struct {
	Listen string `json:"listen"`
	Think  string `json:"think"`
	Speak  string `json:"speak"`
}

// TenantEngines is the durable one-active-engine-per-slot row for a tenant.
type TenantEngines struct {
	TenantID  string
	ListenID  string
	ThinkID   string
	SpeakID   string
	UpdatedAt time.Time
}

// Binding returns the GatewayBinding shape for session pin / API.
func (t TenantEngines) Binding() GatewayBinding {
	return GatewayBinding{Listen: t.ListenID, Think: t.ThinkID, Speak: t.SpeakID}
}

func (s *Store) GetTenantEngines(ctx context.Context, tenantID string) (TenantEngines, error) {
	var te TenantEngines
	err := s.pool.QueryRow(ctx, `
SELECT tenant_id, listen_id, think_id, speak_id, updated_at
FROM tenant_engines WHERE tenant_id=$1
`, tenantID).Scan(&te.TenantID, &te.ListenID, &te.ThinkID, &te.SpeakID, &te.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantEngines{}, ErrNotFound
	}
	if err != nil {
		return TenantEngines{}, err
	}
	return te, nil
}

func (s *Store) UpsertTenantEngines(ctx context.Context, te TenantEngines) (TenantEngines, error) {
	err := s.pool.QueryRow(ctx, `
INSERT INTO tenant_engines (tenant_id, listen_id, think_id, speak_id, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (tenant_id) DO UPDATE SET
  listen_id = EXCLUDED.listen_id,
  think_id = EXCLUDED.think_id,
  speak_id = EXCLUDED.speak_id,
  updated_at = now()
RETURNING tenant_id, listen_id, think_id, speak_id, updated_at
`, te.TenantID, te.ListenID, te.ThinkID, te.SpeakID).Scan(
		&te.TenantID, &te.ListenID, &te.ThinkID, &te.SpeakID, &te.UpdatedAt,
	)
	if err != nil {
		return TenantEngines{}, err
	}
	return te, nil
}

func marshalGatewayBinding(b *GatewayBinding) any {
	if b == nil || (b.Listen == "" && b.Think == "" && b.Speak == "") {
		return nil
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return nil
	}
	return raw
}

func scanGatewayBinding(raw []byte) *GatewayBinding {
	if len(raw) == 0 {
		return nil
	}
	var b GatewayBinding
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil
	}
	if b.Listen == "" && b.Think == "" && b.Speak == "" {
		return nil
	}
	return &b
}

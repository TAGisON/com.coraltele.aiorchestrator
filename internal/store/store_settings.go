package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) GetGatewayCredential(ctx context.Context, tenantID, gatewayID string) (GatewayCredential, error) {
	var c GatewayCredential
	var extra []byte
	err := s.pool.QueryRow(ctx, `
SELECT tenant_id, gateway_id, api_key, extra, updated_at
FROM gateway_credentials WHERE tenant_id=$1 AND gateway_id=$2
`, tenantID, gatewayID).Scan(&c.TenantID, &c.GatewayID, &c.APIKey, &extra, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return GatewayCredential{}, ErrNotFound
	}
	if err != nil {
		return GatewayCredential{}, err
	}
	c.Extra = json.RawMessage(extra)
	return c, nil
}

func (s *Store) UpsertGatewayCredential(ctx context.Context, c GatewayCredential) (GatewayCredential, error) {
	if len(c.Extra) == 0 {
		c.Extra = json.RawMessage(`{}`)
	}
	now := time.Now().UTC()
	_, err := s.pool.Exec(ctx, `
INSERT INTO gateway_credentials (tenant_id, gateway_id, api_key, extra, updated_at)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (tenant_id, gateway_id) DO UPDATE SET
  api_key=EXCLUDED.api_key,
  extra=EXCLUDED.extra,
  updated_at=EXCLUDED.updated_at
`, c.TenantID, c.GatewayID, c.APIKey, []byte(c.Extra), now)
	if err != nil {
		return GatewayCredential{}, err
	}
	c.UpdatedAt = now
	return c, nil
}

func (s *Store) ListGatewayCredentials(ctx context.Context, tenantID string) ([]GatewayCredential, error) {
	rows, err := s.pool.Query(ctx, `
SELECT tenant_id, gateway_id, api_key, extra, updated_at
FROM gateway_credentials WHERE tenant_id=$1 ORDER BY gateway_id
`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GatewayCredential
	for rows.Next() {
		var c GatewayCredential
		var extra []byte
		if err := rows.Scan(&c.TenantID, &c.GatewayID, &c.APIKey, &extra, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.Extra = json.RawMessage(extra)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetSystemSetting(ctx context.Context, tenantID, key string) (SystemSetting, error) {
	var st SystemSetting
	err := s.pool.QueryRow(ctx, `
SELECT tenant_id, key, value, updated_at FROM system_settings WHERE tenant_id=$1 AND key=$2
`, tenantID, key).Scan(&st.TenantID, &st.Key, &st.Value, &st.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SystemSetting{}, ErrNotFound
	}
	if err != nil {
		return SystemSetting{}, err
	}
	return st, nil
}

func (s *Store) UpsertSystemSetting(ctx context.Context, st SystemSetting) (SystemSetting, error) {
	now := time.Now().UTC()
	_, err := s.pool.Exec(ctx, `
INSERT INTO system_settings (tenant_id, key, value, updated_at)
VALUES ($1,$2,$3,$4)
ON CONFLICT (tenant_id, key) DO UPDATE SET
  value=EXCLUDED.value,
  updated_at=EXCLUDED.updated_at
`, st.TenantID, st.Key, st.Value, now)
	if err != nil {
		return SystemSetting{}, err
	}
	st.UpdatedAt = now
	return st, nil
}

func (s *Store) ListSystemSettings(ctx context.Context, tenantID string) ([]SystemSetting, error) {
	rows, err := s.pool.Query(ctx, `
SELECT tenant_id, key, value, updated_at FROM system_settings WHERE tenant_id=$1 ORDER BY key
`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SystemSetting
	for rows.Next() {
		var st SystemSetting
		if err := rows.Scan(&st.TenantID, &st.Key, &st.Value, &st.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

package store

import (
	"context"
)

// ListProfiles returns profiles ordered by id (lab / POC console).
func (s *Store) ListProfiles(ctx context.Context, limit int) ([]Profile, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, COALESCE(tenant_id,''), COALESCE(display_name,''), created_at, updated_at
FROM profile
ORDER BY id
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Profile
	for rows.Next() {
		var p Profile
		if err := rows.Scan(&p.ID, &p.TenantID, &p.DisplayName, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListSessions returns recent sessions (lab / POC console).
func (s *Store) ListSessions(ctx context.Context, limit int) ([]Session, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, COALESCE(tenant_id,''), profile_id, profile_version, clock, state,
       COALESCE(owner_instance,''), canonical_sample_rate_hz, COALESCE(coral_user_id,''),
       caller, recording_ref, metadata, gateway_binding, created_at, updated_at
FROM session
ORDER BY created_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var sess Session
		var rec *string
		var binding []byte
		if err := rows.Scan(
			&sess.ID, &sess.TenantID, &sess.ProfileID, &sess.ProfileVersion, &sess.Clock, &sess.State,
			&sess.OwnerInstance, &sess.CanonicalSampleRateHz, &sess.CoralUserID,
			&sess.Caller, &rec, &sess.Metadata, &binding, &sess.CreatedAt, &sess.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if rec != nil {
			sess.RecordingRef = *rec
		}
		sess.GatewayBinding = scanGatewayBinding(binding)
		out = append(out, sess)
	}
	return out, rows.Err()
}

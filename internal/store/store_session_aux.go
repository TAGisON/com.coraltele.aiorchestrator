package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) UpsertSessionAttributes(ctx context.Context, sessionID string, attrs []SessionAttribute) error {
	if len(attrs) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, a := range attrs {
		batch.Queue(`
INSERT INTO session_attributes (session_id, key, value, class, updated_at)
VALUES ($1,$2,$3,$4, now())
ON CONFLICT (session_id, key) DO UPDATE SET value=EXCLUDED.value, class=EXCLUDED.class, updated_at=now()`,
			sessionID, a.Key, a.Value, a.Class)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range attrs {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListSessionAttributes(ctx context.Context, sessionID string) ([]SessionAttribute, error) {
	rows, err := s.pool.Query(ctx, `
SELECT session_id, key, value, class, updated_at FROM session_attributes
WHERE session_id=$1 ORDER BY key`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionAttribute
	for rows.Next() {
		var a SessionAttribute
		if err := rows.Scan(&a.SessionID, &a.Key, &a.Value, &a.Class, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) UpsertCallerPreference(ctx context.Context, p CallerPreference) (CallerPreference, error) {
	var out CallerPreference
	err := s.pool.QueryRow(ctx, `
INSERT INTO caller_preference (tenant_id, ani, preferred_language, source, updated_at)
VALUES ($1,$2,$3,$4, now())
ON CONFLICT (tenant_id, ani) DO UPDATE SET
  preferred_language=EXCLUDED.preferred_language,
  source=EXCLUDED.source,
  updated_at=now()
RETURNING tenant_id, ani, preferred_language, source, updated_at`,
		p.TenantID, p.ANI, p.PreferredLanguage, p.Source).
		Scan(&out.TenantID, &out.ANI, &out.PreferredLanguage, &out.Source, &out.UpdatedAt)
	return out, err
}

func (s *Store) GetCallerPreference(ctx context.Context, tenantID, ani string) (CallerPreference, error) {
	var out CallerPreference
	err := s.pool.QueryRow(ctx, `
SELECT tenant_id, ani, preferred_language, source, updated_at
FROM caller_preference WHERE tenant_id=$1 AND ani=$2`,
		tenantID, ani).Scan(&out.TenantID, &out.ANI, &out.PreferredLanguage, &out.Source, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CallerPreference{}, ErrNotFound
	}
	return out, err
}

func (s *Store) CountActiveSessions(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
SELECT count(*) FROM session WHERE tenant_id=$1 AND state IN ('Created','Attached','Running','Draining')`,
		tenantID).Scan(&n)
	return n, err
}

// PurgeSessionData deletes derived rows for a session (erasure worker).
func (s *Store) PurgeSessionData(ctx context.Context, sessionID string) error {
	statements := []string{
		`DELETE FROM session_attributes WHERE session_id=$1`,
		`DELETE FROM transcript_turn WHERE session_id=$1`,
	}
	for _, q := range statements {
		if _, err := s.pool.Exec(ctx, q, sessionID); err != nil {
			return err
		}
	}
	return nil
}

var _ = time.Now

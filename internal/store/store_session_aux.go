package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

func (s *Store) AppendSkillInvocation(ctx context.Context, inv SkillInvocation) (SkillInvocation, error) {
	if len(inv.Args) == 0 {
		inv.Args = json.RawMessage(`{}`)
	}
	if len(inv.Output) == 0 {
		inv.Output = json.RawMessage(`{}`)
	}
	var out SkillInvocation
	err := s.pool.QueryRow(ctx, `
INSERT INTO skill_invocation (session_id, tenant_id, skill, idempotency_key, status, args, output, error)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING id, session_id, tenant_id, skill, idempotency_key, status, args, output, error, created_at`,
		inv.SessionID, inv.TenantID, inv.Skill, inv.IdempotencyKey, inv.Status,
		[]byte(inv.Args), []byte(inv.Output), inv.Error).Scan(
		&out.ID, &out.SessionID, &out.TenantID, &out.Skill, &out.IdempotencyKey,
		&out.Status, &out.Args, &out.Output, &out.Error, &out.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "skill_invocation_idem_idx") {
			return s.getSkillInvocationByKey(ctx, inv.IdempotencyKey)
		}
		return SkillInvocation{}, err
	}
	return out, nil
}

func (s *Store) getSkillInvocationByKey(ctx context.Context, key string) (SkillInvocation, error) {
	var out SkillInvocation
	err := s.pool.QueryRow(ctx, `
SELECT id, session_id, tenant_id, skill, idempotency_key, status, args, output, error, created_at
FROM skill_invocation WHERE idempotency_key=$1`, key).Scan(
		&out.ID, &out.SessionID, &out.TenantID, &out.Skill, &out.IdempotencyKey,
		&out.Status, &out.Args, &out.Output, &out.Error, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SkillInvocation{}, ErrNotFound
	}
	return out, err
}

func (s *Store) ListSkillInvocations(ctx context.Context, sessionID string) ([]SkillInvocation, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, session_id, tenant_id, skill, idempotency_key, status, args, output, error, created_at
FROM skill_invocation WHERE session_id=$1 ORDER BY id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SkillInvocation
	for rows.Next() {
		var i SkillInvocation
		if err := rows.Scan(&i.ID, &i.SessionID, &i.TenantID, &i.Skill, &i.IdempotencyKey,
			&i.Status, &i.Args, &i.Output, &i.Error, &i.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *Store) AppendPIIAccess(ctx context.Context, ev PIIAccess) (PIIAccess, error) {
	var out PIIAccess
	err := s.pool.QueryRow(ctx, `
INSERT INTO pii_access_audit (tenant_id, session_id, actor, keys, reason)
VALUES ($1,$2,$3,$4,$5)
RETURNING id, tenant_id, session_id, actor, keys, reason, created_at`,
		ev.TenantID, ev.SessionID, ev.Actor, ev.Keys, ev.Reason).Scan(
		&out.ID, &out.TenantID, &out.SessionID, &out.Actor, &out.Keys, &out.Reason, &out.CreatedAt)
	return out, err
}

func (s *Store) ListPIIAccess(ctx context.Context, sessionID string, limit int) ([]PIIAccess, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, tenant_id, session_id, actor, keys, reason, created_at FROM pii_access_audit
WHERE ($1 = '' OR session_id=$1) ORDER BY id DESC LIMIT $2`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PIIAccess
	for rows.Next() {
		var p PIIAccess
		if err := rows.Scan(&p.ID, &p.TenantID, &p.SessionID, &p.Actor, &p.Keys, &p.Reason, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreateErasureRequest(ctx context.Context, r ErasureRequest) (ErasureRequest, error) {
	var out ErasureRequest
	err := s.pool.QueryRow(ctx, `
INSERT INTO erasure_request (id, tenant_id, subject_ref, scope, status, requested_by)
VALUES ($1,$2,$3,$4,$5,$6)
RETURNING id, tenant_id, subject_ref, scope, status, requested_by, requested_at, completed_at`,
		r.ID, r.TenantID, r.SubjectRef, r.Scope, r.Status, r.RequestedBy).Scan(
		&out.ID, &out.TenantID, &out.SubjectRef, &out.Scope, &out.Status,
		&out.RequestedBy, &out.RequestedAt, &out.CompletedAt)
	return out, err
}

func (s *Store) ListErasureRequests(ctx context.Context, tenantID string) ([]ErasureRequest, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, tenant_id, subject_ref, scope, status, requested_by, requested_at, completed_at
FROM erasure_request WHERE ($1='' OR tenant_id=$1) ORDER BY requested_at DESC LIMIT 200`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ErasureRequest
	for rows.Next() {
		var r ErasureRequest
		if err := rows.Scan(&r.ID, &r.TenantID, &r.SubjectRef, &r.Scope, &r.Status,
			&r.RequestedBy, &r.RequestedAt, &r.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CompleteErasureRequest(ctx context.Context, id string) (ErasureRequest, error) {
	var out ErasureRequest
	err := s.pool.QueryRow(ctx, `
UPDATE erasure_request SET status='completed', completed_at=now() WHERE id=$1
RETURNING id, tenant_id, subject_ref, scope, status, requested_by, requested_at, completed_at`, id).Scan(
		&out.ID, &out.TenantID, &out.SubjectRef, &out.Scope, &out.Status,
		&out.RequestedBy, &out.RequestedAt, &out.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErasureRequest{}, ErrNotFound
	}
	return out, err
}

func (s *Store) UpsertConsent(ctx context.Context, c ConsentRecord) (ConsentRecord, error) {
	var out ConsentRecord
	err := s.pool.QueryRow(ctx, `
INSERT INTO consent_record (tenant_id, phone, state, source, updated_at)
VALUES ($1,$2,$3,$4, now())
ON CONFLICT (tenant_id, phone) DO UPDATE SET state=EXCLUDED.state, source=EXCLUDED.source, updated_at=now()
RETURNING tenant_id, phone, state, source, updated_at`,
		c.TenantID, c.Phone, c.State, c.Source).Scan(&out.TenantID, &out.Phone, &out.State, &out.Source, &out.UpdatedAt)
	return out, err
}

func (s *Store) GetConsent(ctx context.Context, tenantID, phone string) (ConsentRecord, error) {
	var out ConsentRecord
	err := s.pool.QueryRow(ctx, `
SELECT tenant_id, phone, state, source, updated_at FROM consent_record WHERE tenant_id=$1 AND phone=$2`,
		tenantID, phone).Scan(&out.TenantID, &out.Phone, &out.State, &out.Source, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ConsentRecord{}, ErrNotFound
	}
	return out, err
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
		`DELETE FROM skill_invocation WHERE session_id=$1`,
	}
	for _, q := range statements {
		if _, err := s.pool.Exec(ctx, q, sessionID); err != nil {
			return err
		}
	}
	return nil
}

var _ = time.Now

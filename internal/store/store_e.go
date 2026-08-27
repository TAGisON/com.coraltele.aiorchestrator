package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) AppendAuditEvent(ctx context.Context, ev AuditEvent) (AuditEvent, error) {
	var out AuditEvent
	var sid, tid *string
	var payload []byte
	err := s.pool.QueryRow(ctx, `
INSERT INTO audit_event (session_id, tenant_id, event_type, payload)
VALUES (NULLIF($1,''), NULLIF($2,''), $3, $4)
RETURNING id, session_id, tenant_id, event_type, payload, created_at
`, ev.SessionID, ev.TenantID, ev.EventType, nullJSON(ev.Payload)).Scan(
		&out.ID, &sid, &tid, &out.EventType, &payload, &out.CreatedAt,
	)
	if err != nil {
		return AuditEvent{}, err
	}
	if sid != nil {
		out.SessionID = *sid
	}
	if tid != nil {
		out.TenantID = *tid
	}
	out.Payload = payload
	return out, nil
}

func (s *Store) ListAuditEvents(ctx context.Context, sessionID string) ([]AuditEvent, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, COALESCE(session_id,''), COALESCE(tenant_id,''), event_type, payload, created_at
FROM audit_event WHERE session_id=$1 ORDER BY id
`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEvent
	for rows.Next() {
		var ev AuditEvent
		var payload []byte
		if err := rows.Scan(&ev.ID, &ev.SessionID, &ev.TenantID, &ev.EventType, &payload, &ev.CreatedAt); err != nil {
			return nil, err
		}
		ev.Payload = payload
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *Store) AppendAnalyticsEvent(ctx context.Context, ev AnalyticsEvent) (AnalyticsEvent, error) {
	if ev.Value == 0 {
		ev.Value = 1
	}
	var out AnalyticsEvent
	err := s.pool.QueryRow(ctx, `
INSERT INTO analytics_event (tenant_id, profile_id, session_id, metric, value, dimensions)
VALUES (NULLIF($1,''), NULLIF($2,''), NULLIF($3,''), $4, $5, $6)
RETURNING id, COALESCE(tenant_id,''), COALESCE(profile_id,''), COALESCE(session_id,''), metric, value, dimensions, created_at
`, ev.TenantID, ev.ProfileID, ev.SessionID, ev.Metric, ev.Value, nullJSON(ev.Dimensions)).Scan(
		&out.ID, &out.TenantID, &out.ProfileID, &out.SessionID, &out.Metric, &out.Value, &out.Dimensions, &out.CreatedAt,
	)
	if err != nil {
		return AnalyticsEvent{}, err
	}
	return out, nil
}

func (s *Store) ListAnalyticsEvents(ctx context.Context, sessionID string) ([]AnalyticsEvent, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, COALESCE(tenant_id,''), COALESCE(profile_id,''), COALESCE(session_id,''), metric, value, dimensions, created_at
FROM analytics_event WHERE session_id=$1 ORDER BY id
`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AnalyticsEvent
	for rows.Next() {
		var ev AnalyticsEvent
		var dims []byte
		if err := rows.Scan(&ev.ID, &ev.TenantID, &ev.ProfileID, &ev.SessionID, &ev.Metric, &ev.Value, &dims, &ev.CreatedAt); err != nil {
			return nil, err
		}
		ev.Dimensions = dims
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *Store) CreatePostcallJob(ctx context.Context, job PostcallJob) error {
	var open bool
	err := s.pool.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM postcall_job WHERE session_id=$1 AND state IN ($2, $3)
)`, job.SessionID, JobQueued, JobRunning).Scan(&open)
	if err != nil {
		return err
	}
	if open {
		return ErrConflict
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO postcall_job (id, session_id, profile_id, profile_version, state, lease_owner, error_message)
VALUES ($1, $2, $3, $4, $5, NULLIF($6,''), NULLIF($7,''))
`, job.ID, job.SessionID, job.ProfileID, job.ProfileVersion, job.State, job.LeaseOwner, job.ErrorMessage)
	return err
}

func (s *Store) GetPostcallJob(ctx context.Context, id string) (PostcallJob, error) {
	var j PostcallJob
	var owner, errMsg *string
	err := s.pool.QueryRow(ctx, `
SELECT id, session_id, profile_id, profile_version, state, lease_owner, error_message, created_at, updated_at
FROM postcall_job WHERE id=$1
`, id).Scan(&j.ID, &j.SessionID, &j.ProfileID, &j.ProfileVersion, &j.State, &owner, &errMsg, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PostcallJob{}, ErrNotFound
	}
	if err != nil {
		return PostcallJob{}, err
	}
	if owner != nil {
		j.LeaseOwner = *owner
	}
	if errMsg != nil {
		j.ErrorMessage = *errMsg
	}
	return j, nil
}

func (s *Store) UpdatePostcallJob(ctx context.Context, job PostcallJob) error {
	_, err := s.pool.Exec(ctx, `
UPDATE postcall_job SET state=$2, lease_owner=NULLIF($3,''), error_message=NULLIF($4,''), updated_at=now()
WHERE id=$1
`, job.ID, job.State, job.LeaseOwner, job.ErrorMessage)
	return err
}

func (s *Store) LeaseNextPostcallJob(ctx context.Context, owner string) (PostcallJob, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PostcallJob{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var j PostcallJob
	var leaseOwner, errMsg *string
	err = tx.QueryRow(ctx, `
SELECT id, session_id, profile_id, profile_version, state, lease_owner, error_message, created_at, updated_at
FROM postcall_job
WHERE state=$1
ORDER BY created_at
FOR UPDATE SKIP LOCKED
LIMIT 1
`, JobQueued).Scan(&j.ID, &j.SessionID, &j.ProfileID, &j.ProfileVersion, &j.State, &leaseOwner, &errMsg, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PostcallJob{}, ErrNotFound
	}
	if err != nil {
		return PostcallJob{}, err
	}
	j.State = JobRunning
	j.LeaseOwner = owner
	j.UpdatedAt = time.Now().UTC()
	if _, err := tx.Exec(ctx, `
UPDATE postcall_job SET state=$2, lease_owner=$3, leased_until=now() + interval '10 minutes', updated_at=now()
WHERE id=$1
`, j.ID, JobRunning, owner); err != nil {
		return PostcallJob{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PostcallJob{}, err
	}
	return j, nil
}

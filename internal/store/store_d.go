package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreatePlaybackJob(ctx context.Context, job PlaybackJob) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO playback_job (id, tenant_id, file_uri, profile_id, profile_version, state, lease_owner)
VALUES ($1, NULLIF($2,''), $3, $4, $5, $6, NULLIF($7,''))
`, job.ID, job.TenantID, job.FileURI, job.ProfileID, job.ProfileVersion, job.State, job.LeaseOwner)
	return err
}

func (s *Store) GetPlaybackJob(ctx context.Context, id string) (PlaybackJob, error) {
	var j PlaybackJob
	var tenant, owner *string
	err := s.pool.QueryRow(ctx, `
SELECT id, tenant_id, file_uri, profile_id, profile_version, state, lease_owner, created_at, updated_at
FROM playback_job WHERE id=$1
`, id).Scan(&j.ID, &tenant, &j.FileURI, &j.ProfileID, &j.ProfileVersion, &j.State, &owner, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlaybackJob{}, ErrNotFound
	}
	if err != nil {
		return PlaybackJob{}, err
	}
	if tenant != nil {
		j.TenantID = *tenant
	}
	if owner != nil {
		j.LeaseOwner = *owner
	}
	return j, nil
}

func (s *Store) UpdatePlaybackJob(ctx context.Context, job PlaybackJob) error {
	_, err := s.pool.Exec(ctx, `
UPDATE playback_job SET state=$2, lease_owner=NULLIF($3,''), updated_at=now() WHERE id=$1
`, job.ID, job.State, job.LeaseOwner)
	return err
}

func (s *Store) LeaseNextPlaybackJob(ctx context.Context, owner string) (PlaybackJob, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PlaybackJob{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var j PlaybackJob
	var tenant, leaseOwner *string
	err = tx.QueryRow(ctx, `
SELECT id, tenant_id, file_uri, profile_id, profile_version, state, lease_owner, created_at, updated_at
FROM playback_job
WHERE state=$1
ORDER BY created_at
FOR UPDATE SKIP LOCKED
LIMIT 1
`, JobQueued).Scan(&j.ID, &tenant, &j.FileURI, &j.ProfileID, &j.ProfileVersion, &j.State, &leaseOwner, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlaybackJob{}, ErrNotFound
	}
	if err != nil {
		return PlaybackJob{}, err
	}
	j.State = JobRunning
	j.LeaseOwner = owner
	j.UpdatedAt = time.Now().UTC()
	if _, err := tx.Exec(ctx, `
UPDATE playback_job SET state=$2, lease_owner=$3, leased_until=now() + interval '10 minutes', updated_at=now()
WHERE id=$1
`, j.ID, JobRunning, owner); err != nil {
		return PlaybackJob{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PlaybackJob{}, err
	}
	if tenant != nil {
		j.TenantID = *tenant
	}
	return j, nil
}

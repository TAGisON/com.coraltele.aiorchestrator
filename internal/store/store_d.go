package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateKBDocument(ctx context.Context, doc KBDocument) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO kb_document (id, tenant_id, collection, uri, status, error_message)
VALUES ($1, NULLIF($2,''), $3, NULLIF($4,''), $5, NULLIF($6,''))
`, doc.ID, doc.TenantID, doc.Collection, doc.URI, doc.Status, doc.ErrorMessage)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return err
	}
	return nil
}

func (s *Store) GetKBDocument(ctx context.Context, id string) (KBDocument, error) {
	var d KBDocument
	var tenant, uri, errMsg *string
	err := s.pool.QueryRow(ctx, `
SELECT id, tenant_id, collection, uri, status, error_message, created_at, updated_at
FROM kb_document WHERE id=$1
`, id).Scan(&d.ID, &tenant, &d.Collection, &uri, &d.Status, &errMsg, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBDocument{}, ErrNotFound
	}
	if err != nil {
		return KBDocument{}, err
	}
	if tenant != nil {
		d.TenantID = *tenant
	}
	if uri != nil {
		d.URI = *uri
	}
	if errMsg != nil {
		d.ErrorMessage = *errMsg
	}
	return d, nil
}

func (s *Store) UpdateKBDocumentStatus(ctx context.Context, id, status, errMsg string) (KBDocument, error) {
	_, err := s.pool.Exec(ctx, `
UPDATE kb_document SET status=$2, error_message=NULLIF($3,''), updated_at=now() WHERE id=$1
`, id, status, errMsg)
	if err != nil {
		return KBDocument{}, err
	}
	return s.GetKBDocument(ctx, id)
}

func (s *Store) ReplaceKBChunks(ctx context.Context, documentID string, chunks []KBChunk) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM kb_chunk WHERE document_id=$1`, documentID); err != nil {
		return err
	}
	for _, ch := range chunks {
		if _, err := tx.Exec(ctx, `
INSERT INTO kb_chunk (document_id, tenant_id, collection, ordinal, text, source_uri)
VALUES ($1, NULLIF($2,''), $3, $4, $5, NULLIF($6,''))
`, documentID, ch.TenantID, ch.Collection, ch.Ordinal, ch.Text, ch.SourceURI); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ListKBChunks(ctx context.Context, tenantID string, collections []string) ([]KBChunk, error) {
	q := `
SELECT id, document_id, tenant_id, collection, ordinal, text, source_uri, created_at
FROM kb_chunk WHERE 1=1`
	args := []any{}
	n := 1
	if tenantID != "" {
		q += fmt.Sprintf(` AND (tenant_id IS NULL OR tenant_id = $%d)`, n)
		args = append(args, tenantID)
		n++
	}
	if len(collections) > 0 {
		q += fmt.Sprintf(` AND collection = ANY($%d)`, n)
		args = append(args, collections)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []KBChunk
	for rows.Next() {
		var ch KBChunk
		var tenant, src *string
		if err := rows.Scan(&ch.ID, &ch.DocumentID, &tenant, &ch.Collection, &ch.Ordinal, &ch.Text, &src, &ch.CreatedAt); err != nil {
			return nil, err
		}
		if tenant != nil {
			ch.TenantID = *tenant
		}
		if src != nil {
			ch.SourceURI = *src
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

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

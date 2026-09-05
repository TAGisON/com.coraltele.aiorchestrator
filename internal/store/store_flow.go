package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func nullBlank(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullPosInt(n int) any {
	if n <= 0 {
		return nil
	}
	return n
}

func applyFlowPin(sess *Session, flowID *string, flowVer *int) {
	if flowID != nil {
		sess.FlowID = *flowID
	}
	if flowVer != nil {
		sess.FlowVersion = *flowVer
	}
}

func finishSessionPointers(sess *Session, tenant, owner, coral, rec, flowID *string, flowVer *int, caller, meta, binding []byte) Session {
	if tenant != nil {
		sess.TenantID = *tenant
	}
	if owner != nil {
		sess.OwnerInstance = *owner
	}
	if coral != nil {
		sess.CoralUserID = *coral
	}
	if rec != nil {
		sess.RecordingRef = *rec
	}
	sess.Caller = caller
	sess.Metadata = meta
	sess.GatewayBinding = scanGatewayBinding(binding)
	applyFlowPin(sess, flowID, flowVer)
	return *sess
}

// CreateFlow inserts a registry row and an initial draft document.
func (s *Store) CreateFlow(ctx context.Context, f Flow, draftDoc json.RawMessage) (Flow, error) {
	if draftDoc == nil {
		draftDoc = json.RawMessage(`{}`)
	}
	if f.Direction == "" {
		f.Direction = FlowDirectionInbound
	}
	if f.Status == "" {
		f.Status = FlowStatusDraft
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Flow{}, err
	}
	defer tx.Rollback(ctx)

	var out Flow
	err = tx.QueryRow(ctx, `
INSERT INTO flow (id, tenant_id, name, direction, status, current_version)
VALUES ($1, $2, $3, $4, $5, 0)
RETURNING id, tenant_id, name, direction, status, current_version, created_at, updated_at
`, f.ID, f.TenantID, f.Name, f.Direction, f.Status).Scan(
		&out.ID, &out.TenantID, &out.Name, &out.Direction, &out.Status, &out.CurrentVersion, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return Flow{}, ErrConflict
		}
		return Flow{}, err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO flow_draft (flow_id, doc) VALUES ($1, $2)
`, out.ID, draftDoc)
	if err != nil {
		return Flow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Flow{}, err
	}
	return out, nil
}

func (s *Store) GetFlow(ctx context.Context, id string) (Flow, error) {
	var f Flow
	err := s.pool.QueryRow(ctx, `
SELECT id, tenant_id, name, direction, status, current_version, created_at, updated_at
FROM flow WHERE id=$1
`, id).Scan(&f.ID, &f.TenantID, &f.Name, &f.Direction, &f.Status, &f.CurrentVersion, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Flow{}, ErrNotFound
	}
	return f, err
}

func (s *Store) ListFlows(ctx context.Context, tenantID string, limit int) ([]Flow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, tenant_id, name, direction, status, current_version, created_at, updated_at
FROM flow WHERE tenant_id=$1
ORDER BY updated_at DESC
LIMIT $2
`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Flow
	for rows.Next() {
		var f Flow
		if err := rows.Scan(&f.ID, &f.TenantID, &f.Name, &f.Direction, &f.Status, &f.CurrentVersion, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) UpsertFlowDraft(ctx context.Context, flowID string, doc json.RawMessage) (FlowDraft, error) {
	if doc == nil {
		doc = json.RawMessage(`{}`)
	}
	var d FlowDraft
	err := s.pool.QueryRow(ctx, `
INSERT INTO flow_draft (flow_id, doc, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (flow_id) DO UPDATE SET doc = EXCLUDED.doc, updated_at = now()
RETURNING flow_id, doc, updated_at
`, flowID, doc).Scan(&d.FlowID, &d.Doc, &d.UpdatedAt)
	return d, err
}

func (s *Store) GetFlowDraft(ctx context.Context, flowID string) (FlowDraft, error) {
	var d FlowDraft
	err := s.pool.QueryRow(ctx, `
SELECT flow_id, doc, updated_at FROM flow_draft WHERE flow_id=$1
`, flowID).Scan(&d.FlowID, &d.Doc, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return FlowDraft{}, ErrNotFound
	}
	return d, err
}

// PublishFlowVersion inserts an immutable version and bumps flow.current_version.
// Envelope validation is G.2 — this only persists.
func (s *Store) PublishFlowVersion(ctx context.Context, flowID string, doc json.RawMessage, contentHash, publishedBy string) (FlowVersion, error) {
	if doc == nil {
		doc = json.RawMessage(`{}`)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return FlowVersion{}, err
	}
	defer tx.Rollback(ctx)

	var cur int
	err = tx.QueryRow(ctx, `SELECT current_version FROM flow WHERE id=$1 FOR UPDATE`, flowID).Scan(&cur)
	if errors.Is(err, pgx.ErrNoRows) {
		return FlowVersion{}, ErrNotFound
	}
	if err != nil {
		return FlowVersion{}, err
	}
	next := cur + 1
	var fv FlowVersion
	err = tx.QueryRow(ctx, `
INSERT INTO flow_version (flow_id, version, doc, content_hash, published_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING flow_id, version, doc, content_hash, published_by, published_at
`, flowID, next, doc, contentHash, publishedBy).Scan(
		&fv.FlowID, &fv.Version, &fv.Doc, &fv.ContentHash, &fv.PublishedBy, &fv.PublishedAt,
	)
	if err != nil {
		return FlowVersion{}, err
	}
	_, err = tx.Exec(ctx, `
UPDATE flow SET current_version=$2, status=$3, updated_at=now() WHERE id=$1
`, flowID, next, FlowStatusPublished)
	if err != nil {
		return FlowVersion{}, err
	}
	_, _ = tx.Exec(ctx, `
INSERT INTO flow_draft (flow_id, doc, updated_at) VALUES ($1, $2, now())
ON CONFLICT (flow_id) DO UPDATE SET doc = EXCLUDED.doc, updated_at = now()
`, flowID, doc)
	if err := tx.Commit(ctx); err != nil {
		return FlowVersion{}, err
	}
	return fv, nil
}

func (s *Store) GetFlowVersion(ctx context.Context, flowID string, version int) (FlowVersion, error) {
	var fv FlowVersion
	err := s.pool.QueryRow(ctx, `
SELECT flow_id, version, doc, content_hash, published_by, published_at
FROM flow_version WHERE flow_id=$1 AND version=$2
`, flowID, version).Scan(&fv.FlowID, &fv.Version, &fv.Doc, &fv.ContentHash, &fv.PublishedBy, &fv.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return FlowVersion{}, ErrNotFound
	}
	return fv, err
}

func (s *Store) GetLatestFlowVersion(ctx context.Context, flowID string) (FlowVersion, error) {
	var fv FlowVersion
	err := s.pool.QueryRow(ctx, `
SELECT flow_id, version, doc, content_hash, published_by, published_at
FROM flow_version WHERE flow_id=$1
ORDER BY version DESC LIMIT 1
`, flowID).Scan(&fv.FlowID, &fv.Version, &fv.Doc, &fv.ContentHash, &fv.PublishedBy, &fv.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return FlowVersion{}, ErrNotFound
	}
	return fv, err
}

func (s *Store) UpsertBinding(ctx context.Context, b Binding) (Binding, error) {
	if b.Config == nil {
		b.Config = json.RawMessage(`{}`)
	}
	if b.Status == "" {
		b.Status = BindingStatusActive
	}
	var out Binding
	err := s.pool.QueryRow(ctx, `
INSERT INTO binding (id, tenant_id, kind, name, config, status)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  kind = EXCLUDED.kind,
  name = EXCLUDED.name,
  config = EXCLUDED.config,
  status = EXCLUDED.status,
  updated_at = now()
RETURNING id, tenant_id, kind, name, config, status, created_at, updated_at
`, b.ID, b.TenantID, b.Kind, b.Name, b.Config, b.Status).Scan(
		&out.ID, &out.TenantID, &out.Kind, &out.Name, &out.Config, &out.Status, &out.CreatedAt, &out.UpdatedAt,
	)
	return out, err
}

func (s *Store) GetBinding(ctx context.Context, id string) (Binding, error) {
	var b Binding
	err := s.pool.QueryRow(ctx, `
SELECT id, tenant_id, kind, name, config, status, created_at, updated_at
FROM binding WHERE id=$1
`, id).Scan(&b.ID, &b.TenantID, &b.Kind, &b.Name, &b.Config, &b.Status, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Binding{}, ErrNotFound
	}
	return b, err
}

func (s *Store) ListBindings(ctx context.Context, tenantID, kind string, limit int) ([]Binding, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows pgx.Rows
	var err error
	if kind == "" {
		rows, err = s.pool.Query(ctx, `
SELECT id, tenant_id, kind, name, config, status, created_at, updated_at
FROM binding WHERE tenant_id=$1
ORDER BY name
LIMIT $2
`, tenantID, limit)
	} else {
		rows, err = s.pool.Query(ctx, `
SELECT id, tenant_id, kind, name, config, status, created_at, updated_at
FROM binding WHERE tenant_id=$1 AND kind=$2
ORDER BY name
LIMIT $3
`, tenantID, kind, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Binding
	for rows.Next() {
		var b Binding
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Kind, &b.Name, &b.Config, &b.Status, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

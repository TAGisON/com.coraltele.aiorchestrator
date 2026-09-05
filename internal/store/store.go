package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// Profile is a draft profile row.
type Profile struct {
	ID          string
	TenantID    string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProfileVersion is an immutable published document.
type ProfileVersion struct {
	ProfileID   string
	Version     int
	Document    json.RawMessage
	PublishedAt time.Time
}

// Session states (CONTROL_API / SOLUTION lifecycle).
const (
	StateCreated   = "Created"
	StateAttached  = "Attached"
	StateRunning   = "Running"
	StateDraining  = "Draining"
	StateCompleted = "Completed"
	StateCancelled = "Cancelled"
	StateFailed    = "Failed"
)

// Session is a durable session row.
type Session struct {
	ID                    string
	TenantID              string
	ProfileID             string
	ProfileVersion        int
	FlowID                string // published flow pin (nullable in DB)
	FlowVersion           int    // published version pin; 0 = unset
	Clock                 string
	State                 string
	OwnerInstance         string
	CanonicalSampleRateHz int
	CoralUserID           string
	Caller                json.RawMessage
	RecordingRef          string
	RecordingStartedAt    *time.Time
	RecordingStoppedAt    *time.Time
	RecordingStopReason   string
	RecordingBytes        *int64
	Metadata              json.RawMessage
	GatewayBinding        *GatewayBinding
	DetectedLanguage      string
	ActiveLanguage        string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Store is the durable PG-backed repository.
type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) CreateProfile(ctx context.Context, p Profile) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO profile (id, tenant_id, display_name)
VALUES ($1, NULLIF($2,''), NULLIF($3,''))
`, p.ID, p.TenantID, p.DisplayName)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return err
	}
	return nil
}

func (s *Store) GetProfile(ctx context.Context, id string) (Profile, error) {
	var p Profile
	var tenant, display *string
	err := s.pool.QueryRow(ctx, `
SELECT id, tenant_id, display_name, created_at, updated_at FROM profile WHERE id=$1
`, id).Scan(&p.ID, &tenant, &display, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, err
	}
	if tenant != nil {
		p.TenantID = *tenant
	}
	if display != nil {
		p.DisplayName = *display
	}
	return p, nil
}

// PublishVersion inserts the next immutable version (or version 1) for profileID.
func (s *Store) PublishVersion(ctx context.Context, profileID string, doc json.RawMessage) (ProfileVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProfileVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM profile WHERE id=$1)`, profileID).Scan(&exists); err != nil {
		return ProfileVersion{}, err
	}
	if !exists {
		return ProfileVersion{}, ErrNotFound
	}

	var next int
	err = tx.QueryRow(ctx, `
SELECT COALESCE(MAX(version), 0) + 1 FROM profile_version WHERE profile_id=$1
`, profileID).Scan(&next)
	if err != nil {
		return ProfileVersion{}, err
	}

	var pv ProfileVersion
	err = tx.QueryRow(ctx, `
INSERT INTO profile_version (profile_id, version, document)
VALUES ($1, $2, $3::jsonb)
RETURNING profile_id, version, document, published_at
`, profileID, next, doc).Scan(&pv.ProfileID, &pv.Version, &pv.Document, &pv.PublishedAt)
	if err != nil {
		return ProfileVersion{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE profile SET updated_at=now() WHERE id=$1`, profileID); err != nil {
		return ProfileVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ProfileVersion{}, err
	}
	return pv, nil
}

func (s *Store) GetLatestVersion(ctx context.Context, profileID string) (ProfileVersion, error) {
	var pv ProfileVersion
	err := s.pool.QueryRow(ctx, `
SELECT profile_id, version, document, published_at
FROM profile_version WHERE profile_id=$1
ORDER BY version DESC LIMIT 1
`, profileID).Scan(&pv.ProfileID, &pv.Version, &pv.Document, &pv.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProfileVersion{}, ErrNotFound
	}
	if err != nil {
		return ProfileVersion{}, err
	}
	return pv, nil
}

func (s *Store) GetVersion(ctx context.Context, profileID string, version int) (ProfileVersion, error) {
	var pv ProfileVersion
	err := s.pool.QueryRow(ctx, `
SELECT profile_id, version, document, published_at
FROM profile_version WHERE profile_id=$1 AND version=$2
`, profileID, version).Scan(&pv.ProfileID, &pv.Version, &pv.Document, &pv.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProfileVersion{}, ErrNotFound
	}
	if err != nil {
		return ProfileVersion{}, err
	}
	return pv, nil
}

func (s *Store) CreateSession(ctx context.Context, sess Session) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO session (
  id, tenant_id, profile_id, profile_version, clock, state, owner_instance,
  canonical_sample_rate_hz, coral_user_id, caller, recording_ref, metadata, gateway_binding,
  detected_language, active_language, flow_id, flow_version
) VALUES (
  $1, NULLIF($2,''), $3, $4, $5, $6, NULLIF($7,''),
  $8, NULLIF($9,''), $10, NULLIF($11,''), $12, $13, $14, $15, $16, $17
)`,
		sess.ID, sess.TenantID, sess.ProfileID, sess.ProfileVersion, sess.Clock, sess.State, sess.OwnerInstance,
		sess.CanonicalSampleRateHz, sess.CoralUserID, nullJSON(sess.Caller), sess.RecordingRef, nullJSON(sess.Metadata),
		marshalGatewayBinding(sess.GatewayBinding),
		sess.DetectedLanguage, sess.ActiveLanguage,
		nullBlank(sess.FlowID), nullPosInt(sess.FlowVersion),
	)
	return err
}

func (s *Store) GetSession(ctx context.Context, id string) (Session, error) {
	var sess Session
	var tenant, owner, coral, rec, flowID *string
	var caller, meta, binding []byte
	var flowVer *int
	err := s.pool.QueryRow(ctx, `
SELECT id, tenant_id, profile_id, profile_version, clock, state, owner_instance,
       canonical_sample_rate_hz, coral_user_id, caller, recording_ref, metadata, gateway_binding,
       COALESCE(detected_language,''), COALESCE(active_language,''),
       flow_id, flow_version,
       created_at, updated_at
FROM session WHERE id=$1
`, id).Scan(
		&sess.ID, &tenant, &sess.ProfileID, &sess.ProfileVersion, &sess.Clock, &sess.State, &owner,
		&sess.CanonicalSampleRateHz, &coral, &caller, &rec, &meta, &binding,
		&sess.DetectedLanguage, &sess.ActiveLanguage,
		&flowID, &flowVer,
		&sess.CreatedAt, &sess.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	return finishSessionPointers(&sess, tenant, owner, coral, rec, flowID, flowVer, caller, meta, binding), nil
}

func (s *Store) UpdateSessionState(ctx context.Context, id, state string) (Session, error) {
	var sess Session
	var tenant, owner, coral, rec, flowID *string
	var caller, meta, binding []byte
	var flowVer *int
	err := s.pool.QueryRow(ctx, `
UPDATE session SET state=$2, updated_at=now() WHERE id=$1
RETURNING id, tenant_id, profile_id, profile_version, clock, state, owner_instance,
          canonical_sample_rate_hz, coral_user_id, caller, recording_ref, metadata, gateway_binding,
          COALESCE(detected_language,''), COALESCE(active_language,''),
          flow_id, flow_version,
          created_at, updated_at
`, id, state).Scan(
		&sess.ID, &tenant, &sess.ProfileID, &sess.ProfileVersion, &sess.Clock, &sess.State, &owner,
		&sess.CanonicalSampleRateHz, &coral, &caller, &rec, &meta, &binding,
		&sess.DetectedLanguage, &sess.ActiveLanguage,
		&flowID, &flowVer,
		&sess.CreatedAt, &sess.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	return finishSessionPointers(&sess, tenant, owner, coral, rec, flowID, flowVer, caller, meta, binding), nil
}

// UpdateSessionRecordingRef stores the on-disk path of the call recording.
func (s *Store) UpdateSessionRecordingRef(ctx context.Context, id, ref string) (Session, error) {
	var sess Session
	var tenant, owner, coral, rec, flowID *string
	var caller, meta, binding []byte
	var flowVer *int
	err := s.pool.QueryRow(ctx, `
UPDATE session SET recording_ref=NULLIF($2,''), updated_at=now() WHERE id=$1
RETURNING id, tenant_id, profile_id, profile_version, clock, state, owner_instance,
          canonical_sample_rate_hz, coral_user_id, caller, recording_ref, metadata, gateway_binding,
          COALESCE(detected_language,''), COALESCE(active_language,''),
          flow_id, flow_version,
          created_at, updated_at
`, id, ref).Scan(
		&sess.ID, &tenant, &sess.ProfileID, &sess.ProfileVersion, &sess.Clock, &sess.State, &owner,
		&sess.CanonicalSampleRateHz, &coral, &caller, &rec, &meta, &binding,
		&sess.DetectedLanguage, &sess.ActiveLanguage,
		&flowID, &flowVer,
		&sess.CreatedAt, &sess.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	return finishSessionPointers(&sess, tenant, owner, coral, rec, flowID, flowVer, caller, meta, binding), nil
}

func (s *Store) UpdateSessionLanguages(ctx context.Context, id, detected, active string) (Session, error) {
	var sess Session
	var tenant, owner, coral, rec, flowID *string
	var caller, meta, binding []byte
	var flowVer *int
	err := s.pool.QueryRow(ctx, `
UPDATE session SET detected_language=$2, active_language=$3, updated_at=now() WHERE id=$1
RETURNING id, tenant_id, profile_id, profile_version, clock, state, owner_instance,
          canonical_sample_rate_hz, coral_user_id, caller, recording_ref, metadata, gateway_binding,
          COALESCE(detected_language,''), COALESCE(active_language,''),
          flow_id, flow_version,
          created_at, updated_at
`, id, detected, active).Scan(
		&sess.ID, &tenant, &sess.ProfileID, &sess.ProfileVersion, &sess.Clock, &sess.State, &owner,
		&sess.CanonicalSampleRateHz, &coral, &caller, &rec, &meta, &binding,
		&sess.DetectedLanguage, &sess.ActiveLanguage,
		&flowID, &flowVer,
		&sess.CreatedAt, &sess.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	return finishSessionPointers(&sess, tenant, owner, coral, rec, flowID, flowVer, caller, meta, binding), nil
}

func nullJSON(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func isUniqueViolation(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint"))
}

// Open connects to DATABASE_URL-style DSN and applies migrations.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := ApplyMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return New(pool), nil
}

func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

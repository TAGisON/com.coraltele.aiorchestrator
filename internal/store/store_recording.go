package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// P2.5 recording_stop_reason vocabulary.
const (
	RecordingStopNone             = ""
	RecordingStopSessionEnding    = "session_ending"
	RecordingStopSessionCompleted = "session_completed"
	RecordingStopSessionCancelled = "session_cancelled"
	RecordingStopSessionFailed    = "session_failed"
	RecordingStopOrphanReaper     = "orphan_reaper"
	RecordingStopManual           = "manual"
)

// MapRecordingStopReason maps runtime stop hints to the closed P2.5 vocabulary.
func MapRecordingStopReason(endReason string) string {
	switch strings.ToLower(strings.TrimSpace(endReason)) {
	case "", "talk_end", "ending", "draining", "hangup", "transfer":
		return RecordingStopSessionEnding
	case "completed", "complete", "session_completed":
		return RecordingStopSessionCompleted
	case "cancelled", "canceled", "session_cancelled":
		return RecordingStopSessionCancelled
	case "failed", "session_failed":
		return RecordingStopSessionFailed
	case "manual":
		return RecordingStopManual
	case "orphan_reaper":
		return RecordingStopOrphanReaper
	default:
		// Match store.State* strings case-sensitively via lower already applied for common;
		// also accept exact state constants if passed as-is before ToLower... ToLower handles Completed -> completed above.
		return RecordingStopSessionEnding
	}
}

// MarkSessionRecordingStarted sets recording_ref and recording_started_at (once).
func (s *Store) MarkSessionRecordingStarted(ctx context.Context, id, ref string) (Session, error) {
	var sess Session
	var tenant, owner, coral, rec *string
	var started, stopped *time.Time
	var nbytes *int64
	var caller, meta, binding []byte
	err := s.pool.QueryRow(ctx, `
UPDATE session SET
  recording_ref = NULLIF($2,''),
  recording_started_at = COALESCE(recording_started_at, now()),
  updated_at = now()
WHERE id=$1
RETURNING id, tenant_id, profile_id, profile_version, clock, state, owner_instance,
          canonical_sample_rate_hz, coral_user_id, caller, recording_ref, metadata, gateway_binding,
          COALESCE(detected_language,''), COALESCE(active_language,''),
          recording_started_at, recording_stopped_at, COALESCE(recording_stop_reason,''), recording_bytes,
          created_at, updated_at
`, id, ref).Scan(
		&sess.ID, &tenant, &sess.ProfileID, &sess.ProfileVersion, &sess.Clock, &sess.State, &owner,
		&sess.CanonicalSampleRateHz, &coral, &caller, &rec, &meta, &binding,
		&sess.DetectedLanguage, &sess.ActiveLanguage,
		&started, &stopped, &sess.RecordingStopReason, &nbytes,
		&sess.CreatedAt, &sess.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	fillSessionRecordingScan(&sess, tenant, owner, coral, rec, caller, meta, binding, started, stopped, nbytes)
	return sess, nil
}

// MarkSessionRecordingStopped stamps stop time/reason/bytes (idempotent if already stopped).
func (s *Store) MarkSessionRecordingStopped(ctx context.Context, id, reason string, nbytes *int64) (Session, error) {
	reason = MapRecordingStopReason(reason)
	var sess Session
	var tenant, owner, coral, rec *string
	var started, stopped *time.Time
	var outBytes *int64
	var caller, meta, binding []byte
	err := s.pool.QueryRow(ctx, `
UPDATE session SET
  recording_stopped_at = COALESCE(recording_stopped_at, now()),
  recording_stop_reason = CASE
    WHEN COALESCE(recording_stop_reason,'') = '' THEN $2
    ELSE recording_stop_reason
  END,
  recording_bytes = COALESCE(recording_bytes, $3),
  updated_at = now()
WHERE id=$1
RETURNING id, tenant_id, profile_id, profile_version, clock, state, owner_instance,
          canonical_sample_rate_hz, coral_user_id, caller, recording_ref, metadata, gateway_binding,
          COALESCE(detected_language,''), COALESCE(active_language,''),
          recording_started_at, recording_stopped_at, COALESCE(recording_stop_reason,''), recording_bytes,
          created_at, updated_at
`, id, reason, nbytes).Scan(
		&sess.ID, &tenant, &sess.ProfileID, &sess.ProfileVersion, &sess.Clock, &sess.State, &owner,
		&sess.CanonicalSampleRateHz, &coral, &caller, &rec, &meta, &binding,
		&sess.DetectedLanguage, &sess.ActiveLanguage,
		&started, &stopped, &sess.RecordingStopReason, &outBytes,
		&sess.CreatedAt, &sess.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	fillSessionRecordingScan(&sess, tenant, owner, coral, rec, caller, meta, binding, started, stopped, outBytes)
	return sess, nil
}

func fillSessionRecordingScan(sess *Session, tenant, owner, coral, rec *string, caller, meta, binding []byte, started, stopped *time.Time, nbytes *int64) {
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
	sess.RecordingStartedAt = started
	sess.RecordingStoppedAt = stopped
	sess.RecordingBytes = nbytes
}

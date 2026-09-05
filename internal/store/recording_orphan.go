package store

import (
	"context"
	"time"
)

// ListOrphanRecordingSessions returns terminal sessions with a started recording
// that was never stamped stopped (crash / missed stop path).
func (s *Store) ListOrphanRecordingSessions(ctx context.Context, limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, tenant_id, profile_id, profile_version, clock, state, owner_instance,
       canonical_sample_rate_hz, coral_user_id, caller, recording_ref, metadata, gateway_binding,
       COALESCE(detected_language,''), COALESCE(active_language,''),
       recording_started_at, recording_stopped_at, COALESCE(recording_stop_reason,''), recording_bytes,
       created_at, updated_at
FROM session
WHERE state IN ('Completed','Cancelled','Failed')
  AND recording_started_at IS NOT NULL
  AND recording_stopped_at IS NULL
ORDER BY recording_started_at ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var sess Session
		var tenant, owner, coral, rec *string
		var started, stopped *time.Time
		var nbytes *int64
		var caller, meta, binding []byte
		if err := rows.Scan(
			&sess.ID, &tenant, &sess.ProfileID, &sess.ProfileVersion, &sess.Clock, &sess.State, &owner,
			&sess.CanonicalSampleRateHz, &coral, &caller, &rec, &meta, &binding,
			&sess.DetectedLanguage, &sess.ActiveLanguage,
			&started, &stopped, &sess.RecordingStopReason, &nbytes,
			&sess.CreatedAt, &sess.UpdatedAt,
		); err != nil {
			return nil, err
		}
		fillSessionRecordingScan(&sess, tenant, owner, coral, rec, caller, meta, binding, started, stopped, nbytes)
		out = append(out, sess)
	}
	return out, rows.Err()
}

// IsTerminalSessionState reports Completed/Cancelled/Failed.
func IsTerminalSessionState(state string) bool {
	switch state {
	case StateCompleted, StateCancelled, StateFailed:
		return true
	default:
		return false
	}
}

package record

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// ReapOrphans stamps terminal sessions whose recording never received stopped_at.
// Returns the number of sessions reaped.
func ReapOrphans(ctx context.Context, repo store.Repository, limit int) (int, error) {
	if repo == nil {
		return 0, nil
	}
	orphans, err := repo.ListOrphanRecordingSessions(ctx, limit)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, sess := range orphans {
		var nbytes *int64
		ref := strings.TrimSpace(sess.RecordingRef)
		if ref != "" {
			if fi, err := os.Stat(ref); err == nil {
				v := fi.Size()
				nbytes = &v
			}
		}
		stopped, err := repo.MarkSessionRecordingStopped(ctx, sess.ID, store.RecordingStopOrphanReaper, nbytes)
		if err != nil {
			applog.Warn("orphan reaper stamp failed", "session", sess.ID, "err", err)
			continue
		}
		payload := map[string]any{
			"recording_ref": stopped.RecordingRef,
			"reason":        store.RecordingStopOrphanReaper,
			"reaper":        true,
		}
		if stopped.RecordingBytes != nil {
			payload["bytes"] = *stopped.RecordingBytes
		}
		raw, _ := json.Marshal(payload)
		if _, err := repo.AppendAuditEvent(ctx, store.AuditEvent{
			SessionID: sess.ID,
			TenantID:  sess.TenantID,
			EventType: store.AuditRecordingStopped,
			Payload:   raw,
		}); err != nil {
			applog.Warn("orphan reaper audit fail-open", "session", sess.ID, "err", err)
		}
		n++
	}
	return n, nil
}

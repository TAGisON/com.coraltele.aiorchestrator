package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestMemory_ListOrphanRecordingSessions(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()
	_ = mem.CreateProfile(ctx, store.Profile{ID: "p"})
	_, _ = mem.PublishVersion(ctx, "p", json.RawMessage(`{"id":"p","modes":{"think":true}}`))
	now := time.Now().UTC()
	_ = mem.CreateSession(ctx, store.Session{
		ID: "live", ProfileID: "p", ProfileVersion: 1, Clock: "live", State: store.StateRunning,
		CanonicalSampleRateHz: 16000, RecordingStartedAt: &now,
	})
	_ = mem.CreateSession(ctx, store.Session{
		ID: "orphan", ProfileID: "p", ProfileVersion: 1, Clock: "live", State: store.StateFailed,
		CanonicalSampleRateHz: 16000, RecordingRef: "/tmp/x.wav", RecordingStartedAt: &now,
	})
	_ = mem.CreateSession(ctx, store.Session{
		ID: "ok", ProfileID: "p", ProfileVersion: 1, Clock: "live", State: store.StateCompleted,
		CanonicalSampleRateHz: 16000, RecordingStartedAt: &now, RecordingStoppedAt: &now,
		RecordingStopReason: store.RecordingStopSessionEnding,
	})
	list, err := mem.ListOrphanRecordingSessions(ctx, 10)
	if err != nil || len(list) != 1 || list[0].ID != "orphan" {
		t.Fatalf("list %#v err=%v", list, err)
	}
}

package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestMemory_RecordingLifecycleStamps(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()
	_ = mem.CreateProfile(ctx, store.Profile{ID: "p"})
	_, _ = mem.PublishVersion(ctx, "p", json.RawMessage(`{"id":"p","modes":{"think":true}}`))
	_ = mem.CreateSession(ctx, store.Session{
		ID: "s1", ProfileID: "p", ProfileVersion: 1, Clock: "live", State: store.StateRunning,
		CanonicalSampleRateHz: 16000,
	})
	started, err := mem.MarkSessionRecordingStarted(ctx, "s1", "/tmp/rec.wav")
	if err != nil || started.RecordingRef != "/tmp/rec.wav" || started.RecordingStartedAt == nil {
		t.Fatalf("start %#v err=%v", started, err)
	}
	n := int64(1234)
	stopped, err := mem.MarkSessionRecordingStopped(ctx, "s1", "talk_end", &n)
	if err != nil || stopped.RecordingStoppedAt == nil {
		t.Fatalf("stop %#v err=%v", stopped, err)
	}
	if stopped.RecordingStopReason != store.RecordingStopSessionEnding {
		t.Fatalf("reason %q", stopped.RecordingStopReason)
	}
	if stopped.RecordingBytes == nil || *stopped.RecordingBytes != 1234 {
		t.Fatalf("bytes %#v", stopped.RecordingBytes)
	}
	// idempotent second stop keeps first reason/bytes
	again, err := mem.MarkSessionRecordingStopped(ctx, "s1", "failed", func() *int64 { v := int64(9999); return &v }())
	if err != nil {
		t.Fatal(err)
	}
	if again.RecordingStopReason != store.RecordingStopSessionEnding {
		t.Fatalf("reason mutated %q", again.RecordingStopReason)
	}
}

func TestMapRecordingStopReason(t *testing.T) {
	if store.MapRecordingStopReason("Completed") != store.RecordingStopSessionCompleted &&
		store.MapRecordingStopReason("completed") != store.RecordingStopSessionCompleted {
		t.Fatal(store.MapRecordingStopReason("completed"))
	}
	if store.MapRecordingStopReason("manual") != store.RecordingStopManual {
		t.Fatal("manual")
	}
}

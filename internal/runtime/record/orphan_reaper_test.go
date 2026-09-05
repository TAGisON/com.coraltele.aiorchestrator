package record_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/record"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestReapOrphans_StampsOrphanReaper(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()
	_ = mem.CreateProfile(ctx, store.Profile{ID: "p"})
	_, _ = mem.PublishVersion(ctx, "p", json.RawMessage(`{"id":"p","modes":{"think":true}}`))
	dir := t.TempDir()
	path := filepath.Join(dir, "rec.wav")
	if err := os.WriteFile(path, []byte("RIFF...."), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	_ = mem.CreateSession(ctx, store.Session{
		ID: "s1", ProfileID: "p", ProfileVersion: 1, Clock: "live", State: store.StateCancelled,
		CanonicalSampleRateHz: 16000, RecordingRef: path, RecordingStartedAt: &now,
	})
	n, err := record.ReapOrphans(ctx, mem, 10)
	if err != nil || n != 1 {
		t.Fatalf("reaped=%d err=%v", n, err)
	}
	sess, err := mem.GetSession(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.RecordingStoppedAt == nil || sess.RecordingStopReason != store.RecordingStopOrphanReaper {
		t.Fatalf("sess %#v", sess)
	}
	if sess.RecordingBytes == nil || *sess.RecordingBytes != 8 {
		t.Fatalf("bytes %#v", sess.RecordingBytes)
	}
	audits, err := mem.ListAuditEvents(ctx, "s1")
	if err != nil || len(audits) != 1 || audits[0].EventType != store.AuditRecordingStopped {
		t.Fatalf("audits %#v err=%v", audits, err)
	}
}

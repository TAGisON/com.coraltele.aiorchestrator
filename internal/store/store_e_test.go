package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestMemory_AuditAnalyticsPostcall(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()
	_ = mem.CreateProfile(ctx, store.Profile{ID: "p"})
	_, _ = mem.PublishVersion(ctx, "p", json.RawMessage(`{"id":"p","modes":{"think":true}}`))
	_ = mem.CreateSession(ctx, store.Session{
		ID: "s1", ProfileID: "p", ProfileVersion: 1, Clock: "live", State: store.StateRunning,
		CanonicalSampleRateHz: 16000,
	})

	ev, err := mem.AppendAuditEvent(ctx, store.AuditEvent{
		SessionID: "s1", EventType: store.AuditTurnCompleted,
		Payload: json.RawMessage(`{"outcome":"allow"}`),
	})
	if err != nil || ev.ID == 0 {
		t.Fatalf("audit %v %#v", err, ev)
	}
	list, err := mem.ListAuditEvents(ctx, "s1")
	if err != nil || len(list) != 1 {
		t.Fatalf("list audit %v len=%d", err, len(list))
	}

	ae, err := mem.AppendAnalyticsEvent(ctx, store.AnalyticsEvent{
		SessionID: "s1", ProfileID: "p", Metric: store.MetricTurnCompleted, Value: 1,
	})
	if err != nil || ae.ID == 0 {
		t.Fatalf("analytics %v", err)
	}
	ams, _ := mem.ListAnalyticsEvents(ctx, "s1")
	if len(ams) != 1 {
		t.Fatalf("analytics list %d", len(ams))
	}

	job := store.PostcallJob{
		ID: "pc1", SessionID: "s1", ProfileID: "p", ProfileVersion: 1, State: store.JobQueued,
	}
	if err := mem.CreatePostcallJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	if err := mem.CreatePostcallJob(ctx, store.PostcallJob{
		ID: "pc2", SessionID: "s1", ProfileID: "p", ProfileVersion: 1, State: store.JobQueued,
	}); err != store.ErrConflict {
		t.Fatalf("want conflict got %v", err)
	}
	leased, err := mem.LeaseNextPostcallJob(ctx, "worker-1")
	if err != nil || leased.State != store.JobRunning {
		t.Fatalf("lease %v %#v", err, leased)
	}
	leased.State = store.JobCompleted
	if err := mem.UpdatePostcallJob(ctx, leased); err != nil {
		t.Fatal(err)
	}
}

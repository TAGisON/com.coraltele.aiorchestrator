package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestMemory_FlowPublishAndPin(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()

	f, err := mem.CreateFlow(ctx, store.Flow{
		ID: "flow-1", TenantID: "t1", Name: "xfer",
	}, json.RawMessage(`{"schema_id":"coral.flow.v1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if f.Status != store.FlowStatusDraft || f.CurrentVersion != 0 {
		t.Fatalf("create %#v", f)
	}
	d, err := mem.GetFlowDraft(ctx, "flow-1")
	if err != nil || string(d.Doc) == "" {
		t.Fatalf("draft %#v %v", d, err)
	}

	fv, err := mem.PublishFlowVersion(ctx, "flow-1", json.RawMessage(`{"schema_id":"coral.flow.v1","entry_node_id":"n1"}`), "hash1", "ops")
	if err != nil {
		t.Fatal(err)
	}
	if fv.Version != 1 {
		t.Fatalf("version %d", fv.Version)
	}
	got, err := mem.GetFlow(ctx, "flow-1")
	if err != nil || got.CurrentVersion != 1 || got.Status != store.FlowStatusPublished {
		t.Fatalf("after publish %#v %v", got, err)
	}
	latest, err := mem.GetLatestFlowVersion(ctx, "flow-1")
	if err != nil || latest.Version != 1 {
		t.Fatalf("latest %#v %v", latest, err)
	}

	if err := mem.CreateSession(ctx, store.Session{
		ID: "s1", TenantID: "t1", ProfileID: "p1", ProfileVersion: 1,
		Clock: "live", State: store.StateRunning,
		FlowID: "flow-1", FlowVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}
	sess, err := mem.GetSession(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.FlowID != "flow-1" || sess.FlowVersion != 1 {
		t.Fatalf("pin round-trip %#v", sess)
	}
}

func TestMemory_BindingCRUD(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()
	b, err := mem.UpsertBinding(ctx, store.Binding{
		ID: "b1", TenantID: "t1", Kind: store.BindingKindKnowledge, Name: "faq",
		Config: json.RawMessage(`{"mode":"http"}`),
	})
	if err != nil || b.Status != store.BindingStatusActive {
		t.Fatalf("%#v %v", b, err)
	}
	got, err := mem.GetBinding(ctx, "b1")
	if err != nil || got.Name != "faq" {
		t.Fatalf("%#v %v", got, err)
	}
	list, err := mem.ListBindings(ctx, "t1", store.BindingKindKnowledge, 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("%#v %v", list, err)
	}
}

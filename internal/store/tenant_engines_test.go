package store_test

import (
	"context"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestMemory_TenantEnginesRoundTrip(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()

	_, err := mem.GetTenantEngines(ctx, "acme")
	if err != store.ErrNotFound {
		t.Fatalf("want ErrNotFound got %v", err)
	}

	saved, err := mem.UpsertTenantEngines(ctx, store.TenantEngines{
		TenantID: "acme",
		ListenID: "fake-listen",
		ThinkID:  "fake-think",
		SpeakID:  "fake-speak",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ListenID != "fake-listen" || saved.UpdatedAt.IsZero() {
		t.Fatalf("saved %+v", saved)
	}

	got, err := mem.GetTenantEngines(ctx, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if got.ThinkID != "fake-think" || got.SpeakID != "fake-speak" {
		t.Fatalf("got %+v", got)
	}

	b := got.Binding()
	if b.Listen != "fake-listen" || b.Think != "fake-think" || b.Speak != "fake-speak" {
		t.Fatalf("binding %+v", b)
	}
}

func TestMemory_SessionGatewayBinding(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()
	_ = mem.CreateProfile(ctx, store.Profile{ID: "p1"})
	_, err := mem.PublishVersion(ctx, "p1", []byte(`{"id":"p1","modes":{"listen":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	binding := &store.GatewayBinding{Listen: "fake-listen", Think: "fake-think", Speak: "fake-speak"}
	if err := mem.CreateSession(ctx, store.Session{
		ID:             "s1",
		ProfileID:      "p1",
		ProfileVersion: 1,
		Clock:          "live",
		State:          store.StateCreated,
		GatewayBinding: binding,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := mem.GetSession(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got.GatewayBinding == nil || got.GatewayBinding.Listen != "fake-listen" {
		t.Fatalf("gateway_binding %+v", got.GatewayBinding)
	}
}

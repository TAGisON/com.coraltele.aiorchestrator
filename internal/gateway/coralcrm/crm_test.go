package coralcrm_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/coralcrm"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
)

func TestExecute_CreateTicketStub(t *testing.T) {
	g := &coralcrm.Gateway{}
	res, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "s1",
		Name:      "create_ticket",
		Args:      []byte(`{"action":"create_ticket","subject":"billing"}`),
	})
	if err != nil || !res.OK {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestExecute_ResolveCustomerStub(t *testing.T) {
	g := &coralcrm.Gateway{}
	res, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "s1",
		Name:      "resolve_customer",
		Args:      []byte(`{"caller":"+9198","customer_ref":"cust-1"}`),
	})
	if err != nil || !res.OK {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	var out map[string]any
	if err := json.Unmarshal(res.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out["stub"] != true || out["customer_id"] == nil || out["customer_id"] == "" {
		t.Fatalf("want stub customer_id, got %#v", out)
	}
}

func TestExecute_PushDispositionStub(t *testing.T) {
	g := &coralcrm.Gateway{}
	res, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "s1",
		Name:      "push_disposition",
		Args:      []byte(`{"session_id":"s1","suggestion":"resolved","template_id":"cc-disposition-v1"}`),
	})
	if err != nil || !res.OK {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestRegister(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := coralcrm.Register(reg, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Get(coralcrm.ID); !ok {
		t.Fatal("missing")
	}
}

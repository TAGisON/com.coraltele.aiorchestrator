package coralcrm_test

import (
	"context"
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

func TestRegister(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := coralcrm.Register(reg, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Get(coralcrm.ID); !ok {
		t.Fatal("missing")
	}
}

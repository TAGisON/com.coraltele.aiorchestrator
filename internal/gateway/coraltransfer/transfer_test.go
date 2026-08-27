package coraltransfer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/coraltransfer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
)

func TestExecute_Stub(t *testing.T) {
	g := &coraltransfer.Gateway{}
	res, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "s1",
		TenantID:  "t1",
		Name:      "warm_transfer",
		Args:      []byte(`{"intent":"billing","escalation_reason":"low_confidence"}`),
	})
	if err != nil || !res.OK {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	var out map[string]any
	_ = json.Unmarshal(res.Output, &out)
	if out["stub"] != true {
		t.Fatalf("output %+v", out)
	}
}

func TestExecute_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	g := &coraltransfer.Gateway{BaseURL: srv.URL}
	res, err := g.Execute(context.Background(), port.SkillRequest{SessionID: "s1", Name: "warm_transfer", Args: []byte(`{}`)})
	if err != nil || !res.OK {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestRegister(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := coraltransfer.Register(reg, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Get(coraltransfer.ID); !ok {
		t.Fatal("missing")
	}
}

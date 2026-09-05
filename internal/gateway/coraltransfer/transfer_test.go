package coraltransfer_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/coraltransfer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
)

func decode(t *testing.T, res port.SkillResult) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(res.Output, &out); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	return out
}

// A session with no telephony leg (playback / lab clock) must succeed without
// pretending a transfer happened.
func TestExecute_NoLegIsNotAnError(t *testing.T) {
	g := &coraltransfer.Gateway{}
	res, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "s1",
		TenantID:  "t1",
		Name:      "warm_transfer",
		Args:      []byte(`{"intent":"billing","destination":"1001"}`),
	})
	if err != nil || !res.OK {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	out := decode(t, res)
	if out["transferred"] != false {
		t.Fatalf("must not claim a transfer with no leg: %+v", out)
	}
	if out["transfer_skipped"] == nil {
		t.Fatalf("expected transfer_skipped reason: %+v", out)
	}
}

func TestExecute_TransfersLeg(t *testing.T) {
	var got port.TransferRequest
	var gotSession string
	g := &coraltransfer.Gateway{
		DefaultDialplan: "XML",
		DefaultContext:  "calltransfer",
		Transfer: func(_ context.Context, sessionID string, req port.TransferRequest) error {
			gotSession, got = sessionID, req
			return nil
		},
	}
	res, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "sess-9",
		Name:      "warm_transfer",
		Args:      []byte(`{"destination":"1001","escalation_reason":"low_confidence"}`),
	})
	if err != nil || !res.OK {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if gotSession != "sess-9" {
		t.Fatalf("session = %q", gotSession)
	}
	if got.Destination != "1001" {
		t.Fatalf("destination = %q", got.Destination)
	}
	if got.Dialplan != "XML" || got.Context != "calltransfer" {
		t.Fatalf("defaults not applied: %+v", got)
	}
	if got.Reason != "low_confidence" {
		t.Fatalf("reason = %q", got.Reason)
	}
	if out := decode(t, res); out["transferred"] != true {
		t.Fatalf("output %+v", out)
	}
}

func TestExecute_PassesDispositionCode(t *testing.T) {
	var got port.TransferRequest
	g := &coraltransfer.Gateway{
		Transfer: func(_ context.Context, _ string, req port.TransferRequest) error {
			got = req
			return nil
		},
	}
	_, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "s", Name: "warm_transfer",
		Args: []byte(`{"destination":"1001","disposition_code":"transferred_sales","intent":"sales"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.DispositionCode != "transferred_sales" {
		t.Fatalf("disposition_code = %q", got.DispositionCode)
	}
}

// Operators and LLM tool calls spell the destination several ways.
func TestExecute_DestinationAliases(t *testing.T) {
	for _, key := range []string{"destination", "number", "extension", "dest", "transfer_to"} {
		var got string
		g := &coraltransfer.Gateway{
			Transfer: func(_ context.Context, _ string, req port.TransferRequest) error {
				got = req.Destination
				return nil
			},
		}
		args := `{"` + key + `":"2002"}`
		if _, err := g.Execute(context.Background(), port.SkillRequest{
			SessionID: "s", Name: "warm_transfer", Args: []byte(args),
		}); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if got != "2002" {
			t.Fatalf("alias %q not honoured, got %q", key, got)
		}
	}
}

func TestExecute_FallsBackToDefaultDestination(t *testing.T) {
	var got string
	g := &coraltransfer.Gateway{
		DefaultDestination: "9000",
		Transfer: func(_ context.Context, _ string, req port.TransferRequest) error {
			got = req.Destination
			return nil
		},
	}
	if _, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "s", Name: "warm_transfer", Args: []byte(`{}`),
	}); err != nil {
		t.Fatalf("err %v", err)
	}
	if got != "9000" {
		t.Fatalf("default destination not used, got %q", got)
	}
}

func TestExecute_MissingDestinationIsBadRequest(t *testing.T) {
	g := &coraltransfer.Gateway{
		Transfer: func(context.Context, string, port.TransferRequest) error {
			t.Fatal("must not attempt a transfer with no destination")
			return nil
		},
	}
	res, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "s", Name: "warm_transfer", Args: []byte(`{}`),
	})
	if err == nil || res.OK {
		t.Fatalf("expected failure, res=%+v err=%v", res, err)
	}
	ge, ok := port.AsGatewayError(err)
	if !ok || ge.Code != port.CodeBadRequest {
		t.Fatalf("want bad_request, got %v", err)
	}
}

func TestExecute_TransferFailureIsReported(t *testing.T) {
	g := &coraltransfer.Gateway{
		Transfer: func(context.Context, string, port.TransferRequest) error {
			return errors.New("no telephony leg")
		},
	}
	res, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "s", Name: "warm_transfer", Args: []byte(`{"destination":"1001"}`),
	})
	if err == nil || res.OK {
		t.Fatalf("expected failure, res=%+v err=%v", res, err)
	}
	ge, ok := port.AsGatewayError(err)
	if !ok || ge.Code != port.CodeUnavailable {
		t.Fatalf("want unavailable, got %v", err)
	}
}

// A Coral outage must not strand a caller who was promised a human.
func TestExecute_NotifyFailureStillTransfers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	transferred := false
	g := &coraltransfer.Gateway{
		BaseURL: srv.URL,
		Transfer: func(context.Context, string, port.TransferRequest) error {
			transferred = true
			return nil
		},
	}
	res, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "s", Name: "warm_transfer", Args: []byte(`{"destination":"1001"}`),
	})
	if err != nil || !res.OK {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if !transferred {
		t.Fatal("leg must still be transferred when Coral notification fails")
	}
	out := decode(t, res)
	if out["notified"] != false {
		t.Fatalf("notify failure should be recorded: %+v", out)
	}
}

func TestExecute_NotifiesCoral(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	g := &coraltransfer.Gateway{
		BaseURL:  srv.URL,
		Transfer: func(context.Context, string, port.TransferRequest) error { return nil },
	}
	res, err := g.Execute(context.Background(), port.SkillRequest{
		SessionID: "s1", TenantID: "t1", Name: "warm_transfer",
		Args: []byte(`{"destination":"1001","intent":"billing","summary":"needs a human"}`),
	})
	if err != nil || !res.OK {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if gotPath != "/skills/warm-transfer" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["session_id"] != "s1" || gotBody["intent"] != "billing" {
		t.Fatalf("payload %+v", gotBody)
	}
	if out := decode(t, res); out["notified"] != true {
		t.Fatalf("output %+v", out)
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

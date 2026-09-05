package control

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/fallback"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestTransferDispositionFinal(t *testing.T) {
	cases := []struct {
		name string
		req  port.TransferRequest
		want string
	}{
		{"explicit code", port.TransferRequest{DispositionCode: store.DispositionFinalTransferredSales}, store.DispositionFinalTransferredSales},
		{"sales reason", port.TransferRequest{Reason: "intent:sales"}, store.DispositionFinalTransferredSales},
		{"corporate", port.TransferRequest{Reason: "corporate queue"}, store.DispositionFinalTransferredCorporate},
		{"support", port.TransferRequest{Reason: "tech support"}, store.DispositionFinalTransferredSupport},
		{"other", port.TransferRequest{Reason: "warm_transfer"}, store.DispositionFinalTransferredOther},
		{"bad code falls to reason", port.TransferRequest{DispositionCode: "resolved", Reason: "sales"}, store.DispositionFinalTransferredSales},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := transferDispositionFinal(tc.req); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestEnsureTerminalDisposition(t *testing.T) {
	mem := store.NewMemory()
	srv := New(mem, router.NewMemRegistry(), Config{})
	sess := store.Session{ID: "s-term", TenantID: "t1", State: store.StateCancelled}
	_ = mem.CreateSession(context.Background(), sess)

	srv.ensureTerminalDisposition(context.Background(), sess, store.StateCancelled)
	d, err := mem.GetSessionDisposition(context.Background(), "s-term")
	if err != nil {
		t.Fatal(err)
	}
	if d.Final != store.DispositionFinalAbandonedCaller || d.Source != store.DispositionSourceLiveGraph {
		t.Fatalf("got %#v", d)
	}

	// Does not overwrite an existing final.
	_, _ = mem.UpsertSessionDisposition(context.Background(), store.SessionDisposition{
		SessionID: "s-term", Final: store.DispositionFinalTransferredOther, Source: store.DispositionSourceLiveTool,
	})
	srv.ensureTerminalDisposition(context.Background(), sess, store.StateCancelled)
	d, _ = mem.GetSessionDisposition(context.Background(), "s-term")
	if d.Final != store.DispositionFinalTransferredOther {
		t.Fatalf("overwrote final %#v", d)
	}
}

type stubCallControl struct {
	xferErr error
	hangErr error
}

func (s *stubCallControl) ID() port.GatewayID { return "stub-cc" }
func (s *stubCallControl) WritePCM(context.Context, port.PCMFrame) error {
	return nil
}
func (s *stubCallControl) Flush(context.Context) error    { return nil }
func (s *stubCallControl) WaitMark(context.Context) error { return nil }
func (s *stubCallControl) Close(context.Context) error    { return nil }
func (s *stubCallControl) Transfer(context.Context, port.TransferRequest) error {
	return s.xferErr
}
func (s *stubCallControl) Hangup(context.Context, string) error { return s.hangErr }

func startRuntimeWithCC(t *testing.T, cc port.Sink) (*SessionRuntime, *store.Memory, string) {
	t.Helper()
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	mgr := session.NewManager(reg)
	rt := &SessionRuntime{Mgr: mgr, Repo: mem}
	doc, err := profile.Parse([]byte(`{
  "id":"e5",
  "modes":{"listen":true,"speak":true,"think":true,"talk":true},
  "audio":{"canonical_sample_rate_hz":8000},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	sid := "s-e5-" + strings.ReplaceAll(t.Name(), "/", "_")
	if err := rt.StartSession(context.Background(), RuntimeStart{
		SessionID: sid, TenantID: "t1", Clock: "live", SampleRate: 8000, Profile: doc,
		GatewayBinding: &store.GatewayBinding{
			Listen: "fake-listen", Speak: "fake-speak", Think: "fake-think",
		},
	}); err != nil {
		t.Fatal(err)
	}
	a, ok := mgr.Get(sid)
	if !ok {
		t.Fatal("actor missing")
	}
	if cc != nil {
		a.AttachSink(cc, "stub")
	}
	_ = mem.CreateSession(context.Background(), store.Session{
		ID: sid, TenantID: "t1", ProfileID: "e5", ProfileVersion: 1, Clock: "live", State: store.StateRunning,
	})
	return rt, mem, sid
}

func TestTransferWritesLiveToolDisposition(t *testing.T) {
	rt, mem, sid := startRuntimeWithCC(t, &stubCallControl{})
	err := rt.Transfer(context.Background(), sid, port.TransferRequest{
		Destination: "1001", Reason: "sales",
	})
	if err != nil {
		t.Fatal(err)
	}
	d, err := mem.GetSessionDisposition(context.Background(), sid)
	if err != nil {
		t.Fatal(err)
	}
	if d.Final != store.DispositionFinalTransferredSales || d.Source != store.DispositionSourceLiveTool {
		t.Fatalf("got %#v", d)
	}
}

func TestTransferFailWritesSystemFailure(t *testing.T) {
	rt, mem, sid := startRuntimeWithCC(t, &stubCallControl{xferErr: errors.New("busy")})
	err := rt.Transfer(context.Background(), sid, port.TransferRequest{Destination: "1001"})
	if err == nil {
		t.Fatal("want error")
	}
	d, err := mem.GetSessionDisposition(context.Background(), sid)
	if err != nil {
		t.Fatal(err)
	}
	if d.Final != store.DispositionFinalSystemFailure || d.Source != store.DispositionSourceLiveTool {
		t.Fatalf("got %#v", d)
	}
}

func TestFailCallWritesSystemFailure(t *testing.T) {
	// No CallControl: avoid waitCallControlGone; disposition still written.
	rt, mem, sid := startRuntimeWithCC(t, nil)
	rt.FailCall(context.Background(), sid, fallback.ScenarioTimeout, errors.New("stt down"))
	d, err := mem.GetSessionDisposition(context.Background(), sid)
	if err != nil {
		t.Fatal(err)
	}
	if d.Final != store.DispositionFinalSystemFailure || d.Source != store.DispositionSourceLiveTool {
		t.Fatalf("got %#v", d)
	}
}

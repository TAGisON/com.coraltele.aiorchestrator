package control_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// C.3 — same graph evidence as G.7 live cutover, on clock=chat (no TTS required).
func TestChat_EvidenceParity_EdgeToolAuditDisposition(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	rt := &control.SessionRuntime{Mgr: session.NewManager(reg), Repo: mem}
	srv := control.NewWithRuntime(mem, reg, rt, control.Config{}, nil)

	createProfile(t, srv, "c3-prof")
	publishOK(t, srv, "c3-prof", `{
  "id":"c3-prof",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)
	createFlow(t, srv, "c3-flow")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/flows/c3-flow/versions", bytes.NewBufferString(g4TransferFlowDoc()))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"c3-prof","profile_version":"latest",
  "flow_id":"c3-flow","flow_version":"latest","clock":"chat",
  "caller":{"ani":"c3-ani","caller_id_name":"Chat Parity"}
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		SessionID string `json:"session_id"`
		Clock     string `json:"clock"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	if created.Clock != "chat" {
		t.Fatalf("clock %q", created.Clock)
	}
	sid := created.SessionID

	cc := &recordingCC{}
	a, ok := rt.Mgr.Get(sid)
	if !ok {
		t.Fatal("actor missing")
	}
	a.AttachSink(cc, "cc")

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sid+"/answer", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("answer %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sid+"/inject", bytes.NewBufferString(`{"text":"sales"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("inject %d %s", rr.Code, rr.Body.String())
	}

	turns, err := mem.ListTranscriptTurns(t.Context(), sid)
	if err != nil {
		t.Fatal(err)
	}
	var sawEdge, sawTool, sawBot, sawUser bool
	for _, tr := range turns {
		switch tr.EventKind {
		case store.EventKindEdgeTaken:
			sawEdge = true
		case store.EventKindToolLine:
			sawTool = true
		case store.EventKindBotUtterance:
			sawBot = true
		case store.EventKindUserFinal:
			sawUser = true
		}
	}
	if !sawEdge || !sawTool {
		t.Fatalf("want edge_taken and tool_line in %#v", turns)
	}
	if !sawBot {
		t.Fatalf("want bot_utterance (welcome/closing) in %#v", turns)
	}
	if !sawUser {
		t.Fatalf("want user_final from inject in %#v", turns)
	}

	audits, err := mem.ListAuditEvents(t.Context(), sid)
	if err != nil {
		t.Fatal(err)
	}
	var sawGraph, sawToolArmed, sawToolDone bool
	for _, aev := range audits {
		switch aev.EventType {
		case store.AuditGraphEdge:
			sawGraph = true
		case store.AuditToolArmed:
			sawToolArmed = true
		case store.AuditToolExecuted, store.AuditToolFailed:
			sawToolDone = true
		}
	}
	if !sawGraph {
		t.Fatalf("want graph.edge audit in %#v", audits)
	}
	if !sawToolArmed || !sawToolDone {
		t.Fatalf("want tool.armed and tool.executed|failed in %#v", audits)
	}

	disp, err := mem.GetSessionDisposition(t.Context(), sid)
	if err != nil {
		t.Fatal(err)
	}
	if disp.Final == "" {
		t.Fatalf("want disposition final after transfer settle, got %#v", disp)
	}
}

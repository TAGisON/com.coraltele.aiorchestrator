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

func TestLiveSession_RequiresFlowPin(t *testing.T) {
	srv, _ := testServer(t)
	createProfile(t, srv, "cutover-prof")
	publishOK(t, srv, "cutover-prof", `{
  "id":"cutover-prof",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"cutover-prof","profile_version":"latest","clock":"live"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != control.CodeFlowPinRequired {
		t.Fatalf("code %q", env.Error.Code)
	}
}

func TestPlaybackSession_AllowsProfileOnly(t *testing.T) {
	srv, _ := testServer(t)
	createProfile(t, srv, "pb-prof")
	publishOK(t, srv, "pb-prof", `{
  "id":"pb-prof",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"pb-prof","profile_version":"latest","clock":"playback"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestGraph_EmitsEdgeTakenAndToolLine(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	rt := &control.SessionRuntime{Mgr: session.NewManager(reg), Repo: mem}
	srv := control.NewWithRuntime(mem, reg, rt, control.Config{}, nil)

	createProfile(t, srv, "ev-prof")
	publishOK(t, srv, "ev-prof", `{
  "id":"ev-prof",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)
	createFlow(t, srv, "ev-flow")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/flows/ev-flow/versions", bytes.NewBufferString(g4TransferFlowDoc()))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"ev-prof","profile_version":"latest",
  "flow_id":"ev-flow","flow_version":"latest","clock":"live"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		SessionID string `json:"session_id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
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
	var sawEdge, sawTool bool
	for _, tr := range turns {
		if tr.EventKind == store.EventKindEdgeTaken {
			sawEdge = true
		}
		if tr.EventKind == store.EventKindToolLine {
			sawTool = true
		}
	}
	if !sawEdge || !sawTool {
		t.Fatalf("want edge_taken and tool_line in %#v", turns)
	}
	audits, err := mem.ListAuditEvents(t.Context(), sid)
	if err != nil {
		t.Fatal(err)
	}
	var sawGraph bool
	for _, aev := range audits {
		if aev.EventType == store.AuditGraphEdge {
			sawGraph = true
			break
		}
	}
	if !sawGraph {
		t.Fatalf("want graph.edge audit in %#v", audits)
	}
}

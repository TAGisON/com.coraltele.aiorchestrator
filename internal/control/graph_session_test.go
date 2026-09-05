package control_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func g3FlowDoc() string {
	return `{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"welcome","type":"Speak","prompt_ref":"welcome"},
    {"id":"choice","type":"ListenChoice"},
    {"id":"bye","type":"Speak","prompt_ref":"bye"},
    {"id":"end","type":"End"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"welcome","kind":"next"},
    {"id":"e2","from":"welcome","to":"choice","kind":"next"},
    {"id":"e3","from":"choice","to":"bye","kind":"intent","intent":"done"},
    {"id":"e4","from":"bye","to":"end","kind":"next"}
  ],
  "prompts":{
    "welcome":{"en-IN":"Welcome to Coral"},
    "bye":{"en-IN":"Goodbye"}
  },
  "matrix":[],
  "binding_refs":[]
}`
}

func TestGraph_PinnedSession_WelcomeChoiceEnd(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	rt := &control.SessionRuntime{Mgr: session.NewManager(reg), Repo: mem}
	srv := control.NewWithRuntime(mem, reg, rt, control.Config{}, nil)

	createProfile(t, srv, "g3-prof")
	publishOK(t, srv, "g3-prof", `{
  "id":"g3-prof",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)

	createFlow(t, srv, "g3-flow")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/flows/g3-flow/versions", bytes.NewBufferString(g3FlowDoc()))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish flow %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"g3-prof",
  "profile_version":"latest",
  "flow_id":"g3-flow",
  "flow_version":"latest",
  "clock":"live"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create session %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		SessionID   string `json:"session_id"`
		FlowID      string `json:"flow_id"`
		FlowVersion int    `json:"flow_version"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.FlowID != "g3-flow" || created.FlowVersion != 1 {
		t.Fatalf("pins %#v", created)
	}
	sid := created.SessionID

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sid+"/answer", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("answer %d %s", rr.Code, rr.Body.String())
	}
	var ans struct {
		Spoken string `json:"spoken"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &ans); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ans.Spoken, "Welcome to Coral") {
		t.Fatalf("spoken %q", ans.Spoken)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sid+"/inject", bytes.NewBufferString(`{"text":"done","speak":true}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("inject %d %s", rr.Code, rr.Body.String())
	}

	// Graph End should have completed the session.
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sid, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get session %d", rr.Code)
	}
	var sess struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &sess); err != nil {
		t.Fatal(err)
	}
	if sess.State != store.StateCompleted && sess.State != store.StateDraining {
		// allow brief drain race; Completed is expected after EndSessionAfterTalk
		t.Fatalf("state %q want completed", sess.State)
	}

	disp, err := mem.GetSessionDisposition(t.Context(), sid)
	if err != nil {
		t.Fatal(err)
	}
	if disp.Final != store.DispositionFinalHangupCompleted {
		t.Fatalf("disposition %#v", disp)
	}
}

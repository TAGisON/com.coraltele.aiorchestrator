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

func g5RepairFlowDoc() string {
	return `{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"welcome","type":"Speak","prompt_ref":"welcome"},
    {"id":"choice","type":"ListenChoice","repair":{"max_retries":0,"unclear_prompt_ref":"repair_unclear"}},
    {"id":"hang","type":"Tool","tool":"hangup","prompt_ref":"closing_hangup"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"welcome","kind":"next"},
    {"id":"e2","from":"welcome","to":"choice","kind":"next"},
    {"id":"e3","from":"choice","to":"hang","kind":"repair"},
    {"id":"e4","from":"choice","to":"hang","kind":"intent","intent":"ok"}
  ],
  "prompts":{
    "welcome":{"en-IN":"Welcome"},
    "repair_unclear":{"en-IN":"Please say ok"},
    "closing_hangup":{"en-IN":"Goodbye"}
  },
  "matrix":[],
  "binding_refs":[]
}`
}

func TestGraph_RepairExhaust_Hangup(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	rt := &control.SessionRuntime{Mgr: session.NewManager(reg), Repo: mem}
	srv := control.NewWithRuntime(mem, reg, rt, control.Config{}, nil)

	createProfile(t, srv, "g5-prof")
	publishOK(t, srv, "g5-prof", `{
  "id":"g5-prof",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)
	createFlow(t, srv, "g5-flow")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/flows/g5-flow/versions", bytes.NewBufferString(g5RepairFlowDoc()))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"g5-prof","profile_version":"latest",
  "flow_id":"g5-flow","flow_version":"latest","clock":"live"
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

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sid+"/answer", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("answer %d %s", rr.Code, rr.Body.String())
	}

	// max_retries=0 → first unclear exhausts to hangup Tool (no telephony → disposition only).
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sid+"/inject", bytes.NewBufferString(`{"text":"zzz"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("inject %d %s", rr.Code, rr.Body.String())
	}

	disp, err := mem.GetSessionDisposition(t.Context(), sid)
	if err != nil {
		t.Fatal(err)
	}
	if disp.Final != store.DispositionFinalHangupCompleted {
		t.Fatalf("disposition %#v", disp)
	}
}

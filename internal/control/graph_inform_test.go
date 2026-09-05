package control_test

import (
	"bytes"
	"context"
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

func g6InformFlowDoc() string {
	return `{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"welcome","type":"Speak","prompt_ref":"welcome"},
    {"id":"choice","type":"ListenChoice"},
    {"id":"faq","type":"Inform","binding_ref":"lab-faq"},
    {"id":"end","type":"End"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"welcome","kind":"next"},
    {"id":"e2","from":"welcome","to":"choice","kind":"next"},
    {"id":"e3","from":"choice","to":"faq","kind":"intent","intent":"hours"},
    {"id":"e4","from":"faq","to":"end","kind":"next"},
    {"id":"e5","from":"faq","to":"end","kind":"repair"}
  ],
  "prompts":{"welcome":{"en-IN":"Welcome"}},
  "matrix":[],
  "binding_refs":["lab-faq"]
}`
}

func TestGraph_Inform_InlineFAQ(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	_, err := mem.UpsertBinding(context.Background(), store.Binding{
		ID: "lab-faq", TenantID: "default", Kind: store.BindingKindKnowledge, Name: "Lab FAQ",
		Status: store.BindingStatusActive,
		Config: json.RawMessage(`{"mode":"inline_faq","entries":[{"id":"hours","questions":["hours","open"],"text":{"en-IN":"Nine to five."}}]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	rt := &control.SessionRuntime{Mgr: session.NewManager(reg), Repo: mem}
	srv := control.NewWithRuntime(mem, reg, rt, control.Config{}, nil)

	createProfile(t, srv, "g6-prof")
	publishOK(t, srv, "g6-prof", `{
  "id":"g6-prof",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)
	createFlow(t, srv, "g6-flow")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/flows/g6-flow/versions", bytes.NewBufferString(g6InformFlowDoc()))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"g6-prof","profile_version":"latest",
  "flow_id":"g6-flow","flow_version":"latest","clock":"live","tenant_id":"default"
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

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/"+created.SessionID+"/answer", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("answer %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/"+created.SessionID+"/inject", bytes.NewBufferString(`{"text":"hours"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("inject %d %s", rr.Code, rr.Body.String())
	}

	// After Inform → End, session should complete.
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+created.SessionID+"/transcript", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("transcript %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Nine to five") {
		t.Fatalf("transcript missing FAQ answer: %s", body)
	}
}

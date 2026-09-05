package control_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

type recordingCC struct {
	mu       sync.Mutex
	lastXfer port.TransferRequest
	hangups  int
}

func (s *recordingCC) ID() port.GatewayID { return "rec-cc" }
func (s *recordingCC) WritePCM(context.Context, port.PCMFrame) error {
	return nil
}
func (s *recordingCC) Flush(context.Context) error    { return nil }
func (s *recordingCC) WaitMark(context.Context) error { return nil }
func (s *recordingCC) Close(context.Context) error    { return nil }
func (s *recordingCC) Transfer(_ context.Context, req port.TransferRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastXfer = req
	return nil
}
func (s *recordingCC) Hangup(context.Context, string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hangups++
	return nil
}

func g4TransferFlowDoc() string {
	return `{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"welcome","type":"Speak","prompt_ref":"welcome"},
    {"id":"choice","type":"ListenChoice"},
    {"id":"xfer","type":"Tool","tool":"transfer","matrix_intent":"sales","prompt_ref":"closing_transfer"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"welcome","kind":"next"},
    {"id":"e2","from":"welcome","to":"choice","kind":"next"},
    {"id":"e3","from":"choice","to":"xfer","kind":"intent","intent":"sales"}
  ],
  "prompts":{
    "welcome":{"en-IN":"Welcome"},
    "closing_transfer":{"en-IN":"Connecting you"}
  },
  "matrix":[
    {"intent":"sales","owner":"Sales","target":"sales-q","number":"1001","action":"transfer","disposition_code":"transferred_sales"}
  ],
  "binding_refs":[]
}`
}

func TestGraph_ToolTransfer_ArmSpeakExec(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	rt := &control.SessionRuntime{Mgr: session.NewManager(reg), Repo: mem}
	srv := control.NewWithRuntime(mem, reg, rt, control.Config{}, nil)

	createProfile(t, srv, "g4-prof")
	publishOK(t, srv, "g4-prof", `{
  "id":"g4-prof",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)
	createFlow(t, srv, "g4-flow")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/flows/g4-flow/versions", bytes.NewBufferString(g4TransferFlowDoc()))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish flow %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"g4-prof","profile_version":"latest",
  "flow_id":"g4-flow","flow_version":"latest","clock":"live"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
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

	cc.mu.Lock()
	dest := cc.lastXfer.Destination
	code := cc.lastXfer.DispositionCode
	cc.mu.Unlock()
	if dest != "1001" || code != store.DispositionFinalTransferredSales {
		t.Fatalf("xfer dest=%q code=%q", dest, code)
	}

	disp, err := mem.GetSessionDisposition(context.Background(), sid)
	if err != nil {
		t.Fatal(err)
	}
	if disp.Final != store.DispositionFinalTransferredSales {
		t.Fatalf("disposition %#v", disp)
	}
	if !strings.Contains(disp.Source, "tool") && disp.Source != store.DispositionSourceLiveTool {
		// live_tool from Transfer settle
		t.Logf("disposition source %q", disp.Source)
	}
}

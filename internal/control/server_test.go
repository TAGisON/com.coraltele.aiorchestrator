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

func testServer(t *testing.T) (*control.Server, *store.Memory) {
	t.Helper()
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	srv := control.New(mem, reg, control.Config{})
	return srv, mem
}

func testServerEmpty(t *testing.T) (*control.Server, *store.Memory) {
	t.Helper()
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	srv := control.New(mem, reg, control.Config{})
	return srv, mem
}

func TestHealth_OK(t *testing.T) {
	srv, _ := testServer(t)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/health", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestHealth_DBDown(t *testing.T) {
	srv, mem := testServer(t)
	mem.SetHealthy(false)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/health", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestPublish_UnknownGateway_422(t *testing.T) {
	srv, _ := testServer(t)
	createProfile(t, srv, "p1")

	body := `{
  "id":"p1",
  "modes":{"listen":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "routers":{"listen":{"providers":["not-a-real-gateway"]}}
}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/profiles/p1/versions", bytes.NewBufferString(body))
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
	if env.Error.Code != control.CodeProfileInvalid {
		t.Fatalf("code %q", env.Error.Code)
	}
}

func TestSession_CreateGetStop(t *testing.T) {
	srv, _ := testServer(t)
	createProfile(t, srv, "contact-lab")
	publishOK(t, srv, "contact-lab", `{
  "id":"contact-lab",
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
  "profile_id":"contact-lab",
  "profile_version":"latest",
  "clock":"live"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status %d body %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	sid, _ := created["session_id"].(string)
	if sid == "" {
		t.Fatal("missing session_id")
	}
	if created["state"] != "Created" {
		t.Fatalf("state %v", created["state"])
	}
	if created["profile_version"].(float64) != 1 {
		t.Fatalf("version %v", created["profile_version"])
	}
	if created["gateway_binding"] == nil {
		t.Fatal("missing gateway_binding")
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sid, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get status %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	stopReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sid+"/stop", bytes.NewBufferString(`{"reason":"operator"}`))
	stopReq.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, stopReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("stop status %d body %s", rr.Code, rr.Body.String())
	}
	var stopped map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &stopped)
	if stopped["state"] != "Cancelled" {
		t.Fatalf("want Cancelled got %v", stopped["state"])
	}
}

func createProfile(t *testing.T, srv *control.Server, id string) {
	t.Helper()
	rr := httptest.NewRecorder()
	body := `{"id":"` + id + `","display_name":"lab"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/profiles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create profile %d %s", rr.Code, rr.Body.String())
	}
}

func publishOK(t *testing.T, srv *control.Server, id, body string) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/profiles/"+id+"/versions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish %d %s", rr.Code, rr.Body.String())
	}
}

func TestSession_WithRuntime_CreateRunningStop(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	rt := &control.SessionRuntime{Mgr: session.NewManager(reg)}
	srv := control.NewWithRuntime(mem, reg, rt, control.Config{}, nil)
	createProfile(t, srv, "rt-lab")
	publishOK(t, srv, "rt-lab", `{
  "id":"rt-lab",
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
  "profile_id":"rt-lab",
  "profile_version":"latest",
  "clock":"live"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	if created["state"] != "Running" {
		t.Fatalf("want Running got %v", created["state"])
	}
	sid := created["session_id"].(string)
	rr = httptest.NewRecorder()
	stopReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sid+"/stop", bytes.NewBufferString(`{"reason":"operator"}`))
	stopReq.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, stopReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("stop %d %s", rr.Code, rr.Body.String())
	}
	var stopped map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &stopped)
	if stopped["state"] != "Cancelled" {
		t.Fatalf("want Cancelled got %v", stopped["state"])
	}
}

func TestSession_CreateStoresCallerAndMetadata(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	rt := &control.SessionRuntime{Mgr: session.NewManager(reg)}
	srv := control.NewWithRuntime(mem, reg, rt, control.Config{}, nil)
	createProfile(t, srv, "rt-lab")
	publishOK(t, srv, "rt-lab", `{
  "id":"rt-lab",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)
	body := `{
  "profile_id":"rt-lab",
  "profile_version":"latest",
  "clock":"live",
  "caller":{"ani":"5001","caller_id_name":"Desk"},
  "metadata":{"edge":"mod_audio_stream","call_uuid":"fs-uuid-1","sip_call_id":"sip-abc","destination":"101"}
}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	sid := created["session_id"].(string)

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sid, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get %d %s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	caller, _ := got["caller"].(map[string]any)
	meta, _ := got["metadata"].(map[string]any)
	if caller["ani"] != "5001" {
		t.Fatalf("caller %#v", got["caller"])
	}
	if meta["call_uuid"] != "fs-uuid-1" || meta["destination"] != "101" {
		t.Fatalf("metadata %#v", got["metadata"])
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sid+"/audit", nil))
	var audit struct {
		Events []struct {
			Type    string         `json:"event_type"`
			Payload map[string]any `json:"payload"`
		} `json:"audit_events"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &audit)
	found := false
	for _, e := range audit.Events {
		if e.Type != store.AuditSessionLive {
			continue
		}
		found = true
		md, _ := e.Payload["metadata"].(map[string]any)
		if md["sip_call_id"] != "sip-abc" {
			t.Fatalf("audit metadata %#v", e.Payload["metadata"])
		}
	}
	if !found {
		t.Fatal("missing session.live audit")
	}
}

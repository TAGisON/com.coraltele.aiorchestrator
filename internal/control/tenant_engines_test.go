package control_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func seedFakeTenantEngines(t *testing.T, mem *store.Memory) {
	t.Helper()
	_, err := mem.UpsertTenantEngines(context.Background(), store.TenantEngines{
		TenantID: "default",
		ListenID: "fake-listen",
		ThinkID:  "fake-think",
		SpeakID:  "fake-speak",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTenantEngines_GetSeedsFromDefaults(t *testing.T) {
	control.EngineDefaults = store.GatewayBinding{
		Listen: "fake-listen", Think: "fake-think", Speak: "fake-speak",
	}
	srv, _ := testServer(t)

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/tenant/engines", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["source"] != "store" {
		t.Fatalf("source %v", got["source"])
	}
	if got["listen"] != "fake-listen" || got["think"] != "fake-think" || got["speak"] != "fake-speak" {
		t.Fatalf("engines %+v", got)
	}
}

func TestTenantEngines_PutUnknown_422(t *testing.T) {
	srv, _ := testServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/tenant/engines", bytes.NewBufferString(`{
  "listen":"not-a-gateway",
  "think":"fake-think",
  "speak":"fake-speak"
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
	_ = json.Unmarshal(rr.Body.Bytes(), &env)
	if env.Error.Code != control.CodeBadRequest {
		t.Fatalf("code %q", env.Error.Code)
	}
}

func TestTenantEngines_PutAndGet(t *testing.T) {
	srv, _ := testServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/tenant/engines", bytes.NewBufferString(`{
  "listen":"fake-listen",
  "think":"fake-think",
  "speak":"fake-speak"
}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "acme")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put status %d body %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/v1/tenant/engines", nil)
	getReq.Header.Set("X-Tenant-ID", "acme")
	srv.Handler().ServeHTTP(rr, getReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("get status %d", rr.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got["source"] != "store" || got["listen"] != "fake-listen" {
		t.Fatalf("got %+v", got)
	}
}

func TestSession_CreatePersistsGatewayBinding(t *testing.T) {
	srv, mem := testServer(t)
	seedFakeTenantEngines(t, mem)
	createProfile(t, srv, "cc-lab")
	publishOK(t, srv, "cc-lab", `{
  "id":"cc-lab",
  "metadata":{"family":"contact-agent"},
  "modes":{"listen":true,"speak":true,"think":true,"talk":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice"}}
}`)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"cc-lab",
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
	gb, _ := created["gateway_binding"].(map[string]any)
	if gb["listen"] != "fake-listen" || gb["think"] != "fake-think" || gb["speak"] != "fake-speak" {
		t.Fatalf("create gateway_binding %+v", created["gateway_binding"])
	}
	sid, _ := created["session_id"].(string)

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sid, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get status %d", rr.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	gb2, _ := got["gateway_binding"].(map[string]any)
	if gb2["listen"] != "fake-listen" {
		t.Fatalf("get gateway_binding %+v", got["gateway_binding"])
	}
}

package control_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTenantCredentials_PutGetMasked(t *testing.T) {
	srv, _ := testServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/tenant/credentials/sarvam", bytes.NewBufferString(`{"api_key":"sk-test-secret-key"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put status %d %s", rr.Code, rr.Body.String())
	}
	var put map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &put)
	if put["api_key_set"] != true {
		t.Fatalf("put %+v", put)
	}
	if put["api_key_preview"] == "sk-test-secret-key" {
		t.Fatal("must not return raw key")
	}

	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/v1/tenant/credentials/sarvam", nil))
	if rr2.Code != http.StatusOK {
		t.Fatalf("get %d", rr2.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rr2.Body.Bytes(), &got)
	if got["api_key_set"] != true {
		t.Fatalf("get %+v", got)
	}

	rr3 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/v1/tenant/config", nil))
	if rr3.Code != http.StatusOK {
		t.Fatalf("config %d %s", rr3.Code, rr3.Body.String())
	}
}

func TestTenantSettings_PutGet(t *testing.T) {
	srv, _ := testServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/tenant/settings/coral.base_url", bytes.NewBufferString(`{"value":"https://coral.example"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put %d %s", rr.Code, rr.Body.String())
	}
	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/v1/tenant/settings/coral.base_url", nil))
	if rr2.Code != http.StatusOK {
		t.Fatalf("get %d", rr2.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rr2.Body.Bytes(), &got)
	if got["value"] != "https://coral.example" {
		t.Fatalf("%+v", got)
	}
}

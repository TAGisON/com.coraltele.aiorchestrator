package control_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBindings_CRUD(t *testing.T) {
	srv, _ := testServer(t)

	body := `{
  "tenant_id":"default",
  "kind":"knowledge",
  "name":"Lab FAQ",
  "status":"active",
  "config":{
    "mode":"inline_faq",
    "entries":[{"id":"hours","questions":["hours","open"],"text":{"en-IN":"Nine to five."}}]
  }
}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/bindings/lab-faq", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put status %d body %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/bindings/lab-faq", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get status %d", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["id"] != "lab-faq" || got["kind"] != "knowledge" {
		t.Fatalf("got %+v", got)
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/bindings?tenant_id=default&kind=knowledge", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("list status %d", rr.Code)
	}
	var list struct {
		Bindings []map[string]any `json:"bindings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Bindings) < 1 {
		t.Fatal("expected bindings")
	}
}

func TestBindings_RejectBadKind(t *testing.T) {
	srv, _ := testServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/bindings/x", bytes.NewBufferString(`{
  "kind":"desk","name":"no","config":{}
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestBindings_RejectAPIKeyInConfig(t *testing.T) {
	srv, _ := testServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/bindings/bad", bytes.NewBufferString(`{
  "kind":"knowledge","name":"bad","config":{"mode":"inline_faq","api_key":"secret"}
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

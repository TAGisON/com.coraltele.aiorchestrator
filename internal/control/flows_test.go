package control_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
)

func validFlowDoc() string {
	return `{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"welcome","type":"Speak","prompt_ref":"welcome"},
    {"id":"end","type":"End"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"welcome","kind":"next"},
    {"id":"e2","from":"welcome","to":"end","kind":"next"}
  ],
  "prompts":{"welcome":{"en-IN":"Hello"}},
  "matrix":[],
  "binding_refs":[]
}`
}

func createFlow(t *testing.T, srv *control.Server, id string) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/flows", bytes.NewBufferString(`{
  "id":"`+id+`","tenant_id":"t1","name":"Lab flow"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create flow status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestFlow_PublishGetVersion(t *testing.T) {
	srv, _ := testServer(t)
	createFlow(t, srv, "flow-lab")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/flows/flow-lab/draft", bytes.NewBufferString(validFlowDoc()))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put draft %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/flows/flow-lab/versions", bytes.NewBufferString(validFlowDoc()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Published-By", "ops")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish %d %s", rr.Code, rr.Body.String())
	}
	var pub struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &pub); err != nil {
		t.Fatal(err)
	}
	if pub.Version != 1 {
		t.Fatalf("version %d", pub.Version)
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/flows/flow-lab/versions/1", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get version %d %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Doc map[string]any `json:"doc"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Doc["schema_id"] != "coral.flow.v1" {
		t.Fatalf("doc %#v", got.Doc)
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/flows/flow-lab", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get flow %d", rr.Code)
	}
	var f struct {
		Status         string `json:"status"`
		CurrentVersion int    `json:"current_version"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &f); err != nil {
		t.Fatal(err)
	}
	if f.Status != "published" || f.CurrentVersion != 1 {
		t.Fatalf("flow %#v", f)
	}
}

func TestFlow_PublishBadSchema_422(t *testing.T) {
	srv, _ := testServer(t)
	createFlow(t, srv, "flow-bad")
	body := `{
  "schema_id":"x_desk",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[{"id":"entry","type":"Entry"}],
  "edges":[],
  "prompts":{},
  "matrix":[],
  "binding_refs":[]
}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/flows/flow-bad/versions", bytes.NewBufferString(body))
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
	if env.Error.Code != control.CodeFlowInvalid {
		t.Fatalf("code %q", env.Error.Code)
	}
}

package control_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
)

func TestAnswerPins_CRUD(t *testing.T) {
	srv, _ := testServer(t)
	createProfile(t, srv, "pin-prof")
	createAndPublishFlow(t, srv, "pin-flow")

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/tenant/answer-pins", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get empty status %d body %s", rr.Code, rr.Body.String())
	}

	body := `{"pins":[{"profile_id":"pin-prof","flow_id":"pin-flow","flow_version":"latest","did":"+9111","note":"lab"}]}`
	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/tenant/answer-pins", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put status %d body %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Pins []map[string]any `json:"pins"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Pins) != 1 || got.Pins[0]["profile_id"] != "pin-prof" {
		t.Fatalf("pins %+v", got.Pins)
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/tenant/answer-pins", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get status %d", rr.Code)
	}
}

func TestAnswerPins_RejectUnknownFlow(t *testing.T) {
	srv, _ := testServer(t)
	createProfile(t, srv, "pin-prof2")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/tenant/answer-pins", bytes.NewBufferString(
		`{"pins":[{"profile_id":"pin-prof2","flow_id":"missing","flow_version":"latest"}]}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

func createAndPublishFlow(t *testing.T, srv *control.Server, id string) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/flows", bytes.NewBufferString(`{
  "id":"`+id+`","tenant_id":"default","name":"`+id+`"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create flow %d %s", rr.Code, rr.Body.String())
	}
	doc := `{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[{"id":"entry","type":"Entry"},{"id":"end","type":"End"}],
  "edges":[{"id":"e1","from":"entry","to":"end","kind":"next"}],
  "prompts":{},
  "matrix":[],
  "binding_refs":[]
}`
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/flows/"+id+"/versions", bytes.NewBufferString(doc))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish flow %d %s", rr.Code, rr.Body.String())
	}
}

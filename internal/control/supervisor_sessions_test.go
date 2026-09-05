package control_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestSupervisor_SessionListAndRecordingMeta(t *testing.T) {
	srv, mem := testServer(t)
	createProfile(t, srv, "s1-prof")
	publishOK(t, srv, "s1-prof", `{
  "id":"s1-prof",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)
	createFlow(t, srv, "s1-flow")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/flows/s1-flow/versions", bytes.NewBufferString(`{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[{"id":"entry","type":"Entry"},{"id":"end","type":"End"}],
  "edges":[{"id":"e1","from":"entry","to":"end","kind":"next"}],
  "prompts":{},
  "matrix":[],
  "binding_refs":[]
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"s1-prof","profile_version":"latest",
  "flow_id":"s1-flow","flow_version":"latest","clock":"playback"
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

	if _, err := mem.MarkSessionRecordingStarted(t.Context(), sid, "/tmp/s1-lab.wav"); err != nil {
		t.Fatal(err)
	}
	nbytes := int64(42)
	if _, err := mem.MarkSessionRecordingStopped(t.Context(), sid, store.RecordingStopSessionCompleted, &nbytes); err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("list %d %s", rr.Code, rr.Body.String())
	}
	var list struct {
		Sessions []map[string]any `json:"sessions"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &list)
	var found bool
	for _, s := range list.Sessions {
		if s["session_id"] == sid {
			found = true
			if s["flow_id"] != "s1-flow" {
				t.Fatalf("list flow_id %#v", s["flow_id"])
			}
			if s["recording_ref"] != "/tmp/s1-lab.wav" {
				t.Fatalf("list recording_ref %#v", s["recording_ref"])
			}
		}
	}
	if !found {
		t.Fatal("session missing from list")
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sid, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get %d %s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got["recording_ref"] != "/tmp/s1-lab.wav" {
		t.Fatalf("recording_ref %#v", got["recording_ref"])
	}
	if got["recording_started_at"] == nil || got["recording_stopped_at"] == nil {
		t.Fatalf("want recording stamps %#v", got)
	}
	if got["recording_stop_reason"] == "" {
		t.Fatalf("stop_reason %#v", got["recording_stop_reason"])
	}
	if got["recording_bytes"] != float64(42) {
		t.Fatalf("bytes %#v", got["recording_bytes"])
	}
}

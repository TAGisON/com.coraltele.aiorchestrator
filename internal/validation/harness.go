// Package validation is the Validation V1 in-process harness (Control + memory + fakes).
// Tier A always runs; Tier B skips when SARVAM_API_KEY / secrets are absent.
package validation

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

type harness struct {
	Repo *store.Memory
	Reg  *router.MemRegistry
	Mgr  *session.Manager
	Srv  *control.Server
	RT   *control.SessionRuntime
}

func newHarness(t *testing.T, owner string) *harness {
	t.Helper()
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	for _, tid := range []string{"default", "t1"} {
		if _, err := mem.UpsertTenantEngines(context.Background(), store.TenantEngines{
			TenantID: tid,
			ListenID: "fake-listen",
			ThinkID:  "fake-think",
			SpeakID:  "fake-speak",
		}); err != nil {
			t.Fatal(err)
		}
	}
	mgr := session.NewManager(reg)
	rt := &control.SessionRuntime{Mgr: mgr, Repo: mem}
	srv := control.NewWithRuntime(mem, reg, rt, control.Config{OwnerInstance: owner}, nil)
	return &harness{Repo: mem, Reg: reg, Mgr: mgr, Srv: srv, RT: rt}
}

func (h *harness) createProfile(t *testing.T, id string) {
	t.Helper()
	rr := httptest.NewRecorder()
	body := `{"id":"` + id + `","display_name":"validation-v1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/profiles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	h.Srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create profile %d %s", rr.Code, rr.Body.String())
	}
}

func (h *harness) publish(t *testing.T, id, doc string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/profiles/"+id+"/versions", bytes.NewBufferString(doc))
	req.Header.Set("Content-Type", "application/json")
	h.Srv.Handler().ServeHTTP(rr, req)
	return rr
}

func (h *harness) publishOK(t *testing.T, id, doc string) {
	t.Helper()
	rr := h.publish(t, id, doc)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish %d %s", rr.Code, rr.Body.String())
	}
}

func (h *harness) doJSON(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	}
	h.Srv.Handler().ServeHTTP(rr, req)
	return rr
}

func jsonField(t *testing.T, body []byte, key string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	v, _ := m[key].(string)
	return v
}

func hasAudit(events []store.AuditEvent, typ string) bool {
	for _, e := range events {
		if e.EventType == typ {
			return true
		}
	}
	return false
}

func hasMetric(events []store.AnalyticsEvent, metric string) bool {
	for _, e := range events {
		if e.Metric == metric {
			return true
		}
	}
	return false
}

const fakeTalkProfile = `{
  "id":"%s",
  "modes":{"listen":true,"speak":true,"think":true,"talk":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  },
  "templates":{"disposition":{"id":"cc-disposition-v1"}},
  "analytics":{"emit":["containment","handoff"]}
}`

// sarvamKey returns a non-empty key from env or .agent/secrets.local.json (never logs the value).
func sarvamKey() string {
	if k := os.Getenv("SARVAM_API_KEY"); k != "" {
		return k
	}
	// Walk up from cwd to find repo .agent/secrets.local.json
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for i := 0; i < 6; i++ {
		p := filepath.Join(dir, ".agent", "secrets.local.json")
		b, err := os.ReadFile(p)
		if err == nil {
			var m map[string]any
			if json.Unmarshal(b, &m) == nil {
				if k, ok := m["SARVAM_API_KEY"].(string); ok && k != "" {
					return k
				}
				if k, ok := m["sarvam_api_key"].(string); ok && k != "" {
					return k
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

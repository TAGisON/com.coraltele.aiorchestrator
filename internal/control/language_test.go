package control_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestPatchProfileFields_LanguagePrimary(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	ctx := context.Background()
	seedFakeTenantEngines(t, mem)
	_ = mem.CreateProfile(ctx, store.Profile{ID: "p-lang"})
	doc := []byte(`{
  "id":"p-lang",
  "modes":{"listen":true,"speak":true,"think":true,"talk":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "language":{"auto_detect":true,"mid_call_switch":true},
  "hot_swap_allowed":["language.primary"],
  "persona":{"voice":{"fake-speak":"lab-voice"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)
	_, err := mem.PublishVersion(ctx, "p-lang", doc)
	if err != nil {
		t.Fatal(err)
	}
	mgr := session.NewManager(reg)
	srv := control.NewWithRuntime(mem, reg, &control.SessionRuntime{Mgr: mgr, Repo: mem}, control.Config{OwnerInstance: "lang-test"}, nil)

	create := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"p-lang","clock":"playback"
}`))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, create)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	sid, _ := created["session_id"].(string)
	if sid == "" {
		t.Fatal("no session_id")
	}

	// Lock via actor first confident final
	a, ok := mgr.Get(sid)
	if !ok {
		t.Fatal("actor missing")
	}
	a.OnListenFinal(port.ListenFinal{Text: "hello", Language: "hi-IN", Confidence: 0.95})

	get := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sid, nil)
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, get)
	if rr.Code != http.StatusOK {
		t.Fatalf("get %d", rr.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got["active_language"] != "hi-IN" || got["detected_language"] != "hi-IN" {
		t.Fatalf("get languages %+v", got)
	}

	patch := httptest.NewRequest(http.MethodPatch, "/v1/sessions/"+sid+"/profile-fields",
		bytes.NewBufferString(`{"language.primary":"en-IN"}`))
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, patch)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch %d %s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got["active_language"] != "en-IN" {
		t.Fatalf("patch active %+v", got)
	}
	if a.DetectedLanguage() != "hi-IN" || a.ActiveLanguage() != "en-IN" {
		t.Fatalf("actor det=%q act=%q", a.DetectedLanguage(), a.ActiveLanguage())
	}

	// ambient cannot flip after PATCH
	a.OnListenFinal(port.ListenFinal{Language: "ta-IN", Confidence: 1})
	if a.ActiveLanguage() != "en-IN" {
		t.Fatal("ambient flipped after patch")
	}
}

func TestPatchProfileFields_RejectsDisallowedAndNoMidCall(t *testing.T) {
	reg := router.NewMemRegistry()
	_ = fake.RegisterAll(reg)
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	ctx := context.Background()
	seedFakeTenantEngines(t, mem)
	_ = mem.CreateProfile(ctx, store.Profile{ID: "p2"})
	_, _ = mem.PublishVersion(ctx, "p2", []byte(`{
  "id":"p2",
  "modes":{"listen":true},
  "language":{"mid_call_switch":false},
  "hot_swap_allowed":["language.primary"],
  "routers":{"listen":{"providers":["fake-listen"]}}
}`))
	mgr := session.NewManager(reg)
	srv := control.NewWithRuntime(mem, reg, &control.SessionRuntime{Mgr: mgr, Repo: mem}, control.Config{}, nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{"profile_id":"p2","clock":"playback"}`)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	sid, _ := created["session_id"].(string)
	if sid == "" {
		t.Fatal("no session_id")
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/v1/sessions/"+sid+"/profile-fields",
		bytes.NewBufferString(`{"language.primary":"hi-IN"}`)))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 mid_call got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/v1/sessions/"+sid+"/profile-fields",
		bytes.NewBufferString(`{"money.max":"1"}`)))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 disallowed got %d", rr.Code)
	}
}

func TestMemory_SessionLanguages(t *testing.T) {
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	ctx := context.Background()
	_ = mem.CreateProfile(ctx, store.Profile{ID: "p1"})
	_, _ = mem.PublishVersion(ctx, "p1", []byte(`{"id":"p1","modes":{"listen":true}}`))
	_ = mem.CreateSession(ctx, store.Session{ID: "s1", ProfileID: "p1", ProfileVersion: 1, Clock: "live", State: store.StateCreated})
	updated, err := mem.UpdateSessionLanguages(ctx, "s1", "hi-IN", "hi-IN")
	if err != nil {
		t.Fatal(err)
	}
	if updated.DetectedLanguage != "hi-IN" || updated.ActiveLanguage != "hi-IN" {
		t.Fatalf("%+v", updated)
	}
	got, _ := mem.GetSession(ctx, "s1")
	if got.ActiveLanguage != "hi-IN" {
		t.Fatalf("%+v", got)
	}
}

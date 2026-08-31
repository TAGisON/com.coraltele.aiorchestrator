package control_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/coraltransfer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestInject_ClipTurnAndTranscript(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	if err := coraltransfer.Register(reg, nil); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	mgr := session.NewManager(reg)
	srv := control.NewWithRuntime(mem, reg, &control.SessionRuntime{Mgr: mgr, Repo: mem}, control.Config{OwnerInstance: "inj"}, nil)

	createProfile(t, srv, "cc-sales")
	publishOK(t, srv, "cc-sales", `{
  "id":"cc-sales",
  "metadata":{"family":"contact-agent","display_name":"CC Sales"},
  "modes":{"listen":true,"speak":true,"think":true,"talk":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "language":{"behaviour":"none","auto_detect":true,"mid_call_switch":true},
  "hot_swap_allowed":["language.primary"],
  "persona":{"name":"Sales","voice":{"fake-speak":"lab-voice-sales"}},
  "response":{
    "ladder":["clip","template","llm"],
    "clips":{
      "clip-apology-en":{"text":"Sorry — connecting you."},
      "greeting-en":{"text":"Hi — Sales desk.","when":{"regex":"(?i)\\b(hi|hello|hey)\\b"}}
    }
  },
  "fallback":{
    "think_down":{"speak_canned":"clip-apology-en","skill":"warm_transfer"}
  },
  "skills":{
    "allowed":["warm_transfer"],
    "definitions":{"warm_transfer":{"gateway":"coral-transfer","authority":"act","confirm":true}}
  }
}`)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"cc-sales","profile_version":"latest","clock":"live"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	sid := created["session_id"].(string)
	gb, _ := created["gateway_binding"].(map[string]any)
	if gb == nil || gb["listen"] != "fake-listen" || gb["think"] != "fake-think" || gb["speak"] != "fake-speak" {
		t.Fatalf("gateway_binding %+v", created["gateway_binding"])
	}

	rr = httptest.NewRecorder()
	inj := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sid+"/inject", bytes.NewBufferString(`{"text":"hello","speak":true}`))
	inj.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, inj)
	if rr.Code != http.StatusOK {
		t.Fatalf("inject %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sid+"/transcript", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("transcript %d %s", rr.Code, rr.Body.String())
	}
	var tr struct {
		Turns []struct {
			Role string `json:"role"`
			Text string `json:"text"`
		} `json:"turns"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &tr); err != nil {
		t.Fatal(err)
	}
	if len(tr.Turns) < 2 {
		t.Fatalf("want ≥2 transcript turns, got %#v", tr.Turns)
	}
	if tr.Turns[0].Role != "user" || tr.Turns[0].Text != "hello" {
		t.Fatalf("user turn %#v", tr.Turns[0])
	}
	if tr.Turns[1].Role != "assistant" || tr.Turns[1].Text == "" {
		t.Fatalf("assistant turn %#v", tr.Turns[1])
	}
}

func TestAnswer_GreetingBeforeInject(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	if err := coraltransfer.Register(reg, nil); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	seedFakeTenantEngines(t, mem)
	mgr := session.NewManager(reg)
	srv := control.NewWithRuntime(mem, reg, &control.SessionRuntime{Mgr: mgr, Repo: mem}, control.Config{OwnerInstance: "ans"}, nil)

	createProfile(t, srv, "cc-answer")
	publishOK(t, srv, "cc-answer", `{
  "id":"cc-answer",
  "metadata":{"family":"contact-agent","display_name":"CC Answer"},
  "modes":{"listen":true,"speak":true,"think":true,"talk":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "language":{"behaviour":"none","auto_detect":true,"mid_call_switch":true},
  "hot_swap_allowed":["language.primary"],
  "persona":{"name":"Assist","voice":{"fake-speak":"lab-voice"}},
  "response":{
    "ladder":["clip","llm"],
    "clips":{
      "greeting-en":{"text":"Namaste — Coral Assist.","when":{"regex":"(?i)hi"}}
    }
  },
  "fallback":{"think_down":{"speak_canned":"greeting-en"}}
}`)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"cc-answer","profile_version":"latest","clock":"live"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	sid := created["session_id"].(string)

	rr = httptest.NewRecorder()
	ans := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sid+"/answer", bytes.NewBufferString(`{}`))
	ans.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, ans)
	if rr.Code != http.StatusOK {
		t.Fatalf("answer %d %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["spoken"] != "Namaste — Coral Assist." {
		t.Fatalf("spoken %#v", body["spoken"])
	}

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sid+"/transcript", nil))
	var tr struct {
		Turns []struct {
			Role string `json:"role"`
			Text string `json:"text"`
		} `json:"turns"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &tr)
	if len(tr.Turns) != 1 || tr.Turns[0].Role != "assistant" {
		t.Fatalf("want opening assistant only, got %#v", tr.Turns)
	}
}


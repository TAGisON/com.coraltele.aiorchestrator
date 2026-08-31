package control_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/composer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/observe"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestCC5_GetTranscriptOrdered(t *testing.T) {
	setFakeSystemEngines(t)
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	mgr := session.NewManager(reg)
	srv := control.NewWithRuntime(mem, reg, &control.SessionRuntime{Mgr: mgr}, control.Config{OwnerInstance: "cc5"}, nil)
	createProfile(t, srv, "cc5-lab")
	publishOK(t, srv, "cc5-lab", `{
  "id":"cc5-lab",
  "modes":{"listen":true,"speak":true,"think":true,"talk":true},
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
  "profile_id":"cc5-lab","profile_version":"latest","clock":"live","tenant_id":"t1"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	sid := created["session_id"].(string)

	_, _ = mem.AppendTranscriptTurn(context.Background(), store.TranscriptTurn{
		SessionID: sid, Role: store.RoleUser, Text: "first", TurnID: "t1",
	})
	_, _ = mem.AppendTranscriptTurn(context.Background(), store.TranscriptTurn{
		SessionID: sid, Role: store.RoleAssistant, Text: "reply", TurnID: "t1",
	})

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sid+"/transcript", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("transcript %d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		SessionID string `json:"session_id"`
		Turns     []struct {
			Seq    int    `json:"seq"`
			TurnID string `json:"turn_id"`
			Role   string `json:"role"`
			Text   string `json:"text"`
		} `json:"turns"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.SessionID != sid || len(body.Turns) != 2 {
		t.Fatalf("body %#v", body)
	}
	if body.Turns[0].Seq != 1 || body.Turns[0].Role != "user" || body.Turns[1].Role != "assistant" {
		t.Fatalf("order %#v", body.Turns)
	}
	if body.Turns[0].TurnID != "t1" || body.Turns[1].TurnID != "t1" {
		t.Fatalf("shared turn_id %#v", body.Turns)
	}
}

func TestCC5_PostcallDispositionGET(t *testing.T) {
	setFakeSystemEngines(t)
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	mgr := session.NewManager(reg)
	srv := control.NewWithRuntime(mem, reg, &control.SessionRuntime{Mgr: mgr}, control.Config{OwnerInstance: "cc5-disp"}, nil)
	createProfile(t, srv, "cc5-disp")
	publishOK(t, srv, "cc5-disp", `{
  "id":"cc5-disp",
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
}`)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"cc5-disp","profile_version":"latest","clock":"live","tenant_id":"t1"
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
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sid+"/disposition", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404 before postcall, got %d %s", rr.Code, rr.Body.String())
	}

	actor, ok := mgr.Get(sid)
	if !ok {
		t.Fatal("actor missing")
	}
	talk, err := composer.NewTalk(actor.Profile, reg, actor.Bus, actor.Memory, "live", port.SessionID(sid))
	if err != nil {
		t.Fatal(err)
	}
	talk.Obs = &observe.Observer{Repo: mem, Meta: observe.SessionMeta{
		SessionID: sid, TenantID: "t1", ProfileID: "cc5-disp", ProfileVersion: 1, Clock: "live",
	}}
	if err := talk.InjectFinal(context.Background(), "hello disposition"); err != nil {
		t.Fatal(err)
	}

	turns, _ := mem.ListTranscriptTurns(context.Background(), sid)
	if len(turns) < 2 {
		t.Fatalf("want durable transcript after turn, got %#v", turns)
	}
	audits, _ := mem.ListAuditEvents(context.Background(), sid)
	foundTurnID := false
	for _, a := range audits {
		if a.EventType != store.AuditTurnCompleted {
			continue
		}
		var p map[string]any
		_ = json.Unmarshal(a.Payload, &p)
		if tid, _ := p["turn_id"].(string); tid != "" && tid == turns[0].TurnID {
			foundTurnID = true
		}
	}
	if !foundTurnID {
		t.Fatalf("want turn.completed turn_id matching transcript; audits=%#v turns=%#v", audits, turns)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = srv.StartPostcallWorker(ctx)

	rr = httptest.NewRecorder()
	stopReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sid+"/stop", bytes.NewBufferString(`{}`))
	stopReq.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, stopReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("stop %d %s", rr.Code, rr.Body.String())
	}

	deadline := time.Now().Add(3 * time.Second)
	var dispOK bool
	for time.Now().Before(deadline) {
		rr = httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sid+"/disposition", nil))
		if rr.Code == http.StatusOK {
			var d map[string]any
			_ = json.Unmarshal(rr.Body.Bytes(), &d)
			if d["suggestion"] == nil || d["source"] != "postcall_worker" {
				t.Fatalf("disposition body %#v", d)
			}
			if d["template_id"] != "cc-disposition-v1" {
				t.Fatalf("template %#v", d)
			}
			dispOK = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !dispOK {
		t.Fatal("disposition GET never returned 200 after postcall")
	}
}

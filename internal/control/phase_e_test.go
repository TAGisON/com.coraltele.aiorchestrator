package control_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/token"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/bus"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/composer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/observe"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestPhaseE_SessionTerminalAuditAnalyticsPostcall(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	mgr := session.NewManager(reg)
	srv := control.NewWithRuntime(mem, reg, &control.SessionRuntime{Mgr: mgr}, control.Config{OwnerInstance: "e-worker"})
	createProfile(t, srv, "e-lab")
	publishOK(t, srv, "e-lab", `{
  "id":"e-lab",
  "modes":{"listen":true,"speak":true,"think":true,"talk":true},
  "audio":{"canonical_sample_rate_hz":16000},
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
  "profile_id":"e-lab","profile_version":"latest","clock":"live","tenant_id":"t1"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	sid := created["session_id"].(string)

	audits, _ := mem.ListAuditEvents(context.Background(), sid)
	foundStart := false
	for _, a := range audits {
		if a.EventType == store.AuditSessionStarted {
			foundStart = true
		}
	}
	if !foundStart {
		t.Fatalf("want session.started audit %#v", audits)
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
		SessionID: sid, TenantID: "t1", ProfileID: "e-lab", ProfileVersion: 1, Clock: "live",
	}}
	if err := talk.InjectFinal(context.Background(), "hello phase e"); err != nil {
		t.Fatal(err)
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

	audits, _ = mem.ListAuditEvents(context.Background(), sid)
	foundTerm, foundTurn := false, false
	for _, a := range audits {
		if a.EventType == store.AuditSessionTerminal {
			foundTerm = true
		}
		if a.EventType == store.AuditTurnCompleted {
			foundTurn = true
		}
	}
	if !foundTerm || !foundTurn {
		t.Fatalf("term=%v turn=%v audits=%#v", foundTerm, foundTurn, audits)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		audits, _ = mem.ListAuditEvents(context.Background(), sid)
		for _, a := range audits {
			if a.EventType == store.AuditDisposition {
				return
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatal("disposition audit not written")
}

func TestPhaseE_SSE_SessionStateAndTurn(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	mgr := session.NewManager(reg)
	srv := control.NewWithRuntime(mem, reg, &control.SessionRuntime{Mgr: mgr}, control.Config{})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	createProfile(t, srv, "sse-lab")
	publishOK(t, srv, "sse-lab", `{
  "id":"sse-lab",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"sse-lab","profile_version":"latest","clock":"live"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	sid := jsonString(t, rr.Body.Bytes(), "session_id")

	actor, ok := mgr.Get(sid)
	if !ok {
		t.Fatal("no actor")
	}

	resp, err := http.Get(ts.URL + "/v1/sessions/" + sid + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sse status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type %q", ct)
	}

	go func() {
		time.Sleep(30 * time.Millisecond)
		actor.Bus.PublishEvent(bus.Event{Kind: "turn.completed", Data: map[string]any{"outcome": "allow"}})
	}()

	gotState, gotTurn := false, false
	sc := bufio.NewScanner(resp.Body)
	deadline := time.After(2 * time.Second)
	lineCh := make(chan string, 32)
	go func() {
		for sc.Scan() {
			lineCh <- sc.Text()
		}
		close(lineCh)
	}()
	for !(gotState && gotTurn) {
		select {
		case <-deadline:
			t.Fatalf("gotState=%v gotTurn=%v", gotState, gotTurn)
		case line, ok := <-lineCh:
			if !ok {
				t.Fatalf("stream ended gotState=%v gotTurn=%v", gotState, gotTurn)
			}
			if strings.HasPrefix(line, "event: session.state") {
				gotState = true
			}
			if strings.HasPrefix(line, "event: turn.completed") {
				gotTurn = true
			}
		}
	}
}

func TestPhaseE_EdgeGoneTerminalAuditPostcall(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	mgr := session.NewManager(reg)
	srv := control.NewWithRuntime(mem, reg, &control.SessionRuntime{Mgr: mgr}, control.Config{OwnerInstance: "e-edge"})
	createProfile(t, srv, "edge-gone-lab")
	publishOK(t, srv, "edge-gone-lab", `{
  "id":"edge-gone-lab",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  },
  "templates":{"disposition":{"id":"cc-disposition-v1"}}
}`)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"edge-gone-lab","profile_version":"latest","clock":"live","tenant_id":"t1"
}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	sid := jsonString(t, rr.Body.Bytes(), "session_id")

	binder := srv.NewEdgeBinder(mgr)
	_, _, onGone, err := binder.BindEdge(token.Claims{SessionID: sid, TenantID: "t1"}, 16000)
	if err != nil {
		t.Fatal(err)
	}
	if onGone == nil {
		t.Fatal("expected onGone")
	}
	onGone()

	sess, err := mem.GetSession(context.Background(), sid)
	if err != nil {
		t.Fatal(err)
	}
	if sess.State != store.StateCancelled {
		t.Fatalf("state=%s want Cancelled", sess.State)
	}

	audits, _ := mem.ListAuditEvents(context.Background(), sid)
	foundTerm := false
	for _, a := range audits {
		if a.EventType == store.AuditSessionTerminal {
			foundTerm = true
		}
	}
	if !foundTerm {
		t.Fatalf("want session.terminal audit %#v", audits)
	}

	ams, _ := mem.ListAnalyticsEvents(context.Background(), sid)
	foundCompleted := false
	for _, a := range ams {
		if a.Metric == store.MetricSessionCompleted {
			foundCompleted = true
		}
	}
	if !foundCompleted {
		t.Fatalf("want session_completed analytics %#v", ams)
	}

	job, err := mem.LeaseNextPostcallJob(context.Background(), "assert-edge")
	if err != nil {
		t.Fatalf("postcall not enqueued: %v", err)
	}
	if job.SessionID != sid || job.State != store.JobRunning {
		t.Fatalf("unexpected job %#v", job)
	}
}

func TestPhaseE_PostcallLeaseDisposition(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	srv := control.New(mem, reg, control.Config{OwnerInstance: "test-worker"})
	createProfile(t, srv, "pc-lab")
	publishOK(t, srv, "pc-lab", `{
  "id":"pc-lab",
  "modes":{"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "routers":{"think":{"providers":["fake-think"]}},
  "templates":{"disposition":{"id":"cc-disposition-v1"}}
}`)
	_ = mem.CreateSession(context.Background(), store.Session{
		ID: "sess-pc", ProfileID: "pc-lab", ProfileVersion: 1,
		Clock: "live", State: store.StateCompleted, CanonicalSampleRateHz: 16000,
	})
	if err := mem.CreatePostcallJob(context.Background(), store.PostcallJob{
		ID: "job-pc", SessionID: "sess-pc", ProfileID: "pc-lab", ProfileVersion: 1, State: store.JobQueued,
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = srv.StartPostcallWorker(ctx)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := mem.GetPostcallJob(context.Background(), "job-pc")
		if err == nil && job.State == store.JobCompleted {
			audits, _ := mem.ListAuditEvents(context.Background(), "sess-pc")
			for _, a := range audits {
				if a.EventType == store.AuditDisposition {
					return
				}
			}
			t.Fatal("job completed but no disposition audit")
		}
		time.Sleep(40 * time.Millisecond)
	}
	job, _ := mem.GetPostcallJob(context.Background(), "job-pc")
	t.Fatalf("postcall not completed: %#v", job)
}

func jsonString(t *testing.T, body []byte, key string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	s, _ := m[key].(string)
	if s == "" {
		t.Fatalf("missing %s in %s", key, body)
	}
	return s
}

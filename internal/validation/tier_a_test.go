package validation

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/token"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/bus"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/composer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/observe"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// TestValidationV1_TierA covers scenarios V1-A01 … V1-A08 (Control + memory + fakes).
func TestValidationV1_TierA(t *testing.T) {
	t.Run("V1-A01_health", func(t *testing.T) {
		h := newHarness(t, "v1-a01")
		rr := h.doJSON(t, http.MethodGet, "/v1/health", "")
		if rr.Code != http.StatusOK {
			t.Fatalf("health %d %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("V1-A02_profile_lifecycle", func(t *testing.T) {
		h := newHarness(t, "v1-a02")
		h.createProfile(t, "v1-prof")
		h.publishOK(t, "v1-prof", fmt.Sprintf(fakeTalkProfile, "v1-prof"))

		bad := `{
  "id":"v1-prof",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice"}},
  "routers":{
    "listen":{"providers":["not-a-real-gateway"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`
		rr := h.publish(t, "v1-prof", bad)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422 for bad gateway, got %d %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("V1-A03_session_lifecycle", func(t *testing.T) {
		h := newHarness(t, "v1-a03")
		h.createProfile(t, "v1-sess")
		h.publishOK(t, "v1-sess", fmt.Sprintf(fakeTalkProfile, "v1-sess"))

		rr := h.doJSON(t, http.MethodPost, "/v1/sessions", `{
  "profile_id":"v1-sess","profile_version":"latest","clock":"live","tenant_id":"t1"
}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create %d %s", rr.Code, rr.Body.String())
		}
		sid := jsonField(t, rr.Body.Bytes(), "session_id")
		if sid == "" {
			t.Fatal("missing session_id")
		}

		rr = h.doJSON(t, http.MethodGet, "/v1/sessions/"+sid, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("get %d %s", rr.Code, rr.Body.String())
		}
		var got map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &got)
		state, _ := got["state"].(string)
		if state == "" {
			t.Fatalf("missing state: %s", rr.Body.String())
		}

		rr = h.doJSON(t, http.MethodPost, "/v1/sessions/"+sid+"/stop", `{}`)
		if rr.Code != http.StatusOK {
			t.Fatalf("stop %d %s", rr.Code, rr.Body.String())
		}
		sess, err := h.Repo.GetSession(context.Background(), sid)
		if err != nil {
			t.Fatal(err)
		}
		if sess.State != store.StateCompleted && sess.State != store.StateCancelled {
			t.Fatalf("terminal state=%s", sess.State)
		}
	})

	t.Run("V1-A04_audit_after_stop", func(t *testing.T) {
		h := newHarness(t, "v1-a04")
		h.createProfile(t, "v1-audit")
		h.publishOK(t, "v1-audit", fmt.Sprintf(fakeTalkProfile, "v1-audit"))

		rr := h.doJSON(t, http.MethodPost, "/v1/sessions", `{
  "profile_id":"v1-audit","profile_version":"latest","clock":"live","tenant_id":"t1"
}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create %d %s", rr.Code, rr.Body.String())
		}
		sid := jsonField(t, rr.Body.Bytes(), "session_id")

		actor, ok := h.Mgr.Get(sid)
		if !ok {
			t.Fatal("actor missing")
		}
		talk, err := composer.NewTalk(actor.Profile, h.Reg, actor.Bus, actor.Memory, "live", port.SessionID(sid))
		if err != nil {
			t.Fatal(err)
		}
		talk.Obs = &observe.Observer{Repo: h.Repo, Meta: observe.SessionMeta{
			SessionID: sid, TenantID: "t1", ProfileID: "v1-audit", ProfileVersion: 1, Clock: "live",
		}}
		if err := talk.InjectFinal(context.Background(), "hello validation"); err != nil {
			t.Fatal(err)
		}

		rr = h.doJSON(t, http.MethodPost, "/v1/sessions/"+sid+"/stop", `{}`)
		if rr.Code != http.StatusOK {
			t.Fatalf("stop %d %s", rr.Code, rr.Body.String())
		}

		audits, _ := h.Repo.ListAuditEvents(context.Background(), sid)
		if !hasAudit(audits, store.AuditSessionStarted) {
			t.Fatalf("missing session.started %#v", audits)
		}
		if !hasAudit(audits, store.AuditSessionTerminal) {
			t.Fatalf("missing session.terminal %#v", audits)
		}
		if !hasAudit(audits, store.AuditTurnCompleted) {
			t.Fatalf("missing turn.completed %#v", audits)
		}
	})

	t.Run("V1-A05_analytics", func(t *testing.T) {
		h := newHarness(t, "v1-a05")
		h.createProfile(t, "v1-an")
		h.publishOK(t, "v1-an", fmt.Sprintf(fakeTalkProfile, "v1-an"))

		rr := h.doJSON(t, http.MethodPost, "/v1/sessions", `{
  "profile_id":"v1-an","profile_version":"latest","clock":"live","tenant_id":"t1"
}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create %d %s", rr.Code, rr.Body.String())
		}
		sid := jsonField(t, rr.Body.Bytes(), "session_id")

		ams, _ := h.Repo.ListAnalyticsEvents(context.Background(), sid)
		if !hasMetric(ams, store.MetricSessionStarted) {
			t.Fatalf("missing session_started %#v", ams)
		}

		rr = h.doJSON(t, http.MethodPost, "/v1/sessions/"+sid+"/stop", `{}`)
		if rr.Code != http.StatusOK {
			t.Fatalf("stop %d %s", rr.Code, rr.Body.String())
		}
		ams, _ = h.Repo.ListAnalyticsEvents(context.Background(), sid)
		if !hasMetric(ams, store.MetricSessionCompleted) && !hasMetric(ams, store.MetricSessionFailed) {
			t.Fatalf("missing session_completed/failed %#v", ams)
		}
	})

	t.Run("V1-A06_sse_catalog", func(t *testing.T) {
		h := newHarness(t, "v1-a06")
		h.createProfile(t, "v1-sse")
		h.publishOK(t, "v1-sse", fmt.Sprintf(fakeTalkProfile, "v1-sse"))
		ts := httptest.NewServer(h.Srv.Handler())
		defer ts.Close()

		rr := h.doJSON(t, http.MethodPost, "/v1/sessions", `{
  "profile_id":"v1-sse","profile_version":"latest","clock":"live"
}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create %d %s", rr.Code, rr.Body.String())
		}
		sid := jsonField(t, rr.Body.Bytes(), "session_id")
		actor, ok := h.Mgr.Get(sid)
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
	})

	t.Run("V1-A07_edge_gone", func(t *testing.T) {
		h := newHarness(t, "v1-a07")
		h.createProfile(t, "v1-edge")
		h.publishOK(t, "v1-edge", fmt.Sprintf(fakeTalkProfile, "v1-edge"))

		rr := h.doJSON(t, http.MethodPost, "/v1/sessions", `{
  "profile_id":"v1-edge","profile_version":"latest","clock":"live","tenant_id":"t1"
}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create %d %s", rr.Code, rr.Body.String())
		}
		sid := jsonField(t, rr.Body.Bytes(), "session_id")

		binder := h.Srv.NewEdgeBinder(h.Mgr)
		_, _, onGone, err := binder.BindEdge(token.Claims{SessionID: sid, TenantID: "t1"}, 16000)
		if err != nil {
			t.Fatal(err)
		}
		if onGone == nil {
			t.Fatal("expected onGone")
		}
		onGone()

		sess, err := h.Repo.GetSession(context.Background(), sid)
		if err != nil {
			t.Fatal(err)
		}
		if sess.State != store.StateCancelled {
			t.Fatalf("state=%s want Cancelled", sess.State)
		}
		audits, _ := h.Repo.ListAuditEvents(context.Background(), sid)
		if !hasAudit(audits, store.AuditSessionTerminal) {
			t.Fatalf("want session.terminal %#v", audits)
		}
		ams, _ := h.Repo.ListAnalyticsEvents(context.Background(), sid)
		if !hasMetric(ams, store.MetricSessionCompleted) && !hasMetric(ams, store.MetricSessionFailed) {
			t.Fatalf("want session_completed/failed %#v", ams)
		}
		job, err := h.Repo.LeaseNextPostcallJob(context.Background(), "assert-edge")
		if err != nil {
			t.Fatalf("postcall not enqueued: %v", err)
		}
		if job.SessionID != sid {
			t.Fatalf("unexpected job %#v", job)
		}
	})

	t.Run("V1-A08_postcall_disposition", func(t *testing.T) {
		h := newHarness(t, "v1-a08")
		h.createProfile(t, "v1-pc")
		h.publishOK(t, "v1-pc", `{
  "id":"v1-pc",
  "modes":{"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "routers":{"think":{"providers":["fake-think"]}},
  "templates":{"disposition":{"id":"cc-disposition-v1"}}
}`)
		_ = h.Repo.CreateSession(context.Background(), store.Session{
			ID: "sess-v1-pc", ProfileID: "v1-pc", ProfileVersion: 1,
			Clock: "live", State: store.StateCompleted, CanonicalSampleRateHz: 16000,
		})
		if err := h.Repo.CreatePostcallJob(context.Background(), store.PostcallJob{
			ID: "job-v1-pc", SessionID: "sess-v1-pc", ProfileID: "v1-pc", ProfileVersion: 1, State: store.JobQueued,
		}); err != nil {
			t.Fatal(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_ = h.Srv.StartPostcallWorker(ctx)

		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			job, err := h.Repo.GetPostcallJob(context.Background(), "job-v1-pc")
			if err == nil && job.State == store.JobCompleted {
				audits, _ := h.Repo.ListAuditEvents(context.Background(), "sess-v1-pc")
				if hasAudit(audits, store.AuditDisposition) {
					return
				}
				t.Fatal("job completed but no disposition audit")
			}
			time.Sleep(40 * time.Millisecond)
		}
		job, _ := h.Repo.GetPostcallJob(context.Background(), "job-v1-pc")
		t.Fatalf("postcall not completed: %#v", job)
	})
}

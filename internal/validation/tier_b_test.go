package validation

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// TestValidationV1_TierB is env-gated (SARVAM_API_KEY or .agent/secrets.local.json).
// Without a key every subtest records skip — Tier A remains the CI gate.
func TestValidationV1_TierB(t *testing.T) {
	key := sarvamKey()
	if key == "" {
		t.Skip("V1-B*: SARVAM_API_KEY / secrets.local absent — Tier B skipped")
	}

	t.Run("V1-B01_key_present", func(t *testing.T) {
		if len(key) < 8 {
			t.Fatal("key too short")
		}
	})

	t.Run("V1-B02_failover_profile_publish", func(t *testing.T) {
		h := newHarness(t, "v1-b02")
		h.createProfile(t, "v1-sarvam")
		doc := `{
  "id":"v1-sarvam",
  "modes":{"listen":true,"speak":true,"think":true,"talk":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab-voice","sarvam-tts":"shubh"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`
		h.publishOK(t, "v1-sarvam", doc)
	})

	t.Run("V1-B03_talk_inject_fakes", func(t *testing.T) {
		h := newHarness(t, "v1-b03")
		h.createProfile(t, "v1-live")
		h.publishOK(t, "v1-live", fmt.Sprintf(fakeTalkProfile, "v1-live"))
		rr := h.doJSON(t, http.MethodPost, "/v1/sessions", `{
  "profile_id":"v1-live","profile_version":"latest","clock":"live","tenant_id":"t1"
}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create %d %s", rr.Code, rr.Body.String())
		}
		sid := jsonField(t, rr.Body.Bytes(), "session_id")
		rr = h.doJSON(t, http.MethodPost, "/v1/sessions/"+sid+"/inject", `{"text":"hello","speak":true}`)
		if rr.Code != http.StatusOK {
			t.Fatalf("inject %d %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("V1-B04_audit_after_inject", func(t *testing.T) {
		h := newHarness(t, "v1-b04")
		h.createProfile(t, "v1-b4")
		h.publishOK(t, "v1-b4", fmt.Sprintf(fakeTalkProfile, "v1-b4"))
		rr := h.doJSON(t, http.MethodPost, "/v1/sessions", `{
  "profile_id":"v1-b4","profile_version":"latest","clock":"live","tenant_id":"t1"
}`)
		sid := jsonField(t, rr.Body.Bytes(), "session_id")
		_ = h.doJSON(t, http.MethodPost, "/v1/sessions/"+sid+"/inject", `{"text":"ping","speak":true}`)
		_ = h.doJSON(t, http.MethodPost, "/v1/sessions/"+sid+"/stop", `{}`)
		audits, _ := h.Repo.ListAuditEvents(context.Background(), sid)
		if !hasAudit(audits, store.AuditSessionStarted) {
			t.Fatalf("missing session.started %#v", audits)
		}
	})

	t.Run("V1-B05_select_failover_to_fake", func(t *testing.T) {
		reg := router.NewMemRegistry()
		if err := fake.RegisterAll(reg); err != nil {
			t.Fatal(err)
		}
		_ = reg.Register(port.Registration{
			ID:           "decoy-listen",
			Port:         port.PortListen,
			Capabilities: port.Capability{Streaming: true, Batch: true},
			Instance:     &fake.Listen{},
			Probe: func(ctx context.Context) port.Health {
				return port.Health{Healthy: false, LastError: "forced down"}
			},
		})
		rec, err := router.Select(reg, []port.GatewayID{"decoy-listen", "fake-listen"}, port.PortListen, router.SelectOptions{Clock: "live"})
		if err != nil {
			t.Fatal(err)
		}
		if rec.ID != "fake-listen" {
			t.Fatalf("want fake-listen got %s", rec.ID)
		}
	})
}

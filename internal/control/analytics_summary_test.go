package control_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestAnalyticsSummary_LightCounts(t *testing.T) {
	srv, mem := testServer(t)
	createProfile(t, srv, "agg-prof")
	publishOK(t, srv, "agg-prof", `{
  "id":"agg-prof",
  "modes":{"listen":true,"speak":true,"think":true},
  "audio":{"canonical_sample_rate_hz":16000},
  "persona":{"voice":{"fake-speak":"lab"}},
  "routers":{
    "listen":{"providers":["fake-listen"]},
    "speak":{"providers":["fake-speak"]},
    "think":{"providers":["fake-think"]}
  }
}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(`{
  "profile_id":"agg-prof","profile_version":"latest","clock":"playback"
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

	_, _ = mem.UpdateSessionState(t.Context(), sid, store.StateCompleted)
	_, _ = mem.UpsertSessionDisposition(t.Context(), store.SessionDisposition{
		SessionID: sid, Final: store.DispositionFinalHangupCompleted, Source: store.DispositionSourceLiveGraph,
	})
	_, _ = mem.AppendAnalyticsEvent(t.Context(), store.AnalyticsEvent{
		SessionID: sid, Metric: store.MetricContained, Value: 1,
	})
	_, _ = mem.AppendAnalyticsEvent(t.Context(), store.AnalyticsEvent{
		SessionID: sid, Metric: store.MetricSessionCompleted, Value: 1,
	})

	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/analytics/summary?limit=50", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("summary %d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		SessionsTotal int                `json:"sessions_total"`
		ByState       map[string]int     `json:"by_state"`
		ByClock       map[string]int     `json:"by_clock"`
		DispFinal     map[string]int     `json:"disposition_final"`
		Metrics       map[string]float64 `json:"metrics"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.SessionsTotal < 1 {
		t.Fatalf("sessions_total %d", body.SessionsTotal)
	}
	if body.ByClock["playback"] < 1 {
		t.Fatalf("by_clock %#v", body.ByClock)
	}
	if body.ByState[store.StateCompleted] < 1 {
		t.Fatalf("by_state %#v", body.ByState)
	}
	if body.DispFinal[store.DispositionFinalHangupCompleted] < 1 {
		t.Fatalf("disposition %#v", body.DispFinal)
	}
	if body.Metrics[store.MetricContained] < 1 || body.Metrics[store.MetricSessionCompleted] < 1 {
		t.Fatalf("metrics %#v", body.Metrics)
	}
}

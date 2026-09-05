package control

import (
	"net/http"
	"strconv"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// lightMetricAllow is the closed set summed into GET /v1/analytics/summary (OD-13-8).
var lightMetricAllow = map[string]bool{
	store.MetricSessionStarted:   true,
	store.MetricSessionCompleted: true,
	store.MetricSessionFailed:    true,
	store.MetricHandoff:          true,
	store.MetricContained:        true,
	store.MetricTurnCompleted:    true,
}

func (s *Server) handleAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}
	list, err := s.repo.ListSessions(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list sessions failed", nil)
		return
	}
	byState := map[string]int{}
	byClock := map[string]int{}
	withRec := 0
	dispFinal := map[string]int{}
	metrics := map[string]float64{}

	for _, sess := range list {
		byState[sess.State]++
		clock := sess.Clock
		if clock == "" {
			clock = "(unset)"
		}
		byClock[clock]++
		if sess.RecordingRef != "" {
			withRec++
		}
		if d, err := s.repo.GetSessionDisposition(r.Context(), sess.ID); err == nil && d.Final != "" {
			dispFinal[d.Final]++
		}
		if evs, err := s.repo.ListAnalyticsEvents(r.Context(), sess.ID); err == nil {
			for _, ev := range evs {
				if !lightMetricAllow[ev.Metric] {
					continue
				}
				metrics[ev.Metric] += ev.Value
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"limit":              limit,
		"sessions_total":     len(list),
		"by_state":           byState,
		"by_clock":           byClock,
		"with_recording":     withRec,
		"disposition_final":  dispFinal,
		"metrics":            metrics,
		"note":               "light aggregate over recent sessions (OD-13-8); not a QM dashboard",
	})
}

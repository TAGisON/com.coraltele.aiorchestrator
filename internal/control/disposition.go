package control

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// V1 disposition codes (docs/phases/P2.6_disposition.md). Closed vocabulary.
var dispositionAllowlist = []string{
	"transferred_sales",
	"transferred_corporate",
	"transferred_support",
	"transferred_other",
	"hangup_completed",
	"hangup_silence",
	"hangup_abuse",
	"out_of_scope",
	"abandoned_caller",
	"system_failure",
}

func (s *Server) handlePatchDisposition(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Final string `json:"final"`
		Actor string `json:"actor"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	final := strings.TrimSpace(req.Final)
	if final == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "final required", nil)
		return
	}
	valid := false
	for _, d := range dispositionAllowlist {
		if d == final {
			valid = true
			break
		}
	}
	if !valid {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "unknown disposition code",
			map[string]any{"allowed": dispositionAllowlist})
		return
	}
	cur, err := s.repo.GetSessionDisposition(r.Context(), id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get disposition failed", nil)
		return
	}
	out, err := s.repo.UpsertSessionDisposition(r.Context(), store.SessionDisposition{
		SessionID:  id,
		Suggestion: cur.Suggestion,
		TemplateID: cur.TemplateID,
		Source:     "ops_patch",
		Final:      final,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "set disposition failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": out.SessionID, "suggestion": out.Suggestion,
		"final": out.Final, "source": out.Source, "updated_at": out.UpdatedAt,
	})
}

// EndSessionAfterTalk stops a session after talk teardown (neutral session-end hook).
func (s *Server) EndSessionAfterTalk(ctx context.Context, sessionID, disposition string) {
	sess, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		applog.Warn("talk end: session lookup failed", "session", sessionID, "err", err)
		return
	}
	switch sess.State {
	case store.StateCompleted, store.StateCancelled, store.StateFailed:
		return
	}
	if _, err := s.repo.UpdateSessionState(ctx, sessionID, store.StateDraining); err != nil {
		applog.Warn("talk end: drain failed", "session", sessionID, "err", err)
	}
	terminal := store.StateCompleted
	if s.rt != nil {
		if term, err := s.rt.StopSession(ctx, sessionID, "talk_end"); err == nil && term != "" {
			terminal = term
		}
	}
	updated, err := s.repo.UpdateSessionState(ctx, sessionID, terminal)
	if err != nil {
		applog.Warn("talk end: terminal state failed", "session", sessionID, "err", err)
		return
	}
	s.onSessionTerminal(ctx, updated, terminal)
	applog.Info("talk end complete", "session", sessionID, "state", terminal, "disposition", disposition)
}

package control

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

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
	if !store.ValidDispositionFinal(final) {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "unknown disposition code",
			map[string]any{"allowed": store.DispositionFinalAllowlist})
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
		Source:     store.DispositionSourceOpsPatch,
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

// transferDispositionFinal maps a settled transfer to a P2.6 final code.
func transferDispositionFinal(req port.TransferRequest) string {
	if store.ValidDispositionFinal(strings.TrimSpace(req.DispositionCode)) {
		return strings.TrimSpace(req.DispositionCode)
	}
	hay := strings.ToLower(strings.TrimSpace(req.DispositionCode) + " " + strings.TrimSpace(req.Reason))
	switch {
	case strings.Contains(hay, "sales"):
		return store.DispositionFinalTransferredSales
	case strings.Contains(hay, "corporate") || strings.Contains(hay, "corp"):
		return store.DispositionFinalTransferredCorporate
	case strings.Contains(hay, "support") || strings.Contains(hay, "tech"):
		return store.DispositionFinalTransferredSupport
	default:
		return store.DispositionFinalTransferredOther
	}
}

// ensureTerminalDisposition writes a P2.6 final when Ending left final empty (P2.6 edge 3).
func (s *Server) ensureTerminalDisposition(ctx context.Context, sess store.Session, terminal string) {
	if s.repo == nil {
		return
	}
	cur, err := s.repo.GetSessionDisposition(ctx, sess.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		applog.Warn("terminal disposition lookup", "session", sess.ID, "err", err)
		return
	}
	if err == nil && strings.TrimSpace(cur.Final) != "" {
		return
	}
	final := store.DispositionFinalOutOfScope
	switch terminal {
	case store.StateCancelled:
		final = store.DispositionFinalAbandonedCaller
	case store.StateFailed:
		final = store.DispositionFinalSystemFailure
	}
	d := store.SessionDisposition{
		SessionID:  sess.ID,
		Suggestion: cur.Suggestion,
		TemplateID: cur.TemplateID,
		Source:     store.DispositionSourceLiveGraph,
		Final:      final,
	}
	if _, err := s.repo.UpsertSessionDisposition(ctx, d); err != nil {
		applog.Warn("terminal disposition fill-in", "session", sess.ID, "final", final, "err", err)
	}
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

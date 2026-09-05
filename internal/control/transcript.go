package control

import (
	"errors"
	"net/http"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func (s *Server) handleGetTranscript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.repo.GetSession(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "session not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternal, "get session failed", nil)
		return
	}
	turns, err := s.repo.ListTranscriptTurns(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list transcript failed", nil)
		return
	}
	items := make([]map[string]any, 0, len(turns))
	for _, t := range turns {
		item := map[string]any{
			"seq":               t.Seq,
			"turn_id":           t.TurnID,
			"role":              t.Role,
			"text":              t.Text,
			"event_kind":        t.EventKind,
			"actionable_reason": t.ActionableReason,
			"language":          t.Language,
			"created_at":        t.CreatedAt,
		}
		if t.Actionable != nil {
			item["actionable"] = *t.Actionable
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": id,
		"turns":      items,
	})
}

func (s *Server) handleGetDisposition(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.repo.GetSession(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "session not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternal, "get session failed", nil)
		return
	}
	d, err := s.repo.GetSessionDisposition(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "disposition not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternal, "get disposition failed", nil)
		return
	}
	var final any
	if d.Final != "" {
		final = d.Final
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id":  d.SessionID,
		"suggestion":  d.Suggestion,
		"template_id": d.TemplateID,
		"source":      d.Source,
		"final":       final,
		"updated_at":  d.UpdatedAt,
	})
}

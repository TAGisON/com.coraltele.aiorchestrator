package control

import (
	"net/http"
	"strings"
)

// handleAnswer implements POST /v1/sessions/{id}/answer — call answered; bot speaks opening.
func (s *Server) handleAnswer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "session id required", nil)
		return
	}
	if _, err := s.repo.GetSession(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, CodeNotFound, "session not found", nil)
		return
	}
	if s.rt == nil {
		writeError(w, http.StatusServiceUnavailable, CodeInternal, "runtime not configured", nil)
		return
	}
	spoken, err := s.rt.AnswerCall(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, err.Error(), nil)
		return
	}
	out := map[string]any{
		"session_id": id,
		"answered":   true,
		"spoken":     spoken,
	}
	if det, act, ok := s.rt.Languages(id); ok {
		out["detected_language"] = det
		out["active_language"] = act
	}
	writeJSON(w, http.StatusOK, out)
}

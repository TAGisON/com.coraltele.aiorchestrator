package control

import (
	"net/http"
	"strings"
)

type injectReq struct {
	Text  string `json:"text"`
	Speak *bool  `json:"speak"` // reserved; Talk path always speaks response when present
}

// handleInject implements POST /v1/sessions/{id}/inject (lab text → Talk turn).
func (s *Server) handleInject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "session id required", nil)
		return
	}
	var req injectReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "text required", nil)
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
	if err := s.rt.InjectText(r.Context(), id, req.Text); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, err.Error(), nil)
		return
	}
	out := map[string]any{
		"session_id": id,
		"injected":   true,
		"text":       req.Text,
	}
	if det, act, ok := s.rt.Languages(id); ok {
		out["detected_language"] = det
		out["active_language"] = act
	}
	writeJSON(w, http.StatusOK, out)
}

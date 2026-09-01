package control

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	spoken, err := s.rt.AnswerCall(ctx, id)
	if err != nil {
		var ae *AnswerError
		if errors.As(err, &ae) {
			if ae.RetryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(ae.RetryAfter))
			}
			writeError(w, ae.HTTPStatus, ae.Code, ae.Message, nil)
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, ErrWelcomeTimeout.Code, ErrWelcomeTimeout.Message, nil)
			return
		}
		writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, err.Error(), nil)
		return
	}
	applog.Info("answer spoken", "session", id, "chars", len(spoken))

	welcomeCompleted := true
	if sr, ok := s.rt.(*SessionRuntime); ok {
		if view, ok := sr.SessionMedia(id); ok {
			welcomeCompleted = view.WelcomeCompleted
		}
	}

	out := map[string]any{
		"session_id":         id,
		"welcome_completed":  welcomeCompleted,
		"answered":           welcomeCompleted,
		"spoken":             spoken,
	}
	if det, act, ok := s.rt.Languages(id); ok {
		out["detected_language"] = det
		out["active_language"] = act
	}
	writeJSON(w, http.StatusOK, out)
}

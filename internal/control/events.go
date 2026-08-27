package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/bus"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// SSE event catalog (CONTROL_API.md §3 / ANALYTICS_AND_POSTCALL.md §4).
// Locked event: names — caption | session.state | skill.completed | turn.completed | error
//
// Bus Kind → SSE event mapping:
//
//	state            → session.state   data: {session_id, state}
//	turn.completed   → turn.completed  data: {session_id, outcome, skill_name?, skill_ok?, barge_in?}
//	skill.completed  → skill.completed data: {session_id, name, ok}
//	error            → error           data: {session_id, code, message}
//	SubscribeText    → caption         data: {session_id, partial, text, language, ts_ms}
//
// Backpressure: drop partial captions first when the write buffer is slow;
// finals are attempted once then skipped (simple Phase E policy).

func (s *Server) handleSessionEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.repo.GetSession(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "session not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get session failed", nil)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, CodeInternal, "streaming unsupported", nil)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Snapshot current durable state immediately.
	_ = writeSSE(w, flusher, "session.state", map[string]any{
		"session_id": sess.ID,
		"state":      sess.State,
	})

	mgr := s.sessionMgr()
	if mgr == nil {
		// No live actor — hold connection briefly then end (lab).
		select {
		case <-r.Context().Done():
		case <-time.After(50 * time.Millisecond):
		}
		return
	}
	actor, ok := mgr.Get(id)
	if !ok {
		select {
		case <-r.Context().Done():
		case <-time.After(50 * time.Millisecond):
		}
		return
	}

	evs := actor.Bus.SubscribeEvents(64)
	texts := actor.Bus.SubscribeText(64)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case te, ok := <-texts:
			if !ok {
				texts = nil
				continue
			}
			partial := te.Role == "partial"
			payload := map[string]any{
				"session_id": id,
				"partial":    partial,
				"text":       te.Text,
				"language":   "",
				"ts_ms":      te.At.UnixMilli(),
			}
			if err := writeSSE(w, flusher, "caption", payload); err != nil {
				if partial {
					continue // drop partials under backpressure
				}
				return
			}
		case ev, ok := <-evs:
			if !ok {
				return
			}
			name, data := mapBusToSSE(id, ev)
			if name == "" {
				continue
			}
			if err := writeSSE(w, flusher, name, data); err != nil {
				return
			}
		}
	}
}

func mapBusToSSE(sessionID string, ev bus.Event) (string, map[string]any) {
	switch ev.Kind {
	case "state":
		state, _ := ev.Data.(string)
		return "session.state", map[string]any{"session_id": sessionID, "state": state}
	case "turn.completed":
		data := map[string]any{"session_id": sessionID}
		if m, ok := ev.Data.(map[string]any); ok {
			for k, v := range m {
				data[k] = v
			}
		}
		return "turn.completed", data
	case "skill.completed":
		data := map[string]any{"session_id": sessionID}
		if m, ok := ev.Data.(map[string]any); ok {
			for k, v := range m {
				data[k] = v
			}
		}
		return "skill.completed", data
	case "error":
		data := map[string]any{"session_id": sessionID}
		switch d := ev.Data.(type) {
		case map[string]any:
			for k, v := range d {
				data[k] = v
			}
		case string:
			data["message"] = d
			data["code"] = "error"
		}
		return "error", data
	default:
		return "", nil
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// sessionMgr extracts Manager when Runtime is SessionRuntime.
func (s *Server) sessionMgr() *session.Manager {
	if sr, ok := s.rt.(*SessionRuntime); ok {
		return sr.Mgr
	}
	return nil
}

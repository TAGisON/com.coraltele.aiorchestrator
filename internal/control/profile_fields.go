package control

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// handlePatchProfileFields implements PATCH /v1/sessions/{id}/profile-fields (cc-2: language.primary).
func (s *Server) handlePatchProfileFields(w http.ResponseWriter, r *http.Request) {
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
	pv, err := s.repo.GetVersion(r.Context(), sess.ProfileID, sess.ProfileVersion)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "pinned profile version not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get profile version failed", nil)
		return
	}
	doc, err := profile.Parse(pv.Document)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "pinned profile corrupt", nil)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "read body failed", nil)
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid json", nil)
		return
	}
	if len(fields) == 0 {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "empty patch", nil)
		return
	}
	allowed := map[string]bool{}
	for _, k := range doc.HotSwapAllowed {
		allowed[strings.TrimSpace(k)] = true
	}
	for k := range fields {
		if !allowed[k] {
			writeError(w, http.StatusUnprocessableEntity, CodeProfileInvalid, "key not in hot_swap_allowed: "+k, map[string]any{"field": k})
			return
		}
	}
	primaryRaw, hasPrimary := fields["language.primary"]
	if !hasPrimary {
		writeError(w, http.StatusUnprocessableEntity, CodeProfileInvalid, "only language.primary is supported in this phase", nil)
		return
	}
	if !doc.Language.MidCallSwitch {
		writeError(w, http.StatusUnprocessableEntity, CodeProfileInvalid, "language.mid_call_switch required for language.primary", map[string]any{"field": "language.mid_call_switch"})
		return
	}
	var primary string
	if err := json.Unmarshal(primaryRaw, &primary); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "language.primary must be a string", nil)
		return
	}
	primary = strings.TrimSpace(primary)
	if primary == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "language.primary required", nil)
		return
	}
	if s.rt != nil {
		if err := s.rt.SwitchLanguage(id, primary); err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "switch language failed: "+err.Error(), nil)
			return
		}
	}
	detected := sess.DetectedLanguage
	if s.rt != nil {
		if d, _, ok := s.rt.Languages(id); ok {
			detected = d
		}
	}
	updated, err := s.repo.UpdateSessionLanguages(r.Context(), id, detected, primary)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "persist language failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id":         updated.ID,
		"detected_language":  updated.DetectedLanguage,
		"active_language":    updated.ActiveLanguage,
		"language.primary":   primary,
	})
}

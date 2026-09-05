package control

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

const settingAnswerPins = "answer_pins"

// AnswerPin associates a Talk profile with a published flow pin (A.5).
// DID is an optional operator label for lab/edge documentation — not a telephony provisioner.
type AnswerPin struct {
	ProfileID   string `json:"profile_id"`
	FlowID      string `json:"flow_id"`
	FlowVersion string `json:"flow_version"` // "latest" or decimal version
	DID         string `json:"did,omitempty"`
	Note        string `json:"note,omitempty"`
}

type answerPinsBody struct {
	Pins []AnswerPin `json:"pins"`
}

func parseAnswerPinsJSON(raw string) ([]AnswerPin, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []AnswerPin{}, nil
	}
	var pins []AnswerPin
	if err := json.Unmarshal([]byte(raw), &pins); err != nil {
		// Allow wrapped {"pins":[...]} for resilience.
		var wrap answerPinsBody
		if err2 := json.Unmarshal([]byte(raw), &wrap); err2 != nil {
			return nil, errors.New("answer_pins setting is not valid JSON array")
		}
		pins = wrap.Pins
	}
	if pins == nil {
		pins = []AnswerPin{}
	}
	return pins, nil
}

func (s *Server) handleGetAnswerPins(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, "")
	st, err := s.repo.GetSystemSetting(r.Context(), tenantID, settingAnswerPins)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "pins": []AnswerPin{}})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get answer pins failed", nil)
		return
	}
	pins, err := parseAnswerPinsJSON(st.Value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "pins": pins})
}

func (s *Server) handlePutAnswerPins(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, "")
	var req answerPinsBody
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	if req.Pins == nil {
		req.Pins = []AnswerPin{}
	}
	seen := map[string]struct{}{}
	cleaned := make([]AnswerPin, 0, len(req.Pins))
	for i, p := range req.Pins {
		pid := strings.TrimSpace(p.ProfileID)
		fid := strings.TrimSpace(p.FlowID)
		ver := strings.TrimSpace(p.FlowVersion)
		if pid == "" || fid == "" {
			writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, "profile_id and flow_id required", map[string]any{"index": i})
			return
		}
		if ver == "" {
			ver = "latest"
		}
		if ver != "latest" {
			n, err := strconv.Atoi(ver)
			if err != nil || n < 1 {
				writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, "flow_version must be latest or positive integer", map[string]any{"index": i})
				return
			}
		}
		if _, dup := seen[pid]; dup {
			writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, "duplicate profile_id", map[string]any{"profile_id": pid})
			return
		}
		seen[pid] = struct{}{}
		if _, err := s.repo.GetProfile(r.Context(), pid); errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, "profile not found", map[string]any{"profile_id": pid})
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "lookup profile failed", nil)
			return
		}
		if _, err := s.repo.GetFlow(r.Context(), fid); errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, "flow not found", map[string]any{"flow_id": fid})
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "lookup flow failed", nil)
			return
		}
		if ver != "latest" {
			n, _ := strconv.Atoi(ver)
			if _, err := s.repo.GetFlowVersion(r.Context(), fid, n); errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, "flow version not found", map[string]any{"flow_id": fid, "flow_version": n})
				return
			} else if err != nil {
				writeError(w, http.StatusInternalServerError, CodeInternal, "lookup flow version failed", nil)
				return
			}
		}
		cleaned = append(cleaned, AnswerPin{
			ProfileID: pid, FlowID: fid, FlowVersion: ver,
			DID: strings.TrimSpace(p.DID), Note: strings.TrimSpace(p.Note),
		})
	}
	raw, err := json.Marshal(cleaned)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "encode pins failed", nil)
		return
	}
	st, err := s.repo.UpsertSystemSetting(r.Context(), store.SystemSetting{
		TenantID: tenantID,
		Key:      settingAnswerPins,
		Value:    string(raw),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "save answer pins failed", nil)
		return
	}
	pins, _ := parseAnswerPinsJSON(st.Value)
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "pins": pins})
}

package control

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/fallback"
)

// Fallback prompts are operator-managed system assets: the audio a caller hears
// when the AI pipeline cannot serve the call. They are uploaded once and reused
// across every profile, because the profile pipeline is exactly what is broken
// when they are needed.
//
//	GET    /v1/tenant/fallback              list prompts visible to the tenant
//	GET    /v1/tenant/fallback/{scenario}   download the stored WAV
//	PUT    /v1/tenant/fallback/{scenario}   upload (audio/wav body)
//	DELETE /v1/tenant/fallback/{scenario}   remove the tenant override

func (s *Server) fallbackStore(w http.ResponseWriter) (*fallback.Store, bool) {
	if s.fallback == nil {
		writeError(w, http.StatusServiceUnavailable, CodeGatewayUnavailable,
			"fallback prompt store is not configured", nil)
		return nil, false
	}
	return s.fallback, true
}

func scenarioFromPath(w http.ResponseWriter, r *http.Request) (fallback.Scenario, bool) {
	sc, err := fallback.ParseScenario(r.PathValue("scenario"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(),
			map[string]any{"valid_scenarios": fallback.Scenarios})
		return "", false
	}
	return sc, true
}

func (s *Server) handleListFallback(w http.ResponseWriter, r *http.Request) {
	fs, ok := s.fallbackStore(w)
	if !ok {
		return
	}
	tenantID := resolveTenantID(r, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id":       tenantID,
		"root":            fs.Root(),
		"valid_scenarios": fallback.Scenarios,
		"prompts":         fs.List(tenantID),
	})
}

func (s *Server) handleGetFallback(w http.ResponseWriter, r *http.Request) {
	fs, ok := s.fallbackStore(w)
	if !ok {
		return
	}
	sc, ok := scenarioFromPath(w, r)
	if !ok {
		return
	}
	tenantID := resolveTenantID(r, "")

	// Report what would actually play, following the tenant→default and
	// scenario→generic widening, so operators can verify coverage.
	asset, found := fs.Resolve(tenantID, sc)
	if !found {
		writeError(w, http.StatusNotFound, CodeNotFound, "no prompt resolves for this scenario", nil)
		return
	}
	if strings.EqualFold(r.URL.Query().Get("download"), "true") {
		raw, err := fs.Raw(asset.TenantID, sc)
		if err != nil {
			// The resolved file may belong to another scenario/tenant; re-read by path.
			writeError(w, http.StatusNotFound, CodeNotFound, "prompt file unavailable", nil)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+string(sc)+".wav\"")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
		return
	}
	asset.PCM = nil
	writeJSON(w, http.StatusOK, asset)
}

func (s *Server) handlePutFallback(w http.ResponseWriter, r *http.Request) {
	fs, ok := s.fallbackStore(w)
	if !ok {
		return
	}
	sc, ok := scenarioFromPath(w, r)
	if !ok {
		return
	}
	tenantID := resolveTenantID(r, "")

	body, err := readUploadBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}

	asset, err := fs.Put(tenantID, sc, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), map[string]any{
			"expected": "uncompressed 16-bit PCM WAVE, mono or stereo, 8000–48000 Hz",
		})
		return
	}
	applog.Info("fallback prompt uploaded", "tenant", tenantID, "scenario", string(sc),
		"path", asset.Path, "duration_ms", asset.DurationMs, "rate", asset.SampleRate)
	writeJSON(w, http.StatusOK, asset)
}

func (s *Server) handleDeleteFallback(w http.ResponseWriter, r *http.Request) {
	fs, ok := s.fallbackStore(w)
	if !ok {
		return
	}
	sc, ok := scenarioFromPath(w, r)
	if !ok {
		return
	}
	tenantID := resolveTenantID(r, "")
	if err := fs.Delete(tenantID, sc); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "delete prompt failed", nil)
		return
	}
	applog.Info("fallback prompt deleted", "tenant", tenantID, "scenario", string(sc))
	w.WriteHeader(http.StatusNoContent)
}

// readUploadBody accepts either a raw audio/wav body or a multipart form with a
// "file" part, so both curl --data-binary and a browser form upload work.
func readUploadBody(r *http.Request) ([]byte, error) {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(fallback.MaxUploadBytes); err != nil {
			return nil, errors.New("invalid multipart upload: " + err.Error())
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			return nil, errors.New("multipart upload needs a \"file\" part")
		}
		defer f.Close()
		return io.ReadAll(io.LimitReader(f, fallback.MaxUploadBytes+1))
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, fallback.MaxUploadBytes+1))
	if err != nil {
		return nil, errors.New("read upload failed")
	}
	if len(body) == 0 {
		return nil, errors.New("empty body: send the WAV as the request body or a multipart \"file\" part")
	}
	return body, nil
}

package control

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

type gatewayCredentialResponse struct {
	TenantID      string          `json:"tenant_id"`
	GatewayID     string          `json:"gateway_id"`
	APIKeySet     bool            `json:"api_key_set"`
	APIKeyPreview string          `json:"api_key_preview,omitempty"`
	Extra         json.RawMessage `json:"extra,omitempty"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type putGatewayCredentialReq struct {
	APIKey string          `json:"api_key"`
	Extra  json.RawMessage `json:"extra"`
}

type systemSettingResponse struct {
	TenantID  string    `json:"tenant_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

type putSystemSettingReq struct {
	Value string `json:"value"`
}

type tenantSettingsBundle struct {
	TenantID    string                      `json:"tenant_id"`
	Engines     *tenantEnginesResponse      `json:"engines,omitempty"`
	Credentials []gatewayCredentialResponse `json:"credentials"`
	Settings    []systemSettingResponse     `json:"settings"`
}

func maskKey(k string) string {
	k = strings.TrimSpace(k)
	if k == "" {
		return ""
	}
	if len(k) <= 4 {
		return "****"
	}
	return "****" + k[len(k)-4:]
}

func credToResp(c store.GatewayCredential) gatewayCredentialResponse {
	return gatewayCredentialResponse{
		TenantID:      c.TenantID,
		GatewayID:     c.GatewayID,
		APIKeySet:     strings.TrimSpace(c.APIKey) != "",
		APIKeyPreview: maskKey(c.APIKey),
		Extra:         c.Extra,
		UpdatedAt:     c.UpdatedAt,
	}
}

func (s *Server) handleListGatewayCredentials(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, "")
	list, err := s.repo.ListGatewayCredentials(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list credentials failed", nil)
		return
	}
	out := make([]gatewayCredentialResponse, 0, len(list))
	for _, c := range list {
		out = append(out, credToResp(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "credentials": out})
}

func (s *Server) handleGetGatewayCredential(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, "")
	gatewayID := strings.TrimSpace(r.PathValue("gateway_id"))
	if gatewayID == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "gateway_id required", nil)
		return
	}
	c, err := s.repo.GetGatewayCredential(r.Context(), tenantID, gatewayID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "credential not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get credential failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, credToResp(c))
}

func (s *Server) handlePutGatewayCredential(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, "")
	gatewayID := strings.TrimSpace(r.PathValue("gateway_id"))
	if gatewayID == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "gateway_id required", nil)
		return
	}
	var req putGatewayCredentialReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	if strings.TrimSpace(req.APIKey) == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "api_key required", nil)
		return
	}
	c, err := s.repo.UpsertGatewayCredential(r.Context(), store.GatewayCredential{
		TenantID:  tenantID,
		GatewayID: gatewayID,
		APIKey:    strings.TrimSpace(req.APIKey),
		Extra:     req.Extra,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "upsert credential failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, credToResp(c))
}

func (s *Server) handleListSystemSettings(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, "")
	list, err := s.repo.ListSystemSettings(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list settings failed", nil)
		return
	}
	out := make([]systemSettingResponse, 0, len(list))
	for _, st := range list {
		out = append(out, systemSettingResponse{
			TenantID: st.TenantID, Key: st.Key, Value: st.Value, UpdatedAt: st.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "settings": out})
}

func (s *Server) handleGetSystemSetting(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, "")
	key := strings.TrimSpace(r.PathValue("key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "key required", nil)
		return
	}
	st, err := s.repo.GetSystemSetting(r.Context(), tenantID, key)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "setting not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get setting failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, systemSettingResponse{
		TenantID: st.TenantID, Key: st.Key, Value: st.Value, UpdatedAt: st.UpdatedAt,
	})
}

func (s *Server) handlePutSystemSetting(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, "")
	key := strings.TrimSpace(r.PathValue("key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "key required", nil)
		return
	}
	var req putSystemSettingReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	st, err := s.repo.UpsertSystemSetting(r.Context(), store.SystemSetting{
		TenantID: tenantID,
		Key:      key,
		Value:    req.Value,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "upsert setting failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, systemSettingResponse{
		TenantID: st.TenantID, Key: st.Key, Value: st.Value, UpdatedAt: st.UpdatedAt,
	})
}

func (s *Server) handleGetTenantSettings(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, "")
	bundle := tenantSettingsBundle{TenantID: tenantID, Credentials: []gatewayCredentialResponse{}, Settings: []systemSettingResponse{}}
	if binding, source, err := s.resolveTenantEngines(r.Context(), tenantID); err == nil {
		bundle.Engines = &tenantEnginesResponse{
			TenantID: tenantID, Listen: binding.Listen, Think: binding.Think, Speak: binding.Speak, Source: source,
		}
	}
	if list, err := s.repo.ListGatewayCredentials(r.Context(), tenantID); err == nil {
		for _, c := range list {
			bundle.Credentials = append(bundle.Credentials, credToResp(c))
		}
	}
	if list, err := s.repo.ListSystemSettings(r.Context(), tenantID); err == nil {
		for _, st := range list {
			bundle.Settings = append(bundle.Settings, systemSettingResponse{
				TenantID: st.TenantID, Key: st.Key, Value: st.Value, UpdatedAt: st.UpdatedAt,
			})
		}
	}
	writeJSON(w, http.StatusOK, bundle)
}

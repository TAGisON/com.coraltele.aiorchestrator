package control

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

type bindingResponse struct {
	ID        string          `json:"id"`
	TenantID  string          `json:"tenant_id"`
	Kind      string          `json:"kind"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	Status    string          `json:"status"`
	CreatedAt any             `json:"created_at,omitempty"`
	UpdatedAt any             `json:"updated_at,omitempty"`
}

type putBindingReq struct {
	TenantID string          `json:"tenant_id"`
	Kind     string          `json:"kind"`
	Name     string          `json:"name"`
	Config   json.RawMessage `json:"config"`
	Status   string          `json:"status"`
}

func bindingToResp(b store.Binding) bindingResponse {
	cfg := b.Config
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}
	return bindingResponse{
		ID: b.ID, TenantID: b.TenantID, Kind: b.Kind, Name: b.Name,
		Config: cfg, Status: b.Status, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
}

func validateBindingKind(kind string) bool {
	return kind == store.BindingKindKnowledge || kind == store.BindingKindCRM
}

func validateBindingStatus(status string) bool {
	return status == store.BindingStatusActive || status == store.BindingStatusDisabled
}

func validateKnowledgeConfig(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return errors.New("config must be JSON object")
	}
	if _, ok := cfg["api_key"]; ok {
		return errors.New("config must not contain api_key (use gateway credentials)")
	}
	mode, _ := cfg["mode"].(string)
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "inline_faq"
	}
	if mode != "inline_faq" && mode != "http_retrieve" {
		return errors.New("knowledge config.mode must be inline_faq or http_retrieve")
	}
	return nil
}

func (s *Server) handleListBindings(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, r.URL.Query().Get("tenant_id"))
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	if kind != "" && !validateBindingKind(kind) {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid kind", map[string]any{
			"allowed": []string{store.BindingKindKnowledge, store.BindingKindCRM},
		})
		return
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	list, err := s.repo.ListBindings(r.Context(), tenantID, kind, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list bindings failed", nil)
		return
	}
	items := make([]bindingResponse, 0, len(list))
	for _, b := range list {
		items = append(items, bindingToResp(b))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "bindings": items})
}

func (s *Server) handleGetBinding(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "id required", nil)
		return
	}
	b, err := s.repo.GetBinding(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "binding not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get binding failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, bindingToResp(b))
}

func (s *Server) handlePutBinding(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "id required", nil)
		return
	}
	var req putBindingReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	kind := strings.TrimSpace(req.Kind)
	if !validateBindingKind(kind) {
		writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, "invalid kind", map[string]any{
			"allowed": []string{store.BindingKindKnowledge, store.BindingKindCRM},
		})
		return
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = store.BindingStatusActive
	}
	if !validateBindingStatus(status) {
		writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, "invalid status", map[string]any{
			"allowed": []string{store.BindingStatusActive, store.BindingStatusDisabled},
		})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = id
	}
	cfg := req.Config
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}
	if kind == store.BindingKindKnowledge {
		if err := validateKnowledgeConfig(cfg); err != nil {
			writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, err.Error(), nil)
			return
		}
	}
	tenantID := resolveTenantID(r, req.TenantID)
	out, err := s.repo.UpsertBinding(r.Context(), store.Binding{
		ID: id, TenantID: tenantID, Kind: kind, Name: name, Config: cfg, Status: status,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "upsert binding failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, bindingToResp(out))
}

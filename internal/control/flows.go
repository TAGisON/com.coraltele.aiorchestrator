package control

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/flow"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

type createFlowReq struct {
	ID        string          `json:"id"`
	TenantID  string          `json:"tenant_id"`
	Name      string          `json:"name"`
	Direction string          `json:"direction"`
	Doc       json.RawMessage `json:"doc"`
}

func (s *Server) handleCreateFlow(w http.ResponseWriter, r *http.Request) {
	var req createFlowReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "id required", nil)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = id
	}
	dir := strings.TrimSpace(req.Direction)
	if dir == "" {
		dir = store.FlowDirectionInbound
	}
	draft := req.Doc
	if len(draft) == 0 {
		draft = json.RawMessage(`{"schema_id":"coral.flow.v1","entry_node_id":"","default_locale":"","nodes":[],"edges":[],"prompts":{},"matrix":[],"binding_refs":[]}`)
	}
	out, err := s.repo.CreateFlow(r.Context(), store.Flow{
		ID:        id,
		TenantID:  req.TenantID,
		Name:      name,
		Direction: dir,
	}, draft)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, CodeConflict, "flow already exists", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "create flow failed", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":              out.ID,
		"tenant_id":       out.TenantID,
		"name":            out.Name,
		"direction":       out.Direction,
		"status":          out.Status,
		"current_version": out.CurrentVersion,
	})
}

func (s *Server) handleListFlows(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant_id")
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	list, err := s.repo.ListFlows(r.Context(), tenant, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list flows failed", nil)
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, f := range list {
		items = append(items, map[string]any{
			"id": f.ID, "tenant_id": f.TenantID, "name": f.Name,
			"direction": f.Direction, "status": f.Status, "current_version": f.CurrentVersion,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"flows": items})
}

func (s *Server) handleGetFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f, err := s.repo.GetFlow(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "flow not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get flow failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": f.ID, "tenant_id": f.TenantID, "name": f.Name,
		"direction": f.Direction, "status": f.Status, "current_version": f.CurrentVersion,
		"created_at": f.CreatedAt, "updated_at": f.UpdatedAt,
	})
}

func (s *Server) handleGetFlowDraft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, err := s.repo.GetFlowDraft(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "flow draft not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get draft failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"flow_id": d.FlowID, "doc": json.RawMessage(d.Doc), "updated_at": d.UpdatedAt,
	})
}

func (s *Server) handlePutFlowDraft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	raw, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "read body failed", nil)
		return
	}
	if !json.Valid(raw) {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid json", nil)
		return
	}
	// Allow saving incomplete drafts; publish validates.
	d, err := s.repo.UpsertFlowDraft(r.Context(), id, json.RawMessage(raw))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "flow not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "save draft failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"flow_id": d.FlowID, "doc": json.RawMessage(d.Doc), "updated_at": d.UpdatedAt,
	})
}

func (s *Server) handlePublishFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	raw, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "read body failed", nil)
		return
	}
	doc, err := flow.Parse(raw)
	if err != nil {
		var ve *flow.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, CodeFlowInvalid, ve.Message, ve.Details)
			return
		}
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	if err := flow.Validate(doc); err != nil {
		var ve *flow.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, CodeFlowInvalid, ve.Message, ve.Details)
			return
		}
		writeError(w, http.StatusUnprocessableEntity, CodeFlowInvalid, err.Error(), nil)
		return
	}
	publishedBy := strings.TrimSpace(r.Header.Get("X-Published-By"))
	fv, err := s.repo.PublishFlowVersion(r.Context(), id, json.RawMessage(raw), flow.ContentHash(raw), publishedBy)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "flow not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "publish failed", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"flow_id": fv.FlowID, "version": fv.Version,
		"content_hash": fv.ContentHash, "published_by": fv.PublishedBy,
		"published_at": fv.PublishedAt,
	})
}

func (s *Server) handleGetFlowVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	verStr := r.PathValue("ver")
	ver, err := strconv.Atoi(verStr)
	if err != nil || ver < 1 {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "version must be a positive integer", nil)
		return
	}
	fv, err := s.repo.GetFlowVersion(r.Context(), id, ver)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "flow version not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get flow version failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"flow_id": fv.FlowID, "version": fv.Version, "doc": json.RawMessage(fv.Doc),
		"content_hash": fv.ContentHash, "published_by": fv.PublishedBy, "published_at": fv.PublishedAt,
	})
}

package control

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// LabExtras optional flags exposed on /v1/lab/status.
type LabExtras struct {
	StoreBackend    string // postgres | memory
	SarvamConfigured bool
	HTTPAddr        string
}

func (s *Server) SetLabExtras(x LabExtras) { s.lab = x }

// mountLabRoutes registers POC console APIs and static UI.
func (s *Server) mountLabRoutes(labFS fs.FS) {
	s.mux.HandleFunc("GET /v1/profiles", s.handleListProfiles)
	s.mux.HandleFunc("GET /v1/profiles/{id}/versions/{ver}", s.handleGetProfileVersion)
	s.mux.HandleFunc("GET /v1/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /v1/sessions/{id}/audit", s.handleSessionAudit)
	s.mux.HandleFunc("GET /v1/sessions/{id}/analytics", s.handleSessionAnalytics)
	s.mux.HandleFunc("GET /v1/gateways", s.handleListGateways)
	s.mux.HandleFunc("GET /v1/lab/status", s.handleLabStatus)

	if labFS != nil {
		ui, err := fs.Sub(labFS, "lab")
		if err == nil {
			fileServer := http.FileServer(http.FS(ui))
			s.mux.Handle("GET /lab/", http.StripPrefix("/lab/", fileServer))
			s.mux.HandleFunc("GET /lab", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/lab/", http.StatusFound)
			})
			s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/" {
					http.Redirect(w, r, "/lab/", http.StatusFound)
					return
				}
				http.NotFound(w, r)
			})
		}
	}
}

func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.repo.ListProfiles(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list profiles failed", nil)
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, p := range list {
		items = append(items, map[string]any{
			"id": p.ID, "tenant_id": p.TenantID, "display_name": p.DisplayName,
			"created_at": p.CreatedAt, "updated_at": p.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": items})
}

func (s *Server) handleGetProfileVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	verRaw := r.PathValue("ver")
	var pv store.ProfileVersion
	var err error
	if verRaw == "latest" {
		pv, err = s.repo.GetLatestVersion(r.Context(), id)
	} else {
		v, _ := strconv.Atoi(verRaw)
		pv, err = s.repo.GetVersion(r.Context(), id, v)
	}
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "profile version not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get version failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"profile_id": pv.ProfileID, "version": pv.Version,
		"document": json.RawMessage(pv.Document), "published_at": pv.PublishedAt,
	})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.repo.ListSessions(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list sessions failed", nil)
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, sess := range list {
		items = append(items, map[string]any{
			"session_id": sess.ID, "profile_id": sess.ProfileID, "profile_version": sess.ProfileVersion,
			"clock": sess.Clock, "state": sess.State, "tenant_id": sess.TenantID,
			"created_at": sess.CreatedAt, "updated_at": sess.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": items})
}

func (s *Server) handleSessionAudit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	evs, err := s.repo.ListAuditEvents(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list audit failed", nil)
		return
	}
	items := make([]map[string]any, 0, len(evs))
	for _, e := range evs {
		items = append(items, map[string]any{
			"id": e.ID, "session_id": e.SessionID, "tenant_id": e.TenantID,
			"event_type": e.EventType, "payload": json.RawMessage(e.Payload), "created_at": e.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit_events": items})
}

func (s *Server) handleSessionAnalytics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	evs, err := s.repo.ListAnalyticsEvents(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list analytics failed", nil)
		return
	}
	items := make([]map[string]any, 0, len(evs))
	for _, e := range evs {
		items = append(items, map[string]any{
			"id": e.ID, "session_id": e.SessionID, "tenant_id": e.TenantID,
			"metric": e.Metric, "value": e.Value, "dimensions": json.RawMessage(e.Dimensions),
			"created_at": e.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"analytics_events": items})
}

func (s *Server) handleListGateways(w http.ResponseWriter, r *http.Request) {
	kinds := []port.PortKind{
		port.PortListen, port.PortSpeak, port.PortThink,
		port.PortTranslate, port.PortKnowledge, port.PortSkill,
	}
	out := map[string]any{}
	for _, k := range kinds {
		regs := s.reg.List(k)
		entries := make([]map[string]any, 0, len(regs))
		for _, reg := range regs {
			entries = append(entries, map[string]any{
				"id":           string(reg.ID),
				"port":         string(reg.Port),
				"capabilities": reg.Capabilities,
			})
		}
		out[string(k)] = entries
	}
	writeJSON(w, http.StatusOK, map[string]any{"gateways": out})
}

func (s *Server) handleLabStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dbOK := s.repo.Ping(ctx) == nil
	backend := s.lab.StoreBackend
	if backend == "" {
		backend = "unknown"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"db":                dbOK,
		"store_backend":     backend,
		"sarvam_configured": s.lab.SarvamConfigured,
		"http_addr":         s.lab.HTTPAddr,
		"lab_ui":            "/lab/",
	})
}

func (s *Server) authMiddlewareLabBypass(path string) bool {
	return strings.HasPrefix(path, "/lab") || path == "/"
}

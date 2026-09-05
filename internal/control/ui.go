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

// UIExtras optional flags for platform status (store backend, listen addr).
type UIExtras struct {
	StoreBackend string // postgres | memory
	HTTPAddr     string
}

// LabExtras is deprecated alias kept for call-site compatibility during rename.
type LabExtras = UIExtras

func (s *Server) SetUIExtras(x UIExtras) { s.ui = x }

// SetLabExtras is deprecated; use SetUIExtras.
func (s *Server) SetLabExtras(x LabExtras) { s.SetUIExtras(x) }

// mountUIRoutes registers console JSON APIs and static production shells (U.2).
func (s *Server) mountUIRoutes(uiFS fs.FS) {
	s.mux.HandleFunc("GET /v1/profiles", s.handleListProfiles)
	s.mux.HandleFunc("GET /v1/profiles/{id}/versions/{ver}", s.handleGetProfileVersion)
	s.mux.HandleFunc("GET /v1/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /v1/sessions/{id}/audit", s.handleSessionAudit)
	s.mux.HandleFunc("GET /v1/sessions/{id}/analytics", s.handleSessionAnalytics)
	s.mux.HandleFunc("GET /v1/analytics/summary", s.handleAnalyticsSummary)
	s.mux.HandleFunc("GET /v1/gateways", s.handleListGateways)
	s.mux.HandleFunc("GET /v1/platform/status", s.handlePlatformStatus)

	if uiFS == nil {
		return
	}

	s.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		b, err := fs.ReadFile(uiFS, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})

	mountStatic := func(urlPrefix, dir string) {
		sub, err := fs.Sub(uiFS, dir)
		if err != nil {
			return
		}
		fileServer := http.FileServer(http.FS(sub))
		// Exact prefix without trailing slash → redirect to /
		s.mux.HandleFunc("GET "+urlPrefix, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, urlPrefix+"/", http.StatusFound)
		})
		s.mux.Handle("GET "+urlPrefix+"/", http.StripPrefix(urlPrefix+"/", fileServer))
	}
	mountStatic("/admin", "admin")
	mountStatic("/supervisor", "supervisor")
	mountStatic("/chat", "chat")
	mountStatic("/shared", "shared")
}

func (s *Server) authMiddlewareUIBypass(path string) bool {
	if path == "/" {
		return true
	}
	for _, p := range []string{"/admin", "/supervisor", "/chat", "/shared"} {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
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
		item := map[string]any{
			"session_id": sess.ID, "profile_id": sess.ProfileID, "profile_version": sess.ProfileVersion,
			"clock": sess.Clock, "state": sess.State, "tenant_id": sess.TenantID,
			"detected_language": sess.DetectedLanguage, "active_language": sess.ActiveLanguage,
			"recording_ref": sess.RecordingRef,
			"created_at":    sess.CreatedAt, "updated_at": sess.UpdatedAt,
		}
		if sess.FlowID != "" {
			item["flow_id"] = sess.FlowID
			item["flow_version"] = sess.FlowVersion
		}
		if sess.GatewayBinding != nil {
			item["gateway_binding"] = sess.GatewayBinding
		}
		if len(sess.Caller) > 0 {
			item["caller"] = json.RawMessage(sess.Caller)
		}
		if len(sess.Metadata) > 0 {
			item["metadata"] = json.RawMessage(sess.Metadata)
		}
		items = append(items, item)
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

func (s *Server) handlePlatformStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dbOK := s.repo.Ping(ctx) == nil
	backend := s.ui.StoreBackend
	if backend == "" {
		backend = "unknown"
	}

	blockers := make([]string, 0)
	warnings := make([]string, 0)
	if !dbOK {
		blockers = append(blockers, "database_unreachable")
	}
	if backend == "memory" {
		warnings = append(warnings, "store_memory_not_durable")
	}

	enginesConfigured := false
	var engines map[string]any
	if te, err := s.repo.GetTenantEngines(ctx, defaultTenantID); err == nil {
		enginesConfigured = true
		engines = map[string]any{
			"configured": true,
			"listen":     te.ListenID,
			"think":      te.ThinkID,
			"speak":      te.SpeakID,
			"source":     "store",
		}
		for _, id := range []string{te.ListenID, te.ThinkID, te.SpeakID} {
			if _, ok := s.reg.Get(port.GatewayID(id)); !ok {
				blockers = append(blockers, "gateway_not_registered:"+id)
			}
		}
		needsKey := strings.HasPrefix(te.ListenID, "sarvam") ||
			strings.HasPrefix(te.ThinkID, "sarvam") ||
			strings.HasPrefix(te.SpeakID, "sarvam")
		if needsKey {
			sarvamSet := false
			for _, gid := range []string{"sarvam", "sarvam-stt", "sarvam-llm", "sarvam-tts"} {
				if c, err := s.repo.GetGatewayCredential(ctx, defaultTenantID, gid); err == nil && strings.TrimSpace(c.APIKey) != "" {
					sarvamSet = true
					break
				}
			}
			if !sarvamSet {
				warnings = append(warnings, "sarvam_credential_missing")
			}
		}
	} else if errors.Is(err, store.ErrNotFound) {
		engines = map[string]any{"configured": false}
		blockers = append(blockers, "tenant_engines_missing")
	} else {
		engines = map[string]any{"configured": false, "error": "lookup_failed"}
		blockers = append(blockers, "tenant_engines_lookup_failed")
	}

	credCount := 0
	if list, err := s.repo.ListGatewayCredentials(ctx, defaultTenantID); err == nil {
		credCount = len(list)
	}

	profileCount := 0
	if list, err := s.repo.ListProfiles(ctx, 500); err == nil {
		profileCount = len(list)
	}
	if profileCount == 0 {
		blockers = append(blockers, "no_profiles")
	}

	gwListen := len(s.reg.List(port.PortListen))
	gwThink := len(s.reg.List(port.PortThink))
	gwSpeak := len(s.reg.List(port.PortSpeak))
	if gwListen == 0 || gwThink == 0 || gwSpeak == 0 {
		blockers = append(blockers, "gateway_registry_incomplete")
	}

	ready := len(blockers) == 0
	status := "ok"
	if !ready {
		status = "not_ready"
	} else if len(warnings) > 0 {
		status = "degraded"
	}

	code := http.StatusOK
	if !dbOK {
		code = http.StatusServiceUnavailable
		status = "unavailable"
	}

	writeJSON(w, code, map[string]any{
		"status":             status,
		"db":                 dbOK,
		"store_backend":      backend,
		"http_addr":          s.ui.HTTPAddr,
		"engines":            engines,
		"engines_configured": enginesConfigured,
		"credentials":        map[string]any{"count": credCount},
		"profiles":           map[string]any{"count": profileCount},
		"gateways": map[string]any{
			"listen": gwListen, "think": gwThink, "speak": gwSpeak,
		},
		"ready_for_sessions": ready,
		"blockers":           blockers,
		"warnings":           warnings,
	})
}

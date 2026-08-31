package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/token"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/observe"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// Runtime is the minimal session-actor surface Control needs (Phase C).
type Runtime interface {
	StartSession(ctx context.Context, p RuntimeStart) error
	StopSession(ctx context.Context, sessionID, reason string) (terminalState string, err error)
}

// RuntimeStart is create-time actor spawn input.
type RuntimeStart struct {
	SessionID      string
	TenantID       string
	Clock          string
	SampleRate     int
	Profile        profile.Document
	Document       json.RawMessage
	GatewayBinding *store.GatewayBinding
}

// Config for the control HTTP server.
type Config struct {
	// AuthToken when non-empty requires Authorization: Bearer <token> (lab stub).
	// Empty disables auth (lab default).
	AuthToken string
	// OwnerInstance stamped on new sessions.
	OwnerInstance string
	// EdgeBaseURL used for edge_wss_url.
	EdgeBaseURL string
	// EdgeTokenSecret HMAC key for signed edge tokens. Empty → random lab secret at New.
	EdgeTokenSecret []byte
	// EdgeTokenTTL for issued tokens (default 5m).
	EdgeTokenTTL time.Duration
}

// Server serves CONTROL_API Phase B routes (+ Phase C runtime glue).
type Server struct {
	repo store.Repository
	reg  port.Registry
	rt   Runtime
	cfg  Config
	mux  *http.ServeMux
	lab  LabExtras
	labFS fs.FS
}

func New(repo store.Repository, reg port.Registry, cfg Config) *Server {
	return NewWithRuntime(repo, reg, nil, cfg, nil)
}

// NewWithRuntime wires an optional runtime manager for session start/stop.
// labFS is optional embed root containing a "lab/" directory for the POC console.
func NewWithRuntime(repo store.Repository, reg port.Registry, rt Runtime, cfg Config, labFS fs.FS) *Server {
	if cfg.OwnerInstance == "" {
		cfg.OwnerInstance = "local"
	}
	if cfg.EdgeBaseURL == "" {
		cfg.EdgeBaseURL = "wss://localhost/edge/fs"
	}
	if len(cfg.EdgeTokenSecret) == 0 {
		var b [32]byte
		_, _ = rand.Read(b[:])
		cfg.EdgeTokenSecret = b[:]
	}
	if cfg.EdgeTokenTTL <= 0 {
		cfg.EdgeTokenTTL = 5 * time.Minute
	}
	s := &Server{repo: repo, reg: reg, rt: rt, cfg: cfg, mux: http.NewServeMux(), labFS: labFS}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.requestLogMiddleware(s.authMiddleware(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /v1/tenant/engines", s.handleGetTenantEngines)
	s.mux.HandleFunc("PUT /v1/tenant/engines", s.handlePutTenantEngines)
	s.mux.HandleFunc("POST /v1/profiles", s.handleCreateProfile)
	s.mux.HandleFunc("POST /v1/profiles/{id}/versions", s.handlePublishProfile)
	s.mux.HandleFunc("POST /v1/sessions", s.handleCreateSession)
	s.mux.HandleFunc("GET /v1/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("POST /v1/sessions/{id}/stop", s.handleStopSession)
	s.mux.HandleFunc("GET /v1/sessions/{id}/events", s.handleSessionEvents)
	s.mux.HandleFunc("POST /v1/jobs/playback", s.handlePlaybackCreate)
	s.mux.HandleFunc("GET /v1/jobs/{id}", s.handleJobGet)
	s.mux.HandleFunc("POST /v1/kb/documents", s.handleKBUpload)
	s.mux.HandleFunc("GET /v1/kb/documents/{id}", s.handleKBGet)
	s.mountLabRoutes(s.labFS)
}

func (s *Server) requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		applog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Edge uses signed query token (EDGE_FS.md), not Bearer.
		if strings.HasPrefix(r.URL.Path, "/edge/") {
			next.ServeHTTP(w, r)
			return
		}
		// Lab POC static UI does not require Bearer.
		if s.authMiddlewareLabBypass(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if s.cfg.AuthToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		h := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) || strings.TrimPrefix(h, prefix) != s.cfg.AuthToken {
			writeError(w, http.StatusUnauthorized, CodeUnauthorized, "missing or invalid token", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	status := "ok"
	code := http.StatusOK
	dbOK := true
	if err := s.repo.Ping(ctx); err != nil {
		status = "unhealthy"
		code = http.StatusServiceUnavailable
		dbOK = false
	}
	writeJSON(w, code, map[string]any{
		"status":        status,
		"db":            dbOK,
		"store_backend": s.lab.StoreBackend,
	})
}

type createProfileReq struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	DisplayName string `json:"display_name"`
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var req createProfileReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "id required", nil)
		return
	}
	err := s.repo.CreateProfile(r.Context(), store.Profile{
		ID:          req.ID,
		TenantID:    req.TenantID,
		DisplayName: req.DisplayName,
	})
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, CodeConflict, "profile already exists", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "create profile failed", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           req.ID,
		"tenant_id":    req.TenantID,
		"display_name": req.DisplayName,
	})
}

func (s *Server) handlePublishProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "read body failed", nil)
		return
	}
	doc, err := profile.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	if doc.ID != "" && doc.ID != id {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "document id must match path", nil)
		return
	}
	if err := profile.Validate(doc, s.reg); err != nil {
		var ve *profile.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, CodeProfileInvalid, ve.Message, ve.Details)
			return
		}
		writeError(w, http.StatusUnprocessableEntity, CodeProfileInvalid, err.Error(), nil)
		return
	}
	pv, err := s.repo.PublishVersion(r.Context(), id, json.RawMessage(raw))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "profile not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "publish failed", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"profile_id": pv.ProfileID,
		"version":    pv.Version,
	})
}

type createSessionReq struct {
	ProfileID      string          `json:"profile_id"`
	ProfileVersion json.RawMessage `json:"profile_version"` // "latest" or number
	Clock          string          `json:"clock"`
	Caller         json.RawMessage `json:"caller"`
	RecordingRef   string          `json:"recording_ref"`
	Metadata       json.RawMessage `json:"metadata"`
	TenantID       string          `json:"tenant_id"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	if strings.TrimSpace(req.ProfileID) == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "profile_id required", nil)
		return
	}
	clock := req.Clock
	if clock == "" {
		clock = "live"
	}
	pv, err := s.resolveProfileVersion(r.Context(), req.ProfileID, req.ProfileVersion)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "profile version not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	doc, err := profile.Parse(pv.Document)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "pinned profile corrupt", nil)
		return
	}
	tenantID := resolveTenantID(r, req.TenantID)
	binding, _, err := s.resolveTenantEngines(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "resolve tenant engines failed", nil)
		return
	}
	if err := validateGatewayBinding(s.reg, binding); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeProfileInvalid, err.Error(), map[string]any{"reason": "capability_gate"})
		return
	}
	warnProfileEngineConflict(doc, binding)
	bindingCopy := binding
	sid, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "id generate failed", nil)
		return
	}
	edgeToken, err := token.Issue(s.cfg.EdgeTokenSecret, token.Claims{
		TenantID:  tenantID,
		SessionID: sid,
		ProfileID: pv.ProfileID,
	}, s.cfg.EdgeTokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "token generate failed", nil)
		return
	}
	rate := profile.SampleRateHz(doc)
	sess := store.Session{
		ID:                    sid,
		TenantID:              tenantID,
		ProfileID:             pv.ProfileID,
		ProfileVersion:        pv.Version,
		Clock:                 clock,
		State:                 store.StateCreated,
		OwnerInstance:         s.cfg.OwnerInstance,
		CanonicalSampleRateHz: rate,
		Caller:                req.Caller,
		RecordingRef:          req.RecordingRef,
		Metadata:              req.Metadata,
		GatewayBinding:        &bindingCopy,
	}
	if err := s.repo.CreateSession(r.Context(), sess); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "create session failed", nil)
		return
	}
	state := sess.State
	if s.rt != nil {
		if err := s.rt.StartSession(r.Context(), RuntimeStart{
			SessionID:      sess.ID,
			TenantID:       sess.TenantID,
			Clock:          sess.Clock,
			SampleRate:     sess.CanonicalSampleRateHz,
			Profile:        doc,
			Document:       pv.Document,
			GatewayBinding: sess.GatewayBinding,
		}); err != nil {
			_, _ = s.repo.UpdateSessionState(r.Context(), sess.ID, store.StateFailed)
			ge, ok := port.AsGatewayError(err)
			if ok && ge.Code == port.CodeUnsupported {
				writeError(w, http.StatusUnprocessableEntity, CodeProfileInvalid, err.Error(), map[string]any{"reason": "capability_gate"})
				return
			}
			writeError(w, http.StatusInternalServerError, CodeInternal, "runtime start failed: "+err.Error(), nil)
			return
		}
		updated, err := s.repo.UpdateSessionState(r.Context(), sess.ID, store.StateRunning)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "set running failed", nil)
			return
		}
		state = updated.State
		obs := &observe.Observer{Repo: s.repo, Meta: observe.SessionMeta{
			SessionID: sess.ID, TenantID: sess.TenantID,
			ProfileID: sess.ProfileID, ProfileVersion: sess.ProfileVersion,
			Clock: sess.Clock, RecordingRef: sess.RecordingRef,
		}}
		obs.OnSessionStarted(r.Context())
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"session_id":               sess.ID,
		"profile_id":               sess.ProfileID,
		"profile_version":          sess.ProfileVersion,
		"clock":                    sess.Clock,
		"canonical_sample_rate_hz": sess.CanonicalSampleRateHz,
		"state":                    state,
		"gateway_binding":          sess.GatewayBinding,
		"edge_token":               edgeToken,
		"edge_wss_url":             s.cfg.EdgeBaseURL + "?token=" + edgeToken,
	})
}

func (s *Server) resolveProfileVersion(ctx context.Context, profileID string, verRaw json.RawMessage) (store.ProfileVersion, error) {
	if len(verRaw) == 0 || string(verRaw) == `null` || string(verRaw) == `"latest"` {
		return s.repo.GetLatestVersion(ctx, profileID)
	}
	// numeric
	var n int
	if err := json.Unmarshal(verRaw, &n); err == nil {
		return s.repo.GetVersion(ctx, profileID, n)
	}
	var sver string
	if err := json.Unmarshal(verRaw, &sver); err == nil {
		if sver == "" || sver == "latest" {
			return s.repo.GetLatestVersion(ctx, profileID)
		}
		n, err := strconv.Atoi(sver)
		if err != nil {
			return store.ProfileVersion{}, errors.New("profile_version must be latest or integer")
		}
		return s.repo.GetVersion(ctx, profileID, n)
	}
	return store.ProfileVersion{}, errors.New("profile_version must be latest or integer")
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
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
	out := map[string]any{
		"session_id":               sess.ID,
		"profile_id":               sess.ProfileID,
		"profile_version":          sess.ProfileVersion,
		"clock":                    sess.Clock,
		"state":                    sess.State,
		"owner_instance":           sess.OwnerInstance,
		"canonical_sample_rate_hz": sess.CanonicalSampleRateHz,
	}
	if sess.GatewayBinding != nil {
		out["gateway_binding"] = sess.GatewayBinding
	}
	writeJSON(w, http.StatusOK, out)
}

type stopReq struct {
	Reason string `json:"reason"`
}

func (s *Server) handleStopSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req stopReq
	_ = decodeJSON(r, &req) // body optional
	sess, err := s.repo.GetSession(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "session not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get session failed", nil)
		return
	}
	switch sess.State {
	case store.StateCompleted, store.StateCancelled, store.StateFailed:
		writeJSON(w, http.StatusOK, map[string]any{
			"session_id": sess.ID,
			"state":      sess.State,
		})
		return
	}
	terminal := store.StateCompleted
	if req.Reason == "operator" || req.Reason == "cancel" {
		terminal = store.StateCancelled
	}
	_, err = s.repo.UpdateSessionState(r.Context(), id, store.StateDraining)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "drain failed", nil)
		return
	}
	if s.rt != nil {
		term, err := s.rt.StopSession(r.Context(), id, req.Reason)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "runtime stop failed", nil)
			return
		}
		if term == "Cancelled" {
			terminal = store.StateCancelled
		} else if term == "Failed" {
			terminal = store.StateFailed
		} else if term == "Completed" {
			terminal = store.StateCompleted
		}
	}
	sess, err = s.repo.UpdateSessionState(r.Context(), id, terminal)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "stop failed", nil)
		return
	}
	s.onSessionTerminal(r.Context(), sess, terminal)
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sess.ID,
		"state":      sess.State,
		"reason":     req.Reason,
	})
}

func (s *Server) onSessionTerminal(ctx context.Context, sess store.Session, terminal string) {
	doc := profile.Document{}
	if pv, err := s.repo.GetVersion(ctx, sess.ProfileID, sess.ProfileVersion); err == nil {
		if parsed, err := profile.Parse(pv.Document); err == nil {
			doc = parsed
		}
	}
	audits, _ := s.repo.ListAuditEvents(ctx, sess.ID)
	handoff := DetectHandoffFromAudit(audits)
	obs := &observe.Observer{Repo: s.repo, Meta: observe.SessionMeta{
		SessionID: sess.ID, TenantID: sess.TenantID,
		ProfileID: sess.ProfileID, ProfileVersion: sess.ProfileVersion,
		Clock: sess.Clock, RecordingRef: sess.RecordingRef,
	}}
	obs.OnSessionTerminal(ctx, terminal, handoff,
		analyticsEmitSet(doc, "contained"),
		analyticsEmitSet(doc, "handoff"),
	)
	s.enqueuePostcall(ctx, sess)
}

// EdgeTokenSecret returns the HMAC key used for edge tokens (for MountEdge).
func (s *Server) EdgeTokenSecret() []byte {
	return s.cfg.EdgeTokenSecret
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

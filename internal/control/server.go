package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// Config for the control HTTP server.
type Config struct {
	// AuthToken when non-empty requires Authorization: Bearer <token> (lab stub).
	// Empty disables auth (lab default).
	AuthToken string
	// OwnerInstance stamped on new sessions.
	OwnerInstance string
	// EdgeBaseURL used for stub edge_wss_url (no FS in Phase B).
	EdgeBaseURL string
}

// Server serves CONTROL_API Phase B routes.
type Server struct {
	repo store.Repository
	reg  port.Registry
	cfg  Config
	mux  *http.ServeMux
}

func New(repo store.Repository, reg port.Registry, cfg Config) *Server {
	if cfg.OwnerInstance == "" {
		cfg.OwnerInstance = "local"
	}
	if cfg.EdgeBaseURL == "" {
		cfg.EdgeBaseURL = "wss://localhost/edge/fs"
	}
	s := &Server{repo: repo, reg: reg, cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.authMiddleware(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /v1/health", s.handleHealth)
	s.mux.HandleFunc("POST /v1/profiles", s.handleCreateProfile)
	s.mux.HandleFunc("POST /v1/profiles/{id}/versions", s.handlePublishProfile)
	s.mux.HandleFunc("POST /v1/sessions", s.handleCreateSession)
	s.mux.HandleFunc("GET /v1/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("POST /v1/sessions/{id}/stop", s.handleStopSession)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		"status": status,
		"db":     dbOK,
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
	sid, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "id generate failed", nil)
		return
	}
	edgeToken, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "token generate failed", nil)
		return
	}
	rate := profile.SampleRateHz(doc)
	sess := store.Session{
		ID:                    sid,
		TenantID:              req.TenantID,
		ProfileID:             pv.ProfileID,
		ProfileVersion:        pv.Version,
		Clock:                 clock,
		State:                 store.StateCreated,
		OwnerInstance:         s.cfg.OwnerInstance,
		CanonicalSampleRateHz: rate,
		Caller:                req.Caller,
		RecordingRef:          req.RecordingRef,
		Metadata:              req.Metadata,
	}
	if err := s.repo.CreateSession(r.Context(), sess); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "create session failed", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"session_id":               sess.ID,
		"profile_id":               sess.ProfileID,
		"profile_version":          sess.ProfileVersion,
		"clock":                    sess.Clock,
		"canonical_sample_rate_hz": sess.CanonicalSampleRateHz,
		"state":                    sess.State,
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
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id":               sess.ID,
		"profile_id":               sess.ProfileID,
		"profile_version":          sess.ProfileVersion,
		"clock":                    sess.Clock,
		"state":                    sess.State,
		"owner_instance":           sess.OwnerInstance,
		"canonical_sample_rate_hz": sess.CanonicalSampleRateHz,
	})
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
	// Phase B: no runtime actor — Draining then Completed (or Cancelled on operator reason).
	terminal := store.StateCompleted
	if req.Reason == "operator" || req.Reason == "cancel" {
		terminal = store.StateCancelled
	}
	_, err = s.repo.UpdateSessionState(r.Context(), id, store.StateDraining)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "drain failed", nil)
		return
	}
	sess, err = s.repo.UpdateSessionState(r.Context(), id, terminal)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "stop failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sess.ID,
		"state":      sess.State,
		"reason":     req.Reason,
	})
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

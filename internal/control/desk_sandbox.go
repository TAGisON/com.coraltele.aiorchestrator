package control

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/desk"
)

// A desk sandbox is a stateful text call against a published desk. It exercises
// the real engine and the real connectors, so an operator can rehearse a whole
// journey from the console before any telephony or vendor keys exist (§18 GUI).
type deskSandbox struct {
	ID        string       `json:"id"`
	DeskID    string       `json:"desk_id"`
	TenantID  string       `json:"tenant_id"`
	Version   int          `json:"version"`
	StartedAt time.Time    `json:"started_at"`
	Turns     []sandboxLog `json:"turns"`

	eng *desk.Engine
	mu  sync.Mutex
}

type sandboxLog struct {
	At          time.Time    `json:"at"`
	User        string       `json:"user"`
	Assistant   string       `json:"assistant"`
	Language    string       `json:"language"`
	PathID      string       `json:"path_id"`
	StepID      string       `json:"step_id"`
	Intent      string       `json:"intent"`
	End         bool         `json:"end"`
	Disposition string       `json:"disposition,omitempty"`
	Skills      []skillLogJS `json:"skills,omitempty"`
}

type skillLogJS struct {
	Name   string         `json:"name"`
	Status string         `json:"status"`
	Output map[string]any `json:"output,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type sandboxRegistry struct {
	mu   sync.Mutex
	byID map[string]*deskSandbox
}

func newSandboxRegistry() *sandboxRegistry {
	return &sandboxRegistry{byID: map[string]*deskSandbox{}}
}

func (r *sandboxRegistry) put(s *deskSandbox) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Keep the console light: drop the oldest rehearsals past a small cap.
	if len(r.byID) >= 64 {
		var oldestID string
		var oldest time.Time
		for id, cur := range r.byID {
			if oldestID == "" || cur.StartedAt.Before(oldest) {
				oldestID, oldest = id, cur.StartedAt
			}
		}
		delete(r.byID, oldestID)
	}
	r.byID[s.ID] = s
}

func (r *sandboxRegistry) get(id string) (*deskSandbox, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byID[id]
	return s, ok
}

func (r *sandboxRegistry) list() []*deskSandbox {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*deskSandbox, 0, len(r.byID))
	for _, s := range r.byID {
		out = append(out, s)
	}
	return out
}

func (s *Server) mountDeskSandboxRoutes() {
	s.mux.HandleFunc("POST /v1/desk-calls", s.handleSandboxStart)
	s.mux.HandleFunc("GET /v1/desk-calls/{id}", s.handleSandboxGet)
	s.mux.HandleFunc("POST /v1/desk-calls/{id}/turn", s.handleSandboxTurn)
	s.mux.HandleFunc("POST /v1/desk-calls/{id}/silence", s.handleSandboxSilence)
	s.mux.HandleFunc("POST /v1/desk-calls/{id}/language", s.handleSandboxLanguage)
}

func (s *Server) handleSandboxStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeskID   string `json:"desk_id"`
		TenantID string `json:"tenant_id"`
		Version  int    `json:"version"`
		Language string `json:"language"`
		ANI      string `json:"ani"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	deskID := strings.TrimSpace(req.DeskID)
	if deskID == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "desk_id required", nil)
		return
	}
	doc, err := s.loadDeskDoc(r.Context(), deskID, req.Version)
	if err != nil {
		writeError(w, http.StatusNotFound, CodeNotFound, "desk not found: "+err.Error(), nil)
		return
	}
	tenantID := resolveTenantID(r, pickString(req.TenantID, doc.TenantID))
	id := "call-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	eng := desk.NewEngine(doc, newSkillRunner(s.reg, doc, id, tenantID))
	if lang := strings.TrimSpace(req.Language); lang != "" {
		eng.SetLanguage(lang)
	}
	ani := strings.TrimSpace(req.ANI)
	if ani == "" {
		ani = "919800000000"
	}
	eng.SetAttribute(desk.AttrANI, ani)

	sb := &deskSandbox{
		ID: id, DeskID: doc.ID, TenantID: tenantID, Version: req.Version,
		StartedAt: time.Now().UTC(), eng: eng,
	}
	sb.record("", eng.Welcome())
	s.sandboxes.put(sb)
	writeJSON(w, http.StatusCreated, sb.snapshot())
}

func (sb *deskSandbox) record(user string, out desk.Outcome) {
	entry := sandboxLog{
		At: time.Now().UTC(), User: user, Assistant: out.Text, Language: out.Language,
		PathID: out.PathID, StepID: out.StepID, Intent: out.Intent,
		End: out.End, Disposition: out.Disposition,
	}
	for _, c := range out.SkillCalls {
		entry.Skills = append(entry.Skills, skillLogJS{
			Name: c.Name, Status: c.Status, Output: c.Output, Error: c.Error,
		})
	}
	sb.Turns = append(sb.Turns, entry)
}

func (sb *deskSandbox) snapshot() map[string]any {
	attrs := sb.eng.Attributes()
	masked := make(map[string]string, len(attrs))
	for k, v := range attrs {
		masked[k] = desk.Mask(k, v)
	}
	return map[string]any{
		"id": sb.ID, "desk_id": sb.DeskID, "tenant_id": sb.TenantID,
		"started_at": sb.StartedAt, "turns": sb.Turns,
		"language": sb.eng.Language(), "ended": sb.eng.Ended(),
		"disposition": sb.eng.Disposition(), "attributes": masked,
		"handoff": sb.eng.HandoffPack(), "state": sb.eng.State(),
	}
}

func (s *Server) sandboxFor(w http.ResponseWriter, r *http.Request) (*deskSandbox, bool) {
	sb, ok := s.sandboxes.get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, "desk call not found", nil)
		return nil, false
	}
	return sb, true
}

func (s *Server) handleSandboxGet(w http.ResponseWriter, r *http.Request) {
	sb, ok := s.sandboxFor(w, r)
	if !ok {
		return
	}
	sb.mu.Lock()
	defer sb.mu.Unlock()
	writeJSON(w, http.StatusOK, sb.snapshot())
}

func (s *Server) handleSandboxTurn(w http.ResponseWriter, r *http.Request) {
	sb, ok := s.sandboxFor(w, r)
	if !ok {
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "text required", nil)
		return
	}
	sb.mu.Lock()
	defer sb.mu.Unlock()
	out := sb.eng.Turn(r.Context(), req.Text)
	sb.record(req.Text, out)
	writeJSON(w, http.StatusOK, sb.snapshot())
}

func (s *Server) handleSandboxSilence(w http.ResponseWriter, r *http.Request) {
	sb, ok := s.sandboxFor(w, r)
	if !ok {
		return
	}
	sb.mu.Lock()
	defer sb.mu.Unlock()
	out := sb.eng.Silence(r.Context())
	sb.record("(silence)", out)
	writeJSON(w, http.StatusOK, sb.snapshot())
}

func (s *Server) handleSandboxLanguage(w http.ResponseWriter, r *http.Request) {
	sb, ok := s.sandboxFor(w, r)
	if !ok {
		return
	}
	var req struct {
		Language string `json:"language"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.eng.SetLanguage(strings.TrimSpace(req.Language))
	writeJSON(w, http.StatusOK, sb.snapshot())
}

func pickString(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

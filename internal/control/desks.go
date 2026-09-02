package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/desk"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// SkillLedger is the operator surface a stub skill gateway may expose
// (ledger read, failure injection, agent availability) without control
// importing the gateway package.
type SkillLedger interface {
	Ledger() map[string]any
	SetFailure(skill, status string)
	SetAgentAvailable(target string, available bool)
	Reset()
}

func (s *Server) mountDeskRoutes() {
	s.mux.HandleFunc("GET /v1/desks", s.handleListDesks)
	s.mux.HandleFunc("POST /v1/desks", s.handleCreateDesk)
	s.mux.HandleFunc("GET /v1/desks/{id}", s.handleGetDesk)
	s.mux.HandleFunc("PUT /v1/desks/{id}/draft", s.handlePutDeskDraft)
	s.mux.HandleFunc("GET /v1/desks/{id}/checklist", s.handleDeskChecklist)
	s.mux.HandleFunc("POST /v1/desks/{id}/publish", s.handlePublishDesk)
	s.mux.HandleFunc("GET /v1/desks/{id}/versions", s.handleListDeskVersions)
	s.mux.HandleFunc("POST /v1/desks/{id}/simulate", s.handleSimulateDesk)
	s.mux.HandleFunc("POST /v1/desk-presets/{name}", s.handleInstallPreset)
	s.mux.HandleFunc("GET /v1/desk-catalog", s.handleDeskCatalog)

	s.mux.HandleFunc("GET /v1/desk-skills/ledger", s.handleSkillLedger)
	s.mux.HandleFunc("POST /v1/desk-skills/failures", s.handleSkillFailure)
	s.mux.HandleFunc("POST /v1/desk-skills/agents", s.handleSkillAgent)
	s.mux.HandleFunc("POST /v1/desk-skills/reset", s.handleSkillReset)

	s.mux.HandleFunc("GET /v1/sessions/{id}/attributes", s.handleSessionAttributes)
	s.mux.HandleFunc("GET /v1/sessions/{id}/desk-state", s.handleSessionDeskState)
	s.mux.HandleFunc("GET /v1/sessions/{id}/handoff", s.handleSessionHandoff)
	s.mux.HandleFunc("GET /v1/sessions/{id}/skills", s.handleSessionSkills)
	s.mux.HandleFunc("PATCH /v1/sessions/{id}/disposition", s.handlePatchDisposition)

	s.mux.HandleFunc("GET /v1/tenant/properties", s.handleGetProperties)
	s.mux.HandleFunc("PUT /v1/tenant/properties", s.handlePutProperties)
	s.mux.HandleFunc("GET /v1/compliance/erasure", s.handleListErasure)
	s.mux.HandleFunc("POST /v1/compliance/erasure", s.handleCreateErasure)
	s.mux.HandleFunc("POST /v1/compliance/erasure/{id}/complete", s.handleCompleteErasure)
	s.mux.HandleFunc("GET /v1/compliance/pii-access", s.handleListPIIAccess)
	s.mountDeskSandboxRoutes()
}

type createDeskReq struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Purpose   string `json:"purpose"`
	TenantID  string `json:"tenant_id"`
	Preset    string `json:"preset"`
}

func (s *Server) handleCreateDesk(w http.ResponseWriter, r *http.Request) {
	var req createDeskReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "id required", nil)
		return
	}
	tenantID := resolveTenantID(r, req.TenantID)
	doc := desk.Doc{
		ID:        id,
		TenantID:  tenantID,
		Name:      strings.TrimSpace(req.Name),
		Direction: strings.TrimSpace(req.Direction),
		Purpose:   strings.TrimSpace(req.Purpose),
	}
	if req.Preset == "coral-tfn" {
		doc = desk.PresetCoralTFN(tenantID)
		doc.ID = id
		if strings.TrimSpace(req.Name) != "" {
			doc.Name = req.Name
		}
	}
	if req.Preset == "coral-xfer" {
		doc = desk.PresetCoralXfer(tenantID)
		doc.ID = id
		if strings.TrimSpace(req.Name) != "" {
			doc.Name = req.Name
		}
	}
	doc.Normalize()
	saved, err := s.saveDesk(r.Context(), doc, store.DeskStatusDraft)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "create desk failed: "+err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"desk": deskJSON(saved), "document": doc})
}

func (s *Server) saveDesk(ctx context.Context, doc desk.Doc, status string) (store.Desk, error) {
	rec, err := s.repo.UpsertDesk(ctx, store.Desk{
		ID:        doc.ID,
		TenantID:  doc.TenantID,
		Name:      doc.Name,
		Direction: doc.Direction,
		Purpose:   doc.Purpose,
		Status:    status,
	})
	if err != nil {
		return store.Desk{}, err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return store.Desk{}, err
	}
	if _, err := s.repo.SaveDeskDraft(ctx, doc.ID, raw); err != nil {
		return store.Desk{}, err
	}
	return rec, nil
}

func (s *Server) handleListDesks(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, r.URL.Query().Get("tenant_id"))
	list, err := s.repo.ListDesks(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list desks failed", nil)
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, d := range list {
		items = append(items, deskJSON(d))
	}
	writeJSON(w, http.StatusOK, map[string]any{"desks": items})
}

func (s *Server) handleGetDesk(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := s.repo.GetDesk(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "desk not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get desk failed", nil)
		return
	}
	out := map[string]any{"desk": deskJSON(rec)}
	if draft, err := s.repo.GetDeskDraft(r.Context(), id); err == nil {
		out["document"] = json.RawMessage(draft.Doc)
		out["draft_updated_at"] = draft.UpdatedAt
	}
	if rec.CurrentVersion > 0 {
		if v, err := s.repo.GetDeskVersion(r.Context(), id, rec.CurrentVersion); err == nil {
			out["published"] = map[string]any{
				"version": v.Version, "profile_id": v.ProfileID,
				"profile_version": v.ProfileVersion, "content_hash": v.ContentHash,
				"published_at": v.PublishedAt,
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePutDeskDraft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	doc, err := decodeDeskDoc(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	doc.ID = id
	if doc.TenantID == "" {
		doc.TenantID = defaultTenantID
	}
	doc.Normalize()
	if _, err := s.repo.GetDesk(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "desk not found", nil)
		return
	}
	rec, err := s.saveDesk(r.Context(), doc, store.DeskStatusDraft)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "save draft failed", nil)
		return
	}
	check := desk.Validate(doc)
	writeJSON(w, http.StatusOK, map[string]any{"desk": deskJSON(rec), "checklist": check})
}

func decodeDeskDoc(r *http.Request) (desk.Doc, error) {
	var body struct {
		Document json.RawMessage `json:"document"`
	}
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		return desk.Doc{}, err
	}
	if err := json.Unmarshal(raw, &body); err == nil && len(body.Document) > 0 {
		var d desk.Doc
		if err := json.Unmarshal(body.Document, &d); err != nil {
			return desk.Doc{}, fmt.Errorf("document invalid: %w", err)
		}
		return d, nil
	}
	var d desk.Doc
	if err := json.Unmarshal(raw, &d); err != nil {
		return desk.Doc{}, fmt.Errorf("desk document invalid: %w", err)
	}
	return d, nil
}

func (s *Server) loadDeskDoc(ctx context.Context, id string, version int) (desk.Doc, error) {
	if version > 0 {
		v, err := s.repo.GetDeskVersion(ctx, id, version)
		if err != nil {
			return desk.Doc{}, err
		}
		var d desk.Doc
		if err := json.Unmarshal(v.Doc, &d); err != nil {
			return desk.Doc{}, err
		}
		d.Normalize()
		return d, nil
	}
	draft, err := s.repo.GetDeskDraft(ctx, id)
	if err != nil {
		return desk.Doc{}, err
	}
	var d desk.Doc
	if err := json.Unmarshal(draft.Doc, &d); err != nil {
		return desk.Doc{}, err
	}
	d.Normalize()
	return d, nil
}

func (s *Server) handleDeskChecklist(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	doc, err := s.loadDeskDoc(r.Context(), id, 0)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "desk draft not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "load draft failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"checklist": desk.Validate(doc)})
}

type publishDeskReq struct {
	PublishedBy string `json:"published_by"`
}

func (s *Server) handlePublishDesk(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req publishDeskReq
	_ = decodeJSON(r, &req)

	doc, err := s.loadDeskDoc(r.Context(), id, 0)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "desk draft not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "load draft failed", nil)
		return
	}
	check := desk.Validate(doc)
	if !check.Publishable {
		writeError(w, http.StatusUnprocessableEntity, CodeProfileInvalid, "desk checklist incomplete",
			map[string]any{"checklist": check})
		return
	}
	if err := desk.ValidateRuntimeEngineCoverage(doc, desk.LabEngineLanguages()); err != nil {
		var ce *desk.CompileError
		details := map[string]any{}
		if errors.As(err, &ce) {
			details = ce.Details
		}
		writeError(w, http.StatusUnprocessableEntity, CodeProfileInvalid, err.Error(), details)
		return
	}
	profileRaw, err := desk.Compile(doc)
	if err != nil {
		var ce *desk.CompileError
		details := map[string]any{}
		if errors.As(err, &ce) {
			details = ce.Details
		}
		writeError(w, http.StatusUnprocessableEntity, CodeProfileInvalid, "desk compile failed: "+err.Error(), details)
		return
	}
	parsed, err := profile.Parse(profileRaw)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "compiled profile unreadable", nil)
		return
	}
	if err := profile.Validate(parsed, s.reg); err != nil {
		var ve *profile.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, CodeProfileInvalid, "compiled profile invalid: "+ve.Message, ve.Details)
			return
		}
		writeError(w, http.StatusUnprocessableEntity, CodeProfileInvalid, err.Error(), nil)
		return
	}
	if _, err := s.repo.GetProfile(r.Context(), doc.ID); errors.Is(err, store.ErrNotFound) {
		if err := s.repo.CreateProfile(r.Context(), store.Profile{
			ID: doc.ID, TenantID: doc.TenantID, DisplayName: doc.Name,
		}); err != nil && !errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusInternalServerError, CodeInternal, "create profile failed", nil)
			return
		}
	}
	pv, err := s.repo.PublishVersion(r.Context(), doc.ID, profileRaw)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "publish profile failed", nil)
		return
	}
	hash, _ := desk.ContentHash(doc)
	deskRaw, _ := json.Marshal(doc)
	dv, err := s.repo.PublishDeskVersion(r.Context(), store.DeskVersion{
		DeskID:         doc.ID,
		Doc:            deskRaw,
		ProfileID:      pv.ProfileID,
		ProfileVersion: pv.Version,
		ContentHash:    hash,
		PublishedBy:    strings.TrimSpace(req.PublishedBy),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "publish desk failed", nil)
		return
	}
	applog.Info("desk published", "desk", doc.ID, "desk_version", dv.Version,
		"profile", pv.ProfileID, "profile_version", pv.Version)
	writeJSON(w, http.StatusCreated, map[string]any{
		"desk_id":         dv.DeskID,
		"desk_version":    dv.Version,
		"profile_id":      pv.ProfileID,
		"profile_version": pv.Version,
		"content_hash":    dv.ContentHash,
		"checklist":       check,
	})
}

func (s *Server) handleListDeskVersions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	list, err := s.repo.ListDeskVersions(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list versions failed", nil)
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, v := range list {
		items = append(items, map[string]any{
			"version": v.Version, "profile_id": v.ProfileID, "profile_version": v.ProfileVersion,
			"content_hash": v.ContentHash, "published_by": v.PublishedBy, "published_at": v.PublishedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": items})
}

type simulateReq struct {
	Turns          []string `json:"turns"`
	Language       string   `json:"language"`
	Version        int      `json:"version"`
	ANI            string   `json:"ani"`
	IncludeWelcome *bool    `json:"include_welcome"`
	Silence        []int    `json:"silence_after"`
}

func (s *Server) handleSimulateDesk(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req simulateReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	doc, err := s.loadDeskDoc(r.Context(), id, req.Version)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "desk not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "load desk failed", nil)
		return
	}
	tenantID := resolveTenantID(r, doc.TenantID)
	sessionID := "sim-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	eng := desk.NewEngine(doc, newSkillRunner(s.reg, doc, sessionID, tenantID))
	if lang := strings.TrimSpace(req.Language); lang != "" {
		eng.SetLanguage(lang)
	}
	if ani := strings.TrimSpace(req.ANI); ani != "" {
		eng.SetAttribute(desk.AttrANI, ani)
	}

	steps := make([]map[string]any, 0, len(req.Turns)+1)
	if req.IncludeWelcome == nil || *req.IncludeWelcome {
		out := eng.Welcome()
		steps = append(steps, simStep("", out))
	}
	silenceAfter := map[int]bool{}
	for _, idx := range req.Silence {
		silenceAfter[idx] = true
	}
	for i, turn := range req.Turns {
		out := eng.Turn(r.Context(), turn)
		steps = append(steps, simStep(turn, out))
		if silenceAfter[i] {
			sout := eng.Silence(r.Context())
			steps = append(steps, simStep("(silence)", sout))
			if sout.End {
				break
			}
		}
		if out.End {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id":  sessionID,
		"desk_id":     doc.ID,
		"steps":       steps,
		"attributes":  eng.Attributes(),
		"disposition": eng.Disposition(),
		"ended":       eng.Ended(),
		"handoff":     eng.HandoffPack(),
		"state":       eng.State(),
	})
}

func simStep(user string, out desk.Outcome) map[string]any {
	item := map[string]any{
		"user":        user,
		"assistant":   out.Text,
		"language":    out.Language,
		"path_id":     out.PathID,
		"step_id":     out.StepID,
		"intent":      out.Intent,
		"tier":        out.Tier,
		"end":         out.End,
		"disposition": out.Disposition,
	}
	if len(out.SkillCalls) > 0 {
		calls := make([]map[string]any, 0, len(out.SkillCalls))
		for _, c := range out.SkillCalls {
			calls = append(calls, map[string]any{
				"name": c.Name, "status": c.Status, "output": c.Output, "error": c.Error,
			})
		}
		item["skills"] = calls
	}
	return item
}

func (s *Server) handleInstallPreset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req createDeskReq
	_ = decodeJSON(r, &req)
	tenantID := resolveTenantID(r, req.TenantID)
	var doc desk.Doc
	switch name {
	case "coral-tfn":
		doc = desk.PresetCoralTFN(tenantID)
	case "coral-xfer":
		doc = desk.PresetCoralXfer(tenantID)
	default:
		writeError(w, http.StatusNotFound, CodeNotFound, "unknown preset", map[string]any{"preset": name})
		return
	}
	if id := strings.TrimSpace(req.ID); id != "" {
		doc.ID = id
	}
	doc.Normalize()
	rec, err := s.saveDesk(r.Context(), doc, store.DeskStatusDraft)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "install preset failed: "+err.Error(), nil)
		return
	}
	if name == "coral-tfn" || name == "coral-xfer" {
		if err := s.seedCoralProductKB(r.Context(), tenantID); err != nil {
			applog.Warn("coral product kb seed failed", "err", err)
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"desk":      deskJSON(rec),
		"document":  doc,
		"checklist": desk.Validate(doc),
	})
}

func (s *Server) seedCoralProductKB(ctx context.Context, tenantID string) error {
	id, err := newID()
	if err != nil {
		return err
	}
	doc := store.KBDocument{
		ID: id, TenantID: tenantID, Collection: "coral-products",
		URI: "preset://coral-tfn", Status: store.KBIndexing,
	}
	if err := s.repo.CreateKBDocument(ctx, doc); err != nil {
		return err
	}
	chunks := chunkText(id, tenantID, "coral-products", doc.URI, desk.CoralProductKnowledge())
	if err := s.repo.ReplaceKBChunks(ctx, id, chunks); err != nil {
		_, _ = s.repo.UpdateKBDocumentStatus(ctx, id, store.KBFailed, err.Error())
		return err
	}
	_, err = s.repo.UpdateKBDocumentStatus(ctx, id, store.KBReady, "")
	return err
}

func (s *Server) handleDeskCatalog(w http.ResponseWriter, r *http.Request) {
	skillNames := make([]string, 0, len(desk.SkillAuthority))
	for name := range desk.SkillAuthority {
		skillNames = append(skillNames, name)
	}
	sort.Strings(skillNames)
	skills := make([]map[string]any, 0, len(skillNames))
	for _, name := range skillNames {
		skills = append(skills, map[string]any{"name": name, "authority": desk.SkillAuthority[name]})
	}

	productIDs := make([]string, 0, len(desk.ProductVocabulary))
	for id := range desk.ProductVocabulary {
		productIDs = append(productIDs, id)
	}
	sort.Strings(productIDs)
	products := make([]map[string]any, 0, len(productIDs))
	for _, id := range productIDs {
		products = append(products, map[string]any{
			"id":    id,
			"en-IN": desk.DisplayValue(desk.AttrProduct, id, "en-IN"),
			"hi-IN": desk.DisplayValue(desk.AttrProduct, id, "hi-IN"),
		})
	}

	knowledge := make([]string, 0, 2)
	for _, rec := range s.reg.List(port.PortKnowledge) {
		knowledge = append(knowledge, string(rec.ID))
	}
	sort.Strings(knowledge)

	writeJSON(w, http.StatusOK, map[string]any{
		"step_types":   []string{desk.StepSay, desk.StepAsk, desk.StepConfirm, desk.StepChoice, desk.StepAction, desk.StepEnd},
		"validations":  []string{desk.ValidateFreeText, desk.ValidateEmail, desk.ValidatePhone, desk.ValidateNumber, desk.ValidateChoice, desk.ValidateProduct, desk.ValidateYesNo},
		"branches":     []string{desk.BranchOK, desk.BranchFail, desk.BranchDuplicate, desk.BranchTimeout, desk.BranchUnavailable},
		"repairs":      []string{desk.RepairReprompt, desk.RepairNext, desk.RepairClarify, desk.RepairFallback, desk.RepairEnd},
		"dispositions":        desk.Dispositions,
		"purposes":            desk.Purposes,
		"products":            products,
		"skills":              skills,
		"knowledge_providers": knowledge,
		"languages":           []string{"en-IN", "hi-IN"},
		"defaults":            desk.DefaultCX(),
		"prompt_slots": []string{desk.PromptWelcome, desk.PromptClarify, desk.PromptSilence1, desk.PromptSilence2,
			desk.PromptSilenceGoodbye, desk.PromptClosing, desk.PromptAnythingElse, desk.PromptSystemDown, desk.PromptHold},
	})
}

func (s *Server) skillLedger() (SkillLedger, bool) {
	rec, ok := s.reg.Get(port.GatewayID(desk.DefaultSkillGateway))
	if !ok {
		return nil, false
	}
	l, cast := rec.Instance.(SkillLedger)
	return l, cast
}

func (s *Server) handleSkillLedger(w http.ResponseWriter, r *http.Request) {
	l, ok := s.skillLedger()
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, "desk-skills gateway not registered", nil)
		return
	}
	writeJSON(w, http.StatusOK, l.Ledger())
}

func (s *Server) handleSkillFailure(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Skill  string `json:"skill"`
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	l, ok := s.skillLedger()
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, "desk-skills gateway not registered", nil)
		return
	}
	l.SetFailure(strings.TrimSpace(req.Skill), strings.TrimSpace(req.Status))
	writeJSON(w, http.StatusOK, l.Ledger())
}

func (s *Server) handleSkillAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target    string `json:"target"`
		Available bool   `json:"available"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	l, ok := s.skillLedger()
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, "desk-skills gateway not registered", nil)
		return
	}
	l.SetAgentAvailable(strings.TrimSpace(req.Target), req.Available)
	writeJSON(w, http.StatusOK, map[string]any{"target": req.Target, "available": req.Available})
}

func (s *Server) handleSkillReset(w http.ResponseWriter, r *http.Request) {
	l, ok := s.skillLedger()
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, "desk-skills gateway not registered", nil)
		return
	}
	l.Reset()
	writeJSON(w, http.StatusOK, map[string]any{"reset": true})
}

func (s *Server) handleSessionAttributes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	reveal := r.URL.Query().Get("reveal") == "true"
	actor := strings.TrimSpace(r.URL.Query().Get("actor"))
	rows, err := s.repo.ListSessionAttributes(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list attributes failed", nil)
		return
	}
	if reveal {
		if actor == "" {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "actor required to reveal confidential attributes", nil)
			return
		}
		var keys []string
		for _, a := range rows {
			if desk.ClassOf(a.Key) == "confidential" {
				keys = append(keys, a.Key)
			}
		}
		tenantID := ""
		if sess, err := s.repo.GetSession(r.Context(), id); err == nil {
			tenantID = sess.TenantID
		}
		if _, err := s.repo.AppendPIIAccess(r.Context(), store.PIIAccess{
			TenantID: tenantID, SessionID: id, Actor: actor,
			Keys: strings.Join(keys, ","), Reason: r.URL.Query().Get("reason"),
		}); err != nil {
			applog.Warn("pii access audit failed", "session", id, "err", err)
		}
	}
	items := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		value := a.Value
		if !reveal {
			value = desk.Mask(a.Key, a.Value)
		}
		items = append(items, map[string]any{
			"key": a.Key, "value": value, "class": a.Class, "updated_at": a.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"attributes": items, "revealed": reveal})
}

func (s *Server) deskControllerFor(sessionID string) (*deskController, bool) {
	sr, ok := s.rt.(*SessionRuntime)
	if !ok || sr == nil {
		return nil, false
	}
	return sr.DeskController(sessionID)
}

func (s *Server) handleSessionDeskState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctrl, ok := s.deskControllerFor(id)
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, "no desk running for session", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"desk_id": ctrl.Engine().Doc().ID,
		"state":   ctrl.Engine().State(),
	})
}

func (s *Server) handleSessionHandoff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if ctrl, ok := s.deskControllerFor(id); ok {
		writeJSON(w, http.StatusOK, map[string]any{"handoff": ctrl.HandoffPack()})
		return
	}
	rows, err := s.repo.ListSessionAttributes(r.Context(), id)
	if err != nil || len(rows) == 0 {
		writeError(w, http.StatusNotFound, CodeNotFound, "no handoff pack for session", nil)
		return
	}
	attrs := map[string]string{}
	for _, a := range rows {
		attrs[a.Key] = a.Value
	}
	writeJSON(w, http.StatusOK, map[string]any{"handoff": map[string]any{
		"target":     attrs[desk.AttrTransferTarget],
		"language":   attrs[desk.AttrLanguage],
		"summary":    attrs[desk.AttrSummary],
		"ticket_id":  attrs[desk.AttrTicketID],
		"attributes": attrs,
	}})
}

func (s *Server) handleSessionSkills(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rows, err := s.repo.ListSkillInvocations(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list skill invocations failed", nil)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, inv := range rows {
		items = append(items, map[string]any{
			"id": inv.ID, "skill": inv.Skill, "status": inv.Status,
			"args": json.RawMessage(inv.Args), "output": json.RawMessage(inv.Output),
			"error": inv.Error, "created_at": inv.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"skill_invocations": items})
}

func (s *Server) handlePatchDisposition(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Final string `json:"final"`
		Actor string `json:"actor"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	final := strings.TrimSpace(req.Final)
	if final == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "final required", nil)
		return
	}
	valid := false
	for _, d := range desk.Dispositions {
		if d == final {
			valid = true
			break
		}
	}
	if !valid {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "unknown disposition code",
			map[string]any{"allowed": desk.Dispositions})
		return
	}
	cur, err := s.repo.GetSessionDisposition(r.Context(), id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get disposition failed", nil)
		return
	}
	out, err := s.repo.UpsertSessionDisposition(r.Context(), store.SessionDisposition{
		SessionID:  id,
		Suggestion: cur.Suggestion,
		TemplateID: cur.TemplateID,
		Source:     "supervisor",
		Final:      final,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "set disposition failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": out.SessionID, "suggestion": out.Suggestion,
		"final": out.Final, "source": out.Source, "updated_at": out.UpdatedAt,
	})
}

// Tenant properties are stored as system settings with a stable prefix (§6.10).
const propertyPrefix = "property."

var knownProperties = []string{
	"max_concurrent_sessions", "admission_mode", "retention_transcript_days",
	"retention_attributes_days", "retention_audit_days", "retention_recording_days",
	"pii_reveal_requires_reason",
}

func (s *Server) handleGetProperties(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, r.URL.Query().Get("tenant_id"))
	list, err := s.repo.ListSystemSettings(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list properties failed", nil)
		return
	}
	props := defaultProperties()
	for _, st := range list {
		if strings.HasPrefix(st.Key, propertyPrefix) {
			props[strings.TrimPrefix(st.Key, propertyPrefix)] = st.Value
		}
	}
	active := 0
	if n, err := s.repo.CountActiveSessions(r.Context(), tenantID); err == nil {
		active = n
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id": tenantID, "properties": props,
		"known": knownProperties, "active_sessions": active,
	})
}

func defaultProperties() map[string]string {
	return map[string]string{
		"max_concurrent_sessions":    "20",
		"admission_mode":             "reject",
		"retention_transcript_days":  "90",
		"retention_attributes_days":  "90",
		"retention_audit_days":       "365",
		"retention_recording_days":   "30",
		"pii_reveal_requires_reason": "false",
	}
}

func (s *Server) handlePutProperties(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID   string            `json:"tenant_id"`
		Properties map[string]string `json:"properties"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	tenantID := resolveTenantID(r, req.TenantID)
	for k, v := range req.Properties {
		if _, err := s.repo.UpsertSystemSetting(r.Context(), store.SystemSetting{
			TenantID: tenantID, Key: propertyPrefix + k, Value: v,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "save property failed", nil)
			return
		}
	}
	s.handleGetProperties(w, r)
}

func (s *Server) handleListErasure(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, r.URL.Query().Get("tenant_id"))
	list, err := s.repo.ListErasureRequests(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list erasure failed", nil)
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, e := range list {
		items = append(items, map[string]any{
			"id": e.ID, "subject_ref": e.SubjectRef, "scope": e.Scope, "status": e.Status,
			"requested_by": e.RequestedBy, "requested_at": e.RequestedAt, "completed_at": e.CompletedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"erasure_requests": items})
}

func (s *Server) handleCreateErasure(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID    string `json:"tenant_id"`
		SubjectRef  string `json:"subject_ref"`
		Scope       string `json:"scope"`
		RequestedBy string `json:"requested_by"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	if strings.TrimSpace(req.SubjectRef) == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "subject_ref required", nil)
		return
	}
	id, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "id generate failed", nil)
		return
	}
	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		scope = "all"
	}
	out, err := s.repo.CreateErasureRequest(r.Context(), store.ErasureRequest{
		ID: id, TenantID: resolveTenantID(r, req.TenantID), SubjectRef: strings.TrimSpace(req.SubjectRef),
		Scope: scope, Status: "queued", RequestedBy: strings.TrimSpace(req.RequestedBy),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "create erasure failed", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": out.ID, "subject_ref": out.SubjectRef, "scope": out.Scope,
		"status": out.Status, "requested_at": out.RequestedAt,
	})
}

func (s *Server) handleCompleteErasure(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	out, err := s.repo.CompleteErasureRequest(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "erasure request not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "complete erasure failed", nil)
		return
	}
	if strings.HasPrefix(out.SubjectRef, "session:") {
		sessionID := strings.TrimPrefix(out.SubjectRef, "session:")
		if err := s.repo.PurgeSessionData(r.Context(), sessionID); err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "purge failed", nil)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": out.ID, "status": out.Status, "completed_at": out.CompletedAt,
	})
}

func (s *Server) handleListPIIAccess(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.repo.ListPIIAccess(r.Context(), sessionID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "list pii access failed", nil)
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, e := range list {
		items = append(items, map[string]any{
			"id": e.ID, "session_id": e.SessionID, "actor": e.Actor,
			"keys": e.Keys, "reason": e.Reason, "created_at": e.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"pii_access": items})
}

// EndSessionFromDesk stops a session after the guided path speaks its closing line.
func (s *Server) EndSessionFromDesk(ctx context.Context, sessionID, disposition string) {
	sess, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		applog.Warn("desk end: session lookup failed", "session", sessionID, "err", err)
		return
	}
	switch sess.State {
	case store.StateCompleted, store.StateCancelled, store.StateFailed:
		return
	}
	if _, err := s.repo.UpdateSessionState(ctx, sessionID, store.StateDraining); err != nil {
		applog.Warn("desk end: drain failed", "session", sessionID, "err", err)
	}
	terminal := store.StateCompleted
	if s.rt != nil {
		if term, err := s.rt.StopSession(ctx, sessionID, "desk_end"); err == nil && term != "" {
			terminal = term
		}
	}
	updated, err := s.repo.UpdateSessionState(ctx, sessionID, terminal)
	if err != nil {
		applog.Warn("desk end: terminal state failed", "session", sessionID, "err", err)
		return
	}
	s.onSessionTerminal(ctx, updated, terminal)
	applog.Info("desk end complete", "session", sessionID, "state", terminal, "disposition", disposition)
}

func deskJSON(d store.Desk) map[string]any {
	return map[string]any{
		"id": d.ID, "tenant_id": d.TenantID, "name": d.Name,
		"direction": d.Direction, "purpose": d.Purpose, "status": d.Status,
		"current_version": d.CurrentVersion, "profile_id": d.ProfileID,
		"created_at": d.CreatedAt, "updated_at": d.UpdatedAt,
	}
}

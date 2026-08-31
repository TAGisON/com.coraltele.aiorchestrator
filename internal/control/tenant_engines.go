package control

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

const (
	defaultTenantID    = "default"
	defaultListenID    = "sarvam-stt"
	defaultThinkID     = "sarvam-llm"
	defaultSpeakID     = "sarvam-tts"
	enginesSourceStore = "store"
	enginesSourceProps = "properties"
)

// EngineDefaults are boot-properties seed when no tenant_engines row exists.
var EngineDefaults = store.GatewayBinding{
	Listen: defaultListenID,
	Think:  defaultThinkID,
	Speak:  defaultSpeakID,
}

type tenantEnginesResponse struct {
	TenantID string `json:"tenant_id"`
	Listen   string `json:"listen"`
	Think    string `json:"think"`
	Speak    string `json:"speak"`
	Source   string `json:"source"`
}

type putTenantEnginesReq struct {
	Listen string `json:"listen"`
	Think  string `json:"think"`
	Speak  string `json:"speak"`
}

func (s *Server) handleGetTenantEngines(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, "")
	binding, source, err := s.resolveTenantEngines(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "resolve tenant engines failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, tenantEnginesResponse{
		TenantID: tenantID,
		Listen:   binding.Listen,
		Think:    binding.Think,
		Speak:    binding.Speak,
		Source:   source,
	})
}

func (s *Server) handlePutTenantEngines(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantID(r, "")
	var req putTenantEnginesReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	req.Listen = strings.TrimSpace(req.Listen)
	req.Think = strings.TrimSpace(req.Think)
	req.Speak = strings.TrimSpace(req.Speak)
	if req.Listen == "" || req.Think == "" || req.Speak == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "listen, think, and speak required", nil)
		return
	}
	binding := store.GatewayBinding{Listen: req.Listen, Think: req.Think, Speak: req.Speak}
	if err := validateGatewayBinding(s.reg, binding); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeBadRequest, err.Error(), map[string]any{"reason": "unknown_or_invalid_gateway"})
		return
	}
	te, err := s.repo.UpsertTenantEngines(r.Context(), store.TenantEngines{
		TenantID: tenantID,
		ListenID: binding.Listen,
		ThinkID:  binding.Think,
		SpeakID:  binding.Speak,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "upsert tenant engines failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, tenantEnginesResponse{
		TenantID: te.TenantID,
		Listen:   te.ListenID,
		Think:    te.ThinkID,
		Speak:    te.SpeakID,
		Source:   enginesSourceStore,
	})
}

func resolveTenantID(r *http.Request, bodyTenant string) string {
	if h := strings.TrimSpace(r.Header.Get("X-Tenant-ID")); h != "" {
		return h
	}
	if strings.TrimSpace(bodyTenant) != "" {
		return strings.TrimSpace(bodyTenant)
	}
	return defaultTenantID
}

func propertiesGatewayDefaults() store.GatewayBinding {
	b := EngineDefaults
	if b.Listen == "" {
		b.Listen = defaultListenID
	}
	if b.Think == "" {
		b.Think = defaultThinkID
	}
	if b.Speak == "" {
		b.Speak = defaultSpeakID
	}
	return b
}

func (s *Server) resolveTenantEngines(ctx context.Context, tenantID string) (store.GatewayBinding, string, error) {
	te, err := s.repo.GetTenantEngines(ctx, tenantID)
	if err == nil {
		return te.Binding(), enginesSourceStore, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return store.GatewayBinding{}, "", err
	}
	// Seed from boot properties defaults into DB when possible so UI/Control see store source.
	def := propertiesGatewayDefaults()
	if te2, err := s.repo.UpsertTenantEngines(ctx, store.TenantEngines{
		TenantID: tenantID,
		ListenID: def.Listen,
		ThinkID:  def.Think,
		SpeakID:  def.Speak,
	}); err == nil {
		return te2.Binding(), enginesSourceStore, nil
	}
	return def, enginesSourceProps, nil
}

func validateGatewayBinding(reg port.Registry, b store.GatewayBinding) error {
	checks := []struct {
		id   string
		kind port.PortKind
		slot string
	}{
		{b.Listen, port.PortListen, "listen"},
		{b.Think, port.PortThink, "think"},
		{b.Speak, port.PortSpeak, "speak"},
	}
	for _, c := range checks {
		if c.id == "" {
			return fmt.Errorf("%s gateway id required", c.slot)
		}
		rec, ok := reg.Get(port.GatewayID(c.id))
		if !ok {
			return fmt.Errorf("gateway id %s not registered", c.id)
		}
		if rec.Port != c.kind {
			return fmt.Errorf("gateway id %s wrong port (want %s got %s)", c.id, c.kind, rec.Port)
		}
	}
	return nil
}

func warnProfileEngineConflict(doc profile.Document, binding store.GatewayBinding) {
	if profile.Family(doc) != "contact-agent" {
		return
	}
	warnSlot := func(slot string, providers []string, want string) {
		if len(providers) == 0 || want == "" {
			return
		}
		for _, p := range providers {
			if p != "" && p != want {
				applog.Info("tenant_engines_conflict",
					"family", "contact-agent",
					"slot", slot,
					"profile_provider", p,
					"tenant_engine", want,
					"resolution", "tenant_engines_win",
				)
				return
			}
		}
	}
	warnSlot("listen", doc.Routers.Listen.Providers, binding.Listen)
	warnSlot("think", doc.Routers.Think.Providers, binding.Think)
	warnSlot("speak", doc.Routers.Speak.Providers, binding.Speak)
}

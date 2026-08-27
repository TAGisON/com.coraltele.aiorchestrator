package profile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

// Document is the PROFILE_SCHEMA subset needed for Phase B validation and session pin.
type Document struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	Audio    struct {
		CanonicalSampleRateHz int `json:"canonical_sample_rate_hz"`
		FrameMs               int `json:"frame_ms"`
	} `json:"audio"`
	Modes struct {
		Listen bool `json:"listen"`
		Speak  bool `json:"speak"`
		Think  bool `json:"think"`
		Talk   bool `json:"talk"`
	} `json:"modes"`
	Language struct {
		Behaviour string `json:"behaviour"`
	} `json:"language"`
	Grounding struct {
		Required bool `json:"required"`
	} `json:"grounding"`
	Routers struct {
		Listen     RouterProviders `json:"listen"`
		Think      RouterProviders `json:"think"`
		Speak      RouterProviders `json:"speak"`
		Knowledge  RouterProviders `json:"knowledge"`
		Translate  RouterProviders `json:"translate"`
	} `json:"routers"`
	Skills struct {
		Allowed     []string                    `json:"allowed"`
		Definitions map[string]SkillDefinition  `json:"definitions"`
	} `json:"skills"`
	Knowledge struct {
		HTTPKB *struct {
			Gateway string `json:"gateway"`
		} `json:"http_kb"`
	} `json:"knowledge"`
}

type RouterProviders struct {
	Providers []string `json:"providers"`
}

type SkillDefinition struct {
	Gateway string `json:"gateway"`
}

// ValidationError is returned for profile_invalid (HTTP 422).
type ValidationError struct {
	Message string
	Details map[string]any
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// Parse unmarshals a profile document JSON body.
func Parse(raw json.RawMessage) (Document, error) {
	var d Document
	if err := json.Unmarshal(raw, &d); err != nil {
		return Document{}, fmt.Errorf("invalid profile json: %w", err)
	}
	return d, nil
}

// Validate checks PROFILE_SCHEMA §4 minimum gates that do not require composers,
// and that every referenced gateway id exists in the registry on the expected port.
func Validate(doc Document, reg port.Registry) error {
	if !doc.Modes.Listen && !doc.Modes.Speak && !doc.Modes.Think && !doc.Modes.Talk {
		return &ValidationError{Message: "at least one mode must be on", Details: map[string]any{"field": "modes"}}
	}
	if doc.Audio.CanonicalSampleRateHz != 0 {
		r := doc.Audio.CanonicalSampleRateHz
		if r < 8000 || r > 48000 {
			return &ValidationError{
				Message: fmt.Sprintf("audio.canonical_sample_rate_hz out of range: %d", r),
				Details: map[string]any{"field": "audio.canonical_sample_rate_hz"},
			}
		}
	}
	if doc.Grounding.Required && len(doc.Routers.Knowledge.Providers) == 0 {
		return &ValidationError{
			Message: "grounding.required needs at least one knowledge provider",
			Details: map[string]any{"field": "routers.knowledge.providers"},
		}
	}
	beh := strings.TrimSpace(doc.Language.Behaviour)
	if beh != "" && beh != "none" && len(doc.Routers.Translate.Providers) == 0 {
		return &ValidationError{
			Message: "language.behaviour requires translate providers",
			Details: map[string]any{"field": "routers.translate.providers"},
		}
	}
	for _, name := range doc.Skills.Allowed {
		if doc.Skills.Definitions == nil {
			return &ValidationError{
				Message: "skills.allowed entry missing definitions: " + name,
				Details: map[string]any{"skill": name},
			}
		}
		if _, ok := doc.Skills.Definitions[name]; !ok {
			return &ValidationError{
				Message: "skills.allowed entry missing definitions: " + name,
				Details: map[string]any{"skill": name},
			}
		}
	}

	type ref struct {
		id   string
		kind port.PortKind
		path string
	}
	var refs []ref
	for _, id := range doc.Routers.Listen.Providers {
		refs = append(refs, ref{id, port.PortListen, "routers.listen.providers"})
	}
	for _, id := range doc.Routers.Speak.Providers {
		refs = append(refs, ref{id, port.PortSpeak, "routers.speak.providers"})
	}
	for _, id := range doc.Routers.Think.Providers {
		refs = append(refs, ref{id, port.PortThink, "routers.think.providers"})
	}
	for _, id := range doc.Routers.Knowledge.Providers {
		refs = append(refs, ref{id, port.PortKnowledge, "routers.knowledge.providers"})
	}
	for _, id := range doc.Routers.Translate.Providers {
		refs = append(refs, ref{id, port.PortTranslate, "routers.translate.providers"})
	}
	if doc.Knowledge.HTTPKB != nil && doc.Knowledge.HTTPKB.Gateway != "" {
		refs = append(refs, ref{doc.Knowledge.HTTPKB.Gateway, port.PortKnowledge, "knowledge.http_kb.gateway"})
	}
	if doc.Skills.Definitions != nil {
		for name, def := range doc.Skills.Definitions {
			if def.Gateway == "" {
				continue
			}
			refs = append(refs, ref{def.Gateway, port.PortSkill, "skills.definitions." + name + ".gateway"})
		}
	}

	for _, r := range refs {
		rec, ok := reg.Get(port.GatewayID(r.id))
		if !ok {
			return &ValidationError{
				Message: "gateway id " + r.id + " not registered",
				Details: map[string]any{"gateway_id": r.id, "path": r.path},
			}
		}
		if rec.Port != r.kind {
			return &ValidationError{
				Message: fmt.Sprintf("gateway id %s wrong port (want %s got %s)", r.id, r.kind, rec.Port),
				Details: map[string]any{"gateway_id": r.id, "path": r.path, "want_port": string(r.kind), "got_port": string(rec.Port)},
			}
		}
	}
	return nil
}

// SampleRateHz returns the pinned rate (default 16000).
func SampleRateHz(doc Document) int {
	if doc.Audio.CanonicalSampleRateHz == 0 {
		return 16000
	}
	return doc.Audio.CanonicalSampleRateHz
}

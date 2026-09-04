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
	Metadata struct {
		DisplayName string `json:"display_name"`
		Family      string `json:"family"`
	} `json:"metadata"`
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
	// Persona holds voice/tone/instructions (PROFILE_SCHEMA). Voice required when Talk/Speak.
	Persona  Persona `json:"persona"`
	Language struct {
		Behaviour     string   `json:"behaviour"`
		Primary       string   `json:"primary"`
		Allowed       []string `json:"allowed"`
		AutoDetect    bool     `json:"auto_detect"`
		MidCallSwitch bool     `json:"mid_call_switch"`
	} `json:"language"`
	HotSwapAllowed []string `json:"hot_swap_allowed"`
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
	// Rules are declarative Think-path policies (RULES_AND_SKILLS.md).
	Rules []Rule `json:"rules"`
	// Playbook is an optional FSM for playbook-grounded profiles.
	Playbook *Playbook `json:"playbook"`
	// Response is the Talk response ladder (clip → template → llm). Omit = ladder no-op.
	Response *ResponseConfig `json:"response"`
	// Fallback is hop degradation (listen/think/speak down) — PROFILE_SCHEMA.
	Fallback *FallbackConfig `json:"fallback"`
	Templates struct {
		Disposition *struct {
			ID string `json:"id"`
		} `json:"disposition"`
		MoM *struct {
			ID string `json:"id"`
		} `json:"mom"`
	} `json:"templates"`
	Analytics struct {
		Emit []string `json:"emit"`
	} `json:"analytics"`
	// XDesk is a legacy profile extension field. Ignored at runtime (P1.8);
	// present so published docs with x_desk still parse without failing answer.
	// Do not invent x_flow here (P2.7).
	XDesk json.RawMessage `json:"x_desk,omitempty"`
}

// ResponseConfig is response.ladder + clips + turn templates (distinct from post-call templates.*).
type ResponseConfig struct {
	Ladder    []string                   `json:"ladder"`
	Clips     map[string]CannedUtterance `json:"clips"`
	Templates map[string]CannedUtterance `json:"templates"`
}

// CannedUtterance is a clip or turn-template body (V1 = text via Speak).
type CannedUtterance struct {
	Text string         `json:"text"`
	When map[string]any `json:"when"`
}

// FallbackConfig holds per-hop degradation actions.
type FallbackConfig struct {
	ListenDown *FallbackAction `json:"listen_down"`
	ThinkDown  *FallbackAction `json:"think_down"`
	SpeakDown  *FallbackAction `json:"speak_down"`
}

// FallbackAction is one hop-down action (canned clip and/or skill).
type FallbackAction struct {
	SpeakCanned string `json:"speak_canned"`
	Skill       string `json:"skill"`
	TextSink    bool   `json:"text_sink"`
}

// Persona is the profile persona subsection (voice, tone, instructions).
type Persona struct {
	Name         string            `json:"name,omitempty"`
	Instructions string            `json:"instructions,omitempty"`
	VoiceID      string            `json:"voice_id,omitempty"`
	Voice        map[string]string `json:"-"` // gateway_id → speaker/voice ref
}

// UnmarshalJSON accepts persona.voice as a map or as a string alias for voice_id.
func (p *Persona) UnmarshalJSON(b []byte) error {
	type alias struct {
		Name         string          `json:"name"`
		Instructions string          `json:"instructions"`
		VoiceID      string          `json:"voice_id"`
		Voice        json.RawMessage `json:"voice"`
	}
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	p.Name = a.Name
	p.Instructions = a.Instructions
	p.VoiceID = strings.TrimSpace(a.VoiceID)
	p.Voice = nil
	if len(a.Voice) == 0 || string(a.Voice) == "null" {
		return nil
	}
	var scalar string
	if err := json.Unmarshal(a.Voice, &scalar); err == nil {
		scalar = strings.TrimSpace(scalar)
		if scalar != "" && p.VoiceID == "" {
			p.VoiceID = scalar
		}
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(a.Voice, &m); err != nil {
		return fmt.Errorf("persona.voice: want string or object: %w", err)
	}
	p.Voice = m
	return nil
}

// MarshalJSON emits voice_id and optional voice map.
func (p Persona) MarshalJSON() ([]byte, error) {
	out := map[string]any{}
	if p.Name != "" {
		out["name"] = p.Name
	}
	if p.Instructions != "" {
		out["instructions"] = p.Instructions
	}
	if p.VoiceID != "" {
		out["voice_id"] = p.VoiceID
	}
	if len(p.Voice) > 0 {
		out["voice"] = p.Voice
	}
	return json.Marshal(out)
}

// HasPersonaVoice reports whether persona has a usable voice_id or non-empty voice map entry.
func HasPersonaVoice(doc Document) bool {
	if strings.TrimSpace(doc.Persona.VoiceID) != "" {
		return true
	}
	for _, v := range doc.Persona.Voice {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// ResolveVoiceID returns the Speak VoiceID for a bound speak gateway id.
// Precedence: persona.voice[speakGatewayID], then persona.voice_id (incl. string voice alias).
func ResolveVoiceID(doc Document, speakGatewayID string) string {
	speakGatewayID = strings.TrimSpace(speakGatewayID)
	if speakGatewayID != "" && doc.Persona.Voice != nil {
		if v, ok := doc.Persona.Voice[speakGatewayID]; ok {
			if t := strings.TrimSpace(v); t != "" {
				return t
			}
		}
	}
	return strings.TrimSpace(doc.Persona.VoiceID)
}

type RouterProviders struct {
	Providers []string `json:"providers"`
}

type SkillDefinition struct {
	Gateway   string `json:"gateway"`
	Authority string `json:"authority"` // inform | decide | act
	Confirm   bool   `json:"confirm"`
}

// Rule is one profile rules[] entry.
type Rule struct {
	ID      string         `json:"id"`
	Phase   string         `json:"phase"`
	When    map[string]any `json:"when"`
	Action  string         `json:"action"`
	Skill   string         `json:"skill"`
	Message string         `json:"message"`
	Text    string         `json:"text"`
}

// Playbook is a minimal FSM (entry + states/slots).
type Playbook struct {
	Entry  string                   `json:"entry"`
	States map[string]PlaybookState `json:"states"`
}

// PlaybookState is one FSM node.
type PlaybookState struct {
	OnIntent   string   `json:"on_intent"`
	Fallback   string   `json:"fallback"`
	Slots      []string `json:"slots"`
	OnComplete string   `json:"on_complete"`
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
	if (doc.Modes.Talk || doc.Modes.Speak) && !HasPersonaVoice(doc) {
		return &ValidationError{
			Message: "persona.voice or persona.voice_id required when talk or speak mode is on",
			Details: map[string]any{"field": "persona.voice"},
		}
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
	if err := validateResponseAndFallback(doc); err != nil {
		return err
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

var allowedLadderTiers = map[string]bool{
	"clip": true, "template": true, "llm": true,
}

func validateResponseAndFallback(doc Document) error {
	if doc.Response != nil {
		for i, tok := range doc.Response.Ladder {
			t := strings.TrimSpace(tok)
			if t == "" || !allowedLadderTiers[t] {
				return &ValidationError{
					Message: "unknown response.ladder token: " + tok,
					Details: map[string]any{"field": "response.ladder", "index": i, "token": tok},
				}
			}
		}
	}
	clips := map[string]CannedUtterance{}
	if doc.Response != nil && doc.Response.Clips != nil {
		clips = doc.Response.Clips
	}
	checkCanned := func(path, id string) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil
		}
		if _, ok := clips[id]; !ok {
			return &ValidationError{
				Message: "fallback speak_canned references unknown clip: " + id,
				Details: map[string]any{"field": path, "clip": id},
			}
		}
		return nil
	}
	if doc.Fallback != nil {
		if doc.Fallback.ListenDown != nil {
			if err := checkCanned("fallback.listen_down.speak_canned", doc.Fallback.ListenDown.SpeakCanned); err != nil {
				return err
			}
		}
		if doc.Fallback.ThinkDown != nil {
			if err := checkCanned("fallback.think_down.speak_canned", doc.Fallback.ThinkDown.SpeakCanned); err != nil {
				return err
			}
		}
		if doc.Fallback.SpeakDown != nil {
			if err := checkCanned("fallback.speak_down.speak_canned", doc.Fallback.SpeakDown.SpeakCanned); err != nil {
				return err
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

// Family returns metadata.family (trimmed), empty if unset.
func Family(doc Document) string {
	return strings.TrimSpace(doc.Metadata.Family)
}

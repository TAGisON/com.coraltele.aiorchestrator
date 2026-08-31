package desk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// DefaultSkillGateway serves every desk skill contract (stub or live per bind).
const DefaultSkillGateway = "desk-skills"

// SkillAuthority is the frozen authority per skill contract (§9).
var SkillAuthority = map[string]string{
	"resolve_caller":            "inform",
	"search_knowledge":          "inform",
	"find_open_complaint":       "inform",
	"transfer_to_queue":         "act",
	"create_service_complaint":  "act",
	"send_complaint_email":      "act",
	"register_sales_enquiry":    "act",
	"schedule_callback":         "act",
	"push_disposition":          "act",
	"scrub_outbound_consent":    "decide",
}

// CompileError reports a desk that cannot become a profile.
type CompileError struct {
	Message string
	Details map[string]any
}

func (e *CompileError) Error() string { return e.Message }

// Compile turns a desk document into a contact-agent profile document (§4.3).
// The full desk doc is embedded as x_desk so the pinned profile version carries
// everything the runtime needs — no second lookup, versioning stays on the profile.
func Compile(d Doc) (json.RawMessage, error) {
	d.Normalize()
	if d.SchemaVersion > SchemaVersion {
		return nil, &CompileError{Message: fmt.Sprintf("desk schema_version %d newer than compiler %d", d.SchemaVersion, SchemaVersion)}
	}
	if errs := StructuralErrors(d); len(errs) > 0 {
		return nil, &CompileError{Message: "desk paths invalid: " + errs[0], Details: map[string]any{"errors": errs}}
	}
	if strings.TrimSpace(d.VoiceID) == "" && len(d.Voice) == 0 {
		return nil, &CompileError{Message: "desk voice required", Details: map[string]any{"field": "voice"}}
	}

	clips := map[string]any{}
	for id, p := range d.Prompts {
		text := pickLocale(p.Text, d.DefaultLanguage, d.DefaultLanguage)
		if strings.TrimSpace(text) == "" {
			continue
		}
		clips[id] = map[string]any{"text": text}
	}

	skillsAllowed := d.EnabledSkills()
	defs := map[string]any{}
	for _, name := range skillsAllowed {
		bind := d.Skills[name]
		gw := strings.TrimSpace(bind.Gateway)
		if gw == "" {
			gw = DefaultSkillGateway
		}
		authority := SkillAuthority[name]
		if authority == "" {
			authority = "inform"
		}
		defs[name] = map[string]any{
			"gateway":   gw,
			"authority": authority,
			"confirm":   false,
		}
	}

	knowledgeProviders := []string{}
	seenProvider := map[string]bool{}
	for _, k := range d.Knowledge {
		p := strings.TrimSpace(k.Provider)
		if p == "" {
			p = DefaultKnowledgeProvider
		}
		if !seenProvider[p] {
			seenProvider[p] = true
			knowledgeProviders = append(knowledgeProviders, p)
		}
	}

	persona := map[string]any{
		"name":         d.Name,
		"instructions": systemPack(d),
	}
	if strings.TrimSpace(d.VoiceID) != "" {
		persona["voice_id"] = d.VoiceID
	}
	if len(d.Voice) > 0 {
		persona["voice"] = d.Voice
	}

	deskRaw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}

	doc := map[string]any{
		"id":        d.ID,
		"tenant_id": d.TenantID,
		"metadata": map[string]any{
			"display_name": d.Name,
			"family":       "contact-agent",
			"desk_id":      d.ID,
			"direction":    d.Direction,
			"purpose":      d.Purpose,
		},
		"modes": map[string]any{
			"listen": true, "speak": true, "think": true, "talk": true,
		},
		"audio": map[string]any{"canonical_sample_rate_hz": 16000},
		"language": map[string]any{
			"behaviour":       "none",
			"primary":         d.DefaultLanguage,
			"allowed":         d.Languages,
			"auto_detect":     true,
			"mid_call_switch": true,
		},
		"hot_swap_allowed": []string{"language.primary"},
		"persona":          persona,
		"grounding":        map[string]any{"required": false},
		"routers": map[string]any{
			"knowledge": map[string]any{"providers": knowledgeProviders},
		},
		"skills": map[string]any{
			"allowed":     skillsAllowed,
			"definitions": defs,
		},
		"response": map[string]any{
			"ladder": []string{"clip", "llm"},
			"clips":  clips,
		},
		"templates": map[string]any{
			"disposition": map[string]any{"id": "contact-desk-disposition"},
		},
		"analytics": map[string]any{
			"emit": []string{"contained", "handoff"},
		},
		"x_desk": json.RawMessage(deskRaw),
	}

	if _, ok := d.Prompts[PromptSystemDown]; ok {
		fb := map[string]any{"speak_canned": PromptSystemDown}
		if d.Skills["transfer_to_queue"].Enabled {
			fb["skill"] = "transfer_to_queue"
		}
		doc["fallback"] = map[string]any{"think_down": fb}
	}

	return json.Marshal(doc)
}

// systemPack is the dev-owned LLM law text. The Configurator never edits it (§14.3).
func systemPack(d Doc) string {
	tone := "professional and concise"
	if d.Tone == "warm" {
		tone = "warm, friendly and concise"
	}
	langs := strings.Join(d.Languages, ", ")
	return strings.Join([]string{
		"You are the AI voice assistant for " + d.Name + ".",
		"Speak " + tone + ". Keep every reply short and conversational; ask one question at a time.",
		"Supported languages: " + langs + ". Answer in the caller's language and switch when they ask.",
		"Never invent a ticket id, an email confirmation, a transfer result, pricing, or company policy.",
		"Only state a ticket id or email status that a system action returned.",
		"Do not repeat questions that are already answered. Maintain context for the whole call.",
		"If you cannot help, offer to connect the caller to the right team.",
	}, " ")
}

// ContentHash is the stable publish hash for a desk document (§4.5).
func ContentHash(d Doc) (string, error) {
	d.Normalize()
	raw, err := json.Marshal(canonical(d))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func canonical(d Doc) any {
	raw, err := json.Marshal(d)
	if err != nil {
		return nil
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil
	}
	return sortMaps(generic)
}

func sortMaps(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([][2]any, 0, len(keys))
		for _, k := range keys {
			out = append(out, [2]any{k, sortMaps(t[k])})
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, e := range t {
			out = append(out, sortMaps(e))
		}
		return out
	default:
		return v
	}
}

// FromProfileDocument extracts the embedded desk doc from a pinned profile version.
func FromProfileDocument(raw json.RawMessage) (Doc, bool) {
	var envelope struct {
		XDesk json.RawMessage `json:"x_desk"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.XDesk) == 0 {
		return Doc{}, false
	}
	var d Doc
	if err := json.Unmarshal(envelope.XDesk, &d); err != nil {
		return Doc{}, false
	}
	d.Normalize()
	return d, true
}

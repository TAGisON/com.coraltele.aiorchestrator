// Package desk implements the Contact Desk vertical: GUI-authored desk documents,
// compilation into contact-agent profiles, and the guided-path runtime controller.
// See docs/product/CONTACT_DESK_POC_SOLUTION.md.
package desk

import (
	"encoding/json"
	"sort"
	"strings"
)

// SchemaVersion is the current desk document schema. Compiler accepts this and N-1.
const SchemaVersion = 1

// Direction values (§6.1).
const (
	DirectionInbound  = "inbound"
	DirectionOutbound = "outbound"
)

// Desk status values (§6.1).
const (
	StatusDraft       = "draft"
	StatusPublished   = "published"
	StatusUnpublished = "unpublished"
)

// Purpose values (§6.1).
var Purposes = []string{"support", "sales", "survey", "collections", "other"}

// Step types (§6.2).
const (
	StepSay     = "Say"
	StepAsk     = "Ask"
	StepConfirm = "Confirm"
	StepChoice  = "Choice"
	StepAction  = "Action"
	StepEnd     = "End"
)

// Skill result branches (§6.7).
const (
	BranchOK          = "ok"
	BranchFail        = "fail"
	BranchDuplicate   = "duplicate"
	BranchTimeout     = "timeout"
	BranchUnavailable = "unavailable"
)

// Repair actions for on_no_input / on_no_match (§6.9).
const (
	RepairReprompt = "reprompt"
	RepairNext     = "next"
	RepairClarify  = "clarify"
	RepairFallback = "route_fallback"
	RepairEnd      = "end"
)

// Validation kinds for Ask steps (§6.9).
const (
	ValidateFreeText = "free_text"
	ValidateEmail    = "email"
	ValidatePhone    = "phone"
	ValidateNumber   = "number"
	ValidateChoice   = "choice"
	ValidateProduct  = "product"
	ValidateYesNo    = "yes_no"
)

// Well-known prompt ids the runtime resolves by name (§14, §16).
const (
	PromptWelcome        = "welcome"
	PromptClarify        = "clarify"
	PromptSilence1       = "silence_1"
	PromptSilence2       = "silence_2"
	PromptSilenceGoodbye = "silence_goodbye"
	PromptClosing        = "closing"
	PromptAnythingElse   = "anything_else"
	PromptSystemDown     = "system_down"
	PromptHold           = "hold"
)

// Doc is the GUI-authored desk document (stored as JSONB; §4.4).
type Doc struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	TenantID      string `json:"tenant_id"`
	Name          string `json:"name"`
	Direction     string `json:"direction"`
	Purpose       string `json:"purpose"`

	Languages       []string          `json:"languages"`
	DefaultLanguage string            `json:"default_language"`
	Tone            string            `json:"tone"`
	VoiceID         string            `json:"voice_id"`
	Voice           map[string]string `json:"voice"`

	Prompts   map[string]Prompt    `json:"prompts"`
	Intents   []Intent             `json:"intents"`
	Paths     map[string]Path      `json:"paths"`
	Matrix    []MatrixRow          `json:"matrix"`
	CX        CXPolicy             `json:"cx"`
	Skills    map[string]SkillBind `json:"skills"`
	Knowledge []KnowledgeAttach    `json:"knowledge"`

	Retention *RetentionOverride `json:"retention,omitempty"`
	Consent   *ConsentConfig     `json:"consent,omitempty"`
}

// Prompt is one locale-aware spoken asset (§4.2).
type Prompt struct {
	ID           string            `json:"id"`
	Label        string            `json:"label"`
	Media        string            `json:"media"` // text_tts | wav
	Text         map[string]string `json:"text"`  // locale → text
	WavRef       map[string]string `json:"wav_ref,omitempty"`
	BargeAllowed *bool             `json:"barge_allowed,omitempty"`
}

// Intent is a named caller goal with example phrases per locale.
type Intent struct {
	ID      string              `json:"id"`
	Display string              `json:"display"`
	Active  bool                `json:"active"`
	Phrases map[string][]string `json:"phrases"`
	PathID  string              `json:"path_id"`
}

// Path is an ordered guided flow.
type Path struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Entry  string `json:"entry"`
	Steps  []Step `json:"steps"`
	Intent string `json:"intent,omitempty"`
}

// Step is one guided-path node (§6.9).
type Step struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label"`

	PromptID   string `json:"prompt_id,omitempty"`
	RepromptID string `json:"reprompt_id,omitempty"`

	SlotKey    string `json:"slot_key,omitempty"`
	Validation string `json:"validation,omitempty"`
	Required   bool   `json:"required,omitempty"`
	// Reask forces the question even when the slot already holds a value. Steps
	// that exist to correct an earlier answer must set it (§22 rule 6).
	Reask bool `json:"reask,omitempty"`

	Options []Option `json:"options,omitempty"`

	SummaryPromptID string `json:"summary_prompt_id,omitempty"`
	OnYes           string `json:"on_yes,omitempty"`
	OnNo            string `json:"on_no,omitempty"`

	Skill    string            `json:"skill,omitempty"`
	ArgMap   map[string]string `json:"arg_map,omitempty"`
	Branches map[string]string `json:"branches,omitempty"`

	ClosingPromptID   string `json:"closing_prompt_id,omitempty"`
	OfferAnythingElse bool   `json:"offer_anything_else,omitempty"`
	DispositionHint   string `json:"disposition_hint,omitempty"`

	SetAttributes map[string]string `json:"set_attributes,omitempty"`
	Next          string            `json:"next,omitempty"`

	BargeAllowed *bool  `json:"barge_allowed,omitempty"`
	MaxRetries   *int   `json:"max_retries,omitempty"`
	OnNoInput    string `json:"on_no_input,omitempty"`
	OnNoMatch    string `json:"on_no_match,omitempty"`
	TimeoutMs    int    `json:"timeout_ms,omitempty"`
}

// Option is one Choice branch.
type Option struct {
	ID        string              `json:"id"`
	Label     string              `json:"label"`
	Utterances map[string][]string `json:"utterances"`
	Next      string              `json:"next"`
}

// MatrixRow is one routing matrix entry (§14.2).
type MatrixRow struct {
	Intent string `json:"intent"`
	Owner  string `json:"owner"`
	Target string `json:"target"`
	Action string `json:"action"` // transfer | ticket | both
}

// CXPolicy holds call-behaviour settings (§6.12).
type CXPolicy struct {
	BargeIn           bool `json:"barge_in"`
	ListenWhileSpeak  bool `json:"listen_while_speak"`
	SilenceNudge1Ms   int  `json:"silence_nudge1_ms"`
	SilenceNudge2Ms   int  `json:"silence_nudge2_ms"`
	SilenceHangupMs   int  `json:"silence_hangup_ms"`
	AskTimeoutMs      int  `json:"ask_timeout_ms"`
	MaxRetries        int  `json:"max_retries"`
	MaxTurnFailures   int  `json:"max_turn_failures"`
	IntentAcceptScore float64 `json:"intent_accept_score"`
	IntentConfirmScore float64 `json:"intent_confirm_score"`
}

// SkillBind is a desk-level skill enable + connector config (§10).
type SkillBind struct {
	Enabled bool              `json:"enabled"`
	Mode    string            `json:"mode"` // stub | live
	Gateway string            `json:"gateway"`
	Config  map[string]string `json:"config,omitempty"`
}

// KnowledgeAttach binds a KB collection to intents. Provider is the registered
// knowledge gateway that serves it; the GUI picks from the registry.
type KnowledgeAttach struct {
	Collection string   `json:"collection"`
	Intents    []string `json:"intents"`
	Provider   string   `json:"provider,omitempty"`
}

// DefaultKnowledgeProvider serves KB collections unless a desk names another.
const DefaultKnowledgeProvider = "ingest-default"

// RetentionOverride is a desk-level retention policy (§11).
type RetentionOverride struct {
	TranscriptDays int `json:"transcript_days"`
	AttributesDays int `json:"attributes_days"`
	AuditDays      int `json:"audit_days"`
	RecordingDays  int `json:"recording_days"`
}

// ConsentConfig holds outbound consent settings (§19.1).
type ConsentConfig struct {
	Required bool   `json:"required"`
	Skill    string `json:"skill"`
}

// DefaultCX returns product defaults (§6.12).
func DefaultCX() CXPolicy {
	return CXPolicy{
		BargeIn:            true,
		ListenWhileSpeak:   true,
		SilenceNudge1Ms:    6000,
		SilenceNudge2Ms:    6000,
		SilenceHangupMs:    8000,
		AskTimeoutMs:       8000,
		MaxRetries:         2,
		MaxTurnFailures:    3,
		IntentAcceptScore:  0.60,
		IntentConfirmScore: 0.40,
	}
}

// Normalize fills defaults and stable ordering so compile output is deterministic.
func (d *Doc) Normalize() {
	if d.SchemaVersion == 0 {
		d.SchemaVersion = SchemaVersion
	}
	if d.Direction == "" {
		d.Direction = DirectionInbound
	}
	if d.Purpose == "" {
		d.Purpose = "support"
	}
	if len(d.Languages) == 0 {
		d.Languages = []string{"en-IN", "hi-IN"}
	}
	if d.DefaultLanguage == "" {
		d.DefaultLanguage = d.Languages[0]
	}
	if d.Tone == "" {
		d.Tone = "professional"
	}
	if d.CX.SilenceNudge1Ms == 0 {
		def := DefaultCX()
		if d.CX.AskTimeoutMs == 0 {
			d.CX = def
		} else {
			d.CX.SilenceNudge1Ms = def.SilenceNudge1Ms
			d.CX.SilenceNudge2Ms = def.SilenceNudge2Ms
			d.CX.SilenceHangupMs = def.SilenceHangupMs
		}
	}
	if d.CX.MaxRetries == 0 {
		d.CX.MaxRetries = DefaultCX().MaxRetries
	}
	if d.CX.MaxTurnFailures == 0 {
		d.CX.MaxTurnFailures = DefaultCX().MaxTurnFailures
	}
	if d.CX.IntentAcceptScore == 0 {
		d.CX.IntentAcceptScore = DefaultCX().IntentAcceptScore
	}
	if d.CX.IntentConfirmScore == 0 {
		d.CX.IntentConfirmScore = DefaultCX().IntentConfirmScore
	}
	if d.Prompts == nil {
		d.Prompts = map[string]Prompt{}
	}
	for id, p := range d.Prompts {
		if p.ID == "" {
			p.ID = id
		}
		if p.Media == "" {
			p.Media = "text_tts"
		}
		d.Prompts[id] = p
	}
	if d.Paths == nil {
		d.Paths = map[string]Path{}
	}
	for id, p := range d.Paths {
		if p.ID == "" {
			p.ID = id
		}
		if p.Entry == "" && len(p.Steps) > 0 {
			p.Entry = p.Steps[0].ID
		}
		d.Paths[id] = p
	}
	if d.Skills == nil {
		d.Skills = map[string]SkillBind{}
	}
	for name, b := range d.Skills {
		if b.Mode == "" {
			b.Mode = "stub"
		}
		d.Skills[name] = b
	}
	for i, k := range d.Knowledge {
		if strings.TrimSpace(k.Provider) == "" {
			d.Knowledge[i].Provider = DefaultKnowledgeProvider
		}
	}
	sort.Slice(d.Intents, func(i, j int) bool { return d.Intents[i].ID < d.Intents[j].ID })
	sort.Slice(d.Matrix, func(i, j int) bool { return d.Matrix[i].Intent < d.Matrix[j].Intent })
}

// PromptText resolves prompt text for a locale with fallback to the desk default
// language and then to any available locale (§13 language rule).
func (d *Doc) PromptText(promptID, locale string) string {
	p, ok := d.Prompts[promptID]
	if !ok {
		return ""
	}
	return pickLocale(p.Text, locale, d.DefaultLanguage)
}

func pickLocale(m map[string]string, locale, fallback string) string {
	if len(m) == 0 {
		return ""
	}
	if s := strings.TrimSpace(m[locale]); s != "" {
		return s
	}
	if base := baseLang(locale); base != "" {
		for k, v := range m {
			if baseLang(k) == base && strings.TrimSpace(v) != "" {
				return v
			}
		}
	}
	if s := strings.TrimSpace(m[fallback]); s != "" {
		return s
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if s := strings.TrimSpace(m[k]); s != "" {
			return s
		}
	}
	return ""
}

func baseLang(l string) string {
	l = strings.TrimSpace(strings.ToLower(l))
	if l == "" {
		return ""
	}
	if i := strings.IndexAny(l, "-_"); i > 0 {
		return l[:i]
	}
	return l
}

// Step looks up a step by id across all paths.
func (d *Doc) Step(pathID, stepID string) (Step, bool) {
	p, ok := d.Paths[pathID]
	if !ok {
		return Step{}, false
	}
	for _, s := range p.Steps {
		if s.ID == stepID {
			return s, true
		}
	}
	return Step{}, false
}

// IntentByID returns the configured intent.
func (d *Doc) IntentByID(id string) (Intent, bool) {
	for _, in := range d.Intents {
		if in.ID == id {
			return in, true
		}
	}
	return Intent{}, false
}

// MatrixFor returns the routing row for an intent.
func (d *Doc) MatrixFor(intent string) (MatrixRow, bool) {
	for _, m := range d.Matrix {
		if m.Intent == intent {
			return m, true
		}
	}
	return MatrixRow{}, false
}

// EnabledSkills returns sorted enabled skill names.
func (d *Doc) EnabledSkills() []string {
	out := make([]string, 0, len(d.Skills))
	for name, b := range d.Skills {
		if b.Enabled {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// Clone deep-copies via JSON (safe for publish snapshots).
func (d *Doc) Clone() (Doc, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return Doc{}, err
	}
	var out Doc
	if err := json.Unmarshal(raw, &out); err != nil {
		return Doc{}, err
	}
	return out, nil
}

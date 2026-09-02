package desk

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// SkillRunner executes one desk skill contract. ok=false means the connector
// reported failure; err is transport/timeout. Output keys become attributes.
type SkillRunner func(ctx context.Context, name string, args map[string]any) (out map[string]any, ok bool, err error)

// Outcome is the engine decision for one caller turn.
type Outcome struct {
	Text        string            `json:"text"`
	Tier        string            `json:"tier"`
	Language    string            `json:"language"`
	StepID      string            `json:"step_id"`
	PathID      string            `json:"path_id"`
	Intent      string            `json:"intent"`
	End         bool              `json:"end"`
	Disposition string            `json:"disposition,omitempty"`
	SkillName   string            `json:"skill_name,omitempty"`
	SkillOK     bool              `json:"skill_ok,omitempty"`
	SkillCalls  []SkillCall       `json:"skill_calls,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Transfer    *Handoff          `json:"transfer,omitempty"`
}

// SkillCall records one connector invocation for audit / evidence.
type SkillCall struct {
	Name   string         `json:"name"`
	Args   map[string]any `json:"args"`
	Status string         `json:"status"`
	Output map[string]any `json:"output,omitempty"`
	Error  string         `json:"error,omitempty"`
}

// Handoff is the warm-transfer payload handed to the human agent (§9).
//
// Number is the extension the caller's leg is blind-transferred to, resolved
// from the routing matrix. Empty means the desk has no destination configured
// for this queue and the telephony transfer cannot complete.
type Handoff struct {
	Target      string            `json:"target"`
	Owner       string            `json:"owner"`
	Number      string            `json:"number,omitempty"`
	Priority    string            `json:"priority"`
	Language    string            `json:"language"`
	Summary     string            `json:"summary"`
	Attributes  map[string]string `json:"attributes"`
	TicketID    string            `json:"ticket_id,omitempty"`
	CollectedAt string            `json:"collected_at,omitempty"`
}

// Tier values reported to analytics.
const (
	TierClip     = "clip"
	TierTemplate = "template"
	TierRefuse   = "refuse"
	TierEscalate = "escalate"
)

// Disposition codes (§6.6).
const (
	DispResolvedInfo       = "resolved_info"
	DispTicketCreated      = "ticket_created"
	DispExistingTicket     = "existing_ticket"
	DispTransferredSales     = "transferred_sales"
	DispTransferredTech      = "transferred_tech"
	DispTransferredService   = "transferred_service"
	DispTransferredCorporate = "transferred_corporate"
	DispCallbackScheduled    = "callback_scheduled"
	DispAbandonedSilence     = "abandoned_silence"
	DispAbandonedAbuse       = "abandoned_abuse"
	DispCallerHungUp         = "caller_hung_up"
	DispSystemFailure        = "system_failure"
	DispOutOfScope           = "out_of_scope"
	DispUnresolvedNoTicket   = "unresolved_no_ticket"
)

// Dispositions is the closed taxonomy shown in the Supervisor console (§6.6).
var Dispositions = []string{
	DispResolvedInfo, DispTicketCreated, DispExistingTicket, DispTransferredSales,
	DispTransferredTech, DispTransferredService, DispTransferredCorporate,
	DispCallbackScheduled, DispAbandonedSilence, DispAbandonedAbuse,
	DispCallerHungUp, DispSystemFailure, DispOutOfScope, DispUnresolvedNoTicket,
}

const maxAdvanceSteps = 64

// Engine runs one session's guided path (§6.9 + §14.3 system laws).
type Engine struct {
	doc    Doc
	skills SkillRunner

	intentClassify IntentClassifier
	localeSynth    LocaleSynthesizer
	turnCtx        context.Context

	mu                sync.Mutex
	language          string
	pathID            string
	stepID            string
	attempts          int
	failures          int
	attrs             map[string]string
	ended             bool
	disposition       string
	askedAnythingElse  bool
	silenceLevel       int
	greeted            bool
	languageLocked     bool
	awaitingLanguage   bool
	languageAskTries   int
	visited            []string
}

// detectLanguage uses the loose detector for the caller's first words and the
// evidence-based detector afterwards, so a mid-call slot answer never flips the
// call language (§17).
func (e *Engine) detectLanguage(text string) string {
	if !e.languageLocked {
		return DetectLanguage(text, e.doc.Languages)
	}
	return SwitchLanguageEvidence(text, e.doc.Languages, e.language)
}

// SetIntentClassifier wires Think enum bridge for multilingual classify (WP4).
func (e *Engine) SetIntentClassifier(c IntentClassifier) {
	e.mu.Lock()
	e.intentClassify = c
	e.mu.Unlock()
}

// SetLocaleSynthesizer wires Think render-in-language for prompt gaps (WP4).
func (e *Engine) SetLocaleSynthesizer(s LocaleSynthesizer) {
	e.mu.Lock()
	e.localeSynth = s
	e.mu.Unlock()
}

// NewEngine builds a per-session engine. skills may be nil (all Action steps fail).
func NewEngine(d Doc, skills SkillRunner) *Engine {
	d.Normalize()
	e := &Engine{
		doc:      d,
		skills:   skills,
		language: d.DefaultLanguage,
		attrs:    map[string]string{},
	}
	e.attrs[AttrDeskID] = d.ID
	e.attrs[AttrDirection] = d.Direction
	e.attrs[AttrPurpose] = d.Purpose
	e.attrs[AttrLanguage] = d.DefaultLanguage
	return e
}

// Doc exposes the compiled desk (read-only use).
func (e *Engine) Doc() Doc { return e.doc }

// SetAttribute seeds a call attribute (ANI, desk_version, …).
func (e *Engine) SetAttribute(key, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if key == "" {
		return
	}
	e.attrs[key] = value
}

// Attributes returns a copy of current contact attributes.
func (e *Engine) Attributes() map[string]string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return copyMap(e.attrs)
}

// Language returns the active call language.
func (e *Engine) Language() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.language
}

// SetLanguage locks the call language (STT detection or operator switch).
func (e *Engine) SetLanguage(lang string) {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.allowedLanguage(lang) {
		return
	}
	e.language = lang
	e.attrs[AttrLanguage] = lang
	e.languageLocked = true
}

func (e *Engine) allowedLanguage(lang string) bool {
	return e.doc.LanguageAllowed(lang)
}

// Ended reports whether the desk finished the call.
func (e *Engine) Ended() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ended
}

// Disposition returns the desk-suggested disposition code.
func (e *Engine) Disposition() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.disposition
}

// State is a supervisor-facing snapshot.
func (e *Engine) State() map[string]any {
	e.mu.Lock()
	defer e.mu.Unlock()
	return map[string]any{
		"language":    e.language,
		"path_id":     e.pathID,
		"step_id":     e.stepID,
		"ended":       e.ended,
		"disposition": e.disposition,
		"attempts":    e.attempts,
		"failures":    e.failures,
		"visited":     append([]string(nil), e.visited...),
		"attributes":  copyMap(e.attrs),
	}
}

// Welcome returns the opening line for the call (§1).
func (e *Engine) Welcome() Outcome {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.greeted = true
	text := e.prompt(PromptWelcome)
	return Outcome{Text: text, Tier: TierClip, Language: e.language, Attributes: copyMap(e.attrs)}
}

// ServicesMenu is the post-welcome capability line. Spoken after welcome completes
// so STT/barge are armed (welcome itself is non-bargeable).
//
// New callers (language not yet locked from ANI preference) hear ask_language when
// that prompt exists; returning callers hear a short confirm + department menu.
func (e *Engine) ServicesMenu() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	ask := strings.TrimSpace(e.prompt("ask_language"))
	if ask != "" && !e.languageLocked {
		e.awaitingLanguage = true
		e.languageAskTries = 0
		return ask
	}
	menu := e.prompt(PromptClarify)
	if e.languageLocked {
		if confirm := strings.TrimSpace(e.prompt("language_confirmed")); confirm != "" {
			return strings.TrimSpace(confirm + " " + menu)
		}
	}
	return menu
}

// Turn advances the desk with one caller utterance.
func (e *Engine) Turn(ctx context.Context, userText string) Outcome {
	e.mu.Lock()
	e.turnCtx = ctx
	defer func() {
		e.turnCtx = nil
		e.mu.Unlock()
	}()

	out := Outcome{Tier: TierClip}
	if e.ended {
		out.End = true
		out.Disposition = e.disposition
		out.Language = e.language
		out.Attributes = copyMap(e.attrs)
		return out
	}
	e.silenceLevel = 0

	text := strings.TrimSpace(userText)
	if text == "" {
		return e.silenceLocked(&out)
	}

	if AbusiveSpeech(text) {
		out.Text = e.prompt("abuse_hangup")
		if out.Text == "" {
			out.Text = e.prompt(PromptClosing)
		}
		e.endLocked(DispAbandonedAbuse)
		out.End = true
		out.Tier = TierRefuse
		return e.finish(&out)
	}

	// Language hard-lock (call-center rule):
	//  1) Explicit "speak Hindi/Punjabi/…" request may switch at any time.
	//  2) First non-switch utterance locks starting language (STT/script detect).
	//  3) Ambient script drift never flips the call after lock.
	allow := e.doc.RuntimeAllowlist()
	if req := LanguageSwitchRequest(text, allow); req != "" {
		if req != e.language && e.allowedLanguage(req) {
			e.language = req
			e.attrs[AttrLanguage] = req
		}
		e.languageLocked = true
		if e.awaitingLanguage {
			e.awaitingLanguage = false
			e.languageAskTries = 0
			confirm := e.prompt("language_confirmed")
			menu := e.prompt(PromptClarify)
			out.Text = strings.TrimSpace(strings.TrimSpace(confirm) + " " + menu)
			return e.finish(&out)
		}
		if isBareLanguageRequest(text) {
			out.Text = e.currentPromptText()
			if out.Text == "" {
				out.Text = e.prompt(PromptClarify)
			}
			return e.finish(&out)
		}
	} else if e.awaitingLanguage {
		return e.handleLanguageAskLocked(text, allow, &out)
	} else if !e.languageLocked {
		if det := DetectLanguage(text, allow); det != "" && e.allowedLanguage(det) {
			e.language = det
			e.attrs[AttrLanguage] = det
		}
		e.languageLocked = true
	}

	if CriticalRequest(text) && e.attrs[AttrPriority] == "" {
		e.attrs[AttrPriority] = "critical"
	}

	step, waiting := e.currentStep()

	// System law: an explicit human ask routes out of the guided path (§14.3 rule 3).
	if e.shouldRouteHuman(text, step, waiting) {
		e.routeHumanLocked(ctx, text, &out)
		return e.finish(&out)
	}

	if !waiting {
		e.selectIntentLocked(ctx, text, &out)
		return e.finish(&out)
	}

	switch step.Type {
	case StepAsk:
		e.handleAskLocked(ctx, step, text, &out)
	case StepConfirm:
		e.handleConfirmLocked(ctx, step, text, &out)
	case StepChoice:
		e.handleChoiceLocked(ctx, step, text, &out)
	case StepEnd:
		e.handleEndAnswerLocked(ctx, step, text, &out)
	default:
		e.advanceLocked(ctx, step.Next, &out)
	}
	return e.finish(&out)
}

// Silence advances the no-response ladder (§19). Returns the nudge or goodbye.
func (e *Engine) Silence(ctx context.Context) Outcome {
	e.mu.Lock()
	e.turnCtx = ctx
	defer func() {
		e.turnCtx = nil
		e.mu.Unlock()
	}()
	out := Outcome{Tier: TierClip}
	return e.silenceLocked(&out)
}

func (e *Engine) silenceLocked(out *Outcome) Outcome {
	if e.ended {
		out.End = true
		out.Disposition = e.disposition
		return e.finish(out)
	}
	e.silenceLevel++
	switch e.silenceLevel {
	case 1:
		out.Text = e.prompt(PromptSilence1)
	case 2:
		out.Text = e.prompt(PromptSilence2)
		if out.Text == "" {
			out.Text = e.prompt(PromptClarify)
		}
	default:
		out.Text = e.prompt(PromptSilenceGoodbye)
		e.endLocked(DispAbandonedSilence)
		out.End = true
	}
	return e.finish(out)
}

func (e *Engine) finish(out *Outcome) Outcome {
	out.Language = e.language
	out.PathID = e.pathID
	out.StepID = e.stepID
	out.Intent = e.attrs[AttrIntent]
	out.Attributes = copyMap(e.attrs)
	if e.ended {
		out.End = true
		out.Disposition = e.disposition
	}
	if out.Tier == "" {
		out.Tier = TierClip
	}
	out.Text = strings.TrimSpace(out.Text)
	return *out
}

func (e *Engine) currentStep() (Step, bool) {
	if e.pathID == "" || e.stepID == "" {
		return Step{}, false
	}
	s, ok := e.doc.Step(e.pathID, e.stepID)
	return s, ok
}

func (e *Engine) currentPromptText() string {
	step, ok := e.currentStep()
	if !ok {
		return e.prompt(PromptClarify)
	}
	switch step.Type {
	case StepConfirm:
		return e.render(e.prompt(pick(step.SummaryPromptID, step.PromptID)))
	default:
		return e.render(e.prompt(step.PromptID))
	}
}

func isBareLanguageRequest(text string) bool {
	return len(tokens(text)) <= 6
}

func (e *Engine) handleLanguageAskLocked(text string, allow []string, out *Outcome) Outcome {
	if det := DetectLanguage(text, allow); det != "" && e.allowedLanguage(det) {
		e.language = det
		e.attrs[AttrLanguage] = det
		e.languageLocked = true
		e.awaitingLanguage = false
		e.languageAskTries = 0
		confirm := e.prompt("language_confirmed")
		menu := e.prompt(PromptClarify)
		out.Text = strings.TrimSpace(strings.TrimSpace(confirm) + " " + menu)
		return e.finish(out)
	}
	// Named language without switch phrasing ("Hindi", "English", …).
	if named := namedLanguageOnly(text, allow); named != "" && e.allowedLanguage(named) {
		e.language = named
		e.attrs[AttrLanguage] = named
		e.languageLocked = true
		e.awaitingLanguage = false
		e.languageAskTries = 0
		confirm := e.prompt("language_confirmed")
		menu := e.prompt(PromptClarify)
		out.Text = strings.TrimSpace(strings.TrimSpace(confirm) + " " + menu)
		return e.finish(out)
	}
	e.languageAskTries++
	if e.languageAskTries >= 3 {
		out.Text = e.prompt("ood_hangup")
		if out.Text == "" {
			out.Text = e.prompt(PromptClosing)
		}
		e.endLocked(DispOutOfScope)
		out.End = true
		return e.finish(out)
	}
	out.Text = e.prompt("ask_language")
	if out.Text == "" {
		out.Text = e.prompt(PromptClarify)
	}
	return e.finish(out)
}

func namedLanguageOnly(text string, allowed []string) string {
	n := normalize(text)
	if n == "" || len(tokens(text)) > 4 {
		return ""
	}
	return LanguageSwitchRequest("please speak in "+n, allowed)
}

func (e *Engine) shouldRouteHuman(text string, step Step, waiting bool) bool {
	if !HumanRequest(text) {
		return false
	}
	if !waiting {
		return true
	}
	// A step that can consume this utterance keeps it: "all agents" answers the
	// impact question, it is not a request to be transferred.
	switch step.Type {
	case StepChoice:
		if _, ok := e.matchOption(step, text); ok {
			return false
		}
		return true
	case StepConfirm:
		if _, ok := YesNo(text); ok {
			return false
		}
		return true
	case StepAsk:
		kind := step.Validation
		if kind == "" {
			kind = ValidateFreeText
		}
		if kind != ValidateFreeText {
			if _, ok := ValidateSlot(kind, text); ok {
				return false
			}
			return true
		}
		// Free text: only an explicit ask routes out; the answer itself may
		// legitimately mention agents, engineers or transfers.
		n := normalize(text)
		if len(tokens(text)) > 8 {
			return false
		}
		for _, s := range humanPhrases {
			if strings.Contains(n, normalize(s)) {
				return true
			}
		}
		return bareHumanRequest(text)
	case StepEnd:
		return true
	default:
		return false
	}
}

func (e *Engine) routeHumanLocked(ctx context.Context, text string, out *Outcome) {
	intent := e.attrs[AttrIntent]
	if intent == "" {
		if id, score := ClassifyIntentBridge(ctx, e.doc, text, e.intentClassify); score >= e.doc.CX.IntentConfirmScore {
			intent = id
		}
	}
	if intent == "" {
		intent = "technical_support"
	}
	e.attrs[AttrIntent] = intent
	target := "transfer_" + intent
	if _, ok := e.doc.Paths[target]; ok {
		e.advanceLocked(ctx, "path:"+target, out)
		return
	}
	if row, ok := e.doc.MatrixFor(intent); ok && row.Target != "" {
		e.transferLocked(ctx, row, out)
		return
	}
	e.advanceLocked(ctx, "", out)
}

func (e *Engine) selectIntentLocked(ctx context.Context, text string, out *Outcome) {
	id, score := ClassifyIntentBridge(ctx, e.doc, text, e.intentClassify)
	if id != "" && score >= e.doc.CX.IntentAcceptScore {
		e.applyIntentLocked(ctx, id, text, out)
		return
	}
	if id != "" && score >= e.doc.CX.IntentConfirmScore && e.attempts == 0 {
		// Weak but plausible: take it rather than looping the menu (§22 rule 5).
		e.applyIntentLocked(ctx, id, text, out)
		return
	}
	e.attempts++
	e.failures++
	if e.attempts >= 3 || e.failures >= e.doc.CX.MaxTurnFailures {
		if hang := strings.TrimSpace(e.prompt("ood_hangup")); hang != "" {
			out.Text = hang
			e.endLocked(DispOutOfScope)
			out.End = true
			return
		}
		out.Text = e.prompt(PromptClarify)
		if t := e.prompt("clarify_2"); t != "" && e.attempts >= 3 {
			out.Text = t
		}
		return
	}
	if e.attempts == 1 {
		out.Text = e.prompt(PromptClarify)
		return
	}
	if t := e.prompt("clarify_2"); t != "" {
		out.Text = t
		return
	}
	out.Text = e.prompt(PromptClarify)
}

func (e *Engine) applyIntentLocked(ctx context.Context, intentID, text string, out *Outcome) {
	in, ok := e.doc.IntentByID(intentID)
	if !ok {
		e.attempts++
		out.Text = e.prompt(PromptClarify)
		return
	}
	e.attrs[AttrIntent] = intentID
	e.attempts = 0
	e.failures = 0
	if p, hit := MatchProduct(text); hit && e.attrs[AttrProduct] == "" {
		e.attrs[AttrProduct] = p
	}
	if e.attrs[AttrPriority] == "critical" {
		if _, hasCritical := e.doc.Paths["critical"]; hasCritical {
			e.advanceLocked(ctx, "path:critical", out)
			return
		}
	}
	e.advanceLocked(ctx, "path:"+in.PathID, out)
}

func (e *Engine) handleAskLocked(ctx context.Context, step Step, text string, out *Outcome) {
	kind := step.Validation
	if kind == "" {
		kind = ValidateFreeText
	}
	value, ok := ValidateSlot(kind, text)
	if !ok {
		e.repairLocked(ctx, step, out, RepairFallback)
		return
	}
	e.attempts = 0
	e.failures = 0
	if step.SlotKey != "" {
		e.attrs[step.SlotKey] = value
	}
	e.applySetAttributes(step)
	e.advanceLocked(ctx, step.Next, out)
}

func (e *Engine) handleConfirmLocked(ctx context.Context, step Step, text string, out *Outcome) {
	yes, ok := YesNo(text)
	if !ok {
		e.repairLocked(ctx, step, out, RepairFallback)
		return
	}
	e.attempts = 0
	e.failures = 0
	e.applySetAttributes(step)
	if yes {
		e.advanceLocked(ctx, step.OnYes, out)
		return
	}
	e.advanceLocked(ctx, step.OnNo, out)
}

func (e *Engine) handleChoiceLocked(ctx context.Context, step Step, text string, out *Outcome) {
	opt, ok := e.matchOption(step, text)
	if !ok {
		e.repairLocked(ctx, step, out, RepairFallback)
		return
	}
	e.attempts = 0
	e.failures = 0
	if step.SlotKey != "" {
		e.attrs[step.SlotKey] = opt.ID
	}
	e.applySetAttributes(step)
	e.advanceLocked(ctx, opt.Next, out)
}

func (e *Engine) handleEndAnswerLocked(ctx context.Context, step Step, text string, out *Outcome) {
	yes, ok := YesNo(text)
	if ok && yes {
		e.pathID = ""
		e.stepID = ""
		e.attempts = 0
		e.askedAnythingElse = false
		clearTurnAttributes(e.attrs)
		out.Text = e.prompt(PromptClarify)
		return
	}
	if !ok {
		// Treat an unrelated sentence as a new request.
		if id, score := ClassifyIntentBridge(ctx, e.doc, text, e.intentClassify); id != "" && score >= e.doc.CX.IntentAcceptScore {
			e.pathID = ""
			e.stepID = ""
			e.askedAnythingElse = false
			clearTurnAttributes(e.attrs)
			e.applyIntentLocked(ctx, id, text, out)
			return
		}
	}
	out.Text = e.prompt(pick(step.ClosingPromptID, PromptClosing))
	e.endLocked(pick(step.DispositionHint, e.inferDisposition()))
}

func (e *Engine) matchOption(step Step, text string) (Option, bool) {
	n := normalize(text)
	if n == "" {
		return Option{}, false
	}
	best, bestScore := Option{}, 0.0
	for i, o := range step.Options {
		score := 0.0
		if fmt.Sprint(i+1) == n {
			score = 1.0
		}
		for _, list := range o.Utterances {
			for _, u := range list {
				un := normalize(u)
				if un == "" {
					continue
				}
				if strings.Contains(n, un) {
					s := 1.0
					if len(un) < 4 {
						s = 0.7
					}
					if s > score {
						score = s
					}
				}
			}
		}
		if ln := normalize(o.Label); ln != "" && strings.Contains(n, ln) && score < 0.9 {
			score = 0.9
		}
		if score > bestScore {
			best, bestScore = o, score
		}
	}
	if bestScore >= 0.6 {
		return best, true
	}
	return Option{}, false
}

func (e *Engine) repairLocked(ctx context.Context, step Step, out *Outcome, defaultAction string) {
	e.attempts++
	e.failures++
	max := e.doc.CX.MaxRetries
	if step.MaxRetries != nil {
		max = *step.MaxRetries
	}
	if e.attempts <= max {
		out.Text = e.render(e.prompt(pick(step.RepromptID, step.PromptID, step.SummaryPromptID)))
		if out.Text == "" {
			out.Text = e.prompt(PromptClarify)
		}
		return
	}
	action := pick(step.OnNoMatch, defaultAction)
	e.attempts = 0
	switch action {
	case RepairNext:
		e.advanceLocked(ctx, step.Next, out)
	case RepairClarify:
		out.Text = e.prompt(PromptClarify)
	case RepairEnd:
		out.Text = e.prompt(PromptClosing)
		e.endLocked(DispOutOfScope)
	default:
		intent := e.attrs[AttrIntent]
		if row, ok := e.doc.MatrixFor(intent); ok {
			e.transferLocked(ctx, row, out)
			return
		}
		out.Text = e.prompt(PromptClosing)
		e.endLocked(DispOutOfScope)
	}
}

// advanceLocked walks Say/Action/End steps until the desk needs the caller again.
func (e *Engine) advanceLocked(ctx context.Context, target string, out *Outcome) {
	var parts []string
	if out.Text != "" {
		parts = append(parts, out.Text)
	}
	for i := 0; i < maxAdvanceSteps; i++ {
		if target == "" {
			e.pathID, e.stepID = "", ""
			break
		}
		if strings.HasPrefix(target, "path:") {
			name := strings.TrimPrefix(target, "path:")
			p, ok := e.doc.Paths[name]
			if !ok {
				e.pathID, e.stepID = "", ""
				break
			}
			e.pathID = name
			target = p.Entry
			continue
		}
		step, ok := e.doc.Step(e.pathID, target)
		if !ok {
			e.pathID, e.stepID = "", ""
			break
		}
		e.stepID = step.ID
		e.visited = append(e.visited, e.pathID+"."+step.ID)
		e.applySetAttributes(step)

		switch step.Type {
		case StepSay:
			if t := e.render(e.prompt(step.PromptID)); t != "" {
				parts = append(parts, t)
			}
			target = step.Next
			continue
		case StepAsk:
			if e.alreadyAnswered(step) {
				target = step.Next
				continue
			}
			if t := e.render(e.prompt(step.PromptID)); t != "" {
				parts = append(parts, t)
			}
			out.Text = strings.Join(parts, " ")
			return
		case StepConfirm:
			if t := e.render(e.prompt(pick(step.SummaryPromptID, step.PromptID))); t != "" {
				parts = append(parts, t)
			}
			out.Text = strings.Join(parts, " ")
			return
		case StepChoice:
			if next, ok := e.answeredOption(step); ok {
				target = next
				continue
			}
			if t := e.render(e.prompt(step.PromptID)); t != "" {
				parts = append(parts, t)
			}
			out.Text = strings.Join(parts, " ")
			return
		case StepAction:
			next, text := e.runActionLocked(ctx, step, out)
			if text != "" {
				parts = append(parts, text)
			}
			target = next
			continue
		case StepEnd:
			if t := e.render(e.prompt(step.PromptID)); t != "" {
				parts = append(parts, t)
			}
			if step.OfferAnythingElse && !e.askedAnythingElse {
				e.askedAnythingElse = true
				if t := e.prompt(PromptAnythingElse); t != "" {
					parts = append(parts, t)
				}
				out.Text = strings.Join(parts, " ")
				return
			}
			if t := e.render(e.prompt(pick(step.ClosingPromptID, PromptClosing))); t != "" {
				parts = append(parts, t)
			}
			e.endLocked(pick(step.DispositionHint, e.inferDisposition()))
			out.Text = strings.Join(parts, " ")
			return
		default:
			target = step.Next
			continue
		}
	}
	out.Text = strings.Join(parts, " ")
	if out.Text == "" {
		out.Text = e.prompt(PromptClarify)
	}
}

// alreadyAnswered reports an Ask whose slot the caller filled earlier in the call.
func (e *Engine) alreadyAnswered(step Step) bool {
	if step.Reask || step.SlotKey == "" {
		return false
	}
	return strings.TrimSpace(e.attrs[step.SlotKey]) != ""
}

// answeredOption resolves a Choice whose slot already holds one of its option ids.
func (e *Engine) answeredOption(step Step) (string, bool) {
	if step.Reask || step.SlotKey == "" {
		return "", false
	}
	value := strings.TrimSpace(e.attrs[step.SlotKey])
	if value == "" {
		return "", false
	}
	for _, o := range step.Options {
		if o.ID == value && o.Next != "" {
			return o.Next, true
		}
	}
	return "", false
}

func (e *Engine) runActionLocked(ctx context.Context, step Step, out *Outcome) (next, text string) {
	args := map[string]any{}
	for arg, src := range step.ArgMap {
		if strings.HasPrefix(src, "=") {
			args[arg] = strings.TrimPrefix(src, "=")
			continue
		}
		if v, ok := e.attrs[src]; ok && v != "" {
			args[arg] = v
		}
	}
	args["desk_id"] = e.doc.ID
	args["language"] = e.language
	if e.attrs[AttrIntent] != "" {
		args["intent"] = e.attrs[AttrIntent]
	}
	if step.Skill == "transfer_to_queue" {
		if row, ok := e.doc.MatrixFor(e.attrs[AttrIntent]); ok {
			if _, has := args["target"]; !has {
				args["target"] = row.Target
			}
			if _, has := args["owner"]; !has {
				args["owner"] = row.Owner
			}
		}
		args["summary"] = e.summaryLine()
		args["attributes"] = copyMapAny(e.attrs)
		args["priority"] = pick(e.attrs[AttrPriority], "normal")
	}

	call := SkillCall{Name: step.Skill, Args: args, Status: BranchOK}
	status := BranchOK
	var output map[string]any
	if e.skills == nil {
		status = BranchUnavailable
		call.Error = "no skill runner"
	} else {
		o, ok, err := e.skills(ctx, step.Skill, args)
		output = o
		switch {
		case err != nil:
			status = BranchTimeout
			call.Error = err.Error()
		case !ok:
			status = BranchFail
		default:
			status = BranchOK
		}
		if o != nil {
			if s, has := o["status"].(string); has && s != "" {
				status = s
			}
		}
	}
	call.Status = status
	call.Output = output
	out.SkillCalls = append(out.SkillCalls, call)
	out.SkillName = step.Skill
	out.SkillOK = status == BranchOK

	for k, v := range output {
		if k == "status" || k == "message" {
			continue
		}
		switch tv := v.(type) {
		case string:
			if tv != "" {
				e.attrs[k] = tv
			}
		case bool:
			e.attrs[k] = boolStr(tv)
		case float64:
			e.attrs[k] = trimFloat(tv)
		}
	}

	if step.Skill == "transfer_to_queue" && status == BranchOK {
		target, _ := args["target"].(string)
		owner, _ := args["owner"].(string)
		number := e.doc.TransferNumberFor(e.attrs[AttrIntent], target)
		if number == "" {
			// Screen-pop alone is not a transfer. Without a dialable extension the
			// path must take the fail/unavailable branch — never transferred_*.
			status = BranchUnavailable
			call.Status = status
			call.Error = "no dialable transfer number in routing matrix"
			out.SkillOK = false
			out.SkillCalls[len(out.SkillCalls)-1] = call
			delete(e.attrs, "transfer_target")
			delete(e.attrs, "transfer_owner")
			delete(e.attrs, AttrTransferTarget)
			delete(e.attrs, AttrTransferNumber)
		} else {
			e.attrs[AttrTransferTarget] = target
			e.attrs[AttrTransferNumber] = number
			out.Transfer = &Handoff{
				Target:     target,
				Owner:      owner,
				Number:     number,
				Priority:   pick(e.attrs[AttrPriority], "normal"),
				Language:   e.language,
				Summary:    e.summaryLine(),
				Attributes: copyMap(e.attrs),
				TicketID:   e.attrs[AttrTicketID],
			}
		}
	}

	if next, ok := step.Branches[status]; ok && next != "" {
		return next, ""
	}
	if status == BranchOK {
		return step.Next, ""
	}
	if next, ok := step.Branches[BranchFail]; ok && next != "" {
		return next, ""
	}
	// No branch authored for a failure: never claim success (§14.3 rule 15/16).
	return "", e.prompt(PromptSystemDown)
}

func (e *Engine) transferLocked(ctx context.Context, row MatrixRow, out *Outcome) {
	if p, ok := e.doc.Paths["transfer_"+row.Intent]; ok && p.Entry != "" {
		e.advanceLocked(ctx, "path:transfer_"+row.Intent, out)
		return
	}
	if p, ok := e.doc.Paths["transfer_generic"]; ok && p.Entry != "" {
		e.attrs[AttrTransferTarget] = row.Target
		e.advanceLocked(ctx, "path:transfer_generic", out)
		return
	}
	out.Text = e.prompt(PromptClosing)
	e.endLocked(DispOutOfScope)
}

func (e *Engine) applySetAttributes(step Step) {
	for k, v := range step.SetAttributes {
		if k == "" {
			continue
		}
		e.attrs[k] = RenderTemplate(v, e.attrs)
	}
}

func (e *Engine) endLocked(disposition string) {
	e.ended = true
	if disposition == "" {
		disposition = e.inferDisposition()
	}
	e.disposition = disposition
	e.attrs[AttrDisposition] = disposition
	e.attrs[AttrSummary] = e.summaryLine()
}

func (e *Engine) inferDisposition() string {
	switch {
	case e.attrs[AttrTicketID] != "":
		return DispTicketCreated
	case e.attrs[AttrTransferTarget] != "" && e.attrs[AttrTransferNumber] != "":
		switch e.attrs[AttrIntent] {
		case "sales_enquiry", "product_information", "sales":
			return DispTransferredSales
		case "service_complaint", "service":
			return DispTransferredService
		case "corporate":
			return DispTransferredCorporate
		case "support", "technical_support":
			return DispTransferredTech
		default:
			return DispTransferredTech
		}
	case e.attrs[AttrCallbackID] != "":
		return DispCallbackScheduled
	case e.attrs[AttrIntent] != "":
		return DispResolvedInfo
	default:
		return DispOutOfScope
	}
}

func (e *Engine) summaryLine() string {
	var parts []string
	if v := e.attrs[AttrIntent]; v != "" {
		parts = append(parts, "intent="+v)
	}
	if v := e.attrs[AttrProduct]; v != "" {
		parts = append(parts, "product="+v)
	}
	if v := e.attrs[AttrProblem]; v != "" {
		parts = append(parts, "problem="+v)
	}
	if v := e.attrs[AttrImpact]; v != "" {
		parts = append(parts, "impact="+v)
	}
	if v := e.attrs[AttrErrorAlarm]; v != "" {
		parts = append(parts, "error="+v)
	}
	if v := e.attrs[AttrTroubleshoot]; v != "" {
		parts = append(parts, "troubleshooting="+v)
	}
	if v := e.attrs[AttrPriority]; v != "" {
		parts = append(parts, "priority="+v)
	}
	return strings.Join(parts, "; ")
}

// HandoffPack builds the agent screen-pop payload at any point in the call.
func (e *Engine) HandoffPack() Handoff {
	e.mu.Lock()
	defer e.mu.Unlock()
	owner := ""
	target := e.attrs[AttrTransferTarget]
	if row, ok := e.doc.MatrixFor(e.attrs[AttrIntent]); ok {
		owner = row.Owner
		if target == "" {
			target = row.Target
		}
	}
	number := pick(e.attrs[AttrTransferNumber], e.doc.TransferNumberFor(e.attrs[AttrIntent], target))
	return Handoff{
		Target:     target,
		Owner:      owner,
		Number:     number,
		Priority:   pick(e.attrs[AttrPriority], "normal"),
		Language:   e.language,
		Summary:    e.summaryLine(),
		Attributes: copyMap(e.attrs),
		TicketID:   e.attrs[AttrTicketID],
	}
}

func (e *Engine) prompt(id string) string {
	if id == "" {
		return ""
	}
	ctx := e.turnCtx
	if ctx == nil {
		ctx = context.Background()
	}
	res := ResolvePromptLocale(e.doc, id, e.language)
	text := res.Text
	if res.NeedsSynthesis && e.localeSynth != nil {
		if syn, ok := e.localeSynth(ctx, res.CanonicalText, e.language); ok && strings.TrimSpace(syn) != "" {
			text = syn
		} else {
			text = res.CanonicalText
		}
	} else if text == "" {
		text = res.CanonicalText
	}
	return e.render(text)
}

func (e *Engine) render(text string) string {
	if text == "" {
		return ""
	}
	spoken := make(map[string]string, len(e.attrs))
	for k, v := range e.attrs {
		spoken[k] = DisplayValue(k, v, e.language)
	}
	return RenderTemplate(text, spoken)
}

func pick(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copyMapAny(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func clearTurnAttributes(attrs map[string]string) {
	for _, k := range []string{AttrIntent, AttrProduct, AttrProblem, AttrImpact, AttrErrorAlarm,
		AttrTroubleshoot, AttrTicketID, AttrEmailSent, AttrTransferTarget, AttrPriority, AttrEnquiryID, AttrCallbackID} {
		delete(attrs, k)
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func trimFloat(f float64) string {
	s := fmt.Sprintf("%.4f", f)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

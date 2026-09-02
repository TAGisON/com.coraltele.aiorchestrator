// Package thinkpath runs the locked Think pipeline: memory → redact → playbook →
// knowledge → rules pre → Think → rules post → Skill → memory.
// Imports port + router only (no gateway/*).
package thinkpath

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
)

// Response tier vocabulary (analytics / audit).
const (
	TierClip     = "clip"
	TierTemplate = "template"
	TierLLM      = "llm"
	TierRefuse   = "refuse"
	TierEscalate = "escalate"
)

// Result is the outcome of one Think-path pass.
type Result struct {
	ResponseText  string
	Action        string // allow | refuse | escalate | block_think | strip_response | inject_text
	ResponseTier string // clip | template | llm | refuse | escalate
	RuleID        string
	KnowledgeHit  bool
	SkillName     string
	SkillOK       bool
	PlaybookState string
	BlockedThink  bool
	// DeskEnd is set when a Contact Desk guided path finished the call.
	DeskEnd     bool
	Disposition string
	DeskStepID  string
	// DeskTransfer is set when the guided path decided to blind-transfer the
	// caller. The runtime performs the actual leg move after this turn's line
	// ("connecting you now") has been spoken.
	DeskTransfer *TransferIntent
}

// TransferIntent is a guided-path decision to move the caller's leg to a human.
type TransferIntent struct {
	Number string // extension/DID to dial (uuid_transfer destination)
	Owner  string // human name announced / shown on screen-pop
	Target string // queue label
	Reason string // summary for audit
}

// Controller is an optional deterministic dialog owner (Contact Desk guided paths).
// When it handles a turn, rules / ladder / Think are skipped for that turn.
type Controller interface {
	Turn(ctx context.Context, userText string) (ControllerResult, bool)
	Welcome() (string, bool)
}

// ControllerResult is one controller-decided turn.
type ControllerResult struct {
	Text        string
	Tier        string
	SkillName   string
	SkillOK     bool
	End         bool
	Disposition string
	StepID      string
	Transfer    *TransferIntent
}

// Deps are resolved gateway instances (from registry Select).
type Deps struct {
	Think     port.Think
	Knowledge port.Knowledge
	Skill     port.Skill
}

// Resolve selects Think/Knowledge/Skill from the registry using profile routers.
func Resolve(reg port.Registry, doc profile.Document, clockKind string) (Deps, error) {
	var d Deps
	opt := router.SelectOptions{Clock: clockKind}
	if len(doc.Routers.Think.Providers) > 0 {
		rec, err := router.Select(reg, toIDs(doc.Routers.Think.Providers), port.PortThink, opt)
		if err != nil {
			return d, err
		}
		th, ok := rec.Instance.(port.Think)
		if !ok {
			return d, fmt.Errorf("think instance type assert failed")
		}
		d.Think = th
	}
	if len(doc.Routers.Knowledge.Providers) > 0 {
		rec, err := router.Select(reg, toIDs(doc.Routers.Knowledge.Providers), port.PortKnowledge, opt)
		if err != nil {
			return d, err
		}
		k, ok := rec.Instance.(port.Knowledge)
		if !ok {
			return d, fmt.Errorf("knowledge instance type assert failed")
		}
		d.Knowledge = k
	}
	// Skill gateway resolved lazily per skill definition.
	_ = clockKind
	return d, nil
}

// Path runs one Think-path invocation for inbound user text.
type Path struct {
	Doc  profile.Document
	Mem  *session.Memory
	Deps Deps
	Reg  port.Registry
	// TenantID for skill execute.
	TenantID string
	Session  port.SessionID
	// ConfirmedSkills lists skill names already confirmed (confirm stub).
	ConfirmedSkills map[string]bool
	// ActiveLanguage returns session active_language when locked (cc-2). Nil/empty → no instruction.
	ActiveLanguage func() string
	// PinnedEngines is true when session gateway_binding is set (CC) — no mid-session vendor hop;
	// Think total failure uses profile.fallback.think_down.
	PinnedEngines bool
	// Desk owns the turn when the pinned profile carries a Contact Desk document.
	Desk Controller
}

// Run executes the locked order for one inbound utterance (or playback transcript).
func (p *Path) Run(ctx context.Context, userText string) (Result, error) {
	res := Result{Action: "allow"}
	if p.Mem == nil {
		p.Mem = session.NewMemory()
	}
	// 1. memory append user
	p.Mem.Append("user", userText)
	// 2. redact stub (no-op)
	redacted := userText

	// 2b. Contact Desk guided path owns the turn when configured.
	if p.Desk != nil {
		if dr, handled := p.Desk.Turn(ctx, redacted); handled {
			res.Action = "allow"
			res.ResponseTier = dr.Tier
			if res.ResponseTier == "" {
				res.ResponseTier = TierClip
			}
			res.ResponseText = dr.Text
			res.SkillName = dr.SkillName
			res.SkillOK = dr.SkillOK
			res.DeskEnd = dr.End
			res.Disposition = dr.Disposition
			res.DeskStepID = dr.StepID
			res.DeskTransfer = dr.Transfer
			if dr.Text != "" {
				p.Mem.Append("assistant", dr.Text)
			}
			return res, nil
		}
	}

	// 3. playbook step (if profile has playbook)
	if p.Doc.Playbook != nil && len(p.Doc.Playbook.States) > 0 {
		res.PlaybookState = stepPlaybook(p.Doc.Playbook, p.Mem, redacted)
	}

	// 4. Knowledge retrieve
	var chunks []string
	hit := false
	if p.Deps.Knowledge != nil {
		kr, err := p.Deps.Knowledge.Retrieve(ctx, port.KnowledgeQuery{
			SessionID: p.Session,
			Query:     redacted,
			TopK:      3,
		})
		if err != nil {
			return res, err
		}
		hit = kr.Hit
		for _, s := range kr.Snippets {
			chunks = append(chunks, s.Text)
		}
	}
	res.KnowledgeHit = hit

	// Grounding required + no hit → refuse/escalate via rules (also enforce here as safety)
	facts := evalFacts{
		GroundingRequired: p.Doc.Grounding.Required,
		KnowledgeHit:      hit,
		Intent:            p.Mem.Intent,
		UserText:          redacted,
		Slots:             p.Mem.GetSlots(),
	}

	// 5. rules pre_think — text/intent predicates only.
	// Grounding-only rules (grounding_required / knowledge_hit) wait until after the clip ladder
	// so greetings and intent clips still fire when the KB misses.
	if rule, ok := firstMatchFiltered(p.Doc.Rules, "pre_think", facts, false); ok {
		applied, newText, err := p.applyPreThinkRule(ctx, &res, rule, redacted)
		if err != nil {
			return res, err
		}
		if applied {
			return res, nil
		}
		redacted = newText
	}

	// 5b. response ladder (clip → template) before free LLM / grounding escalate
	if text, tier, ok := matchLadderCanned(p.Doc.Response, redacted); ok {
		res.Action = "allow"
		res.ResponseTier = tier
		res.ResponseText = text
		p.Mem.Append("assistant", text)
		return res, nil
	}

	// 5c. grounding-only pre_think (e.g. escalate-no-grounding) after clips miss
	if rule, ok := firstMatchFiltered(p.Doc.Rules, "pre_think", facts, true); ok {
		applied, _, err := p.applyPreThinkRule(ctx, &res, rule, redacted)
		if err != nil {
			return res, err
		}
		if applied {
			return res, nil
		}
	}

	if p.Doc.Grounding.Required && !hit {
		res.Action = "refuse"
		res.ResponseTier = TierRefuse
		res.ResponseText = "No grounding hit; cannot invent an answer."
		p.Mem.Append("assistant", res.ResponseText)
		return res, nil
	}

	// Ladder present without llm tier → do not call Think
	if p.Doc.Response != nil && len(p.Doc.Response.Ladder) > 0 && !ladderAllowsLLM(p.Doc.Response.Ladder) {
		res.Action = "refuse"
		res.ResponseTier = TierRefuse
		res.ResponseText = "No clip or template matched."
		p.Mem.Append("assistant", res.ResponseText)
		return res, nil
	}

	// 6. Think gateway
	if p.Deps.Think == nil {
		return res, fmt.Errorf("think gateway not resolved")
	}
	skills := skillDescriptors(p.Doc)
	msgs := p.Mem.Snapshot()
	if p.ActiveLanguage != nil {
		if lang := strings.TrimSpace(p.ActiveLanguage()); lang != "" {
			instr := "Respond in language: " + lang
			msgs = append([]port.ChatMessage{{Role: "system", Content: instr}}, msgs...)
		}
	}
	tr, err := p.Deps.Think.Complete(ctx, port.ThinkRequest{
		SessionID:       p.Session,
		Messages:        msgs,
		GroundingChunks: chunks,
		Skills:          skills,
		Stream:          false,
	})
	if err != nil {
		if deg, used, derr := p.degradeThinkDown(ctx, res); used {
			return deg, derr
		}
		return res, err
	}
	out := tr.Text

	// 7. rules post_think
	facts.UserText = out
	if rule, ok := firstMatch(p.Doc.Rules, "post_think", facts); ok {
		res.RuleID = rule.ID
		res.Action = rule.Action
		switch rule.Action {
		case "strip_response":
			out = ""
		case "refuse":
			out = rule.Message
			res.ResponseTier = TierRefuse
		case "escalate":
			out = rule.Message
			res.ResponseTier = TierEscalate
			if rule.Skill != "" {
				ok, err := p.executeSkill(ctx, rule.Skill, nil)
				if err != nil {
					return res, err
				}
				res.SkillName = rule.Skill
				res.SkillOK = ok
			}
		}
	}

	// 8. Skill if act proposed
	if tr.SkillProposal != nil && tr.SkillProposal.Name != "" {
		ok, err := p.executeSkill(ctx, tr.SkillProposal.Name, tr.SkillProposal.Args)
		if err != nil {
			return res, err
		}
		res.SkillName = tr.SkillProposal.Name
		res.SkillOK = ok
	}

	// 9. memory append assistant
	p.Mem.Append("assistant", out)
	res.ResponseText = out
	if res.Action == "" {
		res.Action = "allow"
	}
	if res.ResponseTier == "" {
		res.ResponseTier = TierLLM
	}
	return res, nil
}

// degradeThinkDown applies fallback.think_down when engines are session-pinned (CC).
// Does not re-Select Think / switch vendors. used=false if no fallback configured.
func (p *Path) degradeThinkDown(ctx context.Context, res Result) (out Result, used bool, err error) {
	if !p.PinnedEngines || p.Doc.Fallback == nil || p.Doc.Fallback.ThinkDown == nil {
		return res, false, nil
	}
	fb := p.Doc.Fallback.ThinkDown
	out = res
	out.Action = "escalate"
	out.ResponseTier = TierEscalate
	if id := strings.TrimSpace(fb.SpeakCanned); id != "" && p.Doc.Response != nil && p.Doc.Response.Clips != nil {
		if clip, ok := p.Doc.Response.Clips[id]; ok {
			out.ResponseText = clip.Text
		}
	}
	if out.ResponseText == "" {
		return res, false, nil // no invented clip
	}
	if skill := strings.TrimSpace(fb.Skill); skill != "" {
		ok, skErr := p.executeSkill(ctx, skill, nil)
		if skErr != nil {
			return out, true, skErr
		}
		out.SkillName = skill
		out.SkillOK = ok
	}
	p.Mem.Append("assistant", out.ResponseText)
	return out, true, nil
}

func ladderAllowsLLM(ladder []string) bool {
	for _, t := range ladder {
		if strings.TrimSpace(t) == "llm" {
			return true
		}
	}
	return false
}

// matchLadderCanned walks response.ladder for clip/template tiers only.
// Empty/absent when.regex does not auto-match (fallback / explicit id only).
func matchLadderCanned(resp *profile.ResponseConfig, userText string) (text, tier string, ok bool) {
	if resp == nil || len(resp.Ladder) == 0 {
		return "", "", false
	}
	for _, tok := range resp.Ladder {
		switch strings.TrimSpace(tok) {
		case "clip":
			if t, hit := firstMatchingCanned(resp.Clips, userText); hit {
				return t, TierClip, true
			}
		case "template":
			if t, hit := firstMatchingCanned(resp.Templates, userText); hit {
				return t, TierTemplate, true
			}
		case "llm":
			return "", "", false // fall through to Think
		}
	}
	return "", "", false
}

func firstMatchingCanned(m map[string]profile.CannedUtterance, userText string) (string, bool) {
	if len(m) == 0 {
		return "", false
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		u := m[k]
		if cannedWhenMatches(u.When, userText) {
			return u.Text, true
		}
	}
	return "", false
}

func cannedWhenMatches(when map[string]any, userText string) bool {
	if len(when) == 0 {
		return false
	}
	pat, _ := when["regex"].(string)
	pat = strings.TrimSpace(pat)
	if pat == "" {
		return false
	}
	re, err := regexp.Compile(pat)
	if err != nil || !re.MatchString(userText) {
		return false
	}
	return true
}

func (p *Path) executeSkill(ctx context.Context, name string, args []byte) (bool, error) {
	def, ok := p.Doc.Skills.Definitions[name]
	if !ok {
		return false, nil
	}
	allowed := false
	for _, a := range p.Doc.Skills.Allowed {
		if a == name {
			allowed = true
			break
		}
	}
	if !allowed {
		return false, nil
	}
	// authority + confirm stub
	if def.Authority == "act" && def.Confirm {
		if p.ConfirmedSkills == nil || !p.ConfirmedSkills[name] {
			return false, nil
		}
	}
	if def.Gateway == "" || p.Reg == nil {
		return false, nil
	}
	rec, err := router.Select(p.Reg, []port.GatewayID{port.GatewayID(def.Gateway)}, port.PortSkill, router.SelectOptions{})
	if err != nil {
		return false, err
	}
	sk, ok := rec.Instance.(port.Skill)
	if !ok {
		return false, fmt.Errorf("skill instance type assert failed")
	}
	if args == nil {
		args = []byte(`{}`)
	}
	sr, err := sk.Execute(ctx, port.SkillRequest{
		SessionID: p.Session,
		Name:      name,
		Args:      args,
		TenantID:  p.TenantID,
	})
	if err != nil {
		return false, err
	}
	return sr.OK, nil
}

type evalFacts struct {
	GroundingRequired bool
	KnowledgeHit      bool
	Intent            string
	UserText          string
	Slots             map[string]string
}

func firstMatch(rules []profile.Rule, phase string, f evalFacts) (profile.Rule, bool) {
	for _, r := range rules {
		if r.Phase != phase {
			continue
		}
		if matchWhen(r.When, f) {
			return r, true
		}
	}
	return profile.Rule{}, false
}

// firstMatchFiltered walks rules for phase.
// groundingOnly=false skips grounding-only when-clauses; groundingOnly=true matches only those.
func firstMatchFiltered(rules []profile.Rule, phase string, f evalFacts, groundingOnly bool) (profile.Rule, bool) {
	for _, r := range rules {
		if r.Phase != phase {
			continue
		}
		if isGroundingOnlyWhen(r.When) != groundingOnly {
			continue
		}
		if matchWhen(r.When, f) {
			return r, true
		}
	}
	return profile.Rule{}, false
}

func isGroundingOnlyWhen(when map[string]any) bool {
	if len(when) == 0 {
		return false
	}
	for k := range when {
		if k != "grounding_required" && k != "knowledge_hit" {
			return false
		}
	}
	return true
}

// applyPreThinkRule applies a matched pre_think rule.
// Returns applied=true when the turn should end; otherwise continues with possibly rewritten user text.
func (p *Path) applyPreThinkRule(ctx context.Context, res *Result, rule profile.Rule, redacted string) (applied bool, newText string, err error) {
	res.RuleID = rule.ID
	res.Action = rule.Action
	switch rule.Action {
	case "refuse":
		res.ResponseTier = TierRefuse
		res.ResponseText = rule.Message
		if res.ResponseText == "" {
			res.ResponseText = "I cannot help with that."
		}
		p.Mem.Append("assistant", res.ResponseText)
		return true, redacted, nil
	case "block_think":
		res.BlockedThink = true
		res.ResponseTier = TierRefuse
		res.ResponseText = rule.Message
		if res.ResponseText == "" {
			res.ResponseText = "Thinking blocked by policy."
		}
		p.Mem.Append("assistant", res.ResponseText)
		return true, redacted, nil
	case "escalate":
		res.ResponseTier = TierEscalate
		res.ResponseText = rule.Message
		if res.ResponseText == "" {
			res.ResponseText = "Escalating to a human."
		}
		if rule.Skill != "" {
			ok, err := p.executeSkill(ctx, rule.Skill, nil)
			if err != nil {
				return false, redacted, err
			}
			res.SkillName = rule.Skill
			res.SkillOK = ok
		}
		p.Mem.Append("assistant", res.ResponseText)
		return true, redacted, nil
	case "inject_text":
		if rule.Text != "" {
			redacted = rule.Text + "\n" + redacted
		}
		return false, redacted, nil
	case "allow":
		return false, redacted, nil
	default:
		return false, redacted, nil
	}
}

func matchWhen(when map[string]any, f evalFacts) bool {
	if len(when) == 0 {
		return true
	}
	for k, v := range when {
		switch k {
		case "grounding_required":
			want, _ := v.(bool)
			if want != f.GroundingRequired {
				return false
			}
		case "knowledge_hit":
			want, _ := v.(bool)
			if want != f.KnowledgeHit {
				return false
			}
		case "intent":
			want, _ := v.(string)
			if want != f.Intent {
				return false
			}
		case "regex":
			pat, _ := v.(string)
			re, err := regexp.Compile(pat)
			if err != nil || !re.MatchString(f.UserText) {
				return false
			}
		case "slot_missing":
			name, _ := v.(string)
			if f.Slots != nil {
				if _, ok := f.Slots[name]; ok && f.Slots[name] != "" {
					return false
				}
			}
		case "caller_request_human":
			want, _ := v.(bool)
			lower := strings.ToLower(f.UserText)
			has := strings.Contains(lower, "human") || strings.Contains(lower, "agent")
			if want != has {
				return false
			}
		case "confidence_below":
			// stub: no confidence on path yet — treat as non-match unless 1.0+
			_ = v
			return false
		default:
			return false
		}
	}
	return true
}

func stepPlaybook(pb *profile.Playbook, mem *session.Memory, text string) string {
	cur := mem.State
	if cur == "" {
		cur = pb.Entry
	}
	st, ok := pb.States[cur]
	if !ok {
		return cur
	}
	lower := strings.ToLower(text)
	// minimal: keyword → intent slot, then transition on_intent
	if strings.Contains(lower, "balance") {
		mem.Intent = "balance_inquiry"
		mem.SetSlot("intent", "balance_inquiry")
	}
	if strings.Contains(lower, "account") {
		mem.SetSlot("account_id", "unknown")
	}
	next := cur
	if mem.Intent != "" && st.OnIntent != "" {
		next = st.OnIntent
	} else if st.Fallback != "" && mem.Intent == "" {
		next = st.Fallback
	}
	// slot completeness → on_complete
	if ns, ok := pb.States[next]; ok && len(ns.Slots) > 0 {
		complete := true
		slots := mem.GetSlots()
		for _, s := range ns.Slots {
			if slots[s] == "" {
				complete = false
				break
			}
		}
		if complete && ns.OnComplete != "" {
			next = ns.OnComplete
		}
	}
	mem.State = next
	return next
}

func skillDescriptors(doc profile.Document) []port.SkillDescriptor {
	var out []port.SkillDescriptor
	for _, name := range doc.Skills.Allowed {
		out = append(out, port.SkillDescriptor{
			Name:        name,
			Description: name,
			InputSchema: []byte(`{}`),
		})
	}
	return out
}

func toIDs(ss []string) []port.GatewayID {
	out := make([]port.GatewayID, len(ss))
	for i, s := range ss {
		out[i] = port.GatewayID(s)
	}
	return out
}

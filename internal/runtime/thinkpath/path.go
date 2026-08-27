// Package thinkpath runs the locked Think pipeline: memory → redact → playbook →
// knowledge → rules pre → Think → rules post → Skill → memory.
// Imports port + router only (no gateway/*).
package thinkpath

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
)

// Result is the outcome of one Think-path pass.
type Result struct {
	ResponseText  string
	Action        string // allow | refuse | escalate | block_think | strip_response | inject_text
	RuleID        string
	KnowledgeHit  bool
	SkillName     string
	SkillOK       bool
	PlaybookState string
	BlockedThink  bool
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

	// 5. rules pre_think
	if rule, ok := firstMatch(p.Doc.Rules, "pre_think", facts); ok {
		res.RuleID = rule.ID
		res.Action = rule.Action
		switch rule.Action {
		case "refuse":
			res.ResponseText = rule.Message
			if res.ResponseText == "" {
				res.ResponseText = "I cannot help with that."
			}
			p.Mem.Append("assistant", res.ResponseText)
			return res, nil
		case "block_think":
			res.BlockedThink = true
			res.ResponseText = rule.Message
			if res.ResponseText == "" {
				res.ResponseText = "Thinking blocked by policy."
			}
			p.Mem.Append("assistant", res.ResponseText)
			return res, nil
		case "escalate":
			res.ResponseText = rule.Message
			if res.ResponseText == "" {
				res.ResponseText = "Escalating to a human."
			}
			if rule.Skill != "" {
				ok, err := p.executeSkill(ctx, rule.Skill, nil)
				if err != nil {
					return res, err
				}
				res.SkillName = rule.Skill
				res.SkillOK = ok
			}
			p.Mem.Append("assistant", res.ResponseText)
			return res, nil
		case "inject_text":
			if rule.Text != "" {
				redacted = rule.Text + "\n" + redacted
			}
		case "allow":
			// continue
		}
	}

	if p.Doc.Grounding.Required && !hit {
		res.Action = "refuse"
		res.ResponseText = "No grounding hit; cannot invent an answer."
		p.Mem.Append("assistant", res.ResponseText)
		return res, nil
	}

	// 6. Think gateway
	if p.Deps.Think == nil {
		return res, fmt.Errorf("think gateway not resolved")
	}
	skills := skillDescriptors(p.Doc)
	tr, err := p.Deps.Think.Complete(ctx, port.ThinkRequest{
		SessionID:       p.Session,
		Messages:        p.Mem.Snapshot(),
		GroundingChunks: chunks,
		Skills:          skills,
		Stream:          false,
	})
	if err != nil {
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
		case "escalate":
			out = rule.Message
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
	return res, nil
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

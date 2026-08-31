package thinkpath_test

import (
	"context"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/thinkpath"
)

func baseReg(t *testing.T) port.Registry {
	t.Helper()
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestThinkPath_PreThinkRefuseRegex(t *testing.T) {
	reg := baseReg(t)
	var doc profile.Document
	doc.Modes.Think = true
	doc.Routers.Think.Providers = []string{"fake-think"}
	doc.Rules = []profile.Rule{{
		ID: "block-card", Phase: "pre_think",
		When: map[string]any{"regex": `\d{12,19}`},
		Action: "refuse", Message: "no cards",
	}}
	deps, err := thinkpath.Resolve(reg, doc, "live")
	if err != nil {
		t.Fatal(err)
	}
	p := &thinkpath.Path{Doc: doc, Mem: session.NewMemory(), Deps: deps, Reg: reg, Session: "s1"}
	res, err := p.Run(context.Background(), "card 4111111111111111 please")
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "refuse" || res.ResponseText != "no cards" {
		t.Fatalf("%+v", res)
	}
}

func TestThinkPath_KnowledgeMiss_GroundingRequired(t *testing.T) {
	reg := baseReg(t)
	var doc profile.Document
	doc.Modes.Think = true
	doc.Grounding.Required = true
	doc.Routers.Think.Providers = []string{"fake-think"}
	doc.Routers.Knowledge.Providers = []string{"fake-knowledge"}
	doc.Rules = []profile.Rule{{
		ID: "no-invent", Phase: "pre_think",
		When: map[string]any{"grounding_required": true, "knowledge_hit": false},
		Action: "escalate", Message: "escalate please", Skill: "warm_transfer",
	}}
	doc.Skills.Allowed = []string{"warm_transfer"}
	doc.Skills.Definitions = map[string]profile.SkillDefinition{
		"warm_transfer": {Gateway: "fake-skill", Authority: "act", Confirm: false},
	}
	deps, err := thinkpath.Resolve(reg, doc, "live")
	if err != nil {
		t.Fatal(err)
	}
	// register knowledge with no snippets → miss
	p := &thinkpath.Path{Doc: doc, Mem: session.NewMemory(), Deps: deps, Reg: reg, Session: "s1"}
	res, err := p.Run(context.Background(), "what is my balance")
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "escalate" || !res.SkillOK {
		t.Fatalf("%+v", res)
	}
	sk := regMustSkill(t, reg)
	if sk.Calls.Load() < 1 {
		t.Fatal("skill not executed")
	}
}

func TestThinkPath_ClipWinsOverGroundingEscalate(t *testing.T) {
	reg := baseReg(t)
	var doc profile.Document
	doc.Modes.Think = true
	doc.Grounding.Required = true
	doc.Routers.Think.Providers = []string{"fake-think"}
	doc.Routers.Knowledge.Providers = []string{"fake-knowledge"}
	doc.Response = &profile.ResponseConfig{
		Ladder: []string{"clip", "llm"},
		Clips: map[string]profile.CannedUtterance{
			"greeting-en": {
				Text: "Welcome to Coral.",
				When: map[string]any{"regex": `(?i)\b(hi|hello|hey)\b`},
			},
		},
	}
	doc.Rules = []profile.Rule{{
		ID: "escalate-no-grounding", Phase: "pre_think",
		When:   map[string]any{"grounding_required": true, "knowledge_hit": false},
		Action: "escalate", Message: "no kb — escalate",
	}}
	deps, err := thinkpath.Resolve(reg, doc, "live")
	if err != nil {
		t.Fatal(err)
	}
	p := &thinkpath.Path{Doc: doc, Mem: session.NewMemory(), Deps: deps, Reg: reg, Session: "s-clip-g"}
	res, err := p.Run(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if res.ResponseTier != thinkpath.TierClip || res.ResponseText != "Welcome to Coral." {
		t.Fatalf("want greeting clip, got %+v", res)
	}
	if res.Action == "escalate" {
		t.Fatal("grounding escalate must not beat greeting clip")
	}
}

func TestThinkPath_KnowledgeHit_Think_Playbook(t *testing.T) {
	reg := router.NewMemRegistry()
	know := &fake.Knowledge{Snippets: map[string][]port.KnowledgeSnippet{
		"hours": {{Text: "Open 9-5", Score: 1}},
	}}
	items := []port.Registration{
		{ID: fake.IDThink, Port: port.PortThink, Capabilities: (&fake.Think{}).Capabilities(), Instance: &fake.Think{}},
		{ID: fake.IDKnowledge, Port: port.PortKnowledge, Capabilities: know.Capabilities(), Instance: know},
		{ID: fake.IDSkill, Port: port.PortSkill, Capabilities: (&fake.Skill{}).Capabilities(), Instance: &fake.Skill{}},
	}
	for _, it := range items {
		if err := reg.Register(it); err != nil {
			t.Fatal(err)
		}
	}
	var doc profile.Document
	doc.Modes.Think = true
	doc.Routers.Think.Providers = []string{"fake-think"}
	doc.Routers.Knowledge.Providers = []string{"fake-knowledge"}
	doc.Playbook = &profile.Playbook{
		Entry: "greet",
		States: map[string]profile.PlaybookState{
			"greet":        {OnIntent: "route_intent", Fallback: "clarify"},
			"route_intent": {Slots: []string{"intent"}, OnComplete: "fulfill"},
			"clarify":      {},
			"fulfill":      {},
		},
	}
	deps, err := thinkpath.Resolve(reg, doc, "playback")
	if err != nil {
		t.Fatal(err)
	}
	p := &thinkpath.Path{Doc: doc, Mem: session.NewMemory(), Deps: deps, Reg: reg, Session: "pb1"}
	res, err := p.Run(context.Background(), "hours")
	if err != nil {
		t.Fatal(err)
	}
	if !res.KnowledgeHit || res.ResponseText == "" {
		t.Fatalf("%+v", res)
	}
	if res.PlaybookState == "" {
		t.Fatal("expected playbook state update")
	}
}

func regMustSkill(t *testing.T, reg port.Registry) *fake.Skill {
	t.Helper()
	rec, ok := reg.Get(fake.IDSkill)
	if !ok {
		t.Fatal("no skill")
	}
	sk, ok := rec.Instance.(*fake.Skill)
	if !ok {
		t.Fatal("type")
	}
	return sk
}

func TestThinkPath_ClipSkipsLLM(t *testing.T) {
	reg := router.NewMemRegistry()
	th := &fake.Think{}
	if err := reg.Register(port.Registration{
		ID: fake.IDThink, Port: port.PortThink, Capabilities: th.Capabilities(), Instance: th,
	}); err != nil {
		t.Fatal(err)
	}
	var doc profile.Document
	doc.Modes.Think = true
	doc.Routers.Think.Providers = []string{"fake-think"}
	doc.Response = &profile.ResponseConfig{
		Ladder: []string{"clip", "template", "llm"},
		Clips: map[string]profile.CannedUtterance{
			"greeting-en": {
				Text: "Welcome to support.",
				When: map[string]any{"regex": `(?i)\b(hi|hello|hey)\b`},
			},
			"clip-apology-en": {Text: "Please hold."}, // no when → ladder skip
		},
	}
	deps, err := thinkpath.Resolve(reg, doc, "live")
	if err != nil {
		t.Fatal(err)
	}
	p := &thinkpath.Path{Doc: doc, Mem: session.NewMemory(), Deps: deps, Reg: reg, Session: "s-clip"}
	res, err := p.Run(context.Background(), "hello there")
	if err != nil {
		t.Fatal(err)
	}
	if res.ResponseTier != thinkpath.TierClip || res.ResponseText != "Welcome to support." {
		t.Fatalf("%+v", res)
	}
	if th.CompleteCalls.Load() != 0 {
		t.Fatalf("Think.Complete called %d times; want 0", th.CompleteCalls.Load())
	}
}

func TestThinkPath_TemplateSkipsLLM(t *testing.T) {
	reg := router.NewMemRegistry()
	th := &fake.Think{}
	if err := reg.Register(port.Registration{
		ID: fake.IDThink, Port: port.PortThink, Capabilities: th.Capabilities(), Instance: th,
	}); err != nil {
		t.Fatal(err)
	}
	var doc profile.Document
	doc.Modes.Think = true
	doc.Routers.Think.Providers = []string{"fake-think"}
	doc.Response = &profile.ResponseConfig{
		Ladder: []string{"clip", "template", "llm"},
		Templates: map[string]profile.CannedUtterance{
			"clarify": {Text: "Could you rephrase that?", When: map[string]any{"regex": `(?i)\b(what|huh)\b`}},
		},
	}
	deps, err := thinkpath.Resolve(reg, doc, "live")
	if err != nil {
		t.Fatal(err)
	}
	p := &thinkpath.Path{Doc: doc, Mem: session.NewMemory(), Deps: deps, Reg: reg, Session: "s-tpl"}
	res, err := p.Run(context.Background(), "huh?")
	if err != nil {
		t.Fatal(err)
	}
	if res.ResponseTier != thinkpath.TierTemplate {
		t.Fatalf("%+v", res)
	}
	if th.CompleteCalls.Load() != 0 {
		t.Fatal("template path called Think")
	}
}

func TestThinkPath_ThinkDownFallbackPinned(t *testing.T) {
	reg := router.NewMemRegistry()
	th := &fake.Think{FailWith: &port.GatewayError{Code: port.CodeUnavailable, Message: "think down", Retryable: true}}
	sk := &fake.Skill{}
	for _, it := range []port.Registration{
		{ID: fake.IDThink, Port: port.PortThink, Capabilities: th.Capabilities(), Instance: th},
		{ID: fake.IDSkill, Port: port.PortSkill, Capabilities: sk.Capabilities(), Instance: sk},
	} {
		if err := reg.Register(it); err != nil {
			t.Fatal(err)
		}
	}
	var doc profile.Document
	doc.Modes.Think = true
	doc.Routers.Think.Providers = []string{"fake-think"}
	doc.Response = &profile.ResponseConfig{
		Ladder: []string{"clip", "llm"},
		Clips: map[string]profile.CannedUtterance{
			"clip-escalate-en": {Text: "Connecting you to an agent."},
		},
	}
	doc.Fallback = &profile.FallbackConfig{
		ThinkDown: &profile.FallbackAction{SpeakCanned: "clip-escalate-en", Skill: "warm_transfer"},
	}
	doc.Skills.Allowed = []string{"warm_transfer"}
	doc.Skills.Definitions = map[string]profile.SkillDefinition{
		"warm_transfer": {Gateway: "fake-skill", Authority: "act", Confirm: false},
	}
	deps, err := thinkpath.Resolve(reg, doc, "live")
	if err != nil {
		t.Fatal(err)
	}
	p := &thinkpath.Path{
		Doc: doc, Mem: session.NewMemory(), Deps: deps, Reg: reg, Session: "s-fail",
		PinnedEngines: true,
	}
	res, err := p.Run(context.Background(), "need help with billing")
	if err != nil {
		t.Fatal(err)
	}
	if res.ResponseTier != thinkpath.TierEscalate || res.ResponseText != "Connecting you to an agent." {
		t.Fatalf("%+v", res)
	}
	if !res.SkillOK || res.SkillName != "warm_transfer" {
		t.Fatalf("skill %+v", res)
	}
	if th.CompleteCalls.Load() != 1 {
		t.Fatalf("want exactly one Think.Complete (no vendor re-Select), got %d", th.CompleteCalls.Load())
	}
	if sk.Calls.Load() < 1 {
		t.Fatal("escalate skill not executed")
	}

	// Without pin → error, no invented clip
	p2 := &thinkpath.Path{Doc: doc, Mem: session.NewMemory(), Deps: deps, Reg: reg, Session: "s-nopin"}
	_, err = p2.Run(context.Background(), "need help")
	if err == nil {
		t.Fatal("want think error without gateway_binding")
	}
}

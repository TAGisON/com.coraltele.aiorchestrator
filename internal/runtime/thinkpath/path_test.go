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

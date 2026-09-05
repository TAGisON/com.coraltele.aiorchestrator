package graph_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/flow"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/graph"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func informDoc(t *testing.T) *flow.Document {
	t.Helper()
	raw := []byte(`{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"choice","type":"ListenChoice"},
    {"id":"faq","type":"Inform","binding_ref":"faq-1"},
    {"id":"again","type":"ListenChoice"},
    {"id":"hang","type":"Tool","tool":"hangup"},
    {"id":"end","type":"End"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"choice","kind":"next"},
    {"id":"e2","from":"choice","to":"faq","kind":"intent","intent":"hours"},
    {"id":"e3","from":"faq","to":"again","kind":"next"},
    {"id":"e4","from":"faq","to":"hang","kind":"repair"},
    {"id":"e5","from":"again","to":"end","kind":"intent","intent":"done"}
  ],
  "prompts":{},
  "matrix":[],
  "binding_refs":["faq-1"]
}`)
	doc, err := flow.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := flow.Validate(doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestCursor_Inform_InlineFAQ(t *testing.T) {
	mem := store.NewMemory()
	_, err := mem.UpsertBinding(context.Background(), store.Binding{
		ID: "faq-1", TenantID: "t1", Kind: store.BindingKindKnowledge, Name: "FAQ",
		Status: store.BindingStatusActive,
		Config: json.RawMessage(`{"mode":"inline_faq","entries":[{"id":"hours","questions":["hours","open","timing"],"text":{"en-IN":"We open at 9."}}]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	doc := informDoc(t)
	cur, err := graph.New(doc, "en-IN")
	if err != nil {
		t.Fatal(err)
	}
	cur.SetInformLookup(graph.BindingInformLookup(mem, "t1", doc.BindingRefs, doc.DefaultLocale))
	if _, err := cur.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	turn, err := cur.HandleUtterance("what are your hours")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(turn.Lines, " ") != "We open at 9." {
		t.Fatalf("lines %#v", turn.Lines)
	}
	if cur.NodeID() != "again" {
		t.Fatalf("node %s", cur.NodeID())
	}
}

func TestCursor_Inform_MissGoesRepair(t *testing.T) {
	mem := store.NewMemory()
	_, _ = mem.UpsertBinding(context.Background(), store.Binding{
		ID: "faq-1", TenantID: "t1", Kind: store.BindingKindKnowledge, Name: "FAQ",
		Status: store.BindingStatusActive,
		Config: json.RawMessage(`{"mode":"inline_faq","entries":[{"id":"hours","questions":["hours"],"text":{"en-IN":"We open at 9."}}]}`),
	})
	doc := informDoc(t)
	cur, err := graph.New(doc, "en-IN")
	if err != nil {
		t.Fatal(err)
	}
	cur.SetInformLookup(graph.BindingInformLookup(mem, "t1", doc.BindingRefs, doc.DefaultLocale))
	_, _ = cur.Bootstrap()
	// Intent "hours" routes to Inform, but query text won't match FAQ keywords if we use a
	// different path — the intent edge still lands on Inform with lastQuery=utterance.
	// Use utterance that matches intent "hours" via contains: "hours" then Inform query is full text.
	// For miss: route via intent by saying "hours" but binding empty questions — already have hours.
	// Disable binding instead.
	_, _ = mem.UpsertBinding(context.Background(), store.Binding{
		ID: "faq-1", TenantID: "t1", Kind: store.BindingKindKnowledge, Name: "FAQ",
		Status: store.BindingStatusDisabled,
		Config: json.RawMessage(`{"mode":"inline_faq","entries":[]}`),
	})
	turn, err := cur.HandleUtterance("hours")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Armed == nil || turn.Armed.Kind != "hangup" {
		t.Fatalf("want hangup via repair, got %#v", turn)
	}
}

func TestValidate_InformBindingRefRequired(t *testing.T) {
	raw := []byte(`{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"faq","type":"Inform","binding_ref":"missing"}
  ],
  "edges":[{"id":"e1","from":"entry","to":"faq","kind":"next"}],
  "prompts":{},
  "matrix":[],
  "binding_refs":[]
}`)
	doc, err := flow.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := flow.Validate(doc); err == nil {
		t.Fatal("expected reject")
	}
}

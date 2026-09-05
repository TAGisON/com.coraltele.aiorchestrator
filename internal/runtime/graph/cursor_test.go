package graph_test

import (
	"strings"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/flow"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/graph"
)

func sampleDoc(t *testing.T) *flow.Document {
	t.Helper()
	raw := []byte(`{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"welcome","type":"Speak","prompt_ref":"welcome"},
    {"id":"choice","type":"ListenChoice"},
    {"id":"bye","type":"Speak","prompt_ref":"bye"},
    {"id":"end","type":"End"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"welcome","kind":"next"},
    {"id":"e2","from":"welcome","to":"choice","kind":"next"},
    {"id":"e3","from":"choice","to":"bye","kind":"intent","intent":"done"},
    {"id":"e4","from":"bye","to":"end","kind":"next"}
  ],
  "prompts":{
    "welcome":{"en-IN":"Welcome to Coral"},
    "bye":{"en-IN":"Goodbye"}
  },
  "matrix":[],
  "binding_refs":[]
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

func TestCursor_WelcomeChoiceEnd(t *testing.T) {
	cur, err := graph.New(sampleDoc(t), "en-IN")
	if err != nil {
		t.Fatal(err)
	}
	boot, err := cur.Bootstrap()
	if err != nil {
		t.Fatal(err)
	}
	if boot.Ended || boot.NoMatch {
		t.Fatalf("boot %#v", boot)
	}
	if len(boot.Lines) != 1 || boot.Lines[0] != "Welcome to Coral" {
		t.Fatalf("lines %#v", boot.Lines)
	}
	if cur.NodeID() != "choice" {
		t.Fatalf("node %s", cur.NodeID())
	}

	miss, err := cur.HandleUtterance("xyz")
	if err != nil {
		t.Fatal(err)
	}
	if !miss.NoMatch || cur.NodeID() != "choice" {
		t.Fatalf("miss %#v node %s", miss, cur.NodeID())
	}

	turn, err := cur.HandleUtterance("I am done thanks")
	if err != nil {
		t.Fatal(err)
	}
	if turn.NoMatch || !turn.Ended {
		t.Fatalf("turn %#v", turn)
	}
	joined := strings.Join(turn.Lines, " ")
	if joined != "Goodbye" {
		t.Fatalf("spoken %q", joined)
	}
	if cur.NodeID() != "end" {
		t.Fatalf("final node %s", cur.NodeID())
	}
}

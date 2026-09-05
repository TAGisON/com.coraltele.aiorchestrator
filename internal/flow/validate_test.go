package flow_test

import (
	"strings"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/flow"
)

func validMinimalJSON() string {
	return `{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"welcome","type":"Speak","prompt_ref":"welcome"},
    {"id":"end","type":"End"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"welcome","kind":"next"},
    {"id":"e2","from":"welcome","to":"end","kind":"next"}
  ],
  "prompts":{"welcome":{"en-IN":"Hello"}},
  "matrix":[],
  "binding_refs":[]
}`
}

func TestValidate_OK(t *testing.T) {
	doc, err := flow.Parse([]byte(validMinimalJSON()))
	if err != nil {
		t.Fatal(err)
	}
	if err := flow.Validate(doc); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_BadSchema(t *testing.T) {
	raw := strings.Replace(validMinimalJSON(), `coral.flow.v1`, `x_desk`, 1)
	doc, err := flow.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	err = flow.Validate(doc)
	if err == nil {
		t.Fatal("expected error")
	}
	ve, ok := err.(*flow.ValidationError)
	if !ok || !strings.Contains(ve.Message, "schema_id") {
		t.Fatalf("got %v", err)
	}
}

func TestValidate_TransferNeedsMatrix(t *testing.T) {
	raw := `{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"xfer","type":"Tool","tool":"transfer","matrix_intent":"sales"},
    {"id":"end","type":"End"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"xfer","kind":"next"},
    {"id":"e2","from":"xfer","to":"end","kind":"next"}
  ],
  "prompts":{},
  "matrix":[],
  "binding_refs":[]
}`
	doc, err := flow.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := flow.Validate(doc); err == nil {
		t.Fatal("expected matrix rejection")
	}
}

func TestValidate_TransferWithMatrix_OK(t *testing.T) {
	raw := `{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"xfer","type":"Tool","tool":"transfer","matrix_intent":"sales","prompt_ref":"closing_transfer"},
    {"id":"end","type":"End"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"xfer","kind":"next"},
    {"id":"e2","from":"xfer","to":"end","kind":"tool_result"}
  ],
  "prompts":{"closing_transfer":{"en-IN":"Connecting you"}},
  "matrix":[
    {"intent":"sales","owner":"Sales","target":"sales-q","number":"1001","action":"transfer","disposition_code":"transferred_sales"}
  ],
  "binding_refs":[]
}`
	doc, err := flow.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := flow.Validate(doc); err != nil {
		t.Fatal(err)
	}
}

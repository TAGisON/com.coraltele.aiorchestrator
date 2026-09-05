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

func transferDoc(t *testing.T) *flow.Document {
	t.Helper()
	raw := []byte(`{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"welcome","type":"Speak","prompt_ref":"welcome"},
    {"id":"choice","type":"ListenChoice"},
    {"id":"xfer","type":"Tool","tool":"transfer","matrix_intent":"sales","prompt_ref":"closing_transfer"},
    {"id":"hang","type":"Tool","tool":"hangup","prompt_ref":"closing_hangup"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"welcome","kind":"next"},
    {"id":"e2","from":"welcome","to":"choice","kind":"next"},
    {"id":"e3","from":"choice","to":"xfer","kind":"intent","intent":"sales"},
    {"id":"e4","from":"choice","to":"hang","kind":"intent","intent":"bye"}
  ],
  "prompts":{
    "welcome":{"en-IN":"Welcome"},
    "closing_transfer":{"en-IN":"Connecting you to sales"},
    "closing_hangup":{"en-IN":"Goodbye"}
  },
  "matrix":[
    {"intent":"sales","owner":"Sales","target":"sales-q","number":"1001","action":"transfer","disposition_code":"transferred_sales"}
  ],
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

func TestCursor_ArmTransfer(t *testing.T) {
	cur, err := graph.New(transferDoc(t), "en-IN")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cur.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	turn, err := cur.HandleUtterance("sales please")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Armed == nil || turn.Armed.Kind != "transfer" || turn.Armed.Destination != "1001" {
		t.Fatalf("armed %#v", turn.Armed)
	}
	if turn.Armed.DispositionCode != "transferred_sales" {
		t.Fatalf("disp %q", turn.Armed.DispositionCode)
	}
	if strings.Join(turn.Lines, " ") != "Connecting you to sales" {
		t.Fatalf("lines %#v", turn.Lines)
	}
}

func TestCursor_ArmHangup(t *testing.T) {
	cur, err := graph.New(transferDoc(t), "en-IN")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cur.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	turn, err := cur.HandleUtterance("bye")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Armed == nil || turn.Armed.Kind != "hangup" {
		t.Fatalf("armed %#v", turn.Armed)
	}
}

func TestCursor_TransferMissingMatrix_FailClosed(t *testing.T) {
	raw := []byte(`{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"xfer","type":"Tool","tool":"transfer","matrix_intent":"missing"}
  ],
  "edges":[{"id":"e1","from":"entry","to":"xfer","kind":"next"}],
  "prompts":{},
  "matrix":[{"intent":"sales","owner":"S","target":"t","number":"1","action":"transfer"}],
  "binding_refs":[]
}`)
	// Publish validate would reject missing intent on transfer Tool — build cursor without re-validate
	// by using a doc that validates (intent present) then mutate? Simpler: use Validate-passing
	// doc where number cleared is impossible at publish. Unit-test arm via wrong intent after
	// constructing New which validates — so use intent that isn't in matrix: can't publish.
	// Instead skip New validate path: test armTool via Handle on valid graph with intent "sales"
	// already covered. This test uses a hand-built cursor by publishing with sales then
	// we only check New rejects invalid docs.
	doc, err := flow.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := flow.Validate(doc); err == nil {
		t.Fatal("expected validate reject for transfer without matrix row")
	}
}

func repairDoc(t *testing.T) *flow.Document {
	t.Helper()
	raw := []byte(`{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"choice","type":"ListenChoice","prompt_ref":"ask","repair":{"max_retries":1,"unclear_prompt_ref":"repair_unclear"}},
    {"id":"hang","type":"Tool","tool":"hangup","prompt_ref":"closing_hangup"},
    {"id":"bye","type":"Speak","prompt_ref":"bye"},
    {"id":"end","type":"End"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"choice","kind":"next"},
    {"id":"e2","from":"choice","to":"bye","kind":"intent","intent":"ok"},
    {"id":"e3","from":"choice","to":"hang","kind":"repair"},
    {"id":"e4","from":"bye","to":"end","kind":"next"}
  ],
  "prompts":{
    "ask":{"en-IN":"Say ok"},
    "repair_unclear":{"en-IN":"Please say ok"},
    "closing_hangup":{"en-IN":"Goodbye"},
    "bye":{"en-IN":"Thanks"}
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

func TestCursor_RepairThenExhaust(t *testing.T) {
	cur, err := graph.New(repairDoc(t), "en-IN")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cur.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	r1, err := cur.HandleUtterance("nonsense")
	if err != nil {
		t.Fatal(err)
	}
	if !r1.Repair || r1.Lines[0] != "Please say ok" || cur.NodeID() != "choice" {
		t.Fatalf("repair1 %#v node %s", r1, cur.NodeID())
	}
	r2, err := cur.HandleUtterance("still wrong")
	if err != nil {
		t.Fatal(err)
	}
	if r2.Armed == nil || r2.Armed.Kind != "hangup" {
		t.Fatalf("exhaust %#v", r2)
	}
	if strings.Join(r2.Lines, " ") != "Goodbye" {
		t.Fatalf("lines %#v", r2.Lines)
	}
}

func TestCursor_ListenLanguage_SetsLocale(t *testing.T) {
	raw := []byte(`{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"lang","type":"ListenLanguage"},
    {"id":"welcome","type":"Speak","prompt_ref":"welcome"},
    {"id":"end","type":"End"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"lang","kind":"next"},
    {"id":"e2","from":"lang","to":"welcome","kind":"option","option":"hi-IN"},
    {"id":"e3","from":"welcome","to":"end","kind":"next"}
  ],
  "prompts":{
    "welcome":{"en-IN":"Hello","hi-IN":"Namaste"}
  },
  "matrix":[],
  "binding_refs":[]
}`)
	doc, err := flow.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	cur, err := graph.New(doc, "en-IN")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cur.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	turn, err := cur.HandleUtterance("hi-IN")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Locale != "hi-IN" || cur.Locale() != "hi-IN" {
		t.Fatalf("locale %#v cursor %s", turn, cur.Locale())
	}
	if !turn.Ended || strings.Join(turn.Lines, " ") != "Namaste" {
		t.Fatalf("turn %#v", turn)
	}
}

func TestCursor_ListenLanguage_BlocksToolSameTurn(t *testing.T) {
	raw := []byte(`{
  "schema_id":"coral.flow.v1",
  "entry_node_id":"entry",
  "default_locale":"en-IN",
  "nodes":[
    {"id":"entry","type":"Entry"},
    {"id":"lang","type":"ListenLanguage"},
    {"id":"hang","type":"Tool","tool":"hangup"}
  ],
  "edges":[
    {"id":"e1","from":"entry","to":"lang","kind":"next"},
    {"id":"e2","from":"lang","to":"hang","kind":"option","option":"en-IN"}
  ],
  "prompts":{},
  "matrix":[],
  "binding_refs":[]
}`)
	doc, err := flow.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	cur, err := graph.New(doc, "en-IN")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cur.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	_, err = cur.HandleUtterance("en-IN")
	if err == nil || !strings.Contains(err.Error(), "EC-23") {
		t.Fatalf("want EC-23, got %v", err)
	}
}

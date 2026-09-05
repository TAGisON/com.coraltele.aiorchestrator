// Package flow parses and validates coral.flow.v1 conversation graph documents
// (P2.7 envelope, P2.8 prompts, P2.9 matrix) for publish-time checks.
package flow

import "encoding/json"

// SchemaID is the locked document envelope identifier.
const SchemaID = "coral.flow.v1"

// Closed node types (docs/03_BRAIN_AND_GRAPH.md / P2.7).
const (
	NodeEntry          = "Entry"
	NodeSpeak          = "Speak"
	NodeListenChoice   = "ListenChoice"
	NodeListenLanguage = "ListenLanguage"
	NodeListenSlot     = "ListenSlot"
	NodeDecide         = "Decide"
	NodeInform         = "Inform"
	NodeTool           = "Tool"
	NodeEnd            = "End"
)

// Closed edge kinds (P2.7).
const (
	EdgeNext       = "next"
	EdgeOption     = "option"
	EdgeIntent     = "intent"
	EdgeRetry      = "retry"
	EdgeBack       = "back"
	EdgeSkip       = "skip"
	EdgeRepair     = "repair"
	EdgeToolResult = "tool_result"
	EdgeGlobal     = "global"
)

// Tool verbs on Tool nodes.
const (
	ToolTransfer = "transfer"
	ToolHangup   = "hangup"
)

// Document is the coral.flow.v1 envelope subset needed for publish validation.
type Document struct {
	SchemaID      string                     `json:"schema_id"`
	EntryNodeID   string                     `json:"entry_node_id"`
	DefaultLocale string                     `json:"default_locale"`
	Nodes         []Node                     `json:"nodes"`
	Edges         []Edge                     `json:"edges"`
	Prompts       map[string]map[string]string `json:"prompts"`
	Matrix        []MatrixRow                `json:"matrix"`
	BindingRefs   []string                   `json:"binding_refs"`
	Globals       []string                   `json:"globals"`
	CX            json.RawMessage            `json:"cx"`
}

// Node is one graph node.
type Node struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	PromptRef  string          `json:"prompt_ref"`
	BindingRef string          `json:"binding_ref"`
	Tool       string          `json:"tool"`
	Repair     json.RawMessage `json:"repair"`
	Slot       json.RawMessage `json:"slot"`
	MatrixIntent string        `json:"matrix_intent"`
}

// Edge is one graph edge.
type Edge struct {
	ID           string `json:"id"`
	From         string `json:"from"`
	To           string `json:"to"`
	Kind         string `json:"kind"`
	Intent       string `json:"intent"`
	Option       string `json:"option"`
	MatrixIntent string `json:"matrix_intent"`
}

// MatrixRow is one routing matrix entry (P2.9).
type MatrixRow struct {
	Intent          string `json:"intent"`
	Owner           string `json:"owner"`
	Target          string `json:"target"`
	Number          string `json:"number"`
	Action          string `json:"action"`
	DispositionCode string `json:"disposition_code"`
}

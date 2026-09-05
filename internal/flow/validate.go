package flow

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// ValidationError is a publish-time structural failure.
type ValidationError struct {
	Message string
	Details map[string]any
}

func (e *ValidationError) Error() string {
	if e == nil {
		return "flow validation error"
	}
	return e.Message
}

func invalid(msg string, details map[string]any) error {
	return &ValidationError{Message: msg, Details: details}
}

var nodeTypes = map[string]struct{}{
	NodeEntry: {}, NodeSpeak: {}, NodeListenChoice: {}, NodeListenLanguage: {},
	NodeListenSlot: {}, NodeDecide: {}, NodeInform: {}, NodeTool: {}, NodeEnd: {},
}

var edgeKinds = map[string]struct{}{
	EdgeNext: {}, EdgeOption: {}, EdgeIntent: {}, EdgeRetry: {}, EdgeBack: {},
	EdgeSkip: {}, EdgeRepair: {}, EdgeToolResult: {}, EdgeGlobal: {},
}

// Parse unmarshals raw JSON into a Document.
func Parse(raw []byte) (*Document, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, invalid("empty document", nil)
	}
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, invalid("invalid json: "+err.Error(), nil)
	}
	return &doc, nil
}

// Validate applies P2.7 / P2.8 / P2.9 publish rules.
func Validate(doc *Document) error {
	if doc == nil {
		return invalid("nil document", nil)
	}
	if doc.SchemaID != SchemaID {
		return invalid("schema_id must be coral.flow.v1", map[string]any{"schema_id": doc.SchemaID})
	}
	if strings.TrimSpace(doc.DefaultLocale) == "" {
		return invalid("default_locale required", nil)
	}
	if doc.Nodes == nil {
		return invalid("nodes required", nil)
	}
	if doc.Edges == nil {
		return invalid("edges required", nil)
	}
	if doc.Prompts == nil {
		return invalid("prompts required (object)", nil)
	}
	if doc.Matrix == nil {
		return invalid("matrix required (array)", nil)
	}
	if doc.BindingRefs == nil {
		return invalid("binding_refs required (array)", nil)
	}

	nodesByID := make(map[string]Node, len(doc.Nodes))
	var entryCount int
	for i, n := range doc.Nodes {
		id := strings.TrimSpace(n.ID)
		if id == "" {
			return invalid("node id required", map[string]any{"index": i})
		}
		if _, dup := nodesByID[id]; dup {
			return invalid("duplicate node id", map[string]any{"node_id": id})
		}
		if _, ok := nodeTypes[n.Type]; !ok {
			return invalid("unknown node type", map[string]any{"node_id": id, "type": n.Type})
		}
		if n.Type == NodeEntry {
			entryCount++
		}
		if n.Type == NodeTool {
			tool := strings.TrimSpace(n.Tool)
			if tool != ToolTransfer && tool != ToolHangup {
				return invalid("Tool node requires tool=transfer|hangup", map[string]any{"node_id": id, "tool": n.Tool})
			}
		}
		nodesByID[id] = n
	}
	if entryCount == 0 {
		return invalid("Entry node required", nil)
	}
	if entryCount > 1 {
		return invalid("exactly one Entry node required", map[string]any{"count": entryCount})
	}
	entryID := strings.TrimSpace(doc.EntryNodeID)
	if entryID == "" {
		return invalid("entry_node_id required", nil)
	}
	en, ok := nodesByID[entryID]
	if !ok {
		return invalid("entry_node_id must reference a node", map[string]any{"entry_node_id": entryID})
	}
	if en.Type != NodeEntry {
		return invalid("entry_node_id must point at Entry", map[string]any{"entry_node_id": entryID, "type": en.Type})
	}

	for i, e := range doc.Edges {
		eid := strings.TrimSpace(e.ID)
		if eid == "" {
			return invalid("edge id required", map[string]any{"index": i})
		}
		if _, ok := edgeKinds[e.Kind]; !ok {
			return invalid("unknown edge kind", map[string]any{"edge_id": eid, "kind": e.Kind})
		}
		if _, ok := nodesByID[e.From]; !ok {
			return invalid("edge.from unknown node", map[string]any{"edge_id": eid, "from": e.From})
		}
		if _, ok := nodesByID[e.To]; !ok {
			return invalid("edge.to unknown node", map[string]any{"edge_id": eid, "to": e.To})
		}
	}

	// P2.8: every prompt_ref must have default_locale text.
	for _, n := range doc.Nodes {
		ref := strings.TrimSpace(n.PromptRef)
		if ref == "" {
			continue
		}
		locales, ok := doc.Prompts[ref]
		if !ok || locales == nil {
			return invalid("prompt_ref missing from prompts", map[string]any{"prompt_ref": ref, "node_id": n.ID})
		}
		if strings.TrimSpace(locales[doc.DefaultLocale]) == "" {
			return invalid("prompt_ref missing default_locale text", map[string]any{
				"prompt_ref": ref, "default_locale": doc.DefaultLocale, "node_id": n.ID,
			})
		}
	}

	// P2.9 matrix.
	matrixByIntent := make(map[string]MatrixRow, len(doc.Matrix))
	for i, row := range doc.Matrix {
		intent := strings.TrimSpace(row.Intent)
		if intent == "" {
			return invalid("matrix intent required", map[string]any{"index": i})
		}
		if _, dup := matrixByIntent[intent]; dup {
			return invalid("duplicate matrix intent", map[string]any{"intent": intent})
		}
		if strings.TrimSpace(row.Owner) == "" || strings.TrimSpace(row.Target) == "" {
			return invalid("matrix owner and target required", map[string]any{"intent": intent})
		}
		action := strings.TrimSpace(row.Action)
		if action == "" {
			return invalid("matrix action required", map[string]any{"intent": intent})
		}
		if action != ToolTransfer {
			return invalid("matrix action must be transfer in V1", map[string]any{"intent": intent, "action": action})
		}
		if strings.TrimSpace(row.Number) == "" {
			return invalid("matrix number required for transfer", map[string]any{"intent": intent})
		}
		if code := strings.TrimSpace(row.DispositionCode); code != "" && !store.ValidDispositionFinal(code) {
			return invalid("matrix disposition_code not allowlisted", map[string]any{"intent": intent, "disposition_code": code})
		}
		matrixByIntent[intent] = row
	}

	var transferTools int
	for _, n := range doc.Nodes {
		if n.Type != NodeTool || strings.TrimSpace(n.Tool) != ToolTransfer {
			continue
		}
		transferTools++
		intent := strings.TrimSpace(n.MatrixIntent)
		if intent == "" {
			// Fall back: look for an outgoing edge with matrix_intent.
			for _, e := range doc.Edges {
				if e.From == n.ID && strings.TrimSpace(e.MatrixIntent) != "" {
					intent = strings.TrimSpace(e.MatrixIntent)
					break
				}
			}
		}
		if intent == "" {
			return invalid("transfer Tool requires matrix_intent", map[string]any{"node_id": n.ID})
		}
		row, ok := matrixByIntent[intent]
		if !ok {
			return invalid("transfer Tool matrix_intent not in matrix", map[string]any{"node_id": n.ID, "intent": intent})
		}
		if strings.TrimSpace(row.Number) == "" {
			return invalid("transfer Tool matrix row missing number", map[string]any{"node_id": n.ID, "intent": intent})
		}
	}
	if transferTools > 0 && len(doc.Matrix) == 0 {
		return invalid("transfer Tool requires non-empty matrix", nil)
	}

	return nil
}

// ContentHash returns a stable hex SHA-256 of the published document bytes.
func ContentHash(raw []byte) string {
	sum := sha256Sum(raw)
	return fmt.Sprintf("%x", sum)
}

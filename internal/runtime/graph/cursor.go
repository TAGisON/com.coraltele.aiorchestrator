// Package graph implements the coral.flow.v1 runtime cursor (G.3 core walk).
package graph

import (
	"fmt"
	"strings"
	"sync"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/flow"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// Turn is one cursor step result for the Talk shell.
type Turn struct {
	Lines    []string // texts to Speak in order (includes Tool closing line)
	Ended    bool     // landed on End (after speaking Lines)
	NoMatch  bool     // ListenChoice had no legal edge and no repair policy
	Repair   bool     // unclear reprompt; cursor stays on listen node
	Locale   string   // set when ListenLanguage matched (BCP-47)
	NodeID   string   // cursor after this turn
	EdgeID   string   // edge taken this turn (if any)
	Armed    *ArmedTool
}

// ArmedTool is a frozen irreversible action ready for arm→speak→exec (G.4).
type ArmedTool struct {
	Kind            string // transfer | hangup
	Destination     string // matrix number (transfer only)
	Owner           string
	Target          string
	Intent          string
	DispositionCode string
	NodeID          string
}

// Cursor sits on exactly one node of a published flow document.
type Cursor struct {
	mu        sync.Mutex
	doc       *flow.Document
	nodeID    string
	locale    string // active language hint; resolve falls back to default_locale
	nodes     map[string]flow.Node
	edges     []flow.Edge
	lastEdge  flow.Edge
	armed     bool           // Tool already armed this cursor lifetime (exec once)
	retries   map[string]int // per listen-node unclear counts
	blockTool bool           // EC-23: set during ListenLanguage same-turn advance
}

// New builds a cursor at entry_node_id.
func New(doc *flow.Document, locale string) (*Cursor, error) {
	if doc == nil {
		return nil, fmt.Errorf("nil flow document")
	}
	if err := flow.Validate(doc); err != nil {
		return nil, err
	}
	nodes := make(map[string]flow.Node, len(doc.Nodes))
	for _, n := range doc.Nodes {
		nodes[n.ID] = n
	}
	loc := strings.TrimSpace(locale)
	if loc == "" {
		loc = doc.DefaultLocale
	}
	c := &Cursor{
		doc:     doc,
		nodeID:  doc.EntryNodeID,
		locale:  loc,
		nodes:   nodes,
		edges:   append([]flow.Edge(nil), doc.Edges...),
		retries: make(map[string]int),
	}
	return c, nil
}

// Locale returns the cursor's active prompt locale.
func (c *Cursor) Locale() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.locale
}

// SetLocale updates prompt resolution language (ListenLanguage / prefs).
func (c *Cursor) SetLocale(locale string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s := strings.TrimSpace(locale); s != "" {
		c.locale = s
	}
}

// NodeID returns the current cursor node.
func (c *Cursor) NodeID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nodeID
}

// Bootstrap advances from Entry through Speak nodes via sole `next` edges until
// ListenChoice, End, Tool, or a non-auto node. Speaks collected Speak prompts.
func (c *Cursor) Bootstrap() (Turn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.advanceSilentLocked()
}

// HandleUtterance matches a listen-node utterance to an option/intent edge,
// then auto-advances Speak/`next` until the next wait or terminal node.
// Unclear → repair policy; ListenLanguage sets locale (never Tool same turn).
func (c *Cursor) HandleUtterance(text string) (Turn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, ok := c.nodes[c.nodeID]
	if !ok {
		return Turn{}, fmt.Errorf("cursor on unknown node %q", c.nodeID)
	}
	switch n.Type {
	case flow.NodeListenChoice:
		return c.handleChoiceLocked(n, text)
	case flow.NodeListenLanguage:
		return c.handleLanguageLocked(n, text)
	default:
		return Turn{}, fmt.Errorf("utterance only valid on ListenChoice/ListenLanguage (at %s/%s)", n.ID, n.Type)
	}
}

func (c *Cursor) handleChoiceLocked(n flow.Node, text string) (Turn, error) {
	edge, ok := matchChoice(c.outEdgesLocked(c.nodeID), text)
	if !ok {
		return c.repairLocked(n)
	}
	c.retries[n.ID] = 0
	if err := c.takeLocked(edge); err != nil {
		return Turn{}, err
	}
	turn := Turn{EdgeID: edge.ID, NodeID: c.nodeID}
	rest, err := c.advanceSilentLocked()
	if err != nil {
		return Turn{}, err
	}
	mergeAdvance(&turn, rest)
	return turn, nil
}

func (c *Cursor) handleLanguageLocked(n flow.Node, text string) (Turn, error) {
	edge, ok := matchChoice(c.outEdgesLocked(c.nodeID), text)
	if !ok {
		return c.repairLocked(n)
	}
	c.retries[n.ID] = 0
	locale := strings.TrimSpace(edge.Intent)
	if locale == "" {
		locale = strings.TrimSpace(edge.Option)
	}
	if locale == "" {
		return Turn{}, fmt.Errorf("ListenLanguage edge %q missing intent/option locale", edge.ID)
	}
	c.locale = locale
	if err := c.takeLocked(edge); err != nil {
		return Turn{}, err
	}
	turn := Turn{EdgeID: edge.ID, NodeID: c.nodeID, Locale: locale}
	c.blockTool = true
	defer func() { c.blockTool = false }()
	rest, err := c.advanceSilentLocked()
	if err != nil {
		return Turn{}, err
	}
	mergeAdvance(&turn, rest)
	turn.Locale = locale
	return turn, nil
}

func mergeAdvance(turn *Turn, rest Turn) {
	turn.Lines = rest.Lines
	turn.Ended = rest.Ended
	turn.Armed = rest.Armed
	turn.NodeID = rest.NodeID
	if turn.EdgeID == "" {
		turn.EdgeID = rest.EdgeID
	}
}

func (c *Cursor) repairLocked(n flow.Node) (Turn, error) {
	pol, has, err := flow.ParseRepair(n.Repair)
	if err != nil {
		return Turn{}, fmt.Errorf("node %s repair: %w", n.ID, err)
	}
	if !has {
		return Turn{NoMatch: true, NodeID: c.nodeID}, nil
	}
	c.retries[n.ID]++
	max := pol.EffectiveMaxRetries()
	if c.retries[n.ID] <= max {
		ref := strings.TrimSpace(pol.UnclearPromptRef)
		if ref == "" {
			ref = strings.TrimSpace(n.PromptRef)
		}
		line, err := c.resolvePromptLocked(ref)
		if err != nil {
			return Turn{}, err
		}
		var lines []string
		if line != "" {
			lines = []string{line}
		}
		return Turn{Repair: true, Lines: lines, NodeID: c.nodeID}, nil
	}
	// Exhausted → drawn repair edge.
	e, err := soleRepair(c.outEdgesLocked(c.nodeID))
	if err != nil {
		return Turn{}, fmt.Errorf("node %s repair exhausted: %w", n.ID, err)
	}
	c.retries[n.ID] = 0
	if err := c.takeLocked(e); err != nil {
		return Turn{}, err
	}
	turn := Turn{EdgeID: e.ID, NodeID: c.nodeID}
	rest, err := c.advanceSilentLocked()
	if err != nil {
		return Turn{}, err
	}
	mergeAdvance(&turn, rest)
	return turn, nil
}

func soleRepair(edges []flow.Edge) (flow.Edge, error) {
	var repairs []flow.Edge
	for _, e := range edges {
		if e.Kind == flow.EdgeRepair {
			repairs = append(repairs, e)
		}
	}
	if len(repairs) == 0 {
		return flow.Edge{}, fmt.Errorf("no repair edge drawn for on_exhausted")
	}
	if len(repairs) > 1 {
		return flow.Edge{}, fmt.Errorf("ambiguous repair edges (%d)", len(repairs))
	}
	return repairs[0], nil
}

func (c *Cursor) advanceSilentLocked() (Turn, error) {
	var out Turn
	for {
		n, ok := c.nodes[c.nodeID]
		if !ok {
			return Turn{}, fmt.Errorf("cursor on unknown node %q", c.nodeID)
		}
		out.NodeID = c.nodeID
		switch n.Type {
		case flow.NodeEntry:
			e, err := soleNext(c.outEdgesLocked(c.nodeID))
			if err != nil {
				return Turn{}, err
			}
			if err := c.takeLocked(e); err != nil {
				return Turn{}, err
			}
			out.EdgeID = e.ID
			continue
		case flow.NodeSpeak:
			line, err := c.resolvePromptLocked(n.PromptRef)
			if err != nil {
				return Turn{}, err
			}
			if line != "" {
				out.Lines = append(out.Lines, line)
			}
			e, err := soleNext(c.outEdgesLocked(c.nodeID))
			if err != nil {
				return Turn{}, err
			}
			if err := c.takeLocked(e); err != nil {
				return Turn{}, err
			}
			out.EdgeID = e.ID
			continue
		case flow.NodeListenChoice, flow.NodeListenLanguage, flow.NodeListenSlot:
			return out, nil
		case flow.NodeEnd:
			out.Ended = true
			return out, nil
		case flow.NodeTool:
			if c.blockTool {
				return Turn{}, fmt.Errorf("ListenLanguage cannot reach Tool same turn (EC-23)")
			}
			armed, err := c.armToolLocked(n)
			if err != nil {
				return Turn{}, err
			}
			if line, err := c.resolvePromptLocked(n.PromptRef); err != nil {
				return Turn{}, err
			} else if line != "" {
				out.Lines = append(out.Lines, line)
			}
			out.Armed = armed
			out.NodeID = c.nodeID
			return out, nil
		case flow.NodeDecide, flow.NodeInform:
			return Turn{}, fmt.Errorf("node type %s at %q not implemented yet", n.Type, n.ID)
		default:
			return Turn{}, fmt.Errorf("unsupported node type %q", n.Type)
		}
	}
}

func (c *Cursor) armToolLocked(n flow.Node) (*ArmedTool, error) {
	if c.armed {
		return nil, fmt.Errorf("tool already armed on this session cursor")
	}
	tool := strings.TrimSpace(n.Tool)
	switch tool {
	case flow.ToolHangup:
		c.armed = true
		return &ArmedTool{
			Kind:            flow.ToolHangup,
			DispositionCode: store.DispositionFinalHangupCompleted,
			NodeID:          n.ID,
		}, nil
	case flow.ToolTransfer:
		intent := strings.TrimSpace(n.MatrixIntent)
		if intent == "" {
			intent = strings.TrimSpace(c.lastEdge.MatrixIntent)
		}
		if intent == "" {
			return nil, fmt.Errorf("transfer Tool %q missing matrix_intent", n.ID)
		}
		var row *flow.MatrixRow
		for i := range c.doc.Matrix {
			if strings.TrimSpace(c.doc.Matrix[i].Intent) == intent {
				row = &c.doc.Matrix[i]
				break
			}
		}
		if row == nil {
			return nil, fmt.Errorf("transfer Tool %q intent %q not in matrix", n.ID, intent)
		}
		num := strings.TrimSpace(row.Number)
		if num == "" {
			return nil, fmt.Errorf("transfer Tool %q matrix row %q has empty number", n.ID, intent)
		}
		c.armed = true
		return &ArmedTool{
			Kind:            flow.ToolTransfer,
			Destination:     num,
			Owner:           strings.TrimSpace(row.Owner),
			Target:          strings.TrimSpace(row.Target),
			Intent:          intent,
			DispositionCode: strings.TrimSpace(row.DispositionCode),
			NodeID:          n.ID,
		}, nil
	default:
		return nil, fmt.Errorf("Tool node %q unknown tool %q", n.ID, tool)
	}
}

func (c *Cursor) takeLocked(e flow.Edge) error {
	if _, ok := c.nodes[e.To]; !ok {
		return fmt.Errorf("edge %s to unknown node %q", e.ID, e.To)
	}
	c.lastEdge = e
	c.nodeID = e.To
	return nil
}

func (c *Cursor) outEdgesLocked(from string) []flow.Edge {
	var out []flow.Edge
	for _, e := range c.edges {
		if e.From == from {
			out = append(out, e)
		}
	}
	return out
}

func (c *Cursor) resolvePromptLocked(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", nil
	}
	locales, ok := c.doc.Prompts[ref]
	if !ok || locales == nil {
		return "", fmt.Errorf("prompt_ref %q missing", ref)
	}
	if t := strings.TrimSpace(locales[c.locale]); t != "" {
		return t, nil
	}
	if t := strings.TrimSpace(locales[c.doc.DefaultLocale]); t != "" {
		return t, nil
	}
	return "", fmt.Errorf("prompt_ref %q has no text for %s or default %s", ref, c.locale, c.doc.DefaultLocale)
}

func soleNext(edges []flow.Edge) (flow.Edge, error) {
	var nexts []flow.Edge
	for _, e := range edges {
		if e.Kind == flow.EdgeNext {
			nexts = append(nexts, e)
		}
	}
	if len(nexts) == 0 {
		return flow.Edge{}, fmt.Errorf("no next edge from node")
	}
	if len(nexts) > 1 {
		return flow.Edge{}, fmt.Errorf("ambiguous next edges (%d)", len(nexts))
	}
	return nexts[0], nil
}

func matchChoice(edges []flow.Edge, utterance string) (flow.Edge, bool) {
	u := normalize(utterance)
	if u == "" {
		return flow.Edge{}, false
	}
	var best flow.Edge
	bestScore := 0
	for _, e := range edges {
		if e.Kind != flow.EdgeOption && e.Kind != flow.EdgeIntent {
			continue
		}
		key := normalize(e.Intent)
		if key == "" {
			key = normalize(e.Option)
		}
		if key == "" {
			continue
		}
		score := 0
		if u == key {
			score = 3
		} else if strings.Contains(u, key) || strings.Contains(key, u) {
			score = 2
		}
		if score > bestScore {
			bestScore = score
			best = e
		}
	}
	if bestScore == 0 {
		return flow.Edge{}, false
	}
	return best, true
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

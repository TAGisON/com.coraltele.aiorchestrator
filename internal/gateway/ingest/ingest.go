// Package ingest implements the first-party ingest-default Knowledge gateway.
package ingest

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

const ID port.GatewayID = "ingest-default"

// Gateway retrieves from durable KB chunks (substring/keyword lab scorer).
type Gateway struct {
	repo store.Repository

	mu    sync.RWMutex
	local []store.KBChunk // optional in-memory overlay for tests without store list
}

// New returns an ingest-default gateway backed by repo (may be nil if only IndexLocal used).
func New(repo store.Repository) *Gateway {
	return &Gateway{repo: repo}
}

func (g *Gateway) ID() port.GatewayID { return ID }

func (g *Gateway) Capabilities() port.Capability {
	return port.Capability{Batch: true}
}

// IndexLocal adds chunks for lab tests without Control upload.
func (g *Gateway) IndexLocal(chunks ...store.KBChunk) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.local = append(g.local, chunks...)
}

func (g *Gateway) Retrieve(ctx context.Context, q port.KnowledgeQuery) (port.KnowledgeResult, error) {
	chunks, err := g.load(ctx, q)
	if err != nil {
		return port.KnowledgeResult{}, err
	}
	query := strings.ToLower(strings.TrimSpace(q.Query))
	if query == "" {
		return port.KnowledgeResult{Hit: false}, nil
	}
	topK := q.TopK
	if topK <= 0 {
		topK = 5
	}
	type scored struct {
		snip  port.KnowledgeSnippet
		score float32
	}
	var hits []scored
	for _, ch := range chunks {
		textLower := strings.ToLower(ch.Text)
		if !strings.Contains(textLower, query) {
			// token overlap
			ok := false
			for _, tok := range strings.Fields(query) {
				if len(tok) >= 3 && strings.Contains(textLower, tok) {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		score := float32(0.5)
		if strings.Contains(textLower, query) {
			score = 1.0
		}
		hits = append(hits, scored{
			snip: port.KnowledgeSnippet{
				Text:       ch.Text,
				SourceURI:  ch.SourceURI,
				Score:      score,
				DocumentID: ch.DocumentID,
			},
			score: score,
		})
	}
	if len(hits) == 0 {
		return port.KnowledgeResult{Hit: false}, nil
	}
	// simple sort by score desc
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].score > hits[i].score {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}
	if len(hits) > topK {
		hits = hits[:topK]
	}
	out := make([]port.KnowledgeSnippet, len(hits))
	for i, h := range hits {
		out[i] = h.snip
	}
	return port.KnowledgeResult{Hit: true, Snippets: out}, nil
}

func (g *Gateway) load(ctx context.Context, q port.KnowledgeQuery) ([]store.KBChunk, error) {
	_ = ctx
	g.mu.RLock()
	local := append([]store.KBChunk(nil), g.local...)
	g.mu.RUnlock()
	// Durable kb_* store retired (M-E / OD-08-4). Lab/tests use IndexLocal only until binding Inform lands.
	return filterCollections(local, q.Collections), nil
}

func filterCollections(chunks []store.KBChunk, collections []string) []store.KBChunk {
	if len(collections) == 0 {
		return chunks
	}
	allow := map[string]struct{}{}
	for _, c := range collections {
		allow[c] = struct{}{}
	}
	var out []store.KBChunk
	for _, ch := range chunks {
		if _, ok := allow[ch.Collection]; ok {
			out = append(out, ch)
		}
	}
	return out
}

// Register adds ingest-default to the registry.
func Register(reg port.Registry, g *Gateway) error {
	return reg.Register(port.Registration{
		ID:           ID,
		Port:         port.PortKnowledge,
		Capabilities: g.Capabilities(),
		Instance:     g,
		Probe: func(ctx context.Context) port.Health {
			return port.Health{Healthy: true, LastOK: time.Now()}
		},
	})
}

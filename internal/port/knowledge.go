package port

import "context"

type KnowledgeQuery struct {
	SessionID   SessionID
	Query       string
	Collections []string
	TopK        int
}

type KnowledgeSnippet struct {
	Text       string
	SourceURI  string
	Score      float32
	DocumentID string
}

type KnowledgeResult struct {
	Hit      bool
	Snippets []KnowledgeSnippet
}

type Knowledge interface {
	ID() GatewayID
	Capabilities() Capability
	Retrieve(ctx context.Context, q KnowledgeQuery) (KnowledgeResult, error)
}

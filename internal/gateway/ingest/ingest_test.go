package ingest_test

import (
	"context"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/ingest"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestRetrieve_HitMiss(t *testing.T) {
	g := ingest.New(nil)
	g.IndexLocal(store.KBChunk{
		DocumentID: "d1",
		Collection: "faq",
		Text:       "Password reset requires identity verification.",
		SourceURI:  "file://faq.txt",
	})
	ctx := context.Background()
	hit, err := g.Retrieve(ctx, port.KnowledgeQuery{Query: "password reset", Collections: []string{"faq"}, TopK: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !hit.Hit || len(hit.Snippets) == 0 {
		t.Fatalf("expected hit %+v", hit)
	}
	miss, err := g.Retrieve(ctx, port.KnowledgeQuery{Query: "quantum flux capacitor", Collections: []string{"faq"}})
	if err != nil {
		t.Fatal(err)
	}
	if miss.Hit {
		t.Fatal("expected miss")
	}
}

func TestRegister_ProfileAccepts(t *testing.T) {
	reg := router.NewMemRegistry()
	g := ingest.New(nil)
	if err := ingest.Register(reg, g); err != nil {
		t.Fatal(err)
	}
	rec, ok := reg.Get(ingest.ID)
	if !ok || rec.Port != port.PortKnowledge {
		t.Fatal("not registered")
	}
}

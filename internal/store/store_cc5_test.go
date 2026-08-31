package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestMemory_TranscriptAndDisposition(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()
	_ = mem.CreateProfile(ctx, store.Profile{ID: "p"})
	_, _ = mem.PublishVersion(ctx, "p", json.RawMessage(`{"id":"p","modes":{"think":true}}`))
	_ = mem.CreateSession(ctx, store.Session{
		ID: "s1", ProfileID: "p", ProfileVersion: 1, Clock: "live", State: store.StateRunning,
		CanonicalSampleRateHz: 16000,
	})

	u, err := mem.AppendTranscriptTurn(ctx, store.TranscriptTurn{
		SessionID: "s1", Role: store.RoleUser, Text: "hello", TurnID: "turn-a",
	})
	if err != nil || u.Seq != 1 {
		t.Fatalf("user turn %#v err=%v", u, err)
	}
	a, err := mem.AppendTranscriptTurn(ctx, store.TranscriptTurn{
		SessionID: "s1", Role: store.RoleAssistant, Text: "hi", TurnID: "turn-a",
	})
	if err != nil || a.Seq != 2 || a.TurnID != "turn-a" {
		t.Fatalf("assistant turn %#v err=%v", a, err)
	}
	list, err := mem.ListTranscriptTurns(ctx, "s1")
	if err != nil || len(list) != 2 {
		t.Fatalf("list %v len=%d", err, len(list))
	}
	if list[0].Seq != 1 || list[1].Seq != 2 {
		t.Fatalf("order %#v", list)
	}

	d, err := mem.UpsertSessionDisposition(ctx, store.SessionDisposition{
		SessionID: "s1", Suggestion: store.DispositionResolved, TemplateID: "cc-disposition-v1",
		Source: "postcall_worker",
	})
	if err != nil || d.Suggestion != store.DispositionResolved {
		t.Fatalf("upsert %#v err=%v", d, err)
	}
	got, err := mem.GetSessionDisposition(ctx, "s1")
	if err != nil || got.TemplateID != "cc-disposition-v1" {
		t.Fatalf("get %#v err=%v", got, err)
	}
	_, err = mem.GetSessionDisposition(ctx, "missing")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want not found got %v", err)
	}
}

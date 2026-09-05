package observe_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/observe"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestOnTurnCompleted_EmitsUserFinalAndBotUtterance(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()
	_ = mem.CreateProfile(ctx, store.Profile{ID: "p"})
	_, _ = mem.PublishVersion(ctx, "p", json.RawMessage(`{"id":"p","modes":{"think":true}}`))
	_ = mem.CreateSession(ctx, store.Session{
		ID: "s1", ProfileID: "p", ProfileVersion: 1, Clock: "live", State: store.StateRunning,
		CanonicalSampleRateHz: 16000,
	})
	obs := &observe.Observer{Repo: mem, Meta: observe.SessionMeta{SessionID: "s1", ProfileID: "p", ProfileVersion: 1}}
	obs.OnTurnCompleted(ctx, observe.TurnCompleted{UserText: "hello", ResponseText: "hi there", TurnID: "t1"})
	list, err := mem.ListTranscriptTurns(ctx, "s1")
	if err != nil || len(list) != 2 {
		t.Fatalf("list len=%d err=%v", len(list), err)
	}
	if list[0].EventKind != store.EventKindUserFinal || list[0].Actionable == nil || !*list[0].Actionable {
		t.Fatalf("user %#v", list[0])
	}
	if list[1].EventKind != store.EventKindBotUtterance {
		t.Fatalf("bot %#v", list[1])
	}
}

func TestAppendUserFinal_NonActionable(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()
	_ = mem.CreateProfile(ctx, store.Profile{ID: "p"})
	_, _ = mem.PublishVersion(ctx, "p", json.RawMessage(`{"id":"p","modes":{"think":true}}`))
	_ = mem.CreateSession(ctx, store.Session{
		ID: "s1", ProfileID: "p", ProfileVersion: 1, Clock: "live", State: store.StateRunning,
		CanonicalSampleRateHz: 16000,
	})
	obs := &observe.Observer{Repo: mem, Meta: observe.SessionMeta{SessionID: "s1"}}
	obs.AppendUserFinal(ctx, observe.UserFinalSpec{
		Text: "echo", Language: "en-IN", Actionable: false, ActionableReason: store.ActionableReasonEchoSuspect,
	})
	list, err := mem.ListTranscriptTurns(ctx, "s1")
	if err != nil || len(list) != 1 {
		t.Fatalf("list %#v err=%v", list, err)
	}
	if list[0].EventKind != store.EventKindUserFinal || list[0].Actionable == nil || *list[0].Actionable {
		t.Fatalf("row %#v", list[0])
	}
	if list[0].ActionableReason != store.ActionableReasonEchoSuspect || list[0].Text != "echo" {
		t.Fatalf("row %#v", list[0])
	}
}

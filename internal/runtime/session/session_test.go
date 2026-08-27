package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/clock"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
)

func talkDoc() profile.Document {
	var doc profile.Document
	doc.ID = "lab"
	doc.Modes.Listen = true
	doc.Modes.Speak = true
	doc.Modes.Think = true
	doc.Modes.Talk = true
	doc.Audio.CanonicalSampleRateHz = 16000
	doc.Audio.FrameMs = 20
	doc.Routers.Listen.Providers = []string{"fake-listen"}
	doc.Routers.Speak.Providers = []string{"fake-speak"}
	doc.Routers.Think.Providers = []string{"fake-think"}
	return doc
}

func TestActor_LiveAndPlayback_StartStop(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mgr := session.NewManager(reg)
	ctx := context.Background()
	doc := talkDoc()

	live, err := mgr.Start(ctx, session.StartParams{
		SessionID: "s-live", Clock: string(clock.Live), Profile: doc, SampleRate: 16000, FrameMs: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if live.State() != session.StateRunning {
		t.Fatalf("state %s", live.State())
	}
	if live.Clock.Kind() != clock.Live || !live.Clock.VADEnabled() {
		t.Fatal("live clock policy")
	}
	audio := live.Bus.SubscribeAudio(4)
	live.FeedPCM(port.PCMFrame{Data: make([]byte, 640), SampleRate: 16000, Seq: 1})
	select {
	case <-audio:
	case <-time.After(time.Second):
		t.Fatal("no audio")
	}
	term, err := mgr.Stop(ctx, "s-live", "operator")
	if err != nil {
		t.Fatal(err)
	}
	if term != "Cancelled" {
		t.Fatalf("term %s", term)
	}
	if live.State() != session.StateTerminal {
		t.Fatalf("want Terminal got %s", live.State())
	}

	pb, err := mgr.Start(ctx, session.StartParams{
		SessionID: "s-pb", Clock: string(clock.Playback), Profile: doc, SampleRate: 16000, FrameMs: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pb.Clock.Kind() != clock.Playback || pb.Clock.VADEnabled() {
		t.Fatal("playback clock policy")
	}
	blob := make([]byte, 640*3)
	if err := pb.FeedPlaybackBlob(ctx, blob); err != nil {
		t.Fatal(err)
	}
	term, err = mgr.Stop(ctx, "s-pb", "done")
	if err != nil {
		t.Fatal(err)
	}
	if term != "Completed" {
		t.Fatalf("term %s", term)
	}
}

func TestActor_Live_RefusesBatchOnlyListen(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	batch := &fake.Listen{BatchOnly: true}
	if err := reg.Register(port.Registration{
		ID: fake.IDListen, Port: port.PortListen,
		Capabilities: batch.Capabilities(), Instance: batch,
	}); err != nil {
		t.Fatal(err)
	}
	mgr := session.NewManager(reg)
	_, err := mgr.Start(context.Background(), session.StartParams{
		SessionID: "bad", Clock: "live", Profile: talkDoc(),
	})
	if err == nil {
		t.Fatal("expected capability gate error")
	}
}

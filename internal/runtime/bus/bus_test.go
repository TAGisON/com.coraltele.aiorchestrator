package bus_test

import (
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/bus"
)

func TestBus_FanOutAudio(t *testing.T) {
	b := bus.New()
	a1 := b.SubscribeAudio(4)
	a2 := b.SubscribeAudio(4)
	frame := port.PCMFrame{Data: []byte{1, 2}, SampleRate: 16000, Seq: 1, At: time.Now()}
	b.PublishAudio(frame)
	got1 := <-a1
	got2 := <-a2
	if got1.Seq != 1 || got2.Seq != 1 {
		t.Fatalf("seq mismatch")
	}
	b.Close()
	_, ok := <-a1
	if ok {
		t.Fatal("expected closed")
	}
}

func TestBus_TextAndEvents(t *testing.T) {
	b := bus.New()
	texts := b.SubscribeText(2)
	evs := b.SubscribeEvents(2)
	b.PublishText(bus.TextEvent{Role: "user", Text: "hi"})
	b.PublishEvent(bus.Event{Kind: "stop"})
	te := <-texts
	if te.Text != "hi" {
		t.Fatalf("text %q", te.Text)
	}
	ev := <-evs
	if ev.Kind != "stop" {
		t.Fatalf("kind %q", ev.Kind)
	}
	b.Close()
}

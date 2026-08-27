// Package bus provides in-process session taps (audio, text, events).
// No Kafka/Redis — channels and fan-out only.
package bus

import (
	"sync"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

// TextEvent is a text-tap message.
type TextEvent struct {
	Role string // user | assistant | system | inject
	Text string
	At   time.Time
}

// Event is a control/lifecycle tap message.
type Event struct {
	Kind string
	Data any
	At   time.Time
}

// Bus is an in-memory session media/control bus with fan-out taps.
type Bus struct {
	mu sync.RWMutex

	audioSubs []chan port.PCMFrame
	textSubs  []chan TextEvent
	eventSubs []chan Event

	closed bool
}

// New creates an empty bus.
func New() *Bus {
	return &Bus{}
}

// SubscribeAudio returns a buffered tap for PCM frames. Caller must not close the channel.
func (b *Bus) SubscribeAudio(buf int) <-chan port.PCMFrame {
	if buf < 1 {
		buf = 16
	}
	ch := make(chan port.PCMFrame, buf)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(ch)
		return ch
	}
	b.audioSubs = append(b.audioSubs, ch)
	return ch
}

// SubscribeText returns a buffered tap for text events.
func (b *Bus) SubscribeText(buf int) <-chan TextEvent {
	if buf < 1 {
		buf = 16
	}
	ch := make(chan TextEvent, buf)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(ch)
		return ch
	}
	b.textSubs = append(b.textSubs, ch)
	return ch
}

// SubscribeEvents returns a buffered tap for session events.
func (b *Bus) SubscribeEvents(buf int) <-chan Event {
	if buf < 1 {
		buf = 16
	}
	ch := make(chan Event, buf)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(ch)
		return ch
	}
	b.eventSubs = append(b.eventSubs, ch)
	return ch
}

// PublishAudio fans out a PCM frame to all audio subscribers (non-blocking drop if full).
func (b *Bus) PublishAudio(frame port.PCMFrame) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for _, ch := range b.audioSubs {
		select {
		case ch <- frame:
		default:
		}
	}
}

// PublishText fans out a text event.
func (b *Bus) PublishText(ev TextEvent) {
	if ev.At.IsZero() {
		ev.At = time.Now()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for _, ch := range b.textSubs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// PublishEvent fans out a session event.
func (b *Bus) PublishEvent(ev Event) {
	if ev.At.IsZero() {
		ev.At = time.Now()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for _, ch := range b.eventSubs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Close closes all subscriber channels. Safe to call once.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, ch := range b.audioSubs {
		close(ch)
	}
	for _, ch := range b.textSubs {
		close(ch)
	}
	for _, ch := range b.eventSubs {
		close(ch)
	}
	b.audioSubs = nil
	b.textSubs = nil
	b.eventSubs = nil
}

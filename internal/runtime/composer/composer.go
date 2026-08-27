// Package composer implements the Talk turn machine and local VAD barge-in.
// Imports port + router only — never gateway/*.
package composer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/bus"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/thinkpath"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/vad"
)

// TurnState is the Talk composer state (RUNTIME.md §6).
type TurnState string

const (
	Listening TurnState = "Listening"
	Capturing TurnState = "Capturing"
	Thinking  TurnState = "Thinking"
	Speaking  TurnState = "Speaking"
)

// SinkBuffer holds unplayed Speak PCM; Flush drops it (barge-in).
type SinkBuffer struct {
	mu    sync.Mutex
	frames []port.PCMFrame
	flushed int
}

func (s *SinkBuffer) Push(f port.PCMFrame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frames = append(s.frames, f)
}

func (s *SinkBuffer) Flush() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.frames)
	s.frames = nil
	s.flushed += n
	return n
}

func (s *SinkBuffer) Flushed() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushed
}

func (s *SinkBuffer) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.frames)
}

// Talk is the Talk composer for one session.
type Talk struct {
	Doc      profile.Document
	Reg      port.Registry
	Bus      *bus.Bus
	Mem      *session.Memory
	VAD      vad.Detector
	Clock    string // live | playback
	Session  port.SessionID
	TenantID string
	Rate     port.SampleRateHz

	Sink *SinkBuffer
	Path *thinkpath.Path

	mu           sync.Mutex
	state        TurnState
	speakStream  port.SpeakStream
	speakCancel  context.CancelFunc
	thinkCancel  context.CancelFunc
	lastBargeIn  bool
	cancelCount  int
}

// NewTalk builds a composer. VAD may be nil (defaults to energy VAD when clock enables it).
func NewTalk(doc profile.Document, reg port.Registry, b *bus.Bus, mem *session.Memory, clockKind string, sid port.SessionID) (*Talk, error) {
	deps, err := thinkpath.Resolve(reg, doc, clockKind)
	if err != nil {
		return nil, err
	}
	t := &Talk{
		Doc:     doc,
		Reg:     reg,
		Bus:     b,
		Mem:     mem,
		VAD:     vad.NewEnergy(),
		Clock:   clockKind,
		Session: sid,
		Rate:    port.SampleRateHz(profile.SampleRateHz(doc)),
		Sink:    &SinkBuffer{},
		Path: &thinkpath.Path{
			Doc:     doc,
			Mem:     mem,
			Deps:    deps,
			Reg:     reg,
			Session: sid,
		},
		state: Listening,
	}
	return t, nil
}

// State returns the current turn state.
func (t *Talk) State() TurnState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

func (t *Talk) setState(s TurnState) {
	t.mu.Lock()
	t.state = s
	t.mu.Unlock()
	if t.Bus != nil {
		t.Bus.PublishEvent(bus.Event{Kind: "turn", Data: string(s)})
	}
}

// LastBargeIn reports whether the last Speaking period ended via barge-in.
func (t *Talk) LastBargeIn() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastBargeIn
}

// CancelCount is how many times Speak.Cancel was invoked (tests).
func (t *Talk) CancelCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cancelCount
}

// OnPCM feeds one bus PCM frame into VAD / barge-in logic.
func (t *Talk) OnPCM(frame port.PCMFrame) {
	if t.VAD == nil {
		return
	}
	// Playback: VAD off unless detector still used for sim tests — gate on clock.
	if t.Clock == "playback" {
		return
	}
	dec := t.VAD.Process(frame)
	st := t.State()
	switch st {
	case Listening:
		if dec == vad.Speech {
			t.setState(Capturing)
		}
	case Capturing:
		// silence endpoint handled by EndCapture / InjectFinal
	case Speaking:
		if dec == vad.Speech {
			t.bargeIn()
		}
	}
}

// EndCapture marks utterance end (silence / Listen final) and runs Think → Speak.
func (t *Talk) EndCapture(ctx context.Context, userText string) error {
	if t.State() != Capturing && t.State() != Listening {
		// allow from Listening for inject-text lab path
		if t.State() != Thinking {
			t.setState(Capturing)
		}
	}
	return t.runThinkSpeak(ctx, userText)
}

// InjectFinal is a lab helper: treat text as STT final and run a turn from Listening.
func (t *Talk) InjectFinal(ctx context.Context, userText string) error {
	t.setState(Capturing)
	return t.runThinkSpeak(ctx, userText)
}

func (t *Talk) runThinkSpeak(ctx context.Context, userText string) error {
	t.setState(Thinking)
	thinkCtx, cancel := context.WithCancel(ctx)
	t.mu.Lock()
	t.thinkCancel = cancel
	t.lastBargeIn = false
	t.mu.Unlock()
	defer cancel()

	res, err := t.Path.Run(thinkCtx, userText)
	if err != nil {
		t.setState(Listening)
		return err
	}
	if res.BlockedThink || res.Action == "refuse" || res.Action == "escalate" || res.Action == "block_think" {
		// still may speak refuse/escalate message
		if res.ResponseText == "" {
			t.setState(Listening)
			return nil
		}
	}
	if res.ResponseText == "" {
		t.setState(Listening)
		return nil
	}
	return t.speak(ctx, res.ResponseText)
}

func (t *Talk) speak(ctx context.Context, text string) error {
	speakGW, err := t.selectSpeak()
	if err != nil {
		t.setState(Listening)
		return err
	}
	speakCtx, cancel := context.WithCancel(ctx)
	stream, err := speakGW.Speak(speakCtx, port.SpeakRequest{
		SessionID:  t.Session,
		Text:       text,
		SampleRate: t.Rate,
	})
	if err != nil {
		cancel()
		t.setState(Listening)
		return err
	}
	t.mu.Lock()
	t.speakStream = stream
	t.speakCancel = cancel
	t.mu.Unlock()
	t.setState(Speaking)

	for {
		select {
		case <-speakCtx.Done():
			t.finishSpeakToListening()
			return nil
		case frame, ok := <-stream.Frames():
			if !ok {
				<-stream.Done()
				t.finishSpeakToListening()
				return nil
			}
			if t.State() != Speaking {
				return nil
			}
			t.Sink.Push(frame)
			if t.Bus != nil {
				t.Bus.PublishAudio(frame)
			}
		case <-stream.Done():
			t.finishSpeakToListening()
			return nil
		}
	}
}

func (t *Talk) finishSpeakToListening() {
	t.mu.Lock()
	if t.state == Speaking {
		t.state = Listening
	}
	t.speakStream = nil
	if t.speakCancel != nil {
		t.speakCancel()
		t.speakCancel = nil
	}
	t.mu.Unlock()
	if t.Bus != nil {
		t.Bus.PublishEvent(bus.Event{Kind: "turn", Data: string(Listening)})
	}
}

func (t *Talk) bargeIn() {
	t.mu.Lock()
	if t.state != Speaking {
		t.mu.Unlock()
		return
	}
	t.lastBargeIn = true
	stream := t.speakStream
	thinkCancel := t.thinkCancel
	t.state = Capturing
	t.mu.Unlock()

	t.Sink.Flush()
	if stream != nil {
		_ = stream.Cancel(context.Background())
		t.mu.Lock()
		t.cancelCount++
		t.mu.Unlock()
	}
	if thinkCancel != nil {
		thinkCancel()
	}
	t.mu.Lock()
	if t.speakCancel != nil {
		t.speakCancel()
		t.speakCancel = nil
	}
	t.speakStream = nil
	t.mu.Unlock()
	if t.Bus != nil {
		t.Bus.PublishEvent(bus.Event{Kind: "barge_in"})
		t.Bus.PublishEvent(bus.Event{Kind: "turn", Data: string(Capturing)})
	}
}

func (t *Talk) selectSpeak() (port.Speak, error) {
	ids := make([]port.GatewayID, 0, len(t.Doc.Routers.Speak.Providers))
	for _, p := range t.Doc.Routers.Speak.Providers {
		ids = append(ids, port.GatewayID(p))
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no speak providers")
	}
	rec, err := router.Select(t.Reg, ids, port.PortSpeak, router.SelectOptions{Clock: t.Clock})
	if err != nil {
		return nil, err
	}
	sp, ok := rec.Instance.(port.Speak)
	if !ok {
		return nil, fmt.Errorf("speak instance type assert failed")
	}
	return sp, nil
}

// SpeakAndWaitForBarge starts Speaking and returns once barge-in occurs or speak completes.
// Used by tests to inject speech frames while Speaking.
func (t *Talk) SpeakText(ctx context.Context, text string) error {
	return t.speak(ctx, text)
}

// WatchPCMForBarge reads audio tap frames and applies OnPCM until ctx done.
func (t *Talk) WatchPCMForBarge(ctx context.Context) {
	if t.Bus == nil {
		return
	}
	ch := t.Bus.SubscribeAudio(32)
	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-ch:
			if !ok {
				return
			}
			t.OnPCM(frame)
		}
	}
}

// IdleBrief yields so speak goroutines can progress in tests.
func IdleBrief() { time.Sleep(5 * time.Millisecond) }

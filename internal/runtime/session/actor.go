// Package session owns one in-process actor per session: lifecycle, bus, memory, attachments.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/bus"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/clock"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// Lifecycle states (RUNTIME.md §5). Terminal covers Completed|Cancelled|Failed.
const (
	StateCreated  = "Created"
	StateRunning  = "Running"
	StateDraining = "Draining"
	StateTerminal = "Terminal"
)

// Attachment is one feeder or sink bound to a session (N attachments model).
type Attachment struct {
	ID   string
	Kind string // feeder | sink | text
	Role string
}

// Memory holds transcript turns and structured slots for the Think path.
type Memory struct {
	mu       sync.RWMutex
	Messages []port.ChatMessage
	Slots    map[string]string
	Intent   string
	State    string // playbook state
}

func NewMemory() *Memory {
	return &Memory{Slots: make(map[string]string)}
}

func (m *Memory) Append(role, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = append(m.Messages, port.ChatMessage{Role: role, Content: content})
}

func (m *Memory) Snapshot() []port.ChatMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]port.ChatMessage, len(m.Messages))
	copy(out, m.Messages)
	return out
}

func (m *Memory) SetSlot(k, v string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Slots == nil {
		m.Slots = make(map[string]string)
	}
	m.Slots[k] = v
}

func (m *Memory) GetSlots() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]string, len(m.Slots))
	for k, v := range m.Slots {
		out[k] = v
	}
	return out
}

// Actor is one session runtime unit.
type Actor struct {
	ID             string
	TenantID       string
	ClockKind      string
	SampleRate     port.SampleRateHz
	FrameMs        int
	Profile        profile.Document
	Reg            port.Registry
	GatewayBinding *store.GatewayBinding // pinned at create (CC); metadata for later phases

	Bus         *bus.Bus
	Clock       clock.Scheduler
	Memory      *Memory
	Attachments []Attachment

	mu                  sync.Mutex
	state               string
	terminal            string // Completed | Cancelled | Failed when Terminal
	cancel              context.CancelFunc
	done                chan struct{}
	drainOnce           sync.Once
	detectedLanguage    string
	activeLanguage      string
	languageLocked      bool
	flushListenPartials bool
	// LanguagePersist is optional durability hook (detected, active) after lock/switch.
	LanguagePersist func(detected, active string)

	feeders []port.Feeder
	sinks   []port.Sink
}

// State returns the current lifecycle state.
func (a *Actor) State() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

// TerminalReason returns Completed|Cancelled|Failed after Terminal, else "".
func (a *Actor) TerminalReason() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.terminal
}

func (a *Actor) setState(s string) {
	a.mu.Lock()
	a.state = s
	a.mu.Unlock()
	a.Bus.PublishEvent(bus.Event{Kind: "state", Data: s})
}

// Start transitions Created → Running and begins the actor loop (clock heartbeat + drain watch).
func (a *Actor) Start(parent context.Context) error {
	a.mu.Lock()
	if a.state != StateCreated {
		a.mu.Unlock()
		return fmt.Errorf("actor %s not Created (state=%s)", a.ID, a.state)
	}
	ctx, cancel := context.WithCancel(parent)
	a.cancel = cancel
	a.done = make(chan struct{})
	a.state = StateRunning
	a.mu.Unlock()
	a.Bus.PublishEvent(bus.Event{Kind: "state", Data: StateRunning})

	go a.loop(ctx)
	return nil
}

func (a *Actor) loop(ctx context.Context) {
	defer close(a.done)
	defer a.Bus.Close()
	<-ctx.Done()
	// Terminal reason set by Drain; default Cancelled if context cancelled elsewhere.
	a.finishTerminal("Cancelled")
}

// Drain requests stop: Running → Draining → Terminal.
func (a *Actor) Drain(reason string) {
	a.drainOnce.Do(func() {
		term := "Completed"
		if reason == "operator" || reason == "cancel" {
			term = "Cancelled"
		}
		a.mu.Lock()
		a.state = StateDraining
		a.terminal = term
		feeders := append([]port.Feeder(nil), a.feeders...)
		sinks := append([]port.Sink(nil), a.sinks...)
		a.mu.Unlock()
		a.Bus.PublishEvent(bus.Event{Kind: "drain", Data: reason})
		a.Bus.PublishEvent(bus.Event{Kind: "state", Data: StateDraining})
		ctx := context.Background()
		for _, f := range feeders {
			_ = f.Close(ctx)
		}
		for _, s := range sinks {
			_ = s.Close(ctx)
		}
		if a.cancel != nil {
			a.cancel()
		}
		select {
		case <-a.done:
		case <-time.After(2 * time.Second):
		}
		a.mu.Lock()
		a.state = StateTerminal
		a.mu.Unlock()
	})
}

func (a *Actor) finishTerminal(reason string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state == StateTerminal {
		return
	}
	if a.terminal == "" {
		a.terminal = reason
	}
	a.state = StateTerminal
}

// Wait blocks until the actor loop exits (after Drain/cancel).
func (a *Actor) Wait(ctx context.Context) error {
	select {
	case <-a.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// FeedPCM publishes one PCM frame onto the audio tap (lab / synthetic feeder).
func (a *Actor) FeedPCM(frame port.PCMFrame) {
	if frame.SampleRate == 0 {
		frame.SampleRate = a.SampleRate
	}
	a.Bus.PublishAudio(frame)
}

// FeedPlaybackBlob feeds a finite PCM blob at playback pace (may be faster than realtime).
func (a *Actor) FeedPlaybackBlob(ctx context.Context, pcm []byte) error {
	n := clock.FrameBytes(int(a.SampleRate), a.FrameMs)
	var seq uint64
	for off := 0; off < len(pcm); off += n {
		end := off + n
		if end > len(pcm) {
			end = len(pcm)
		}
		chunk := make([]byte, n)
		copy(chunk, pcm[off:end])
		seq++
		a.FeedPCM(port.PCMFrame{Data: chunk, SampleRate: a.SampleRate, Seq: seq, At: time.Now()})
		if err := a.Clock.Pace(ctx); err != nil {
			return err
		}
	}
	a.Bus.PublishEvent(bus.Event{Kind: "playback_exhausted"})
	return nil
}

// AttachFeeder binds a feeder and pumps canonical frames onto the session bus until feeder stops or actor drains.
func (a *Actor) AttachFeeder(ctx context.Context, f port.Feeder, attachmentID string) {
	a.mu.Lock()
	a.feeders = append(a.feeders, f)
	a.Attachments = append(a.Attachments, Attachment{ID: attachmentID, Kind: "feeder", Role: string(f.ID())})
	a.mu.Unlock()
	go a.pumpFeeder(ctx, f)
}

// AttachSink registers a sink for outbound PCM (Speak → edge).
func (a *Actor) AttachSink(s port.Sink, attachmentID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sinks = append(a.sinks, s)
	a.Attachments = append(a.Attachments, Attachment{ID: attachmentID, Kind: "sink", Role: string(s.ID())})
}

// Sinks returns attached sinks (copy of slice header).
func (a *Actor) Sinks() []port.Sink {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]port.Sink, len(a.sinks))
	copy(out, a.sinks)
	return out
}

func (a *Actor) pumpFeeder(ctx context.Context, f port.Feeder) {
	frames := f.Frames()
	events := f.Events()
	for {
		select {
		case <-ctx.Done():
			_ = f.Close(context.Background())
			return
		case fr, ok := <-frames:
			if !ok {
				a.Bus.PublishEvent(bus.Event{Kind: "feeder_gone", Data: string(f.ID())})
				return
			}
			a.FeedPCM(fr)
		case ev, ok := <-events:
			if !ok {
				continue
			}
			a.Bus.PublishEvent(bus.Event{Kind: "feeder_" + ev.Kind, Data: ev.Data})
			if ev.Kind == "stop" || ev.Kind == "error" {
				// File (and FS) feeders may enqueue stop/error while PCM is still
				// buffered on Frames(); a fair select must not drop those frames.
				a.drainFeederFrames(frames)
				return
			}
		}
	}
}

// drainFeederFrames publishes any PCM already queued on frames (non-blocking).
func (a *Actor) drainFeederFrames(frames <-chan port.PCMFrame) {
	for {
		select {
		case fr, ok := <-frames:
			if !ok {
				return
			}
			a.FeedPCM(fr)
		default:
			return
		}
	}
}

// StartParams configures a new actor.
type StartParams struct {
	SessionID      string
	TenantID       string
	Clock          string
	SampleRate     int
	FrameMs        int
	Profile        profile.Document
	ProfileRaw     json.RawMessage // optional; if Profile empty, parsed from raw
	Reg            port.Registry
	GatewayBinding *store.GatewayBinding
}

// Manager owns actors keyed by session id.
type Manager struct {
	mu     sync.Mutex
	actors map[string]*Actor
	reg    port.Registry
}

// NewManager creates a runtime manager. reg may be nil if each Start supplies Reg.
func NewManager(reg port.Registry) *Manager {
	return &Manager{actors: make(map[string]*Actor), reg: reg}
}

// Start creates and starts an actor after capability gate.
func (m *Manager) Start(ctx context.Context, p StartParams) (*Actor, error) {
	doc := p.Profile
	if doc.ID == "" && len(p.ProfileRaw) > 0 {
		var err error
		doc, err = profile.Parse(p.ProfileRaw)
		if err != nil {
			return nil, err
		}
	}
	reg := p.Reg
	if reg == nil {
		reg = m.reg
	}
	if reg == nil {
		return nil, fmt.Errorf("registry required")
	}
	clockKind := p.Clock
	if clockKind == "" {
		clockKind = string(clock.Live)
	}
	if err := gateCapabilities(reg, doc, clockKind); err != nil {
		return nil, err
	}
	rate := p.SampleRate
	if rate == 0 {
		rate = profile.SampleRateHz(doc)
	}
	frameMs := p.FrameMs
	if frameMs == 0 {
		frameMs = doc.Audio.FrameMs
	}
	if frameMs == 0 {
		frameMs = 20
	}
	a := &Actor{
		ID:             p.SessionID,
		TenantID:       p.TenantID,
		ClockKind:      clockKind,
		SampleRate:     port.SampleRateHz(rate),
		FrameMs:        frameMs,
		Profile:        doc,
		Reg:            reg,
		GatewayBinding: p.GatewayBinding,
		Bus:            bus.New(),
		Clock:          clock.New(clockKind, frameMs),
		Memory:         NewMemory(),
		Attachments:    nil,
		state:          StateCreated,
	}
	m.mu.Lock()
	if _, exists := m.actors[p.SessionID]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("session actor already exists: %s", p.SessionID)
	}
	m.actors[p.SessionID] = a
	m.mu.Unlock()

	if err := a.Start(ctx); err != nil {
		m.mu.Lock()
		delete(m.actors, p.SessionID)
		m.mu.Unlock()
		return nil, err
	}
	return a, nil
}

// Get returns an actor by id.
func (m *Manager) Get(id string) (*Actor, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.actors[id]
	return a, ok
}

// Stop drains the actor and removes it. Returns terminal reason (Completed|Cancelled|Failed).
func (m *Manager) Stop(ctx context.Context, id, reason string) (string, error) {
	m.mu.Lock()
	a, ok := m.actors[id]
	if ok {
		delete(m.actors, id)
	}
	m.mu.Unlock()
	if !ok {
		return "", nil // no actor — control may still persist Terminal
	}
	a.Drain(reason)
	_ = a.Wait(ctx)
	term := a.TerminalReason()
	if term == "" {
		term = "Completed"
	}
	return term, nil
}

func gateCapabilities(reg port.Registry, doc profile.Document, clockKind string) error {
	opt := router.SelectOptions{Clock: clockKind}
	needListen := doc.Modes.Listen || doc.Modes.Talk
	needSpeak := doc.Modes.Speak || doc.Modes.Talk
	if needListen && len(doc.Routers.Listen.Providers) > 0 {
		ids := toIDs(doc.Routers.Listen.Providers)
		if _, err := router.Select(reg, ids, port.PortListen, opt); err != nil {
			return err
		}
	}
	if needSpeak && len(doc.Routers.Speak.Providers) > 0 {
		ids := toIDs(doc.Routers.Speak.Providers)
		if _, err := router.Select(reg, ids, port.PortSpeak, opt); err != nil {
			return err
		}
	}
	return nil
}

func toIDs(ss []string) []port.GatewayID {
	out := make([]port.GatewayID, len(ss))
	for i, s := range ss {
		out[i] = port.GatewayID(s)
	}
	return out
}

package fake

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

const (
	IDListen    port.GatewayID = "fake-listen"
	IDSpeak     port.GatewayID = "fake-speak"
	IDThink     port.GatewayID = "fake-think"
	IDTranslate port.GatewayID = "fake-translate"
	IDKnowledge port.GatewayID = "fake-knowledge"
	IDSkill     port.GatewayID = "fake-skill"
)

// --- Listen ---

type Listen struct {
	FinalText string
	BatchOnly bool // if true, Streaming=false for router gate tests
}

func (l *Listen) ID() port.GatewayID { return IDListen }

func (l *Listen) Capabilities() port.Capability {
	if l.BatchOnly {
		return port.Capability{Batch: true, Partials: false, Cancel: true}
	}
	return port.Capability{Streaming: true, Batch: true, Partials: true, Cancel: true}
}

func (l *Listen) OpenStream(ctx context.Context, req port.ListenRequest) (port.ListenStream, error) {
	if l.BatchOnly {
		return nil, &port.GatewayError{Code: port.CodeUnsupported, Message: "streaming not supported"}
	}
	text := l.FinalText
	if text == "" {
		text = "hello"
	}
	s := &listenStream{
		partials: make(chan port.ListenPartial, 1),
		finals:   make(chan port.ListenFinal, 1),
		text:     text,
		lang:     req.LanguageHint,
	}
	return s, nil
}

func (l *Listen) RecognizeBatch(ctx context.Context, req port.ListenRequest, pcm []byte) (port.ListenFinal, error) {
	text := l.FinalText
	if text == "" {
		text = "batch-hello"
	}
	return port.ListenFinal{Text: text, Confidence: 1, Language: req.LanguageHint}, nil
}

type listenStream struct {
	partials chan port.ListenPartial
	finals   chan port.ListenFinal
	text     string
	lang     string
	once     sync.Once
	closed   atomic.Bool
}

func (s *listenStream) WritePCM(ctx context.Context, frame port.PCMFrame) error {
	if s.closed.Load() {
		return &port.GatewayError{Code: port.CodeCancelled, Message: "stream closed"}
	}
	s.once.Do(func() {
		select {
		case s.partials <- port.ListenPartial{Text: s.text[:min(1, len(s.text))], Confidence: 0.5, Language: s.lang}:
		default:
		}
		s.finals <- port.ListenFinal{Text: s.text, Confidence: 1, Language: s.lang}
	})
	return nil
}

func (s *listenStream) Partials() <-chan port.ListenPartial { return s.partials }
func (s *listenStream) Finals() <-chan port.ListenFinal     { return s.finals }

func (s *listenStream) Close(ctx context.Context) error {
	if s.closed.Swap(true) {
		return nil
	}
	close(s.partials)
	close(s.finals)
	return nil
}

// --- Speak ---

type Speak struct {
	Delay           time.Duration // used for timeout tests when combined with short ctx
	FrameCount      int           // frames to emit; 0 = 1 (default). >1 for barge-in tests.
	InterFrameDelay time.Duration // pause between frames (barge-in window)
	CancelCalls     atomic.Int64  // tiny test hook
}

func (s *Speak) ID() port.GatewayID { return IDSpeak }

func (s *Speak) Capabilities() port.Capability {
	return port.Capability{Streaming: true, Batch: true, Cancel: true}
}

func (s *Speak) Speak(ctx context.Context, req port.SpeakRequest) (port.SpeakStream, error) {
	nFrames := s.FrameCount
	if nFrames <= 0 {
		nFrames = 1
	}
	st := &speakStream{
		frames:   make(chan port.PCMFrame, 4),
		done:     make(chan struct{}),
		rate:     req.SampleRate,
		delay:    s.Delay,
		nFrames:  nFrames,
		gap:      s.InterFrameDelay,
		onCancel: func() { s.CancelCalls.Add(1) },
	}
	go st.run(ctx, req.Text)
	return st, nil
}

type speakStream struct {
	frames   chan port.PCMFrame
	done     chan struct{}
	rate     port.SampleRateHz
	delay    time.Duration
	nFrames  int
	gap      time.Duration
	cancel   atomic.Bool
	doneOnce sync.Once
	onCancel func()
}

func (s *speakStream) run(ctx context.Context, text string) {
	defer close(s.frames)
	defer s.finish()

	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return
		}
	}
	n := 640
	if s.rate > 0 {
		n = int(s.rate) * 2 / 50
	}
	for i := 0; i < s.nFrames; i++ {
		if s.cancel.Load() {
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		frame := port.PCMFrame{Data: make([]byte, n), SampleRate: s.rate, Seq: uint64(i + 1), At: time.Now()}
		select {
		case s.frames <- frame:
		case <-ctx.Done():
			return
		}
		if s.gap > 0 && i+1 < s.nFrames {
			select {
			case <-time.After(s.gap):
			case <-ctx.Done():
				return
			}
			if s.cancel.Load() {
				return
			}
		}
	}
	_ = text
}

func (s *speakStream) finish() {
	s.doneOnce.Do(func() { close(s.done) })
}

func (s *speakStream) Frames() <-chan port.PCMFrame { return s.frames }
func (s *speakStream) Done() <-chan struct{}         { return s.done }

func (s *speakStream) Cancel(ctx context.Context) error {
	s.cancel.Store(true)
	if s.onCancel != nil {
		s.onCancel()
	}
	s.finish()
	return nil
}

// --- Think ---

type Think struct{}

func (t *Think) ID() port.GatewayID { return IDThink }

func (t *Think) Capabilities() port.Capability {
	return port.Capability{Streaming: true, Batch: true, Cancel: true}
}

func (t *Think) Complete(ctx context.Context, req port.ThinkRequest) (port.ThinkResult, error) {
	if err := ctx.Err(); err != nil {
		return port.ThinkResult{}, &port.GatewayError{Code: port.CodeCancelled, Message: "cancelled", Cause: err}
	}
	out := "ok"
	if len(req.Messages) > 0 {
		out = "echo: " + req.Messages[len(req.Messages)-1].Content
	}
	if len(req.GroundingChunks) > 0 {
		out += " [grounded]"
	}
	return port.ThinkResult{Text: out}, nil
}

func (t *Think) CompleteStream(ctx context.Context, req port.ThinkRequest) (port.ThinkStream, error) {
	res, err := t.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan string, 1)
	ch <- res.Text
	close(ch)
	return &thinkStream{tokens: ch, res: res}, nil
}

type thinkStream struct {
	tokens chan string
	res    port.ThinkResult
}

func (t *thinkStream) Tokens() <-chan string { return t.tokens }
func (t *thinkStream) Result(ctx context.Context) (port.ThinkResult, error) {
	return t.res, nil
}
func (t *thinkStream) Cancel(ctx context.Context) error { return nil }

// --- Translate ---

type Translate struct{}

func (t *Translate) ID() port.GatewayID { return IDTranslate }

func (t *Translate) Capabilities() port.Capability {
	return port.Capability{Batch: true}
}

func (t *Translate) Translate(ctx context.Context, req port.TranslateRequest) (string, error) {
	return "[" + req.Target + "] " + req.Text, nil
}

// --- Knowledge ---

type Knowledge struct {
	Snippets map[string][]port.KnowledgeSnippet // query -> hits
}

func (k *Knowledge) ID() port.GatewayID { return IDKnowledge }

func (k *Knowledge) Capabilities() port.Capability {
	return port.Capability{Batch: true}
}

func (k *Knowledge) Retrieve(ctx context.Context, q port.KnowledgeQuery) (port.KnowledgeResult, error) {
	if k.Snippets != nil {
		if snips, ok := k.Snippets[q.Query]; ok && len(snips) > 0 {
			return port.KnowledgeResult{Hit: true, Snippets: snips}, nil
		}
	}
	return port.KnowledgeResult{Hit: false, Snippets: nil}, nil
}

// --- Skill ---

type Skill struct {
	Calls atomic.Int64
}

func (s *Skill) ID() port.GatewayID { return IDSkill }

func (s *Skill) Capabilities() port.Capability {
	return port.Capability{Batch: true}
}

func (s *Skill) Execute(ctx context.Context, req port.SkillRequest) (port.SkillResult, error) {
	s.Calls.Add(1)
	return port.SkillResult{OK: true, Output: []byte(`{"ok":true,"name":"` + req.Name + `"}`)}, nil
}

// RegisterAll registers default fakes into a registry.
func RegisterAll(reg port.Registry) error {
	items := []port.Registration{
		{ID: IDListen, Port: port.PortListen, Capabilities: (&Listen{}).Capabilities(), Instance: &Listen{}},
		{ID: IDSpeak, Port: port.PortSpeak, Capabilities: (&Speak{}).Capabilities(), Instance: &Speak{}},
		{ID: IDThink, Port: port.PortThink, Capabilities: (&Think{}).Capabilities(), Instance: &Think{}},
		{ID: IDTranslate, Port: port.PortTranslate, Capabilities: (&Translate{}).Capabilities(), Instance: &Translate{}},
		{ID: IDKnowledge, Port: port.PortKnowledge, Capabilities: (&Knowledge{}).Capabilities(), Instance: &Knowledge{}},
		{ID: IDSkill, Port: port.PortSkill, Capabilities: (&Skill{}).Capabilities(), Instance: &Skill{}},
	}
	for _, it := range items {
		it.Probe = func(ctx context.Context) port.Health {
			return port.Health{Healthy: true, LastOK: time.Now()}
		}
		if err := reg.Register(it); err != nil {
			return err
		}
	}
	return nil
}

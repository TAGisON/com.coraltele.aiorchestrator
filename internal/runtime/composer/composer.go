// Package composer implements the Talk turn machine and local VAD barge-in.
// Imports port + router only — never gateway/*.
package composer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/bus"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/observe"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/thinkpath"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/vad"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
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
	// Actor is optional session language surface (cc-2). When set, Listen finals lock language
	// and Speak/Think consume active_language.
	Actor *session.Actor

	Sink *SinkBuffer
	Path *thinkpath.Path
	// Obs is optional best-effort audit/analytics (Phase E). Nil is fine.
	Obs *observe.Observer
	// ProfileVersion pinned at session create (for turn payload / Obs meta).
	ProfileVersion int

	// RecordAgent receives every canonical Speak frame that reaches the edge, so
	// the call recorder captures the agent leg exactly as it was played out.
	// Optional; must not block.
	RecordAgent func(pcm []byte)

	// OnFailure fires when the Think/Speak pipeline cannot serve the turn —
	// engine down, credits exhausted, timeout, or an unclassified error. Control
	// plays the operator's fallback prompt and releases the call. It is invoked
	// at most once per session; a call is only failed out one time.
	OnFailure func(ctx context.Context, err error)

	mu              sync.Mutex
	state           TurnState
	speakStream     port.SpeakStream
	speakCancel     context.CancelFunc
	thinkCancel     context.CancelFunc
	lastBargeIn     bool
	cancelCount     int
	welcomeCompleted bool
	welcoming       bool
	welcomeBargeAllowed bool
	welcomeReadyAt  time.Time
	welcomeFirstPCM bool
	lastActivity    time.Time
	lastSpokenText  string
	failureOnce     sync.Once

	// Local energy-VAD barge while Speaking. Sarvam STT emits no interim
	// transcripts, so a mid-utterance interrupt cannot come from the STT partial
	// path — the caller would talk over the bot and nothing would stop until they
	// paused. Sustained caller speech on the read leg (which carries the caller
	// mic, not the bot's injected TTS) triggers the barge instead.
	bargeEnabled     bool
	bargeMinSpeech   time.Duration
	bargeRunStart    time.Time
}

// LastSpokenText is the most recent agent TTS line (for echo suppression).
func (t *Talk) LastSpokenText() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastSpokenText
}

// ConfigureBarge sets the local energy-VAD barge policy (called by control).
// minSpeech is how long sustained caller speech must last before
// it counts as a barge; <=0 keeps the default.
func (t *Talk) ConfigureBarge(enabled bool, minSpeech time.Duration) {
	t.mu.Lock()
	t.bargeEnabled = enabled
	if minSpeech > 0 {
		t.bargeMinSpeech = minSpeech
	} else if t.bargeMinSpeech == 0 {
		t.bargeMinSpeech = 300 * time.Millisecond
	}
	t.mu.Unlock()
}

// failPipeline routes a pipeline error to OnFailure exactly once. Later failures
// are logged by the caller but must not re-enter the fallback/hangup path: the
// call is already being released.
func (t *Talk) failPipeline(ctx context.Context, err error) {
	if err == nil || t.OnFailure == nil {
		return
	}
	t.failureOnce.Do(func() {
		// Detach from the caller's context: the turn context is usually already
		// cancelled by the time we get here, and the fallback prompt still has
		// to reach the caller.
		go t.OnFailure(context.WithoutCancel(ctx), err)
	})
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

// BindActor wires session language lock + Think/Speak active_language consumption.
func (t *Talk) BindActor(a *session.Actor) {
	t.Actor = a
	if t.Path != nil && a != nil {
		t.Path.ActiveLanguage = a.ActiveLanguage
		t.Path.PinnedEngines = a.GatewayBinding != nil
	}
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

// SetWelcoming marks welcome TTS in flight (suppresses energy barge when welcome_barge_allowed is false).
func (t *Talk) SetWelcoming(on bool) {
	t.mu.Lock()
	t.welcoming = on
	if !on {
		t.welcomeFirstPCM = false
	}
	t.mu.Unlock()
}

// SetWelcomeBargeAllowed configures whether STT barge may flush during welcome.
func (t *Talk) SetWelcomeBargeAllowed(allowed bool) {
	t.mu.Lock()
	t.welcomeBargeAllowed = allowed
	t.mu.Unlock()
}

// SetWelcomeReadyAt anchors cd_welcome_first_audio_ms (Ready → first welcome PCM).
func (t *Talk) SetWelcomeReadyAt(at time.Time) {
	t.mu.Lock()
	t.welcomeReadyAt = at
	t.mu.Unlock()
}

// WelcomeCompleted reports whether the opening speak finished.
func (t *Talk) WelcomeCompleted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.welcomeCompleted
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
		// Sarvam STT never emits interim partials, so STT-only barge (WP1) cannot
		// interrupt mid-utterance. When enabled, sustained caller energy on the
		// uplink (feeder PCM, not TTS) flushes Speak after MinSpeech.
		t.mu.Lock()
		enabled := t.bargeEnabled
		minSpeech := t.bargeMinSpeech
		welcoming := t.welcoming
		welcomeOK := t.welcomeBargeAllowed
		t.mu.Unlock()
		if !enabled {
			return
		}
		if welcoming && !welcomeOK {
			return
		}
		if minSpeech <= 0 {
			minSpeech = 300 * time.Millisecond
		}
		if dec == vad.Speech {
			t.mu.Lock()
			if t.bargeRunStart.IsZero() {
				t.bargeRunStart = time.Now()
			}
			start := t.bargeRunStart
			t.mu.Unlock()
			if time.Since(start) >= minSpeech {
				t.bargeIn()
				t.mu.Lock()
				t.bargeRunStart = time.Time{}
				t.mu.Unlock()
			}
			return
		}
		t.mu.Lock()
		t.bargeRunStart = time.Time{}
		t.mu.Unlock()
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

// OnListenFinal applies language lock (when Actor bound) then runs a Talk turn.
func (t *Talk) OnListenFinal(ctx context.Context, final port.ListenFinal) error {
	if t.Actor != nil {
		t.Actor.OnListenFinal(final)
		if t.Path != nil {
			t.Path.ActiveLanguage = t.Actor.ActiveLanguage
		}
	}
	t.setState(Capturing)
	return t.runThinkSpeak(ctx, final.Text)
}

// InjectFinal is a lab helper: treat text as STT final and run a turn from Listening.
func (t *Talk) InjectFinal(ctx context.Context, userText string) error {
	t.setState(Capturing)
	return t.runThinkSpeak(ctx, userText)
}

// AnswerCall speaks the opening (pre_speak_first inject_text + greeting clip) without Think or a user turn.
// Idempotent: a second call returns ("", nil). Sets welcomeCompleted only after speak mark.
func (t *Talk) AnswerCall(ctx context.Context) (spoken string, err error) {
	t.mu.Lock()
	if t.welcomeCompleted {
		t.mu.Unlock()
		return "", nil
	}
	t.mu.Unlock()

	var parts []string
	for _, rule := range t.Doc.Rules {
		if rule.Phase == "pre_speak_first" && rule.Action == "inject_text" {
			if s := strings.TrimSpace(rule.Text); s != "" {
				parts = append(parts, s)
			}
		}
	}
	if g := openingGreeting(t.Doc); g != "" {
		parts = append(parts, g)
	}
	spoken = strings.TrimSpace(strings.Join(parts, " "))
	if spoken == "" {
		t.mu.Lock()
		t.welcomeCompleted = true
		t.mu.Unlock()
		return "", nil
	}
	if t.Mem != nil {
		t.Mem.Append("assistant", spoken)
	}
	if err := t.speak(ctx, spoken); err != nil {
		// The welcome is the caller's first impression: if TTS cannot serve it,
		// the call is not viable and must be failed out rather than left silent.
		if !errors.Is(err, context.Canceled) {
			t.failPipeline(ctx, err)
		}
		return spoken, fmt.Errorf("answer speak: %w", err)
	}
	t.mu.Lock()
	t.welcomeCompleted = true
	t.mu.Unlock()
	if t.Obs != nil {
		t.Obs.AppendAssistantOnly(ctx, spoken)
	}
	if t.Bus != nil {
		t.Bus.PublishEvent(bus.Event{Kind: "turn.completed", Data: map[string]any{
			"outcome":       "answer",
			"response_tier": "clip",
		}})
	}
	return spoken, nil
}

func openingGreeting(doc profile.Document) string {
	if doc.Response == nil || doc.Response.Clips == nil {
		return ""
	}
	for _, id := range []string{"greeting-en", "welcome", "greeting"} {
		if u, ok := doc.Response.Clips[id]; ok {
			if s := strings.TrimSpace(u.Text); s != "" {
				return s
			}
		}
	}
	for id, u := range doc.Response.Clips {
		low := strings.ToLower(id)
		if strings.HasPrefix(low, "greeting") || strings.HasPrefix(low, "welcome") {
			if s := strings.TrimSpace(u.Text); s != "" {
				return s
			}
		}
	}
	return ""
}

func (t *Talk) runThinkSpeak(ctx context.Context, userText string) error {
	t.setState(Thinking)
	thinkCtx, cancel := context.WithCancel(ctx)
	t.mu.Lock()
	t.thinkCancel = cancel
	t.lastBargeIn = false
	t.lastActivity = time.Now()
	t.mu.Unlock()
	defer cancel()

	if t.Path != nil && t.Actor != nil {
		t.Path.PinnedEngines = t.Actor.GatewayBinding != nil
		t.Path.ActiveLanguage = t.Actor.ActiveLanguage
	}

	started := time.Now()
	res, err := t.Path.Run(thinkCtx, userText)
	if err != nil {
		t.setState(Listening)
		t.emitTurn(ctx, userText, "", res, false, "error", started)
		// A barge-in cancels Think deliberately; that is not a pipeline failure.
		if !errors.Is(err, context.Canceled) {
			t.failPipeline(ctx, err)
		}
		return err
	}
	if res.BlockedThink || res.Action == "refuse" || res.Action == "escalate" || res.Action == "block_think" {
		// still may speak refuse/escalate message
		if res.ResponseText == "" {
			t.setState(Listening)
			t.emitTurn(ctx, userText, "", res, false, res.Action, started)
			return nil
		}
	}
	if res.ResponseText == "" {
		t.setState(Listening)
		t.emitTurn(ctx, userText, "", res, false, res.Action, started)
		return nil
	}
	err = t.speak(ctx, res.ResponseText)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.failPipeline(ctx, err)
	}
	barge := t.LastBargeIn()
	outcome := res.Action
	if outcome == "" {
		outcome = "allow"
	}
	if barge {
		outcome = "barge_in"
	}
	t.mu.Lock()
	t.lastActivity = time.Now()
	t.mu.Unlock()
	t.emitTurn(ctx, userText, res.ResponseText, res, barge, outcome, started)
	return err
}

// LastActivity is when the last turn started or finished (silence watchdog).
func (t *Talk) LastActivity() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastActivity
}

// MarkActivity refreshes the silence watchdog clock.
func (t *Talk) MarkActivity() {
	t.mu.Lock()
	t.lastActivity = time.Now()
	t.mu.Unlock()
}

func (t *Talk) emitTurn(ctx context.Context, userText, response string, res thinkpath.Result, barge bool, outcome string, started time.Time) {
	listenGW, thinkGW := "", ""
	if t.Actor != nil && t.Actor.GatewayBinding != nil {
		if s := strings.TrimSpace(t.Actor.GatewayBinding.Listen); s != "" {
			listenGW = s
		}
		if s := strings.TrimSpace(t.Actor.GatewayBinding.Think); s != "" {
			thinkGW = s
		}
	}
	if listenGW == "" && len(t.Doc.Routers.Listen.Providers) > 0 {
		listenGW = t.Doc.Routers.Listen.Providers[0]
	}
	if thinkGW == "" && len(t.Doc.Routers.Think.Providers) > 0 {
		thinkGW = t.Doc.Routers.Think.Providers[0]
	}
	speakGW := t.speakGatewayID()
	voiceID := profile.ResolveVoiceID(t.Doc, speakGW)
	if t.Obs != nil {
		t.Obs.OnTurnCompleted(ctx, observe.TurnCompleted{
			UserText:      userText,
			ResponseText:  response,
			BargeIn:       barge,
			SkillName:     res.SkillName,
			SkillOK:       res.SkillOK,
			KnowledgeHit:  res.KnowledgeHit,
			GroundingReq:  t.Doc.Grounding.Required,
			ListenGateway: listenGW,
			ThinkGateway:  thinkGW,
			SpeakGateway:  speakGW,
			VoiceID:       voiceID,
			ResponseTier: res.ResponseTier,
			Outcome:       outcome,
			LatencyMs:     time.Since(started).Milliseconds(),
		})
	}
	if t.Bus != nil {
		data := map[string]any{
			"outcome":    outcome,
			"skill_name": res.SkillName,
			"skill_ok":   res.SkillOK,
			"barge_in":   barge,
		}
		if res.ResponseTier != "" {
			data["response_tier"] = res.ResponseTier
		}
		t.Bus.PublishEvent(bus.Event{Kind: "turn.completed", Data: data})
		if res.SkillName != "" {
			t.Bus.PublishEvent(bus.Event{Kind: "skill.completed", Data: map[string]any{
				"name": res.SkillName,
				"ok":   res.SkillOK,
			}})
		}
	}
}

// SpeakLine plays assistant text without Think (post-welcome menu, soft offers).
func (t *Talk) SpeakLine(ctx context.Context, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if t.Mem != nil {
		t.Mem.Append("assistant", text)
	}
	if err := t.speak(ctx, text); err != nil {
		return err
	}
	if t.Obs != nil {
		t.Obs.AppendAssistantOnly(ctx, text)
	}
	t.MarkActivity()
	return nil
}

func (t *Talk) speak(ctx context.Context, text string) error {
	speakGW, speakID, err := t.selectSpeak()
	if err != nil {
		t.setState(Listening)
		return err
	}
	lang := ""
	if t.Actor != nil {
		lang = t.Actor.ActiveLanguage()
	}
	voiceID := profile.ResolveVoiceID(t.Doc, speakID)
	speakCtx, cancel := context.WithCancel(ctx)
	stream, err := speakGW.Speak(speakCtx, port.SpeakRequest{
		SessionID:  t.Session,
		Text:       text,
		SampleRate: t.Rate,
		Language:   lang,
		VoiceID:    voiceID,
	})
	if err != nil {
		cancel()
		t.setState(Listening)
		return err
	}
	t.mu.Lock()
	t.speakStream = stream
	t.speakCancel = cancel
	t.bargeRunStart = time.Time{}
	t.lastSpokenText = strings.TrimSpace(text)
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
				_ = t.waitSinkMarks(ctx)
				t.finishSpeakToListening()
				return nil
			}
			if t.State() != Speaking {
				return nil
			}
			t.Sink.Push(frame)
			t.writeSinks(speakCtx, frame)
			t.noteWelcomeFirstPCM(ctx)
			if t.Bus != nil {
				t.Bus.PublishAudio(frame)
			}
		case <-stream.Done():
			_ = t.waitSinkMarks(ctx)
			t.finishSpeakToListening()
			return nil
		}
	}
}

func (t *Talk) noteWelcomeFirstPCM(ctx context.Context) {
	t.mu.Lock()
	if !t.welcoming || t.welcomeFirstPCM || t.welcomeReadyAt.IsZero() {
		t.mu.Unlock()
		return
	}
	t.welcomeFirstPCM = true
	at := t.welcomeReadyAt
	t.mu.Unlock()
	if t.Obs != nil {
		ms := time.Since(at).Milliseconds()
		t.Obs.Metric(ctx, store.MetricWelcomeFirstAudioMs, float64(ms), nil)
	}
}

// writeSinks fans Speak PCM to attached edge sinks (FreeSWITCH / browser). Bus taps are UI-only.
func (t *Talk) writeSinks(ctx context.Context, frame port.PCMFrame) {
	if t.Actor == nil {
		return
	}
	if t.RecordAgent != nil && len(frame.Data) > 0 {
		t.RecordAgent(frame.Data)
	}
	for _, s := range t.Actor.Sinks() {
		if err := s.WritePCM(ctx, frame); err != nil {
			return
		}
	}
}

func (t *Talk) flushSinks() {
	if t.Actor == nil {
		return
	}
	ctx := context.Background()
	for _, s := range t.Actor.Sinks() {
		_ = s.Flush(ctx)
	}
}

func (t *Talk) waitSinkMarks(ctx context.Context) error {
	if t.Actor == nil {
		return nil
	}
	for _, s := range t.Actor.Sinks() {
		if err := s.WaitMark(ctx); err != nil {
			return err
		}
	}
	return nil
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

// Interrupt stops in-flight Think/Speak (barge-in). Safe from the Listen finals path.
func (t *Talk) Interrupt() {
	t.bargeIn()
}

func (t *Talk) bargeIn() {
	t.mu.Lock()
	st := t.state
	if st != Speaking && st != Thinking {
		t.mu.Unlock()
		return
	}
	t.lastBargeIn = true
	stream := t.speakStream
	thinkCancel := t.thinkCancel
	t.state = Capturing
	t.mu.Unlock()

	t.Sink.Flush()
	t.flushSinks()
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
	if t.Obs != nil {
		t.Obs.OnBargeIn(context.Background())
	}
}

// speakGatewayID is the Speak id used for persona.voice map keys and audit.
// Prefer session gateway_binding.speak when pinned (CC); else first profile speak provider.
func (t *Talk) speakGatewayID() string {
	if t.Actor != nil && t.Actor.GatewayBinding != nil {
		if s := strings.TrimSpace(t.Actor.GatewayBinding.Speak); s != "" {
			return s
		}
	}
	if len(t.Doc.Routers.Speak.Providers) > 0 {
		return t.Doc.Routers.Speak.Providers[0]
	}
	return ""
}

func (t *Talk) selectSpeak() (port.Speak, string, error) {
	var ids []port.GatewayID
	if t.Actor != nil && t.Actor.GatewayBinding != nil {
		if s := strings.TrimSpace(t.Actor.GatewayBinding.Speak); s != "" {
			ids = []port.GatewayID{port.GatewayID(s)}
		}
	}
	if len(ids) == 0 {
		for _, p := range t.Doc.Routers.Speak.Providers {
			ids = append(ids, port.GatewayID(p))
		}
	}
	if len(ids) == 0 {
		return nil, "", fmt.Errorf("no speak providers")
	}
	rec, err := router.Select(t.Reg, ids, port.PortSpeak, router.SelectOptions{Clock: t.Clock})
	if err != nil {
		return nil, "", err
	}
	sp, ok := rec.Instance.(port.Speak)
	if !ok {
		return nil, "", fmt.Errorf("speak instance type assert failed")
	}
	return sp, string(rec.ID), nil
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

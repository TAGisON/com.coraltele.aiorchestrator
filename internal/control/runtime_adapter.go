package control

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/composer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/observe"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// SessionRuntime adapts session.Manager to control.Runtime.
type SessionRuntime struct {
	Mgr  *session.Manager
	Repo store.Repository // optional; persists language on lock/switch + observe

	// OnSessionEnd is called when a desk guided path completes the call.
	// Control sets it to the server stop path so state + postcall run normally.
	OnSessionEnd func(ctx context.Context, sessionID, disposition string)

	mu    sync.Mutex
	talks map[string]*composer.Talk
	lives map[string]*liveTalk
	desks map[string]*deskController
}

type liveTalk struct {
	cancel context.CancelFunc
	stream port.ListenStream
}

func (r *SessionRuntime) StartSession(ctx context.Context, p RuntimeStart) error {
	a, err := r.Mgr.Start(ctx, session.StartParams{
		SessionID:      p.SessionID,
		TenantID:       p.TenantID,
		Clock:          p.Clock,
		SampleRate:     p.SampleRate,
		Profile:        p.Profile,
		ProfileRaw:     p.Document,
		GatewayBinding: p.GatewayBinding,
	})
	if err != nil {
		return err
	}
	if r.Repo != nil && a != nil {
		sid := a.ID
		repo := r.Repo
		a.LanguagePersist = func(detected, active string) {
			_, _ = repo.UpdateSessionLanguages(context.Background(), sid, detected, active)
		}
	}
	return nil
}

func (r *SessionRuntime) StopSession(ctx context.Context, sessionID, reason string) (string, error) {
	r.stopLiveTalk(sessionID)
	r.mu.Lock()
	delete(r.talks, sessionID)
	r.mu.Unlock()
	return r.Mgr.Stop(ctx, sessionID, reason)
}

func (r *SessionRuntime) stopLiveTalk(sessionID string) {
	r.mu.Lock()
	lt := r.lives[sessionID]
	if lt != nil {
		delete(r.lives, sessionID)
	}
	r.mu.Unlock()
	if lt == nil {
		return
	}
	if lt.cancel != nil {
		lt.cancel()
	}
	if lt.stream != nil {
		_ = lt.stream.Close(context.Background())
	}
}

// StartLiveTalk opens Listen on the session bus and drives Talk on finals (edge attach).
func (r *SessionRuntime) StartLiveTalk(ctx context.Context, sessionID string) error {
	if r == nil || r.Mgr == nil {
		return fmt.Errorf("runtime not configured")
	}
	a, ok := r.Mgr.Get(sessionID)
	if !ok {
		return fmt.Errorf("session actor not running")
	}
	r.mu.Lock()
	if r.lives != nil {
		if _, exists := r.lives[sessionID]; exists {
			r.mu.Unlock()
			return nil
		}
	}
	r.mu.Unlock()

	talk, err := r.talkFor(a)
	if err != nil {
		return err
	}
	listenGW, err := selectListen(a)
	if err != nil {
		return err
	}
	lang := ""
	if a != nil {
		lang = a.ActiveLanguage()
	}
	stream, err := listenGW.OpenStream(ctx, port.ListenRequest{
		SessionID:    port.SessionID(sessionID),
		SampleRate:   a.SampleRate,
		LanguageHint: lang,
		Clock:        a.ClockKind,
	})
	if err != nil {
		return err
	}
	liveCtx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	if r.lives == nil {
		r.lives = make(map[string]*liveTalk)
	}
	if _, exists := r.lives[sessionID]; exists {
		r.mu.Unlock()
		cancel()
		_ = stream.Close(context.Background())
		return nil
	}
	r.lives[sessionID] = &liveTalk{cancel: cancel, stream: stream}
	r.mu.Unlock()

	applog.Info("live talk started", "session", sessionID, "listen", string(listenGW.ID()))
	// Direct feeder→Listen tap (bus SubscribeAudio alone dropped/blocked under STT write latency).
	// Also feed OnPCM here — bus PublishAudio includes TTS frames and must not drive barge-in VAD.
	queue := make(chan port.PCMFrame, 128)
	var dropped atomic.Int64
	a.SetPCMTap(func(frame port.PCMFrame) {
		talk.OnPCM(frame)
		select {
		case queue <- frame:
		default:
			dropped.Add(1)
		}
	})
	go func() {
		defer a.SetPCMTap(nil)
		r.drainListenQueue(liveCtx, a, stream, queue, &dropped)
	}()
	go r.consumeListenFinals(liveCtx, talk, stream)
	go r.silenceWatch(liveCtx, a, talk)
	return nil
}

// silenceWatch runs the desk no-response ladder on a live call (§19).
func (r *SessionRuntime) silenceWatch(ctx context.Context, a *session.Actor, talk *composer.Talk) {
	ctrl, ok := r.DeskController(a.ID)
	if !ok {
		return
	}
	cx := ctrl.Engine().Doc().CX
	idle := time.Duration(cx.SilenceNudge1Ms) * time.Millisecond
	if idle <= 0 {
		idle = 6 * time.Second
	}
	talk.MarkActivity()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if talk.State() != composer.Listening {
				talk.MarkActivity()
				continue
			}
			if time.Since(talk.LastActivity()) < idle {
				continue
			}
			out := ctrl.Silence(ctx)
			talk.MarkActivity()
			if text := strings.TrimSpace(out.Text); text != "" {
				if err := talk.SpeakText(ctx, text); err != nil {
					applog.Warn("silence nudge speak", "session", a.ID, "err", err)
				}
				if talk.Obs != nil {
					talk.Obs.AppendAssistantOnly(ctx, text)
				}
			}
			talk.MarkActivity()
			if out.End {
				if r.OnSessionEnd != nil {
					r.OnSessionEnd(context.Background(), a.ID, out.Disposition)
				}
				return
			}
		}
	}
}

func (r *SessionRuntime) drainListenQueue(ctx context.Context, a *session.Actor, stream port.ListenStream, queue <-chan port.PCMFrame, dropped *atomic.Int64) {
	if a == nil || stream == nil || queue == nil {
		return
	}
	var bytesOut int64
	var framesOut int64
	lastLog := time.Now()
	for {
		select {
		case <-ctx.Done():
			applog.Info("live listen pump stop", "session", a.ID, "frames", framesOut, "bytes", bytesOut, "dropped", dropped.Load())
			return
		case frame := <-queue:
			if framesOut == 0 {
				applog.Info("live listen first uplink frame", "session", a.ID, "bytes", len(frame.Data), "rate", frame.SampleRate)
			}
			if err := stream.WritePCM(ctx, frame); err != nil {
				applog.Warn("live listen WritePCM", "session", a.ID, "err", err, "frames", framesOut, "bytes", bytesOut)
				return
			}
			framesOut++
			bytesOut += int64(len(frame.Data))
			if time.Since(lastLog) >= 2*time.Second {
				applog.Info("live listen uplink", "session", a.ID, "frames", framesOut, "bytes", bytesOut, "dropped", dropped.Load())
				lastLog = time.Now()
			}
		}
	}
}

func (r *SessionRuntime) consumeListenFinals(ctx context.Context, talk *composer.Talk, stream port.ListenStream) {
	if talk == nil || stream == nil {
		return
	}
	finals := stream.Finals()
	for {
		select {
		case <-ctx.Done():
			return
		case final, ok := <-finals:
			if !ok {
				applog.Warn("live listen finals closed", "session", string(talk.Session))
				return
			}
			text := strings.TrimSpace(final.Text)
			if text == "" {
				continue
			}
			// Caller speech while bot is Thinking/Speaking → barge-in, then take the final.
			st := talk.State()
			if st == composer.Thinking || st == composer.Speaking {
				applog.Info("live listen final barge", "session", string(talk.Session), "state", string(st), "chars", len(text))
				talk.Interrupt()
			}
			applog.Info("live listen final", "session", string(talk.Session), "lang", final.Language, "chars", len(text))
			if ctrl, ok := r.DeskController(string(talk.Session)); ok && strings.TrimSpace(final.Language) != "" {
				ctrl.Engine().SetLanguage(final.Language)
			}
			// Durable observe must not use liveCtx — stop cancels it and drops transcripts.
			if err := talk.OnListenFinal(context.Background(), final); err != nil {
				applog.Warn("OnListenFinal", "session", string(talk.Session), "err", err)
			}
		}
	}
}

func selectListen(a *session.Actor) (port.Listen, error) {
	if a == nil || a.Reg == nil {
		return nil, fmt.Errorf("actor registry required")
	}
	id := ""
	if a.GatewayBinding != nil {
		id = strings.TrimSpace(a.GatewayBinding.Listen)
	}
	if id == "" && len(a.Profile.Routers.Listen.Providers) > 0 {
		id = strings.TrimSpace(a.Profile.Routers.Listen.Providers[0])
	}
	if id == "" {
		return nil, fmt.Errorf("no listen gateway")
	}
	rec, ok := a.Reg.Get(port.GatewayID(id))
	if !ok {
		return nil, fmt.Errorf("listen gateway %s not registered", id)
	}
	ln, ok := rec.Instance.(port.Listen)
	if !ok {
		return nil, fmt.Errorf("listen gateway %s is not a Listen instance", id)
	}
	return ln, nil
}

func (r *SessionRuntime) SwitchLanguage(sessionID, primary string) error {
	if r == nil || r.Mgr == nil {
		return fmt.Errorf("runtime not configured")
	}
	a, ok := r.Mgr.Get(sessionID)
	if !ok {
		return fmt.Errorf("session actor not running")
	}
	a.SwitchActiveLanguage(primary)
	if ctrl, ok := r.DeskController(sessionID); ok {
		ctrl.Engine().SetLanguage(primary)
	}
	_ = a.ConsumeListenFlush() // mark flush for edge/Listen restart consumers
	return nil
}

func (r *SessionRuntime) Languages(sessionID string) (detected, active string, ok bool) {
	if r == nil || r.Mgr == nil {
		return "", "", false
	}
	a, ok := r.Mgr.Get(sessionID)
	if !ok {
		return "", "", false
	}
	return a.DetectedLanguage(), a.ActiveLanguage(), true
}

// InjectText runs composer.Talk.InjectFinal for lab multi-turn / clip smoke.
func (r *SessionRuntime) InjectText(ctx context.Context, sessionID, text string) error {
	if r == nil || r.Mgr == nil {
		return fmt.Errorf("runtime not configured")
	}
	a, ok := r.Mgr.Get(sessionID)
	if !ok {
		return fmt.Errorf("session actor not running")
	}
	talk, err := r.talkFor(a)
	if err != nil {
		return err
	}
	return talk.InjectFinal(ctx, text)
}

// AnswerCall speaks the profile opening without Think / user text.
func (r *SessionRuntime) AnswerCall(ctx context.Context, sessionID string) (string, error) {
	if r == nil || r.Mgr == nil {
		return "", fmt.Errorf("runtime not configured")
	}
	a, ok := r.Mgr.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("session actor not running")
	}
	talk, err := r.talkFor(a)
	if err != nil {
		return "", err
	}
	return talk.AnswerCall(ctx)
}

func (r *SessionRuntime) talkFor(a *session.Actor) (*composer.Talk, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.talks == nil {
		r.talks = make(map[string]*composer.Talk)
	}
	if t, ok := r.talks[a.ID]; ok {
		return t, nil
	}
	talk, err := composer.NewTalk(a.Profile, a.Reg, a.Bus, a.Memory, a.ClockKind, port.SessionID(a.ID))
	if err != nil {
		return nil, err
	}
	talk.BindActor(a)
	if err := bindThinkFromGateway(talk, a); err != nil {
		return nil, err
	}
	profileVersion := 0
	if r.Repo != nil {
		meta := observe.SessionMeta{
			SessionID: a.ID,
			TenantID:  a.TenantID,
			ProfileID: a.Profile.ID,
			Clock:     a.ClockKind,
		}
		if sess, err := r.Repo.GetSession(context.Background(), a.ID); err == nil {
			meta.ProfileVersion = sess.ProfileVersion
			meta.RecordingRef = sess.RecordingRef
			profileVersion = sess.ProfileVersion
		}
		talk.Obs = &observe.Observer{Repo: r.Repo, Meta: meta}
	}
	if ctrl, ok := newDeskController(a.Profile, a.Reg, r.Repo, a.ID, a.TenantID, profileVersion); ok {
		if a.GatewayBinding != nil {
			ctrl.Engine().SetAttribute("gateway_speak", a.GatewayBinding.Speak)
		}
		if lang := strings.TrimSpace(a.ActiveLanguage()); lang != "" {
			ctrl.Engine().SetLanguage(lang)
		}
		talk.Path.Desk = ctrl
		talk.OnDeskEnd = r.deskEndHandler(a.ID)
		if r.desks == nil {
			r.desks = make(map[string]*deskController)
		}
		r.desks[a.ID] = ctrl
		applog.Info("desk controller bound", "session", a.ID, "desk", ctrl.Engine().Doc().ID)
	}
	r.talks[a.ID] = talk
	return talk, nil
}

// DeskController returns the guided-path controller for a session, if any.
func (r *SessionRuntime) DeskController(sessionID string) (*deskController, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.desks[sessionID]
	return c, ok
}

// deskEndHandler stops the session once the desk speaks its closing line.
func (r *SessionRuntime) deskEndHandler(sessionID string) func(string) {
	return func(disposition string) {
		ctx := context.Background()
		applog.Info("desk ended call", "session", sessionID, "disposition", disposition)
		if r.OnSessionEnd != nil {
			r.OnSessionEnd(ctx, sessionID, disposition)
		}
	}
}

// bindThinkFromGateway fills Think when CC profiles omit routers.think.providers.
func bindThinkFromGateway(talk *composer.Talk, a *session.Actor) error {
	if talk == nil || talk.Path == nil || talk.Path.Deps.Think != nil {
		return nil
	}
	if a == nil || a.GatewayBinding == nil {
		return nil
	}
	id := a.GatewayBinding.Think
	if id == "" || a.Reg == nil {
		return nil
	}
	rec, ok := a.Reg.Get(port.GatewayID(id))
	if !ok {
		return fmt.Errorf("gateway_binding.think %s not registered", id)
	}
	th, ok := rec.Instance.(port.Think)
	if !ok {
		return fmt.Errorf("gateway_binding.think %s is not a Think instance", id)
	}
	talk.Path.Deps.Think = th
	return nil
}

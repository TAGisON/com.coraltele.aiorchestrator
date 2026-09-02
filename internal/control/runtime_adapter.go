package control

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/desk"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/fallback"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/composer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/observe"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/record"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/thinkpath"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// SessionRuntime adapts session.Manager to control.Runtime.
type SessionRuntime struct {
	Mgr  *session.Manager
	Repo store.Repository // optional; persists language on lock/switch + observe

	// OnSessionEnd is called when a desk guided path completes the call.
	// Control sets it to the server stop path so state + postcall run normally.
	OnSessionEnd func(ctx context.Context, sessionID, disposition string)

	// RecordCfg controls per-session call recording. Disabled by default.
	RecordCfg record.Config
	// Fallback holds the operator prompts played when the pipeline fails.
	// Nil means failures release the call without an announcement.
	Fallback *fallback.Store

	mu           sync.Mutex
	talks        map[string]*composer.Talk
	lives        map[string]*liveTalk
	desks        map[string]*deskController
	media        map[string]*sessionMedia
	recorders    map[string]*record.Recorder
	failScenario map[string]string // session → first failure scenario
	// callAction records the first telephony action armed per session:
	// "hangup" or "transfer". Prevents silence/late desk.end from hanging up
	// after a successful transfer, and makes hangup arming idempotent.
	callAction map[string]string
}

type liveTalk struct {
	cancel      context.CancelFunc
	stream      port.ListenStream
	silenceArm  chan struct{}
	silenceOnce sync.Once
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
	ani := callerANIFromJSON(p.Caller)
	if r.Repo != nil && a != nil {
		sid := a.ID
		tenantID := a.TenantID
		repo := r.Repo
		a.LanguagePersist = func(detected, active string) {
			_, _ = repo.UpdateSessionLanguages(context.Background(), sid, detected, active)
			src := prefSourceSTTLock
			if strings.TrimSpace(detected) == "" || detected != active {
				src = prefSourceOperator
			}
			r.saveCallerPreference(tenantID, ani, active, src)
		}
	}
	// Returning caller: pin language before Listen opens so STT is not wild auto-detect.
	if a != nil {
		if pref, ok := r.loadCallerPreference(ctx, p.TenantID, ani); ok {
			a.SwitchActiveLanguage(pref.PreferredLanguage)
			applog.Info("caller preference restored",
				"session", a.ID, "ani", ani, "lang", pref.PreferredLanguage, "source", pref.Source)
		}
	}
	return nil
}

func (r *SessionRuntime) StopSession(ctx context.Context, sessionID, reason string) (string, error) {
	r.stopLiveTalk(sessionID)
	// Finalise the recording before the actor goes away, so trailing audio that
	// is still buffered makes it into the file.
	r.stopRecorder(sessionID, reason)
	r.mu.Lock()
	delete(r.talks, sessionID)
	delete(r.callAction, sessionID)
	r.mu.Unlock()
	return r.Mgr.Stop(ctx, sessionID, reason)
}

func (r *SessionRuntime) stopLiveTalk(sessionID string) {
	r.mu.Lock()
	lt := r.lives[sessionID]
	if lt != nil {
		delete(r.lives, sessionID)
	}
	if m := r.media[sessionID]; m != nil {
		m.markDraining()
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

func (r *SessionRuntime) sessionMedia(sessionID string) *sessionMedia {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.media == nil {
		r.media = make(map[string]*sessionMedia)
	}
	m, ok := r.media[sessionID]
	if !ok {
		m = newSessionMedia()
		r.media[sessionID] = m
	}
	return m
}

func (r *SessionRuntime) hasLiveTalk(sessionID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.lives[sessionID]
	return ok
}

// SessionMedia returns live media phase for GET /v1/sessions/{id}.
func (r *SessionRuntime) SessionMedia(sessionID string) (SessionMediaView, bool) {
	if r == nil || !r.hasLiveTalk(sessionID) {
		return SessionMediaView{}, false
	}
	return r.sessionMedia(sessionID).view(), true
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

	med := r.sessionMedia(sessionID)
	med.onEdgeAttach()
	med.setSettleMs(int(r.rtpSettleFor(sessionID) / time.Millisecond))

	talk, err := r.talkFor(a)
	if err != nil {
		return err
	}
	listenGW, err := selectListen(a)
	if err != nil {
		return err
	}
	// Pin STT to a concrete language from the first frame instead of leaving it
	// on Sarvam auto-detect ("unknown"). Auto-detect re-guesses every utterance
	// and mis-transcribes Hindi/Hinglish as Marathi/Gujarati/Kannada (seen in
	// live sessions), which then breaks language consistency and prompt rendering.
	// The desk default (hi-IN for coral-tfn) is the stable per-call language; a
	// real switch re-pins the stream (see maybeRepinListen).
	lang := r.initialListenLanguage(sessionID, a)
	stream, err := listenGW.OpenStream(ctx, port.ListenRequest{
		SessionID:    port.SessionID(sessionID),
		SampleRate:   a.SampleRate,
		LanguageHint: lang,
		Clock:        a.ClockKind,
	})
	if err != nil {
		return err
	}
	applog.Info("listen language pinned", "session", sessionID, "language", pickNonEmpty(lang, "auto"))
	liveCtx, cancel := context.WithCancel(context.Background())
	silenceArm := make(chan struct{})
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
	r.lives[sessionID] = &liveTalk{cancel: cancel, stream: stream, silenceArm: silenceArm}
	r.mu.Unlock()

	applog.Info("live talk started", "session", sessionID, "listen", string(listenGW.ID()), "media_phase", MediaEstablishing)
	// Direct feeder→Listen tap (bus SubscribeAudio alone dropped/blocked under STT write latency).
	// Also feed OnPCM here — bus PublishAudio includes TTS frames and must not drive barge-in VAD.
	// Recording starts at edge attach: that is the first moment we know the leg
	// is real and can name the file after the telephony call id.
	rec := r.startRecorder(sessionID, a)

	queue := make(chan port.PCMFrame, 128)
	var dropped atomic.Int64
	a.SetPCMTap(func(frame port.PCMFrame) {
		rec.WriteCaller(frame.Data)
		talk.OnPCM(frame)
		select {
		case queue <- frame:
		default:
			dropped.Add(1)
		}
	})
	go func() {
		defer a.SetPCMTap(nil)
		r.drainListenQueue(liveCtx, sessionID, a, stream, queue, &dropped)
	}()
	go r.consumeListenFinals(liveCtx, sessionID, talk, stream)
	go r.consumeListenPartials(liveCtx, sessionID, talk, stream)
	go r.silenceWatch(liveCtx, silenceArm, a, talk)
	go r.rtpSettleWatch(liveCtx, sessionID)
	return nil
}

func (r *SessionRuntime) rtpSettleWatch(ctx context.Context, sessionID string) {
	med := r.sessionMedia(sessionID)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if med.enterReadyFromSettle() {
				applog.Info("media ready (settle)", "session", sessionID)
				return
			}
		}
	}
}

// silenceWatch runs the desk no-response ladder on a live call (§19).
// Armed only after welcome completes (LIVE_TALK §2.3).
func (r *SessionRuntime) silenceWatch(ctx context.Context, arm <-chan struct{}, a *session.Actor, talk *composer.Talk) {
	select {
	case <-ctx.Done():
		return
	case <-arm:
	}
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
				// Transfer already moved the leg — never arm hangup from silence.
				if isTransferDisposition(out.Disposition) || r.sessionCallAction(a.ID) == "transfer" {
					applog.Info("silence end ignored after transfer", "session", a.ID, "disposition", out.Disposition)
					return
				}
				r.releaseAfterDeskDecision(a.ID, out.Disposition)
				return
			}
		}
	}
}

func (r *SessionRuntime) drainListenQueue(ctx context.Context, sessionID string, a *session.Actor, stream port.ListenStream, queue <-chan port.PCMFrame, dropped *atomic.Int64) {
	if a == nil || stream == nil || queue == nil {
		return
	}
	med := r.sessionMedia(sessionID)
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
				if med.noteFirstUplink() {
					applog.Info("media ready (first uplink)", "session", sessionID)
				}
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

func (r *SessionRuntime) consumeListenFinals(ctx context.Context, sessionID string, talk *composer.Talk, stream port.ListenStream) {
	if talk == nil || stream == nil {
		return
	}
	med := r.sessionMedia(sessionID)
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
			policy := r.bargePolicy(sessionID)
			if med.shouldQueueFinal(policy.WelcomeBargeAllowed) {
				med.queueFinal(final)
				applog.Info("live listen final queued (pre-conversing)", "session", sessionID, "chars", len(text))
				continue
			}
			r.deliverListenFinal(ctx, sessionID, talk, final, policy)
		}
	}
}

func (r *SessionRuntime) consumeListenPartials(ctx context.Context, sessionID string, talk *composer.Talk, stream port.ListenStream) {
	if talk == nil || stream == nil {
		return
	}
	med := r.sessionMedia(sessionID)
	partials := stream.Partials()
	var partialSince time.Time
	var lastPartialText string
	for {
		select {
		case <-ctx.Done():
			return
		case partial, ok := <-partials:
			if !ok {
				return
			}
			policy := r.bargePolicy(sessionID)
			if !policy.Allowed || !policy.ListenWhileSpeak {
				continue
			}
			st := talk.State()
			if st != composer.Thinking && st != composer.Speaking {
				partialSince = time.Time{}
				lastPartialText = ""
				continue
			}
			view := med.view()
			if view.WelcomeInProgress && !policy.WelcomeBargeAllowed {
				continue
			}
			text := strings.TrimSpace(partial.Text)
			if text == "" {
				continue
			}
			r.bargeMetric(talk, store.MetricBargeCandidateTotal, map[string]any{"source": "partial"})
			if text != lastPartialText {
				partialSince = time.Now()
				lastPartialText = text
			}
			if !policy.partialCommit(partial, partialSince) {
				continue
			}
			applog.Info("live listen partial barge commit", "session", sessionID, "state", string(st), "chars", len(text))
			r.bargeMetric(talk, store.MetricBargeCommitTotal, map[string]any{"source": "partial"})
			talk.Interrupt()
			partialSince = time.Time{}
			lastPartialText = ""
		}
	}
}

func (r *SessionRuntime) bargeMetric(talk *composer.Talk, metric string, dims map[string]any) {
	if talk == nil || talk.Obs == nil {
		return
	}
	talk.Obs.Metric(context.Background(), metric, 1, dims)
}

func (r *SessionRuntime) auditListenDecision(talk *composer.Talk, eventType, text, lang, reason string) {
	if talk == nil || talk.Obs == nil {
		return
	}
	payload := map[string]any{
		"reason":     reason,
		"chars":      len([]rune(text)),
		"text":       truncateAuditText(text, 512),
		"language":   lang,
		"talk_state": string(talk.State()),
	}
	talk.Obs.Audit(context.Background(), eventType, payload)
	// Suppressed/ignored caller speech still belongs in the transcript for CX review.
	if reason != "accepted" && strings.TrimSpace(text) != "" {
		note := "[" + reason + "] " + strings.TrimSpace(text)
		talk.Obs.AppendUserOnly(context.Background(), note)
	}
}

func truncateAuditText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}

func (r *SessionRuntime) deliverListenFinal(ctx context.Context, sessionID string, talk *composer.Talk, final port.ListenFinal, policy bargePolicy) {
	text := strings.TrimSpace(final.Text)
	if text == "" {
		return
	}
	if ctrl, ok := r.DeskController(sessionID); ok && ctrl.Engine().Ended() {
		applog.Info("live listen final ignored (desk ended)", "session", sessionID, "chars", len(text))
		r.auditListenDecision(talk, "listen.ignored", text, final.Language, "desk_ended")
		return
	}
	if session.IsLikelyTTSEcho(text, talk.LastSpokenText()) {
		r.bargeMetric(talk, store.MetricBargeSuppressEchoTotal, map[string]any{"reason": "tts_echo"})
		applog.Info("live listen final suppressed (tts echo)", "session", sessionID, "chars", len(text))
		r.auditListenDecision(talk, "listen.suppressed", text, final.Language, "tts_echo")
		return
	}
	st := talk.State()
	if st == composer.Thinking || st == composer.Speaking {
		if !policy.Allowed {
			r.bargeMetric(talk, store.MetricBargeSuppressEchoTotal, map[string]any{"reason": "barge_disabled"})
			applog.Info("live listen final suppressed (barge disabled)", "session", string(talk.Session))
			r.auditListenDecision(talk, "listen.suppressed", text, final.Language, "barge_disabled")
			return
		}
		if !policy.textCommit(text) {
			r.bargeMetric(talk, store.MetricBargeSuppressEchoTotal, map[string]any{"reason": "short"})
			applog.Info("live listen final suppressed (short)", "session", string(talk.Session), "chars", len(text))
			r.auditListenDecision(talk, "listen.suppressed", text, final.Language, "short")
			return
		}
		applog.Info("live listen final barge commit", "session", string(talk.Session), "state", string(st), "chars", len(text))
		r.bargeMetric(talk, store.MetricBargeCommitTotal, map[string]any{"source": "final"})
		talk.Interrupt()
	}
	applog.Info("live listen final", "session", string(talk.Session), "lang", final.Language, "chars", len(text))
	r.auditListenDecision(talk, "listen.final", text, final.Language, "accepted")
	// Actor locks language once on the first confident final. Do not flip the desk
	// engine to every ambient STT language guess (Phase C / #2/#3/#10).
	// Pure greetings ("Hello" / "Namaste") must not lock or persist preferred
	// language — they are too weak to choose EN vs HI for an Indian CC desk.
	var err error
	if session.IsGreetingOnly(text) {
		err = talk.InjectFinal(context.Background(), text)
	} else {
		err = talk.OnListenFinal(context.Background(), final)
	}
	if err != nil {
		applog.Warn("OnListenFinal", "session", string(talk.Session), "err", err)
	}
	if ctrl, ok := r.DeskController(string(talk.Session)); ok && talk.Actor != nil {
		if act := strings.TrimSpace(talk.Actor.ActiveLanguage()); act != "" {
			ctrl.Engine().SetLanguage(act)
		}
	}
}

func (r *SessionRuntime) drainQueuedFinals(ctx context.Context, sessionID string, talk *composer.Talk) {
	med := r.sessionMedia(sessionID)
	policy := r.bargePolicy(sessionID)
	for _, final := range med.takeQueuedFinals() {
		r.deliverListenFinal(ctx, sessionID, talk, final, policy)
	}
}

func (r *SessionRuntime) armSilenceWatch(sessionID string) {
	r.mu.Lock()
	lt := r.lives[sessionID]
	r.mu.Unlock()
	if lt == nil {
		return
	}
	lt.silenceOnce.Do(func() {
		close(lt.silenceArm)
	})
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
	// Persist preference for returning ANI when we know the caller id.
	if r.Repo != nil {
		if sess, err := r.Repo.GetSession(context.Background(), sessionID); err == nil {
			r.saveCallerPreference(sess.TenantID, callerANIFromJSON(sess.Caller), primary, prefSourceOperator)
		}
	}
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

	live := r.hasLiveTalk(sessionID)
	if live {
		med := r.sessionMedia(sessionID)
		view := med.view()
		if view.WelcomeCompleted {
			return "", nil
		}
		if err := med.beginWelcome(); err != nil {
			return "", err
		}
		talk.SetWelcoming(true)
		talk.SetWelcomeReadyAt(med.readyAtTime())
	}

	spoken, err := talk.AnswerCall(ctx)
	if live {
		talk.SetWelcoming(false)
	}
	if err != nil {
		if live {
			r.sessionMedia(sessionID).revertWelcome()
		}
		return spoken, err
	}

	if live {
		r.sessionMedia(sessionID).completeWelcome()
		r.armSilenceWatch(sessionID)
		// Post-welcome services menu: conversing phase, barge + continuous STT on.
		if ctrl, ok := r.DeskController(sessionID); ok {
			if menu := strings.TrimSpace(ctrl.Engine().ServicesMenu()); menu != "" {
				if err := talk.SpeakLine(ctx, menu); err != nil {
					applog.Warn("post-welcome menu speak", "session", sessionID, "err", err)
				} else if spoken == "" {
					spoken = menu
				} else {
					spoken = spoken + " " + menu
				}
			}
		}
		r.drainQueuedFinals(ctx, sessionID, talk)
	}
	return spoken, nil
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
	sessionID := a.ID
	// Agent leg of the call recording: every Speak frame that reaches the edge.
	talk.RecordAgent = func(pcm []byte) {
		r.recorderFor(sessionID).WriteAgent(pcm)
	}
	// Unrecoverable pipeline errors play the operator prompt and release the call.
	talk.OnFailure = func(ctx context.Context, err error) {
		r.FailCall(ctx, sessionID, fallback.Classify(err), err)
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
	think := thinkFromActor(a)
	policy := defaultBargePolicy()
	if ctrl, ok := newDeskController(a.Profile, a.Reg, r.Repo, a.ID, a.TenantID, profileVersion, think); ok {
		a.SetLanguageAllowlist(ctrl.Engine().Doc().RuntimeAllowlist())
		if a.GatewayBinding != nil {
			ctrl.Engine().SetAttribute("gateway_speak", a.GatewayBinding.Speak)
		}
		if lang := strings.TrimSpace(a.ActiveLanguage()); lang != "" {
			ctrl.Engine().SetLanguage(lang)
		}
		doc := ctrl.Engine().Doc()
		policy = bargePolicyFromDesk(doc)
		welcomeBarge := doc.CX.WelcomeBargeAllowed != nil && *doc.CX.WelcomeBargeAllowed
		talk.SetWelcomeBargeAllowed(welcomeBarge)
		talk.Path.Desk = ctrl
		talk.OnDeskHangupArm = r.deskHangupArmHandler(a.ID)
		talk.OnDeskEnd = r.deskEndHandler(a.ID)
		talk.OnDeskTransfer = r.deskTransferHandler(a.ID)
		if r.desks == nil {
			r.desks = make(map[string]*deskController)
		}
		r.desks[a.ID] = ctrl
		applog.Info("desk controller bound", "session", a.ID, "desk", doc.ID)
	}
	// Energy-VAD barge while Speaking: Sarvam has no partials, so STT-only barge
	// cannot interrupt mid-utterance. Desk CX BargeIn/MinBargeMs drives this.
	// (Computed above under r.mu — do not call bargePolicy/DeskController here.)
	talk.ConfigureBarge(policy.Allowed, policy.MinBargeMs)
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
// It must Hangup the telephony leg: StopSession alone closes the WebSocket and
// leaves the SIP call up until the caller hangs up (dead air).
func (r *SessionRuntime) deskEndHandler(sessionID string) func(string) {
	return func(disposition string) {
		r.releaseAfterDeskDecision(sessionID, disposition)
	}
}

func (r *SessionRuntime) deskHangupArmHandler(sessionID string) func(string) {
	return func(disposition string) {
		r.armDeskHangup(sessionID, disposition)
	}
}

func isTransferDisposition(disposition string) bool {
	d := strings.TrimSpace(disposition)
	return strings.HasPrefix(d, "transferred_")
}

func (r *SessionRuntime) sessionCallAction(sessionID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.callAction == nil {
		return ""
	}
	return r.callAction[sessionID]
}

// claimCallAction stores the first telephony action for a session.
// Returns true only when this call newly claimed the action.
func (r *SessionRuntime) claimCallAction(sessionID, action string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.callAction == nil {
		r.callAction = make(map[string]string)
	}
	if _, ok := r.callAction[sessionID]; ok {
		return false
	}
	r.callAction[sessionID] = action
	return true
}

// armDeskHangup sends edge Hangup (with playout drain) as early as possible —
// typically before the closing TTS — so WS teardown cannot skip the verb.
func (r *SessionRuntime) armDeskHangup(sessionID, disposition string) {
	if isTransferDisposition(disposition) || r.sessionCallAction(sessionID) == "transfer" {
		applog.Info("desk hangup arm skipped (transfer)", "session", sessionID, "disposition", disposition)
		return
	}
	first := r.claimCallAction(sessionID, "hangup")
	ctx := context.Background()
	if first {
		if talk, ok := r.talkForSession(sessionID); ok && talk != nil && talk.Obs != nil {
			talk.Obs.Audit(ctx, "desk.end", map[string]any{
				"disposition": disposition,
				"action":      "hangup_armed",
			})
		}
		if strings.TrimSpace(disposition) != "" {
			r.recordDisposition(ctx, sessionID, disposition, "desk_end")
		}
	}
	if !first {
		return
	}
	if cc, _, ok := r.callControl(sessionID); ok && cc != nil {
		if err := cc.Hangup(ctx, "NORMAL_CLEARING"); err != nil {
			applog.Warn("desk hangup arm", "session", sessionID, "err", err)
		} else {
			applog.Info("desk hangup armed before/with closing line", "session", sessionID, "disposition", disposition)
		}
	} else {
		applog.Warn("desk hangup arm: no call control", "session", sessionID)
	}
}

// releaseAfterDeskDecision waits for an armed hangup to settle, then ends the session.
//
// Critical ordering: Hangup is preferably armed via armDeskHangup before Speak.
// Do NOT StopSession (closes WebSocket) until after the edge has drained inject
// and hung up. Closing the WS early aborts the armed hangup and leaves the SIP
// call up until the caller disconnects.
func (r *SessionRuntime) releaseAfterDeskDecision(sessionID, disposition string) {
	ctx := context.Background()
	applog.Info("desk ended call", "session", sessionID, "disposition", disposition)

	// Transfer path must never hang up — silence or a late End flag after
	// uuid_transfer would kill the destination leg.
	if isTransferDisposition(disposition) || r.sessionCallAction(sessionID) == "transfer" {
		applog.Info("desk end skipped hangup after transfer", "session", sessionID, "disposition", disposition)
		return
	}

	r.armDeskHangup(sessionID, disposition)
	// Closing line may still be draining on the edge after Speak returned.
	r.waitPlayout(ctx, sessionID, 15*time.Second)
	r.waitCallControlGone(sessionID, 16*time.Second)
	if r.OnSessionEnd != nil {
		r.OnSessionEnd(ctx, sessionID, disposition)
	}
}

// waitCallControlGone waits until the edge telephony sink is gone (FS hung up /
// transferred and closed the WebSocket), or timeout.
func (r *SessionRuntime) waitCallControlGone(sessionID string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, _, ok := r.callControl(sessionID); !ok {
			applog.Info("edge settled after call control", "session", sessionID)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	applog.Warn("edge still present after call-control wait", "session", sessionID, "timeout", timeout.String())
}

func (r *SessionRuntime) talkForSession(sessionID string) (*composer.Talk, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.talks[sessionID]
	return t, ok
}

// initialListenLanguage is the Listen LanguageHint at call start.
// Per LANGUAGE_POLICY: empty until lock (vendor auto-detect), except when a
// returning caller's preference (or mid-call switch) already set active_language.
// Desk canonical/default is for prompt lookup and Welcome() — not an STT pin.
func (r *SessionRuntime) initialListenLanguage(sessionID string, a *session.Actor) string {
	if a != nil {
		if l := strings.TrimSpace(a.ActiveLanguage()); l != "" {
			return l
		}
	}
	return ""
}

func pickNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// deskTransferHandler moves the caller's leg once the desk has spoken its
// "connecting you now" line. The blind transfer is the real telephony action:
//
//	uuid_transfer <call-id> <number> XML calltransfer
//
// SessionRuntime.Transfer drains the queued prompt, sends the transfer to the
// edge, and records the disposition; the session ends when the leg leaves.
func (r *SessionRuntime) deskTransferHandler(sessionID string) func(context.Context, thinkpath.TransferIntent) {
	return func(ctx context.Context, ti thinkpath.TransferIntent) {
		// Always resolve dial destination from the live desk matrix for the
		// current intent — never trust a stale/hardcoded number on the intent.
		if ctrl, ok := r.DeskController(sessionID); ok {
			eng := ctrl.Engine()
			attrs := eng.Attributes()
			intent := attrs[desk.AttrIntent]
			target := ti.Target
			if target == "" {
				target = attrs[desk.AttrTransferTarget]
			}
			doc := eng.Doc()
			if n := doc.TransferNumberFor(intent, target); n != "" {
				ti.Number = n
			}
			if ti.Owner == "" {
				if row, ok := doc.MatrixFor(intent); ok {
					ti.Owner = row.Owner
					if ti.Target == "" {
						ti.Target = row.Target
					}
				}
			}
		}
		applog.Info("desk transfer requested", "session", sessionID,
			"number", ti.Number, "owner", ti.Owner, "target", ti.Target)
		reason := strings.TrimSpace(ti.Reason)
		if reason == "" {
			reason = "desk_transfer_" + ti.Target
		}
		if strings.TrimSpace(ti.Number) == "" {
			applog.Error("desk transfer aborted: no dialable number", "session", sessionID, "target", ti.Target)
			r.FailCall(ctx, sessionID, fallback.ScenarioSystemBusy, fmt.Errorf("no dialable transfer number"))
			return
		}
		// Claim transfer before hangup can win (silence / late End after endStep).
		if !r.claimCallAction(sessionID, "transfer") {
			if r.sessionCallAction(sessionID) == "transfer" {
				applog.Info("desk transfer already armed", "session", sessionID)
				return
			}
			applog.Warn("desk transfer blocked by prior call action", "session", sessionID, "action", r.sessionCallAction(sessionID))
			return
		}
		if talk, ok := r.talkForSession(sessionID); ok && talk != nil && talk.Obs != nil {
			talk.Obs.Audit(ctx, "desk.transfer", map[string]any{
				"number": ti.Number, "target": ti.Target, "owner": ti.Owner, "reason": reason,
				"action": "transfer_armed",
			})
		}
		if err := r.Transfer(ctx, sessionID, port.TransferRequest{
			Destination: ti.Number,
			Reason:      reason,
		}); err != nil {
			applog.Error("desk transfer failed", "session", sessionID,
				"number", ti.Number, "err", err)
			r.FailCall(ctx, sessionID, fallback.ScenarioSystemBusy, err)
			return
		}
		// Keep WS up until FS drains connect-line TTS and executes uuid_transfer.
		// Closing early aborts the armed transfer the same way early StopSession aborts hangup.
		r.waitCallControlGone(sessionID, 16*time.Second)
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

func thinkFromActor(a *session.Actor) port.Think {
	if a == nil || a.Reg == nil {
		return nil
	}
	id := ""
	if a.GatewayBinding != nil {
		id = strings.TrimSpace(a.GatewayBinding.Think)
	}
	if id == "" && len(a.Profile.Routers.Think.Providers) > 0 {
		id = strings.TrimSpace(a.Profile.Routers.Think.Providers[0])
	}
	if id == "" {
		return nil
	}
	rec, ok := a.Reg.Get(port.GatewayID(id))
	if !ok {
		return nil
	}
	th, ok := rec.Instance.(port.Think)
	if !ok {
		return nil
	}
	return th
}

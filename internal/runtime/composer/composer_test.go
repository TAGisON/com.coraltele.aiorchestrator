package composer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/bus"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/composer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/observe"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func talkProfile() profile.Document {
	var doc profile.Document
	doc.ID = "talk"
	doc.Modes.Talk = true
	doc.Modes.Listen = true
	doc.Modes.Speak = true
	doc.Modes.Think = true
	doc.Audio.CanonicalSampleRateHz = 16000
	doc.Persona.Voice = map[string]string{"fake-speak": "lab-voice"}
	doc.Routers.Listen.Providers = []string{"fake-listen"}
	doc.Routers.Speak.Providers = []string{"fake-speak"}
	doc.Routers.Think.Providers = []string{"fake-think"}
	return doc
}

func TestComposer_TurnAndBargeIn(t *testing.T) {
	reg := router.NewMemRegistry()
	slowSpeak := &fake.Speak{FrameCount: 20, InterFrameDelay: 15 * time.Millisecond}
	if err := reg.Register(port.Registration{
		ID: fake.IDSpeak, Port: port.PortSpeak,
		Capabilities: slowSpeak.Capabilities(), Instance: slowSpeak,
	}); err != nil {
		t.Fatal(err)
	}
	for _, it := range []port.Registration{
		{ID: fake.IDListen, Port: port.PortListen, Capabilities: (&fake.Listen{}).Capabilities(), Instance: &fake.Listen{}},
		{ID: fake.IDThink, Port: port.PortThink, Capabilities: (&fake.Think{}).Capabilities(), Instance: &fake.Think{}},
	} {
		if err := reg.Register(it); err != nil {
			t.Fatal(err)
		}
	}

	b := bus.New()
	mem := session.NewMemory()
	talk, err := composer.NewTalk(talkProfile(), reg, b, mem, "live", "sess-1")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- talk.InjectFinal(ctx, "hello there")
	}()

	deadline := time.Now().Add(2 * time.Second)
	for talk.State() != composer.Speaking {
		if time.Now().After(deadline) {
			t.Fatalf("never reached Speaking; state=%s", talk.State())
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Explicit Interrupt() barge while Speaking (STT path). Energy VAD barge is
	// off unless ConfigureBarge(true) — covered by TestComposer_EnergyBargeWhileSpeaking.
	for talk.State() != composer.Speaking {
		if time.Now().After(deadline) {
			t.Fatalf("never reached Speaking; state=%s", talk.State())
		}
		time.Sleep(5 * time.Millisecond)
	}

	talk.Interrupt()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("speak did not finish after barge-in")
	}

	if !talk.LastBargeIn() {
		t.Fatal("expected barge-in")
	}
	if talk.CancelCount() < 1 {
		t.Fatal("expected Speak.Cancel")
	}
	if slowSpeak.CancelCalls.Load() < 1 {
		t.Fatal("fake-speak Cancel not called")
	}
	if talk.Sink.Flushed() < 1 && talk.State() != composer.Capturing && talk.State() != composer.Listening {
		// flush may be 0 if no frames buffered yet; state must leave Speaking
		t.Fatalf("post-barge state %s flushed=%d", talk.State(), talk.Sink.Flushed())
	}
	st := talk.State()
	if st != composer.Capturing && st != composer.Listening {
		t.Fatalf("want Capturing/Listening got %s", st)
	}
}

func TestComposer_EnergyBargeWhileSpeaking(t *testing.T) {
	reg := router.NewMemRegistry()
	slowSpeak := &fake.Speak{FrameCount: 40, InterFrameDelay: 20 * time.Millisecond}
	if err := reg.Register(port.Registration{
		ID: fake.IDSpeak, Port: port.PortSpeak,
		Capabilities: slowSpeak.Capabilities(), Instance: slowSpeak,
	}); err != nil {
		t.Fatal(err)
	}
	for _, it := range []port.Registration{
		{ID: fake.IDListen, Port: port.PortListen, Capabilities: (&fake.Listen{}).Capabilities(), Instance: &fake.Listen{}},
		{ID: fake.IDThink, Port: port.PortThink, Capabilities: (&fake.Think{}).Capabilities(), Instance: &fake.Think{}},
	} {
		if err := reg.Register(it); err != nil {
			t.Fatal(err)
		}
	}

	talk, err := composer.NewTalk(talkProfile(), reg, bus.New(), session.NewMemory(), "live", "sess-energy-barge")
	if err != nil {
		t.Fatal(err)
	}
	talk.ConfigureBarge(true, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- talk.InjectFinal(ctx, "hello there") }()

	deadline := time.Now().Add(2 * time.Second)
	for talk.State() != composer.Speaking {
		if time.Now().After(deadline) {
			t.Fatalf("never reached Speaking; state=%s", talk.State())
		}
		time.Sleep(5 * time.Millisecond)
	}

	speech := make([]byte, 640)
	for i := 0; i+1 < len(speech); i += 2 {
		speech[i], speech[i+1] = 0x00, 0x40
	}
	start := time.Now()
	for time.Since(start) < 200*time.Millisecond && talk.State() == composer.Speaking {
		talk.OnPCM(port.PCMFrame{Data: speech, SampleRate: 16000, At: time.Now()})
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("speak did not finish after energy barge")
	}
	if !talk.LastBargeIn() {
		t.Fatal("expected energy-VAD barge-in")
	}
}

func TestComposer_NoGatewayImport(t *testing.T) {
	// Compiles with port+router only via NewTalk; registry supplies Instances.
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	talk, err := composer.NewTalk(talkProfile(), reg, bus.New(), session.NewMemory(), "live", "s")
	if err != nil {
		t.Fatal(err)
	}
	if err := talk.InjectFinal(context.Background(), "ping"); err != nil {
		t.Fatal(err)
	}
	if talk.State() != composer.Listening {
		t.Fatalf("state %s", talk.State())
	}
}

func TestComposer_OnListenFinal_LocksAndConsumesLanguage(t *testing.T) {
	reg := router.NewMemRegistry()
	spk := &fake.Speak{FrameCount: 1}
	th := &fake.Think{}
	if err := reg.Register(port.Registration{
		ID: fake.IDSpeak, Port: port.PortSpeak, Capabilities: spk.Capabilities(), Instance: spk,
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(port.Registration{
		ID: fake.IDThink, Port: port.PortThink, Capabilities: th.Capabilities(), Instance: th,
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(port.Registration{
		ID: fake.IDListen, Port: port.PortListen, Capabilities: (&fake.Listen{}).Capabilities(), Instance: &fake.Listen{},
	}); err != nil {
		t.Fatal(err)
	}
	actor := &session.Actor{}
	talk, err := composer.NewTalk(talkProfile(), reg, bus.New(), session.NewMemory(), "live", "sess-lang")
	if err != nil {
		t.Fatal(err)
	}
	talk.BindActor(actor)
	if err := talk.OnListenFinal(context.Background(), port.ListenFinal{
		Text: "namaste", Language: "hi-IN", Confidence: 0.9,
	}); err != nil {
		t.Fatal(err)
	}
	if actor.ActiveLanguage() != "hi-IN" {
		t.Fatalf("active=%q", actor.ActiveLanguage())
	}
	if spk.LastLanguage != "hi-IN" {
		t.Fatalf("Speak Language=%q want hi-IN", spk.LastLanguage)
	}
	if len(th.LastMessages) == 0 || th.LastMessages[0].Role != "system" {
		t.Fatalf("want system language instruction, got %+v", th.LastMessages)
	}
	if th.LastMessages[0].Content != "Respond in language: hi-IN" {
		t.Fatalf("system=%q", th.LastMessages[0].Content)
	}
	// second final different language — no flip; Speak still hi-IN
	spk.LastLanguage = ""
	if err := talk.OnListenFinal(context.Background(), port.ListenFinal{
		Text: "hello", Language: "en-IN", Confidence: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if actor.ActiveLanguage() != "hi-IN" {
		t.Fatal("ambient re-detect flipped")
	}
	if spk.LastLanguage != "hi-IN" {
		t.Fatalf("Speak after ambient=%q", spk.LastLanguage)
	}
}

func TestComposer_VoiceIDFromPersonaMapAndBoundSpeak(t *testing.T) {
	reg := router.NewMemRegistry()
	spk := &fake.Speak{FrameCount: 1}
	if err := reg.Register(port.Registration{
		ID: fake.IDSpeak, Port: port.PortSpeak, Capabilities: spk.Capabilities(), Instance: spk,
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(port.Registration{
		ID: fake.IDThink, Port: port.PortThink, Capabilities: (&fake.Think{}).Capabilities(), Instance: &fake.Think{},
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(port.Registration{
		ID: fake.IDListen, Port: port.PortListen, Capabilities: (&fake.Listen{}).Capabilities(), Instance: &fake.Listen{},
	}); err != nil {
		t.Fatal(err)
	}

	doc := talkProfile()
	doc.Persona.Voice = map[string]string{"fake-speak": "bound-speaker"}
	doc.Persona.VoiceID = "scalar-fallback"
	// Conflicting profile list would pick a different order without binding preference.
	doc.Routers.Speak.Providers = []string{"fake-speak"}

	actor := &session.Actor{
		GatewayBinding: &store.GatewayBinding{
			Listen: "fake-listen", Think: "fake-think", Speak: "fake-speak",
		},
	}
	actor.OnListenFinal(port.ListenFinal{Text: "hi", Language: "en-IN", Confidence: 1})

	talk, err := composer.NewTalk(doc, reg, bus.New(), session.NewMemory(), "live", "sess-voice")
	if err != nil {
		t.Fatal(err)
	}
	talk.BindActor(actor)
	if err := talk.InjectFinal(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if spk.LastVoiceID != "bound-speaker" {
		t.Fatalf("VoiceID=%q want bound-speaker", spk.LastVoiceID)
	}
	if spk.LastLanguage != "en-IN" {
		t.Fatalf("Language=%q want en-IN", spk.LastLanguage)
	}

	// Map miss → scalar voice_id
	spk.LastVoiceID = ""
	doc2 := talkProfile()
	doc2.Persona.Voice = map[string]string{"sarvam-tts": "anushka"}
	doc2.Persona.VoiceID = "scalar-only"
	talk2, err := composer.NewTalk(doc2, reg, bus.New(), session.NewMemory(), "live", "sess-voice-2")
	if err != nil {
		t.Fatal(err)
	}
	talk2.BindActor(&session.Actor{
		GatewayBinding: &store.GatewayBinding{Speak: "fake-speak"},
	})
	if err := talk2.InjectFinal(context.Background(), "ping"); err != nil {
		t.Fatal(err)
	}
	if spk.LastVoiceID != "scalar-only" {
		t.Fatalf("VoiceID=%q want scalar-only", spk.LastVoiceID)
	}
}

func TestComposer_ClipPathSkipsLLM(t *testing.T) {
	reg := router.NewMemRegistry()
	spk := &fake.Speak{FrameCount: 1}
	th := &fake.Think{}
	for _, it := range []port.Registration{
		{ID: fake.IDSpeak, Port: port.PortSpeak, Capabilities: spk.Capabilities(), Instance: spk},
		{ID: fake.IDThink, Port: port.PortThink, Capabilities: th.Capabilities(), Instance: th},
		{ID: fake.IDListen, Port: port.PortListen, Capabilities: (&fake.Listen{}).Capabilities(), Instance: &fake.Listen{}},
	} {
		if err := reg.Register(it); err != nil {
			t.Fatal(err)
		}
	}
	doc := talkProfile()
	doc.Response = &profile.ResponseConfig{
		Ladder: []string{"clip", "template", "llm"},
		Clips: map[string]profile.CannedUtterance{
			"greeting-en": {
				Text: "Welcome to support.",
				When: map[string]any{"regex": `(?i)\b(hi|hello|hey)\b`},
			},
		},
	}
	memStore := store.NewMemory()
	obs := &observe.Observer{Repo: memStore, Meta: observe.SessionMeta{
		SessionID: "sess-clip", TenantID: "t1", ProfileID: "talk", ProfileVersion: 1,
	}}
	talk, err := composer.NewTalk(doc, reg, bus.New(), session.NewMemory(), "live", "sess-clip")
	if err != nil {
		t.Fatal(err)
	}
	talk.Obs = obs
	if err := talk.InjectFinal(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if th.CompleteCalls.Load() != 0 {
		t.Fatalf("Think.Complete called %d; clip must skip LLM", th.CompleteCalls.Load())
	}
	if spk.LastText != "Welcome to support." {
		t.Fatalf("Speak text=%q", spk.LastText)
	}
	ams, _ := memStore.ListAnalyticsEvents(context.Background(), "sess-clip")
	found := false
	for _, m := range ams {
		if m.Metric != store.MetricTurnCompleted {
			continue
		}
		var dims map[string]any
		if len(m.Dimensions) > 0 {
			_ = json.Unmarshal(m.Dimensions, &dims)
		}
		if dims["response_tier"] == "clip" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want turn_completed response_tier=clip, got %#v", ams)
	}
}

func TestComposer_AnswerCallWritesAttachedSink(t *testing.T) {
	reg := router.NewMemRegistry()
	spk := &fake.Speak{FrameCount: 3}
	for _, it := range []port.Registration{
		{ID: fake.IDSpeak, Port: port.PortSpeak, Capabilities: spk.Capabilities(), Instance: spk},
		{ID: fake.IDThink, Port: port.PortThink, Capabilities: (&fake.Think{}).Capabilities(), Instance: &fake.Think{}},
		{ID: fake.IDListen, Port: port.PortListen, Capabilities: (&fake.Listen{}).Capabilities(), Instance: &fake.Listen{}},
	} {
		if err := reg.Register(it); err != nil {
			t.Fatal(err)
		}
	}
	doc := talkProfile()
	doc.Response = &profile.ResponseConfig{
		Ladder: []string{"clip", "llm"},
		Clips:  map[string]profile.CannedUtterance{"greeting-en": {Text: "Hello from sink test."}},
	}
	mgr := session.NewManager(reg)
	actor, err := mgr.Start(context.Background(), session.StartParams{
		SessionID: "sess-sink", Clock: "live", SampleRate: 16000, Profile: doc, Reg: reg,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = mgr.Stop(context.Background(), "sess-sink", "test") }()

	sink := &recordingSink{}
	actor.AttachSink(sink, "test-sink")

	talk, err := composer.NewTalk(doc, reg, bus.New(), session.NewMemory(), "live", "sess-sink")
	if err != nil {
		t.Fatal(err)
	}
	talk.BindActor(actor)

	spoken, err := talk.AnswerCall(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if spoken == "" {
		t.Fatal("expected spoken greeting")
	}
	if sink.writes == 0 {
		t.Fatal("Speak must WritePCM to attached edge sink (FS downlink)")
	}
	if sink.marks == 0 {
		t.Fatal("Speak must WaitMark after TTS so playout can drain")
	}
}

type recordingSink struct {
	writes  int
	marks   int
	flushes int
}

func (s *recordingSink) ID() port.GatewayID { return "recording-sink" }
func (s *recordingSink) WritePCM(ctx context.Context, frame port.PCMFrame) error {
	s.writes++
	return nil
}
func (s *recordingSink) Flush(ctx context.Context) error    { s.flushes++; return nil }
func (s *recordingSink) WaitMark(ctx context.Context) error { s.marks++; return nil }
func (s *recordingSink) Close(ctx context.Context) error    { return nil }

func TestComposer_AnswerCallSpeaksGreetingNoUserTurn(t *testing.T) {
	reg := router.NewMemRegistry()
	spk := &fake.Speak{FrameCount: 1}
	th := &fake.Think{}
	for _, it := range []port.Registration{
		{ID: fake.IDSpeak, Port: port.PortSpeak, Capabilities: spk.Capabilities(), Instance: spk},
		{ID: fake.IDThink, Port: port.PortThink, Capabilities: th.Capabilities(), Instance: th},
		{ID: fake.IDListen, Port: port.PortListen, Capabilities: (&fake.Listen{}).Capabilities(), Instance: &fake.Listen{}},
	} {
		if err := reg.Register(it); err != nil {
			t.Fatal(err)
		}
	}
	doc := talkProfile()
	doc.Rules = []profile.Rule{{
		ID: "disclosure", Phase: "pre_speak_first", Action: "inject_text",
		Text: "This call may use AI.",
	}}
	doc.Response = &profile.ResponseConfig{
		Ladder: []string{"clip", "llm"},
		Clips: map[string]profile.CannedUtterance{
			"greeting-en": {Text: "Welcome to Coral.", When: map[string]any{"regex": `(?i)hi`}},
		},
	}
	memStore := store.NewMemory()
	_ = memStore.CreateSession(context.Background(), store.Session{ID: "sess-answer", TenantID: "t1", ProfileID: "talk", State: store.StateRunning})
	obs := &observe.Observer{Repo: memStore, Meta: observe.SessionMeta{
		SessionID: "sess-answer", TenantID: "t1", ProfileID: "talk", ProfileVersion: 1,
	}}
	talk, err := composer.NewTalk(doc, reg, bus.New(), session.NewMemory(), "live", "sess-answer")
	if err != nil {
		t.Fatal(err)
	}
	talk.Obs = obs
	spoken, err := talk.AnswerCall(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := "This call may use AI. Welcome to Coral."
	if spoken != want {
		t.Fatalf("spoken=%q want %q", spoken, want)
	}
	if spk.LastText != want {
		t.Fatalf("Speak text=%q", spk.LastText)
	}
	if th.CompleteCalls.Load() != 0 {
		t.Fatal("Think must not run on answer")
	}
	turns, err := memStore.ListTranscriptTurns(context.Background(), "sess-answer")
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 || turns[0].Role != store.RoleAssistant {
		t.Fatalf("want single assistant turn, got %#v", turns)
	}
	// idempotent
	again, err := talk.AnswerCall(context.Background())
	if err != nil || again != "" {
		t.Fatalf("second AnswerCall spoken=%q err=%v", again, err)
	}
}

func TestComposer_ThinkDownClipEscalateNoVendorSwitch(t *testing.T) {
	reg := router.NewMemRegistry()
	spk := &fake.Speak{FrameCount: 1}
	th := &fake.Think{FailWith: &port.GatewayError{Code: port.CodeUnavailable, Message: "down"}}
	sk := &fake.Skill{}
	for _, it := range []port.Registration{
		{ID: fake.IDSpeak, Port: port.PortSpeak, Capabilities: spk.Capabilities(), Instance: spk},
		{ID: fake.IDThink, Port: port.PortThink, Capabilities: th.Capabilities(), Instance: th},
		{ID: fake.IDListen, Port: port.PortListen, Capabilities: (&fake.Listen{}).Capabilities(), Instance: &fake.Listen{}},
		{ID: fake.IDSkill, Port: port.PortSkill, Capabilities: sk.Capabilities(), Instance: sk},
	} {
		if err := reg.Register(it); err != nil {
			t.Fatal(err)
		}
	}
	doc := talkProfile()
	doc.Response = &profile.ResponseConfig{
		Ladder: []string{"clip", "llm"},
		Clips: map[string]profile.CannedUtterance{
			"clip-escalate-en": {Text: "Connecting you now."},
		},
	}
	doc.Fallback = &profile.FallbackConfig{
		ThinkDown: &profile.FallbackAction{SpeakCanned: "clip-escalate-en", Skill: "warm_transfer"},
	}
	doc.Skills.Allowed = []string{"warm_transfer"}
	doc.Skills.Definitions = map[string]profile.SkillDefinition{
		"warm_transfer": {Gateway: "fake-skill", Authority: "act", Confirm: false},
	}
	actor := &session.Actor{
		GatewayBinding: &store.GatewayBinding{
			Listen: "fake-listen", Think: "fake-think", Speak: "fake-speak",
		},
	}
	talk, err := composer.NewTalk(doc, reg, bus.New(), session.NewMemory(), "live", "sess-down")
	if err != nil {
		t.Fatal(err)
	}
	talk.BindActor(actor)
	if err := talk.InjectFinal(context.Background(), "billing question"); err != nil {
		t.Fatal(err)
	}
	if spk.LastText != "Connecting you now." {
		t.Fatalf("Speak=%q", spk.LastText)
	}
	if th.CompleteCalls.Load() != 1 {
		t.Fatalf("CompleteCalls=%d want 1 (no second Think gateway)", th.CompleteCalls.Load())
	}
	if sk.Calls.Load() < 1 {
		t.Fatal("warm_transfer not run")
	}
}

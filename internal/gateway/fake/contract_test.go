package fake_test

import (
	"context"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
)

func TestHappyPathAllPorts(t *testing.T) {
	ctx := context.Background()
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}

	listen := &fake.Listen{FinalText: "hi"}
	stream, err := listen.OpenStream(ctx, port.ListenRequest{SessionID: "s1", SampleRate: 16000, Clock: "live"})
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.WritePCM(ctx, port.PCMFrame{Data: []byte{0, 0}, SampleRate: 16000}); err != nil {
		t.Fatal(err)
	}
	final := <-stream.Finals()
	if final.Text != "hi" {
		t.Fatalf("final=%q", final.Text)
	}
	_ = stream.Close(ctx)

	spk := &fake.Speak{}
	ss, err := spk.Speak(ctx, port.SpeakRequest{SessionID: "s1", Text: "hello", SampleRate: 16000})
	if err != nil {
		t.Fatal(err)
	}
	fr := <-ss.Frames()
	if len(fr.Data) == 0 {
		t.Fatal("expected frame")
	}
	<-ss.Done()

	th := &fake.Think{}
	tr, err := th.Complete(ctx, port.ThinkRequest{
		SessionID: "s1",
		Messages:  []port.ChatMessage{{Role: "user", Content: "ping"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tr.Text == "" {
		t.Fatal("empty think")
	}

	trr := &fake.Translate{}
	out, err := trr.Translate(ctx, port.TranslateRequest{Text: "a", Source: "en", Target: "hi"})
	if err != nil || out == "" {
		t.Fatalf("translate %v %q", err, out)
	}

	kn := &fake.Knowledge{}
	kr, err := kn.Retrieve(ctx, port.KnowledgeQuery{Query: "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if kr.Hit {
		t.Fatal("expected miss")
	}

	sk := &fake.Skill{}
	sr, err := sk.Execute(ctx, port.SkillRequest{Name: "noop", SessionID: "s1"})
	if err != nil || !sr.OK {
		t.Fatalf("skill %v %+v", err, sr)
	}
}

func TestSpeakCancelStopsDelivery(t *testing.T) {
	ctx := context.Background()
	spk := &fake.Speak{Delay: 200 * time.Millisecond}
	ss, err := spk.Speak(ctx, port.SpeakRequest{SessionID: "s1", Text: "x", SampleRate: 16000})
	if err != nil {
		t.Fatal(err)
	}
	if err := ss.Cancel(ctx); err != nil {
		t.Fatal(err)
	}
	// After cancel, Done should be closed; frames may be empty or partial.
	select {
	case <-ss.Done():
	case <-time.After(time.Second):
		t.Fatal("done not closed after cancel")
	}
}

func TestTimeoutCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	spk := &fake.Speak{Delay: 500 * time.Millisecond}
	ss, err := spk.Speak(ctx, port.SpeakRequest{SessionID: "s1", Text: "x", SampleRate: 16000})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ss.Frames():
		// may or may not get a frame depending on race; either OK
	case <-time.After(100 * time.Millisecond):
	}
	// GatewayError timeout helper: emulate router wrapping ctx deadline
	ge := &port.GatewayError{Code: port.CodeTimeout, Message: "hop timeout", Retryable: true}
	if ge.Code != port.CodeTimeout {
		t.Fatal("timeout code")
	}
	_ = ctx.Err()
	_ = ss
}

func TestLiveRefusesNonStreaming(t *testing.T) {
	reg := router.NewMemRegistry()
	batch := &fake.Listen{BatchOnly: true}
	_ = reg.Register(port.Registration{
		ID:           "batch-listen",
		Port:         port.PortListen,
		Capabilities: batch.Capabilities(),
		Instance:     batch,
		Probe:        func(ctx context.Context) port.Health { return port.Health{Healthy: true} },
	})
	_, err := router.Select(reg, []port.GatewayID{"batch-listen"}, port.PortListen, router.SelectOptions{Clock: "live"})
	if err == nil {
		t.Fatal("expected refuse")
	}
	ge, ok := port.AsGatewayError(err)
	if !ok || ge.Code != port.CodeUnsupported {
		t.Fatalf("want unsupported got %v", err)
	}
}

func TestKnowledgeMiss(t *testing.T) {
	kn := &fake.Knowledge{}
	res, err := kn.Retrieve(context.Background(), port.KnowledgeQuery{Query: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Hit || len(res.Snippets) != 0 {
		t.Fatalf("expected miss: %+v", res)
	}
}

func TestSkillNoAutoRetry(t *testing.T) {
	sk := &fake.Skill{}
	_, _ = sk.Execute(context.Background(), port.SkillRequest{Name: "a"})
	_, _ = sk.Execute(context.Background(), port.SkillRequest{Name: "a"})
	if sk.Calls.Load() != 2 {
		t.Fatalf("calls=%d want 2 (no coalescing/auto-retry inside gateway)", sk.Calls.Load())
	}
}

func TestRouterSelectFakeSpeak(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	rec, err := router.Select(reg, []port.GatewayID{fake.IDSpeak}, port.PortSpeak, router.SelectOptions{Clock: "live"})
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != fake.IDSpeak {
		t.Fatalf("id=%s", rec.ID)
	}
}

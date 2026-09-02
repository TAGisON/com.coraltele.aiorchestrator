package control

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func TestNormalizeANI(t *testing.T) {
	if got := normalizeANI("+91-98123 45678"); got != "919812345678" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeANI("5001"); got != "5001" {
		t.Fatalf("got %q", got)
	}
}

func TestCallerANIFromJSON(t *testing.T) {
	raw := json.RawMessage(`{"ani":"+91 98000 11111","caller_id_name":"Desk"}`)
	if got := callerANIFromJSON(raw); got != "919800011111" {
		t.Fatalf("got %q", got)
	}
}

func TestCallerPreference_RestoreOnStartSession(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()
	_, err := mem.UpsertCallerPreference(ctx, store.CallerPreference{
		TenantID: "default", ANI: "5001", PreferredLanguage: "hi-IN", Source: "stt_lock",
	})
	if err != nil {
		t.Fatal(err)
	}

	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mgr := session.NewManager(reg)
	rt := &SessionRuntime{Mgr: mgr, Repo: mem}

	var doc profile.Document
	doc.ID = "p1"
	doc.Modes.Talk = true
	doc.Modes.Listen = true
	doc.Modes.Speak = true
	doc.Audio.CanonicalSampleRateHz = 16000
	doc.Routers.Listen.Providers = []string{"fake-listen"}
	doc.Routers.Speak.Providers = []string{"fake-speak"}
	doc.Routers.Think.Providers = []string{"fake-think"}

	if err := rt.StartSession(ctx, RuntimeStart{
		SessionID:  "s-pref-1",
		TenantID:   "default",
		Clock:      "live",
		SampleRate: 16000,
		Profile:    doc,
		Caller:     json.RawMessage(`{"ani":"5001"}`),
		GatewayBinding: &store.GatewayBinding{
			Listen: "fake-listen", Speak: "fake-speak", Think: "fake-think",
		},
	}); err != nil {
		t.Fatal(err)
	}

	a, ok := mgr.Get("s-pref-1")
	if !ok {
		t.Fatal("actor missing")
	}
	if a.ActiveLanguage() != "hi-IN" {
		t.Fatalf("want hi-IN restored, got %q", a.ActiveLanguage())
	}
	if !a.LanguageLocked() {
		t.Fatal("restored preference should lock language")
	}
}

func TestCallerPreference_SaveOnLanguageLock(t *testing.T) {
	mem := store.NewMemory()
	ctx := context.Background()
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	mgr := session.NewManager(reg)
	rt := &SessionRuntime{Mgr: mgr, Repo: mem}

	var doc profile.Document
	doc.ID = "p1"
	doc.Modes.Talk = true
	doc.Modes.Listen = true
	doc.Modes.Speak = true
	doc.Audio.CanonicalSampleRateHz = 16000
	doc.Routers.Listen.Providers = []string{"fake-listen"}
	doc.Routers.Speak.Providers = []string{"fake-speak"}
	doc.Routers.Think.Providers = []string{"fake-think"}

	caller := json.RawMessage(`{"ani":"919811122233"}`)
	_ = mem.CreateSession(ctx, store.Session{
		ID: "s-pref-2", TenantID: "default", ProfileID: "p1", ProfileVersion: 1,
		Clock: "live", State: store.StateRunning, Caller: caller,
	})
	if err := rt.StartSession(ctx, RuntimeStart{
		SessionID:  "s-pref-2",
		TenantID:   "default",
		Clock:      "live",
		SampleRate: 16000,
		Profile:    doc,
		Caller:     caller,
		GatewayBinding: &store.GatewayBinding{
			Listen: "fake-listen", Speak: "fake-speak", Think: "fake-think",
		},
	}); err != nil {
		t.Fatal(err)
	}

	a, _ := mgr.Get("s-pref-2")
	a.OnListenFinal(port.ListenFinal{Text: "namaste", Language: "hi-IN", Confidence: 0.9})

	pref, err := mem.GetCallerPreference(ctx, "default", "919811122233")
	if err != nil {
		t.Fatal(err)
	}
	if pref.PreferredLanguage != "hi-IN" {
		t.Fatalf("want hi-IN saved, got %q", pref.PreferredLanguage)
	}
	if pref.Source != prefSourceSTTLock {
		t.Fatalf("source %q", pref.Source)
	}
}

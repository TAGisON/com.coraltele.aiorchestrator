package session_test

import (
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
)

func TestLanguageLock_FirstConfidentFinal(t *testing.T) {
	a := &session.Actor{}
	locked := a.OnListenFinal(port.ListenFinal{Text: "namaste", Language: "hi-IN", Confidence: 0.9})
	if !locked {
		t.Fatal("expected lock")
	}
	if a.DetectedLanguage() != "hi-IN" || a.ActiveLanguage() != "hi-IN" || !a.LanguageLocked() {
		t.Fatalf("detected=%q active=%q locked=%v", a.DetectedLanguage(), a.ActiveLanguage(), a.LanguageLocked())
	}
	if a.ListenLanguageHint() != "hi-IN" {
		t.Fatalf("hint=%q", a.ListenLanguageHint())
	}
}

func TestLanguageLock_IgnoresAmbientRedetect(t *testing.T) {
	a := &session.Actor{}
	_ = a.OnListenFinal(port.ListenFinal{Language: "hi-IN", Confidence: 0.9})
	second := a.OnListenFinal(port.ListenFinal{Language: "en-IN", Confidence: 0.99})
	if second {
		t.Fatal("second final must not re-lock")
	}
	if a.ActiveLanguage() != "hi-IN" || a.DetectedLanguage() != "hi-IN" {
		t.Fatalf("want hi-IN kept, got detected=%q active=%q", a.DetectedLanguage(), a.ActiveLanguage())
	}
}

func TestLanguageLock_RejectsUnknownAndLowConfidence(t *testing.T) {
	a := &session.Actor{}
	if a.OnListenFinal(port.ListenFinal{Language: "unknown", Confidence: 1}) {
		t.Fatal("unknown must not lock")
	}
	if a.OnListenFinal(port.ListenFinal{Language: "", Confidence: 1}) {
		t.Fatal("empty must not lock")
	}
	if a.OnListenFinal(port.ListenFinal{Language: "hi-IN", Confidence: 0.4}) {
		t.Fatal("low confidence must not lock")
	}
	if !a.OnListenFinal(port.ListenFinal{Language: "hi-IN", Confidence: 0}) {
		t.Fatal("absent confidence + BCP-47 must lock")
	}
}

func TestLanguageSwitch_UpdatesActiveOnly(t *testing.T) {
	a := &session.Actor{}
	_ = a.OnListenFinal(port.ListenFinal{Language: "hi-IN", Confidence: 0.9})
	a.SwitchActiveLanguage("en-IN")
	if a.DetectedLanguage() != "hi-IN" {
		t.Fatalf("detected should stay hi-IN got %q", a.DetectedLanguage())
	}
	if a.ActiveLanguage() != "en-IN" {
		t.Fatalf("active=%q", a.ActiveLanguage())
	}
	if a.ListenLanguageHint() != "en-IN" {
		t.Fatalf("hint=%q", a.ListenLanguageHint())
	}
	_ = a.OnListenFinal(port.ListenFinal{Language: "ta-IN", Confidence: 1})
	if a.ActiveLanguage() != "en-IN" {
		t.Fatal("ambient must not flip after switch")
	}
}

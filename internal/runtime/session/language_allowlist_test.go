package session_test

import (
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
)

func TestLanguageLock_RespectsAllowlist(t *testing.T) {
	a := &session.Actor{}
	a.SetLanguageAllowlist([]string{"en-IN", "hi-IN"})
	if a.OnListenFinal(port.ListenFinal{Language: "ta-IN", Confidence: 0.9}) {
		t.Fatal("ta-IN should not lock when outside allowlist")
	}
	if !a.OnListenFinal(port.ListenFinal{Language: "hi-IN", Confidence: 0.9}) {
		t.Fatal("hi-IN should lock")
	}
}

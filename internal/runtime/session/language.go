package session

import (
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

const minLanguageConfidence float32 = 0.5

// ConfidentListenFinal reports whether a Listen final may lock session language
// (LANGUAGE_POLICY.md).
func ConfidentListenFinal(f port.ListenFinal) bool {
	lang := strings.TrimSpace(f.Language)
	if lang == "" || strings.EqualFold(lang, "unknown") {
		return false
	}
	// Confidence > 0 means vendor supplied a probability; require >= 0.5.
	// Confidence == 0 means absent → non-empty BCP-47 alone locks.
	if f.Confidence > 0 && f.Confidence < minLanguageConfidence {
		return false
	}
	return true
}

// DetectedLanguage returns the first locked detect (empty until lock).
func (a *Actor) DetectedLanguage() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.detectedLanguage
}

// ActiveLanguage returns the language Think/Speak consume (empty until lock/switch).
func (a *Actor) ActiveLanguage() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.activeLanguage
}

// LanguageLocked reports whether the first confident detect has locked the session.
func (a *Actor) LanguageLocked() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.languageLocked
}

// ListenLanguageHint is the LanguageHint for the next Listen open.
// Empty before lock (auto-detect); active_language after lock/switch.
func (a *Actor) ListenLanguageHint() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.activeLanguage
}

// OnListenFinal applies language lock on the first confident final.
// Later ambient re-detects are ignored. Returns true if this call locked.
func (a *Actor) OnListenFinal(f port.ListenFinal) bool {
	if !ConfidentListenFinal(f) {
		return false
	}
	lang := strings.TrimSpace(f.Language)
	a.mu.Lock()
	if a.languageLocked {
		a.mu.Unlock()
		return false
	}
	a.detectedLanguage = lang
	a.activeLanguage = lang
	a.languageLocked = true
	a.flushListenPartials = true
	persist := a.LanguagePersist
	det, act := a.detectedLanguage, a.activeLanguage
	a.mu.Unlock()
	if persist != nil {
		persist(det, act)
	}
	return true
}

// SwitchActiveLanguage sets active_language via explicit operator PATCH.
// Does not overwrite detected_language. Marks Listen hint dirty for restart.
func (a *Actor) SwitchActiveLanguage(primary string) {
	lang := strings.TrimSpace(primary)
	a.mu.Lock()
	a.activeLanguage = lang
	a.languageLocked = true // treat as locked so ambient cannot flip
	a.flushListenPartials = true
	persist := a.LanguagePersist
	det, act := a.detectedLanguage, a.activeLanguage
	a.mu.Unlock()
	if persist != nil {
		persist(det, act)
	}
}

// ConsumeListenFlush returns whether Listen partials should be flushed (once).
func (a *Actor) ConsumeListenFlush() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.flushListenPartials {
		return false
	}
	a.flushListenPartials = false
	return true
}

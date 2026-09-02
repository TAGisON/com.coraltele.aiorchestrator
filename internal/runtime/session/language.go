package session

import (
	"strings"
	"unicode"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

const minLanguageConfidence float32 = 0.5

// greetingOnly is normalized forms of pure openers that must not lock language.
var greetingOnly = map[string]struct{}{
	"hello": {}, "hi": {}, "hey": {}, "hii": {}, "hlo": {}, "helo": {},
	"good morning": {}, "good afternoon": {}, "good evening": {},
	"namaste": {}, "namaskar": {}, "namaskaram": {},
	"नमस्ते": {}, "नमस्कार": {},
	"hola": {}, "hai": {},
}

// IsLikelyTTSEcho reports caller STT that is mostly a repeat of the last agent line
// (acoustic echo / listen-while-speak). Used to stop false barge→intent transfers.
func IsLikelyTTSEcho(heard, spoken string) bool {
	h := strings.TrimSpace(heard)
	s := strings.TrimSpace(spoken)
	if h == "" || s == "" {
		return false
	}
	hn := normalizeEcho(h)
	sn := normalizeEcho(s)
	if hn == "" || sn == "" {
		return false
	}
	// Full or near-full containment (menu echoed back).
	hr, sr := []rune(hn), []rune(sn)
	if len(hr) >= 8 {
		if strings.Contains(sn, hn) && len(hr)*2 >= len(sr) {
			return true
		}
		if strings.Contains(hn, sn) && len(sr) >= 8 && len(sr)*2 >= len(hr) {
			return true
		}
	}
	ht := echoTokens(hn)
	st := echoTokens(sn)
	if len(ht) == 0 || len(st) == 0 {
		return false
	}
	matched := 0
	set := map[string]struct{}{}
	for _, t := range st {
		set[t] = struct{}{}
	}
	for _, t := range ht {
		if _, ok := set[t]; ok {
			matched++
		}
	}
	// High overlap of caller tokens against the last agent utterance.
	if float64(matched)/float64(len(ht)) >= 0.75 && matched >= 3 {
		return true
	}
	return false
}

func normalizeEcho(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.Is(unicode.Devanagari, r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func echoTokens(s string) []string {
	var out []string
	for _, t := range strings.Fields(s) {
		if len([]rune(t)) < 2 {
			continue
		}
		out = append(out, t)
	}
	return out
}

// IsGreetingOnly reports whether text is only a call opener (no intent content).
// Used so "Hello" / "Namaste" cannot lock STT/prefs to English on a bilingual desk.
func IsGreetingOnly(text string) bool {
	s := strings.ToLower(strings.TrimSpace(text))
	if s == "" {
		return false
	}
	// Strip common trailing punctuation.
	s = strings.TrimRightFunc(s, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	})
	s = strings.Join(strings.Fields(s), " ")
	_, ok := greetingOnly[s]
	return ok
}

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

// SetLanguageAllowlist restricts first-lock to desk runtime languages (empty = any).
func (a *Actor) SetLanguageAllowlist(langs []string) {
	a.mu.Lock()
	a.languageAllowlist = append([]string(nil), langs...)
	a.mu.Unlock()
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
	if len(a.languageAllowlist) > 0 && !languageInList(lang, a.languageAllowlist) {
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

func languageInList(lang string, list []string) bool {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if lang == "" {
		return false
	}
	base := lang
	if i := strings.IndexAny(lang, "-_"); i > 0 {
		base = lang[:i]
	}
	for _, l := range list {
		l = strings.TrimSpace(strings.ToLower(l))
		if l == lang {
			return true
		}
		lbase := l
		if i := strings.IndexAny(l, "-_"); i > 0 {
			lbase = l[:i]
		}
		if lbase == base {
			return true
		}
	}
	return false
}

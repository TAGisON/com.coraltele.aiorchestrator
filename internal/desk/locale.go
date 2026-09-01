package desk

import "strings"

// IndiaDefaultLanguages is the platform Contact Desk runtime allowlist (LIVE_TALK §6.4).
var IndiaDefaultLanguages = []string{
	"en-IN", "hi-IN", "bn-IN", "ta-IN", "te-IN", "mr-IN",
	"gu-IN", "kn-IN", "ml-IN", "pa-IN", "or-IN", "as-IN",
}

// PromptResolution is the outcome of ResolvePromptLocale.
type PromptResolution struct {
	Text           string
	CanonicalText  string
	Tier           string // exact | canonical | synthesize | missing
	NeedsSynthesis bool
}

// CanonicalLocale returns the desk primary locale for path/prompt lookup.
func (d Doc) CanonicalLocale() string {
	if s := strings.TrimSpace(d.CX.PrimaryLocale); s != "" {
		return s
	}
	return d.DefaultLanguage
}

// RuntimeAllowlist returns languages the runtime may serve on this desk.
func (d Doc) RuntimeAllowlist() []string {
	if len(d.CX.RuntimeLanguages) > 0 {
		return append([]string(nil), d.CX.RuntimeLanguages...)
	}
	return append([]string(nil), d.Languages...)
}

// EffectiveRuntimeLanguages intersects desk runtime demand with engine capability.
func EffectiveRuntimeLanguages(d Doc, engineLangs []string) []string {
	want := d.RuntimeAllowlist()
	if len(want) == 0 {
		want = append([]string(nil), IndiaDefaultLanguages...)
	}
	if len(engineLangs) == 0 {
		return want
	}
	return intersectLanguages(want, engineLangs)
}

func localeHasExact(m map[string]string, locale string) bool {
	if s := strings.TrimSpace(m[locale]); s != "" {
		return true
	}
	base := baseLang(locale)
	for k, v := range m {
		if baseLang(k) == base && strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// ResolvePromptLocale resolves Speak text for active_language (LIVE_TALK §6.3).
func ResolvePromptLocale(d Doc, promptID, activeLang string) PromptResolution {
	p, ok := d.Prompts[promptID]
	if !ok {
		return PromptResolution{Tier: "missing"}
	}
	canonical := d.CanonicalLocale()
	if localeHasExact(p.Text, activeLang) {
		return PromptResolution{
			Text:  pickLocale(p.Text, activeLang, canonical),
			Tier:  "exact",
		}
	}
	canonText := pickLocale(p.Text, canonical, d.DefaultLanguage)
	if canonText == "" {
		return PromptResolution{Tier: "missing"}
	}
	if languagesMatch(activeLang, canonical) {
		return PromptResolution{Text: canonText, Tier: "canonical"}
	}
	synth := true
	if d.CX.LocaleSynthesis != nil {
		synth = *d.CX.LocaleSynthesis
	}
	if !synth {
		return PromptResolution{Text: canonText, Tier: "canonical", CanonicalText: canonText}
	}
	return PromptResolution{
		CanonicalText:  canonText,
		NeedsSynthesis: true,
		Tier:           "synthesize",
	}
}

func languagesMatch(a, b string) bool {
	a = strings.TrimSpace(strings.ToLower(a))
	b = strings.TrimSpace(strings.ToLower(b))
	if a == b {
		return true
	}
	return baseLang(a) != "" && baseLang(a) == baseLang(b)
}

func intersectLanguages(want, have []string) []string {
	if len(want) == 0 || len(have) == 0 {
		return nil
	}
	set := map[string]struct{}{}
	for _, h := range have {
		set[strings.ToLower(strings.TrimSpace(h))] = struct{}{}
		if b := baseLang(h); b != "" {
			set[b] = struct{}{}
		}
	}
	var out []string
	seen := map[string]struct{}{}
	for _, w := range want {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		key := strings.ToLower(w)
		if _, ok := seen[key]; ok {
			continue
		}
		if _, ok := set[key]; ok {
			out = append(out, w)
			seen[key] = struct{}{}
			continue
		}
		if b := baseLang(w); b != "" {
			if _, ok := set[b]; ok {
				out = append(out, w)
				seen[key] = struct{}{}
			}
		}
	}
	return out
}

func languagesCovered(want, have []string) []string {
	if len(want) == 0 {
		return nil
	}
	effective := intersectLanguages(want, have)
	covered := map[string]struct{}{}
	for _, l := range effective {
		covered[strings.ToLower(l)] = struct{}{}
	}
	var missing []string
	for _, w := range want {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		if _, ok := covered[strings.ToLower(w)]; ok {
			continue
		}
		missing = append(missing, w)
	}
	return missing
}

// ValidateRuntimeEngineCoverage fails closed when runtime_languages exceed engine support.
func ValidateRuntimeEngineCoverage(d Doc, engineLangs []string) error {
	want := d.RuntimeAllowlist()
	if len(want) == 0 || len(engineLangs) == 0 {
		return nil
	}
	if missing := languagesCovered(want, engineLangs); len(missing) > 0 {
		return &CompileError{
			Message: "runtime_languages exceed engine language coverage",
			Details: map[string]any{"missing": missing, "engine_languages": engineLangs},
		}
	}
	return nil
}

// LanguageAllowed reports whether a BCP-47 tag is in the desk runtime allowlist.
func (d Doc) LanguageAllowed(lang string) bool {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return false
	}
	for _, l := range d.RuntimeAllowlist() {
		if languagesMatch(l, lang) {
			return true
		}
	}
	return false
}

package desk

import (
	"testing"
)

func TestResolvePromptLocaleExact(t *testing.T) {
	d := Doc{
		DefaultLanguage: "en-IN",
		CX:              CXPolicy{PrimaryLocale: "en-IN"},
		Prompts: map[string]Prompt{
			PromptWelcome: {Text: map[string]string{"en-IN": "Hello", "hi-IN": "नमस्ते"}},
		},
	}
	d.Normalize()
	res := ResolvePromptLocale(d, PromptWelcome, "hi-IN")
	if res.Tier != "exact" || res.Text != "नमस्ते" {
		t.Fatalf("res=%+v", res)
	}
}

func TestResolvePromptLocaleSynthesize(t *testing.T) {
	d := Doc{
		DefaultLanguage: "en-IN",
		Languages:       []string{"en-IN", "hi-IN", "ta-IN"},
		CX:              CXPolicy{PrimaryLocale: "en-IN", RuntimeLanguages: []string{"en-IN", "hi-IN", "ta-IN"}},
		Prompts: map[string]Prompt{
			PromptClarify: {Text: map[string]string{"en-IN": "Please clarify"}},
		},
	}
	d.Normalize()
	res := ResolvePromptLocale(d, PromptClarify, "ta-IN")
	if !res.NeedsSynthesis || res.CanonicalText != "Please clarify" {
		t.Fatalf("res=%+v", res)
	}
}

func TestValidateRuntimeEngineCoverage(t *testing.T) {
	d := Doc{
		Languages: []string{"en-IN", "hi-IN"},
		CX: CXPolicy{
			RuntimeLanguages: []string{"en-IN", "hi-IN", "ta-IN"},
			SilenceNudge1Ms:  6000,
		},
	}
	d.Normalize()
	if err := ValidateRuntimeEngineCoverage(d, []string{"en-IN", "hi-IN"}); err == nil {
		t.Fatal("expected fail closed for ta-IN")
	}
	if err := ValidateRuntimeEngineCoverage(d, LabEngineLanguages()); err != nil {
		t.Fatalf("India lab engines should cover en/hi/ta: %v", err)
	}
}

func TestEffectiveRuntimeLanguages(t *testing.T) {
	d := Doc{
		Languages: []string{"en-IN", "hi-IN"},
		CX: CXPolicy{
			RuntimeLanguages: []string{"en-IN", "hi-IN", "ta-IN", "fr-FR"},
			SilenceNudge1Ms:  6000,
		},
	}
	got := EffectiveRuntimeLanguages(d, LabEngineLanguages())
	if len(got) != 3 {
		t.Fatalf("got %v", got)
	}
}

func TestLanguageAllowed(t *testing.T) {
	d := Doc{Languages: []string{"en-IN"}, CX: CXPolicy{RuntimeLanguages: []string{"en-IN", "hi-IN"}}}
	d.Normalize()
	if !d.LanguageAllowed("hi-IN") {
		t.Fatal("hi-IN should be allowed via runtime_languages")
	}
	if d.LanguageAllowed("fr-FR") {
		t.Fatal("fr-FR must stay outside the India allowlist")
	}
}

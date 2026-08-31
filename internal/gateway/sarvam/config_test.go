package sarvam_test

import (
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvam"
)

func TestSTTLanguageCode(t *testing.T) {
	cases := []struct {
		hint string
		want string
	}{
		{"", "unknown"},
		{"  ", "unknown"},
		{"auto", "unknown"},
		{"AUTO", "unknown"},
		{"Auto", "unknown"},
		{"hi-IN", "hi-IN"},
		{"en-IN", "en-IN"},
	}
	for _, tc := range cases {
		if got := sarvam.STTLanguageCode(tc.hint); got != tc.want {
			t.Fatalf("STTLanguageCode(%q)=%q want %q", tc.hint, got, tc.want)
		}
	}
}

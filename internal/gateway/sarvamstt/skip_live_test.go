package sarvamstt_test

import (
	"os"
	"testing"
)

// TestLiveSkippedWithoutKey documents env-gated lab path; integration lives behind -tags=sarvam_live.
func TestLiveSkippedWithoutKey(t *testing.T) {
	if os.Getenv("SARVAM_API_KEY") != "" {
		t.Skip("key set — use -tags=sarvam_live for live calls")
	}
}

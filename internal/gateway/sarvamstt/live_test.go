//go:build sarvam_live

package sarvamstt_test

import (
	"context"
	"os"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvam"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvamstt"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

// Run with: go test -tags=sarvam_live ./internal/gateway/sarvamstt -count=1
// Requires SARVAM_API_KEY (never commit).
func TestLiveRecognizeBatch(t *testing.T) {
	if os.Getenv("SARVAM_API_KEY") == "" {
		t.Skip("SARVAM_API_KEY unset")
	}
	cfg, err := sarvam.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	g := sarvamstt.New(cfg)
	pcm := make([]byte, 16000*2) // 1s silence — may yield empty transcript; just ensure no transport panic
	_, err = g.RecognizeBatch(context.Background(), port.ListenRequest{
		SessionID: "live", SampleRate: 16000, LanguageHint: "en-IN",
	}, pcm)
	if err != nil {
		t.Logf("live batch err (may be empty audio): %v", err)
	}
}

package control

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

const defaultMinBargeChars = 3

type bargePolicy struct {
	Allowed             bool
	ListenWhileSpeak    bool
	WelcomeBargeAllowed bool
	MinBargeChars       int
	MinBargeMs          time.Duration
	PartialConfidence   float64
}

// defaultBargePolicy matches former DefaultCX numeric defaults (copied once; no CX import):
// BargeIn true, ListenWhileSpeak true, WelcomeBargeAllowed false,
// MinBargeChars 3, MinBargeMs 280, BargePartialConfidence 0.70.
func defaultBargePolicy() bargePolicy {
	return bargePolicy{
		Allowed:             true,
		ListenWhileSpeak:    true,
		WelcomeBargeAllowed: false,
		MinBargeChars:       defaultMinBargeChars,
		MinBargeMs:          280 * time.Millisecond,
		PartialConfidence:   0.70,
	}
}

func (r *SessionRuntime) bargePolicy(sessionID string) bargePolicy {
	_ = sessionID
	return defaultBargePolicy()
}

func (p bargePolicy) textCommit(text string) bool {
	t := strings.TrimSpace(text)
	return utf8.RuneCountInString(t) >= p.MinBargeChars
}

func (p bargePolicy) partialCommit(partial port.ListenPartial, since time.Time) bool {
	if !p.textCommit(partial.Text) {
		return false
	}
	conf := float64(partial.Confidence)
	if conf <= 0 {
		conf = 1
	}
	if conf < p.PartialConfidence {
		return false
	}
	if since.IsZero() {
		return false
	}
	return time.Since(since) >= p.MinBargeMs
}

func (r *SessionRuntime) rtpSettleFor(sessionID string) time.Duration {
	_ = sessionID
	return defaultRTPSettleMs * time.Millisecond
}

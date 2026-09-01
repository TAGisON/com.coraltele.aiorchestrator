package control

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/desk"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

const defaultMinBargeChars = 2

type bargePolicy struct {
	Allowed              bool
	ListenWhileSpeak     bool
	WelcomeBargeAllowed  bool
	MinBargeChars        int
	MinBargeMs           time.Duration
	PartialConfidence    float64
}

func defaultBargePolicy() bargePolicy {
	return bargePolicy{
		Allowed:           true,
		ListenWhileSpeak:  true,
		WelcomeBargeAllowed: false,
		MinBargeChars:     defaultMinBargeChars,
		MinBargeMs:        280 * time.Millisecond,
		PartialConfidence: 0.70,
	}
}

func bargePolicyFromDesk(d desk.Doc) bargePolicy {
	p := defaultBargePolicy()
	cx := d.CX
	if !cx.BargeIn {
		p.Allowed = false
	}
	p.ListenWhileSpeak = cx.ListenWhileSpeak
	if cx.WelcomeBargeAllowed != nil {
		p.WelcomeBargeAllowed = *cx.WelcomeBargeAllowed
	}
	if cx.MinBargeChars > 0 {
		p.MinBargeChars = cx.MinBargeChars
	}
	if cx.MinBargeMs > 0 {
		p.MinBargeMs = time.Duration(cx.MinBargeMs) * time.Millisecond
	}
	if cx.BargePartialConfidence > 0 {
		p.PartialConfidence = cx.BargePartialConfidence
	}
	return p
}

func (r *SessionRuntime) bargePolicy(sessionID string) bargePolicy {
	if ctrl, ok := r.DeskController(sessionID); ok {
		return bargePolicyFromDesk(ctrl.Engine().Doc())
	}
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
	if ctrl, ok := r.DeskController(sessionID); ok {
		if ms := ctrl.Engine().Doc().CX.RTPSettleMs; ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return defaultRTPSettleMs * time.Millisecond
}

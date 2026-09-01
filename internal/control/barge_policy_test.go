package control

import (
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/desk"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

func TestBargePolicyTextCommit(t *testing.T) {
	p := defaultBargePolicy()
	if !p.textCommit("hi") {
		t.Fatal("hi should commit")
	}
	if p.textCommit("a") {
		t.Fatal("single char should not commit")
	}
}

func TestBargePolicyPartialCommit(t *testing.T) {
	p := defaultBargePolicy()
	since := time.Now().Add(-300 * time.Millisecond)
	partial := port.ListenPartial{Text: "hello", Confidence: 0.9}
	if !p.partialCommit(partial, since) {
		t.Fatal("expected partial commit")
	}
	partial = port.ListenPartial{Text: "hello", Confidence: 0.5}
	if p.partialCommit(partial, since) {
		t.Fatal("low confidence should not commit")
	}
}

func TestBargePolicyFromDesk(t *testing.T) {
	allowed := true
	doc := desk.Doc{CX: desk.CXPolicy{
		BargeIn:                true,
		WelcomeBargeAllowed:    &allowed,
		MinBargeChars:          3,
		MinBargeMs:             400,
		BargePartialConfidence: 0.8,
	}}
	p := bargePolicyFromDesk(doc)
	if !p.WelcomeBargeAllowed {
		t.Fatal("welcome barge should be allowed")
	}
	if p.MinBargeChars != 3 {
		t.Fatalf("min chars = %d", p.MinBargeChars)
	}
	if !p.textCommit("abc") {
		t.Fatal("abc should commit with min 3")
	}
}

func TestShouldQueueFinalWelcomeBarge(t *testing.T) {
	m := newSessionMedia()
	m.mu.Lock()
	m.phase = MediaWelcoming
	m.mu.Unlock()
	if !m.shouldQueueFinal(false) {
		t.Fatal("should queue when welcome barge disabled")
	}
	if m.shouldQueueFinal(true) {
		t.Fatal("should not queue when welcome barge enabled")
	}
}

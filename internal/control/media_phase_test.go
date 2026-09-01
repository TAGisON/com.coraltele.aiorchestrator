package control

import (
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

func TestMediaPhaseEnterReadyOnSettle(t *testing.T) {
	m := newSessionMedia()
	m.onEdgeAttach()
	time.Sleep(510 * time.Millisecond)
	if !m.enterReadyFromSettle() {
		t.Fatal("expected ready after settle")
	}
	if m.view().Phase != MediaReady {
		t.Fatalf("phase=%q want ready", m.view().Phase)
	}
}

func TestMediaPhaseEnterReadyOnFirstUplink(t *testing.T) {
	m := newSessionMedia()
	m.onEdgeAttach()
	if !m.noteFirstUplink() {
		t.Fatal("expected ready on first uplink")
	}
	if m.view().Phase != MediaReady {
		t.Fatal("want ready")
	}
}

func TestMediaPhaseBeginWelcomeRequiresReady(t *testing.T) {
	m := newSessionMedia()
	m.onEdgeAttach()
	if err := m.beginWelcome(); err != ErrAnswerNotReady {
		t.Fatalf("got %v want not_ready", err)
	}
}

func TestMediaPhaseQueueFinalsUntilConversing(t *testing.T) {
	m := newSessionMedia()
	m.onEdgeAttach()
	m.noteFirstUplink()
	_ = m.beginWelcome()
	if !m.shouldQueueFinal(false) {
		t.Fatal("should queue during welcoming")
	}
	m.completeWelcome()
	if m.shouldQueueFinal(false) {
		t.Fatal("should not queue during conversing")
	}
	m.queueFinal(port.ListenFinal{Text: "hello"})
	if len(m.takeQueuedFinals()) != 1 {
		t.Fatal("expected queued final")
	}
}

func TestMediaPhaseWelcomeInProgress(t *testing.T) {
	m := newSessionMedia()
	m.onEdgeAttach()
	m.noteFirstUplink()
	_ = m.beginWelcome()
	view := m.view()
	if !view.WelcomeInProgress || view.WelcomeCompleted {
		t.Fatalf("view=%+v", view)
	}
}

func TestAnswerErrorNotReady(t *testing.T) {
	if ErrAnswerNotReady.HTTPStatus != 409 {
		t.Fatal("expected 409")
	}
}

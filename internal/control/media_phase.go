package control

import (
	"sync"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

// MediaPhase is the live-talk media sub-state (LIVE_TALK §16.1).
type MediaPhase string

const (
	MediaEstablishing MediaPhase = "establishing"
	MediaReady        MediaPhase = "ready"
	MediaWelcoming    MediaPhase = "welcoming"
	MediaConversing   MediaPhase = "conversing"
	MediaDraining     MediaPhase = "draining"
)

const defaultRTPSettleMs = 500

// SessionMediaView is exposed on GET /v1/sessions/{id}.
type SessionMediaView struct {
	Phase              MediaPhase
	WelcomeCompleted   bool
	WelcomeInProgress  bool
}

// AnswerError carries HTTP semantics for POST /answer phase gates.
type AnswerError struct {
	Code       string
	HTTPStatus int
	RetryAfter int
	Message    string
}

func (e *AnswerError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

var (
	ErrAnswerNotReady = &AnswerError{Code: "not_ready", HTTPStatus: 409, RetryAfter: 1, Message: "media not ready"}
	ErrWelcomeInProgress = &AnswerError{Code: "welcome_in_progress", HTTPStatus: 409, RetryAfter: 2, Message: "welcome in progress"}
	ErrWelcomeTimeout = &AnswerError{Code: "welcome_timeout", HTTPStatus: 504, Message: "welcome speak timed out"}
)

type sessionMedia struct {
	mu sync.Mutex

	phase            MediaPhase
	welcomeCompleted bool
	liveAttached     bool
	sinkAttached     bool
	firstUplink      bool
	attachAt         time.Time
	settleMs         int

	queuedFinals []port.ListenFinal
}

func newSessionMedia() *sessionMedia {
	return &sessionMedia{
		phase:    MediaEstablishing,
		settleMs: defaultRTPSettleMs,
	}
}

func (m *sessionMedia) view() SessionMediaView {
	m.mu.Lock()
	defer m.mu.Unlock()
	inProgress := m.phase == MediaWelcoming && !m.welcomeCompleted
	return SessionMediaView{
		Phase:             m.phase,
		WelcomeCompleted:  m.welcomeCompleted,
		WelcomeInProgress: inProgress,
	}
}

func (m *sessionMedia) onEdgeAttach() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.liveAttached = true
	m.sinkAttached = true
	if m.phase == "" {
		m.phase = MediaEstablishing
	}
	if m.attachAt.IsZero() {
		m.attachAt = time.Now()
	}
}

func (m *sessionMedia) noteFirstUplink() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.firstUplink {
		return false
	}
	m.firstUplink = true
	return m.tryEnterReadyLocked()
}

func (m *sessionMedia) settleElapsed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.attachAt.IsZero() {
		return false
	}
	return time.Since(m.attachAt) >= time.Duration(m.settleMs)*time.Millisecond
}

func (m *sessionMedia) tryEnterReadyLocked() bool {
	if m.phase != MediaEstablishing {
		return m.phase == MediaReady || m.phase == MediaWelcoming || m.phase == MediaConversing
	}
	if !m.sinkAttached {
		return false
	}
	if !m.firstUplink && time.Since(m.attachAt) < time.Duration(m.settleMs)*time.Millisecond {
		return false
	}
	m.phase = MediaReady
	return true
}

func (m *sessionMedia) enterReadyFromSettle() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.phase != MediaEstablishing || !m.sinkAttached {
		return false
	}
	if time.Since(m.attachAt) < time.Duration(m.settleMs)*time.Millisecond {
		return false
	}
	m.phase = MediaReady
	return true
}

func (m *sessionMedia) phaseAtLeast(want MediaPhase) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return mediaPhaseRank(m.phase) >= mediaPhaseRank(want)
}

func (m *sessionMedia) beginWelcome() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.welcomeCompleted {
		return nil
	}
	if m.phase == MediaWelcoming {
		return ErrWelcomeInProgress
	}
	if mediaPhaseRank(m.phase) < mediaPhaseRank(MediaReady) {
		return ErrAnswerNotReady
	}
	m.phase = MediaWelcoming
	return nil
}

func (m *sessionMedia) completeWelcome() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.welcomeCompleted = true
	m.phase = MediaConversing
}

func (m *sessionMedia) revertWelcome() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.welcomeCompleted {
		return
	}
	if m.phase == MediaWelcoming {
		m.phase = MediaReady
	}
}

func (m *sessionMedia) markDraining() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.phase = MediaDraining
}

func (m *sessionMedia) queueFinal(f port.ListenFinal) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queuedFinals = append(m.queuedFinals, f)
}

func (m *sessionMedia) shouldQueueFinals() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return mediaPhaseRank(m.phase) < mediaPhaseRank(MediaConversing)
}

func (m *sessionMedia) takeQueuedFinals() []port.ListenFinal {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.queuedFinals
	m.queuedFinals = nil
	return out
}

func mediaPhaseRank(p MediaPhase) int {
	switch p {
	case MediaEstablishing:
		return 1
	case MediaReady:
		return 2
	case MediaWelcoming:
		return 3
	case MediaConversing:
		return 4
	case MediaDraining:
		return 5
	default:
		return 0
	}
}

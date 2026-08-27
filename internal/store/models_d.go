package store

import (
	"time"
)

// KB document indexing states (CONTROL_API §5).
const (
	KBIndexing = "indexing"
	KBReady    = "ready"
	KBFailed   = "failed"
)

// KBDocument is an uploaded knowledge document.
type KBDocument struct {
	ID           string
	TenantID     string
	Collection   string
	URI          string
	Status       string
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// KBChunk is one indexed text chunk.
type KBChunk struct {
	ID         int64
	DocumentID string
	TenantID   string
	Collection string
	Ordinal    int
	Text       string
	SourceURI  string
	CreatedAt  time.Time
}

// Playback job states.
const (
	JobQueued    = "Queued"
	JobRunning   = "Running"
	JobCompleted = "Completed"
	JobFailed    = "Failed"
)

// PlaybackJob is a durable file-playback work item.
type PlaybackJob struct {
	ID             string
	TenantID       string
	FileURI        string
	ProfileID      string
	ProfileVersion int
	State          string
	LeaseOwner     string
	SessionID      string // optional runtime session created by worker
	ErrorMessage   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

package store

import (
	"time"
)

// KBChunk is an in-process knowledge snippet for lab ingest IndexLocal overlays.
// Durable kb_document / kb_chunk tables are retired (M-E / OD-08-4); DROP in M-G.
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

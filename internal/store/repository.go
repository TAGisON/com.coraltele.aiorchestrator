package store

import (
	"context"
	"encoding/json"
)

// Repository is the durability surface used by control HTTP.
// Implemented by *Store (Postgres) and *Memory (tests).
type Repository interface {
	Ping(ctx context.Context) error
	CreateProfile(ctx context.Context, p Profile) error
	GetProfile(ctx context.Context, id string) (Profile, error)
	PublishVersion(ctx context.Context, profileID string, doc json.RawMessage) (ProfileVersion, error)
	GetLatestVersion(ctx context.Context, profileID string) (ProfileVersion, error)
	GetVersion(ctx context.Context, profileID string, version int) (ProfileVersion, error)
	CreateSession(ctx context.Context, sess Session) error
	GetSession(ctx context.Context, id string) (Session, error)
	UpdateSessionState(ctx context.Context, id, state string) (Session, error)

	CreateKBDocument(ctx context.Context, doc KBDocument) error
	GetKBDocument(ctx context.Context, id string) (KBDocument, error)
	UpdateKBDocumentStatus(ctx context.Context, id, status, errMsg string) (KBDocument, error)
	ReplaceKBChunks(ctx context.Context, documentID string, chunks []KBChunk) error
	ListKBChunks(ctx context.Context, tenantID string, collections []string) ([]KBChunk, error)

	CreatePlaybackJob(ctx context.Context, job PlaybackJob) error
	GetPlaybackJob(ctx context.Context, id string) (PlaybackJob, error)
	UpdatePlaybackJob(ctx context.Context, job PlaybackJob) error
	LeaseNextPlaybackJob(ctx context.Context, owner string) (PlaybackJob, error)

	AppendAuditEvent(ctx context.Context, ev AuditEvent) (AuditEvent, error)
	ListAuditEvents(ctx context.Context, sessionID string) ([]AuditEvent, error)
	AppendAnalyticsEvent(ctx context.Context, ev AnalyticsEvent) (AnalyticsEvent, error)
	ListAnalyticsEvents(ctx context.Context, sessionID string) ([]AnalyticsEvent, error)

	CreatePostcallJob(ctx context.Context, job PostcallJob) error
	GetPostcallJob(ctx context.Context, id string) (PostcallJob, error)
	UpdatePostcallJob(ctx context.Context, job PostcallJob) error
	LeaseNextPostcallJob(ctx context.Context, owner string) (PostcallJob, error)
}

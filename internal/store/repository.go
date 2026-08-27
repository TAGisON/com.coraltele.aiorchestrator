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
}

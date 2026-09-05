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
	ListProfiles(ctx context.Context, limit int) ([]Profile, error)
	PublishVersion(ctx context.Context, profileID string, doc json.RawMessage) (ProfileVersion, error)
	GetLatestVersion(ctx context.Context, profileID string) (ProfileVersion, error)
	GetVersion(ctx context.Context, profileID string, version int) (ProfileVersion, error)
	CreateSession(ctx context.Context, sess Session) error
	GetSession(ctx context.Context, id string) (Session, error)
	ListSessions(ctx context.Context, limit int) ([]Session, error)
	UpdateSessionState(ctx context.Context, id, state string) (Session, error)
	UpdateSessionLanguages(ctx context.Context, id, detected, active string) (Session, error)
	// UpdateSessionRecordingRef stores the on-disk path of the call recording.
	UpdateSessionRecordingRef(ctx context.Context, id, ref string) (Session, error)
	MarkSessionRecordingStarted(ctx context.Context, id, ref string) (Session, error)
	MarkSessionRecordingStopped(ctx context.Context, id, reason string, nbytes *int64) (Session, error)
	ListOrphanRecordingSessions(ctx context.Context, limit int) ([]Session, error)

	GetTenantEngines(ctx context.Context, tenantID string) (TenantEngines, error)
	UpsertTenantEngines(ctx context.Context, te TenantEngines) (TenantEngines, error)

	GetGatewayCredential(ctx context.Context, tenantID, gatewayID string) (GatewayCredential, error)
	UpsertGatewayCredential(ctx context.Context, c GatewayCredential) (GatewayCredential, error)
	ListGatewayCredentials(ctx context.Context, tenantID string) ([]GatewayCredential, error)
	GetSystemSetting(ctx context.Context, tenantID, key string) (SystemSetting, error)
	UpsertSystemSetting(ctx context.Context, st SystemSetting) (SystemSetting, error)
	ListSystemSettings(ctx context.Context, tenantID string) ([]SystemSetting, error)

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

	AppendTranscriptTurn(ctx context.Context, turn TranscriptTurn) (TranscriptTurn, error)
	ListTranscriptTurns(ctx context.Context, sessionID string) ([]TranscriptTurn, error)
	UpsertSessionDisposition(ctx context.Context, d SessionDisposition) (SessionDisposition, error)
	GetSessionDisposition(ctx context.Context, sessionID string) (SessionDisposition, error)

	UpsertSessionAttributes(ctx context.Context, sessionID string, attrs []SessionAttribute) error
	ListSessionAttributes(ctx context.Context, sessionID string) ([]SessionAttribute, error)

	UpsertCallerPreference(ctx context.Context, p CallerPreference) (CallerPreference, error)
	GetCallerPreference(ctx context.Context, tenantID, ani string) (CallerPreference, error)

	CreateFlow(ctx context.Context, f Flow, draftDoc json.RawMessage) (Flow, error)
	GetFlow(ctx context.Context, id string) (Flow, error)
	ListFlows(ctx context.Context, tenantID string, limit int) ([]Flow, error)
	UpsertFlowDraft(ctx context.Context, flowID string, doc json.RawMessage) (FlowDraft, error)
	GetFlowDraft(ctx context.Context, flowID string) (FlowDraft, error)
	PublishFlowVersion(ctx context.Context, flowID string, doc json.RawMessage, contentHash, publishedBy string) (FlowVersion, error)
	GetFlowVersion(ctx context.Context, flowID string, version int) (FlowVersion, error)
	GetLatestFlowVersion(ctx context.Context, flowID string) (FlowVersion, error)

	UpsertBinding(ctx context.Context, b Binding) (Binding, error)
	GetBinding(ctx context.Context, id string) (Binding, error)
	ListBindings(ctx context.Context, tenantID, kind string, limit int) ([]Binding, error)

	CountActiveSessions(ctx context.Context, tenantID string) (int, error)
	PurgeSessionData(ctx context.Context, sessionID string) error
}

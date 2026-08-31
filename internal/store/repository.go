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

	GetTenantEngines(ctx context.Context, tenantID string) (TenantEngines, error)
	UpsertTenantEngines(ctx context.Context, te TenantEngines) (TenantEngines, error)

	GetGatewayCredential(ctx context.Context, tenantID, gatewayID string) (GatewayCredential, error)
	UpsertGatewayCredential(ctx context.Context, c GatewayCredential) (GatewayCredential, error)
	ListGatewayCredentials(ctx context.Context, tenantID string) ([]GatewayCredential, error)
	GetSystemSetting(ctx context.Context, tenantID, key string) (SystemSetting, error)
	UpsertSystemSetting(ctx context.Context, st SystemSetting) (SystemSetting, error)
	ListSystemSettings(ctx context.Context, tenantID string) ([]SystemSetting, error)

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

	AppendTranscriptTurn(ctx context.Context, turn TranscriptTurn) (TranscriptTurn, error)
	ListTranscriptTurns(ctx context.Context, sessionID string) ([]TranscriptTurn, error)
	UpsertSessionDisposition(ctx context.Context, d SessionDisposition) (SessionDisposition, error)
	GetSessionDisposition(ctx context.Context, sessionID string) (SessionDisposition, error)

	// Contact Desk vertical (CONTACT_DESK_POC_SOLUTION.md §4.4).
	UpsertDesk(ctx context.Context, d Desk) (Desk, error)
	GetDesk(ctx context.Context, id string) (Desk, error)
	ListDesks(ctx context.Context, tenantID string) ([]Desk, error)
	SaveDeskDraft(ctx context.Context, deskID string, doc json.RawMessage) (DeskDraft, error)
	GetDeskDraft(ctx context.Context, deskID string) (DeskDraft, error)
	PublishDeskVersion(ctx context.Context, v DeskVersion) (DeskVersion, error)
	GetDeskVersion(ctx context.Context, deskID string, version int) (DeskVersion, error)
	ListDeskVersions(ctx context.Context, deskID string) ([]DeskVersion, error)

	UpsertSessionAttributes(ctx context.Context, sessionID string, attrs []SessionAttribute) error
	ListSessionAttributes(ctx context.Context, sessionID string) ([]SessionAttribute, error)
	AppendSkillInvocation(ctx context.Context, inv SkillInvocation) (SkillInvocation, error)
	ListSkillInvocations(ctx context.Context, sessionID string) ([]SkillInvocation, error)

	AppendPIIAccess(ctx context.Context, ev PIIAccess) (PIIAccess, error)
	ListPIIAccess(ctx context.Context, sessionID string, limit int) ([]PIIAccess, error)
	CreateErasureRequest(ctx context.Context, r ErasureRequest) (ErasureRequest, error)
	ListErasureRequests(ctx context.Context, tenantID string) ([]ErasureRequest, error)
	CompleteErasureRequest(ctx context.Context, id string) (ErasureRequest, error)
	UpsertConsent(ctx context.Context, c ConsentRecord) (ConsentRecord, error)
	GetConsent(ctx context.Context, tenantID, phone string) (ConsentRecord, error)

	CountActiveSessions(ctx context.Context, tenantID string) (int, error)
	PurgeSessionData(ctx context.Context, sessionID string) error
}

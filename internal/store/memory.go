package store

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// Memory is an in-process Repository for tests without Postgres.
type Memory struct {
	mu            sync.Mutex
	profiles      map[string]Profile
	versions      map[string][]ProfileVersion
	sessions      map[string]Session
	engines       map[string]TenantEngines
	creds         map[string]GatewayCredential // tenantID\x00gatewayID
	settings      map[string]SystemSetting     // tenantID\x00key
	kbDocs        map[string]KBDocument
	kbChunks      map[string][]KBChunk // document_id → chunks
	playJobs      map[string]PlaybackJob
	auditSeq      int64
	audits        []AuditEvent
	analyticsSeq  int64
	analytics     []AnalyticsEvent
	postJobs      map[string]PostcallJob
	transcriptSeq int64
	transcripts   map[string][]TranscriptTurn // session_id → ordered turns
	dispositions  map[string]SessionDisposition
	aux           *sessionAuxMemory
	healthy       bool
}

func NewMemory() *Memory {
	return &Memory{
		profiles:     make(map[string]Profile),
		versions:     make(map[string][]ProfileVersion),
		sessions:     make(map[string]Session),
		engines:      make(map[string]TenantEngines),
		creds:        make(map[string]GatewayCredential),
		settings:     make(map[string]SystemSetting),
		kbDocs:       make(map[string]KBDocument),
		kbChunks:     make(map[string][]KBChunk),
		playJobs:     make(map[string]PlaybackJob),
		postJobs:     make(map[string]PostcallJob),
		transcripts:  make(map[string][]TranscriptTurn),
		dispositions: make(map[string]SessionDisposition),
		healthy:      true,
	}
}

func (m *Memory) SetHealthy(ok bool) { m.mu.Lock(); m.healthy = ok; m.mu.Unlock() }

func (m *Memory) Ping(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.healthy {
		return errors.New("db unavailable")
	}
	return nil
}

func (m *Memory) CreateProfile(ctx context.Context, p Profile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.profiles[p.ID]; ok {
		return ErrConflict
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	m.profiles[p.ID] = p
	return nil
}

func (m *Memory) GetProfile(ctx context.Context, id string) (Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.profiles[id]
	if !ok {
		return Profile{}, ErrNotFound
	}
	return p, nil
}

func (m *Memory) ListProfiles(ctx context.Context, limit int) ([]Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]Profile, 0, len(m.profiles))
	for _, p := range m.profiles {
		out = append(out, p)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *Memory) PublishVersion(ctx context.Context, profileID string, doc json.RawMessage) (ProfileVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.profiles[profileID]
	if !ok {
		return ProfileVersion{}, ErrNotFound
	}
	next := len(m.versions[profileID]) + 1
	pv := ProfileVersion{
		ProfileID:   profileID,
		Version:     next,
		Document:    append(json.RawMessage(nil), doc...),
		PublishedAt: time.Now().UTC(),
	}
	m.versions[profileID] = append(m.versions[profileID], pv)
	p.UpdatedAt = time.Now().UTC()
	m.profiles[profileID] = p
	return pv, nil
}

func (m *Memory) GetLatestVersion(ctx context.Context, profileID string) (ProfileVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	vs := m.versions[profileID]
	if len(vs) == 0 {
		return ProfileVersion{}, ErrNotFound
	}
	return vs[len(vs)-1], nil
}

func (m *Memory) GetVersion(ctx context.Context, profileID string, version int) (ProfileVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.versions[profileID] {
		if v.Version == version {
			return v, nil
		}
	}
	return ProfileVersion{}, ErrNotFound
}

func (m *Memory) CreateSession(ctx context.Context, sess Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[sess.ID]; ok {
		return ErrConflict
	}
	now := time.Now().UTC()
	sess.CreatedAt = now
	sess.UpdatedAt = now
	m.sessions[sess.ID] = sess
	return nil
}

func (m *Memory) GetSession(ctx context.Context, id string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	return s, nil
}

func (m *Memory) ListSessions(ctx context.Context, limit int) ([]Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *Memory) UpdateSessionState(ctx context.Context, id, state string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	s.State = state
	s.UpdatedAt = time.Now().UTC()
	m.sessions[id] = s
	return s, nil
}

func (m *Memory) UpdateSessionRecordingRef(ctx context.Context, id, ref string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	s.RecordingRef = ref
	s.UpdatedAt = time.Now().UTC()
	m.sessions[id] = s
	return s, nil
}

func (m *Memory) UpdateSessionLanguages(ctx context.Context, id, detected, active string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	s.DetectedLanguage = detected
	s.ActiveLanguage = active
	s.UpdatedAt = time.Now().UTC()
	m.sessions[id] = s
	return s, nil
}

func (m *Memory) GetTenantEngines(ctx context.Context, tenantID string) (TenantEngines, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	te, ok := m.engines[tenantID]
	if !ok {
		return TenantEngines{}, ErrNotFound
	}
	return te, nil
}

func (m *Memory) UpsertTenantEngines(ctx context.Context, te TenantEngines) (TenantEngines, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	te.UpdatedAt = time.Now().UTC()
	m.engines[te.TenantID] = te
	return te, nil
}

func credKey(tenantID, gatewayID string) string { return tenantID + "\x00" + gatewayID }
func settingKey(tenantID, key string) string    { return tenantID + "\x00" + key }

func (m *Memory) GetGatewayCredential(ctx context.Context, tenantID, gatewayID string) (GatewayCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.creds[credKey(tenantID, gatewayID)]
	if !ok {
		return GatewayCredential{}, ErrNotFound
	}
	return c, nil
}

func (m *Memory) UpsertGatewayCredential(ctx context.Context, c GatewayCredential) (GatewayCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(c.Extra) == 0 {
		c.Extra = json.RawMessage(`{}`)
	}
	c.UpdatedAt = time.Now().UTC()
	m.creds[credKey(c.TenantID, c.GatewayID)] = c
	return c, nil
}

func (m *Memory) ListGatewayCredentials(ctx context.Context, tenantID string) ([]GatewayCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []GatewayCredential
	for _, c := range m.creds {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *Memory) GetSystemSetting(ctx context.Context, tenantID, key string) (SystemSetting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.settings[settingKey(tenantID, key)]
	if !ok {
		return SystemSetting{}, ErrNotFound
	}
	return st, nil
}

func (m *Memory) UpsertSystemSetting(ctx context.Context, st SystemSetting) (SystemSetting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st.UpdatedAt = time.Now().UTC()
	m.settings[settingKey(st.TenantID, st.Key)] = st
	return st, nil
}

func (m *Memory) ListSystemSettings(ctx context.Context, tenantID string) ([]SystemSetting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []SystemSetting
	for _, st := range m.settings {
		if st.TenantID == tenantID {
			out = append(out, st)
		}
	}
	return out, nil
}

func (m *Memory) CreateKBDocument(ctx context.Context, doc KBDocument) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.kbDocs[doc.ID]; ok {
		return ErrConflict
	}
	now := time.Now().UTC()
	doc.CreatedAt = now
	doc.UpdatedAt = now
	m.kbDocs[doc.ID] = doc
	return nil
}

func (m *Memory) GetKBDocument(ctx context.Context, id string) (KBDocument, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.kbDocs[id]
	if !ok {
		return KBDocument{}, ErrNotFound
	}
	return d, nil
}

func (m *Memory) UpdateKBDocumentStatus(ctx context.Context, id, status, errMsg string) (KBDocument, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.kbDocs[id]
	if !ok {
		return KBDocument{}, ErrNotFound
	}
	d.Status = status
	d.ErrorMessage = errMsg
	d.UpdatedAt = time.Now().UTC()
	m.kbDocs[id] = d
	return d, nil
}

func (m *Memory) ReplaceKBChunks(ctx context.Context, documentID string, chunks []KBChunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.kbDocs[documentID]; !ok {
		return ErrNotFound
	}
	cp := make([]KBChunk, len(chunks))
	copy(cp, chunks)
	m.kbChunks[documentID] = cp
	return nil
}

func (m *Memory) ListKBChunks(ctx context.Context, tenantID string, collections []string) ([]KBChunk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	allow := map[string]struct{}{}
	for _, c := range collections {
		allow[c] = struct{}{}
	}
	var out []KBChunk
	for _, chunks := range m.kbChunks {
		for _, ch := range chunks {
			if tenantID != "" && ch.TenantID != "" && ch.TenantID != tenantID {
				continue
			}
			if len(allow) > 0 {
				if _, ok := allow[ch.Collection]; !ok {
					continue
				}
			}
			out = append(out, ch)
		}
	}
	return out, nil
}

func (m *Memory) CreatePlaybackJob(ctx context.Context, job PlaybackJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.playJobs[job.ID]; ok {
		return ErrConflict
	}
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now
	m.playJobs[job.ID] = job
	return nil
}

func (m *Memory) GetPlaybackJob(ctx context.Context, id string) (PlaybackJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.playJobs[id]
	if !ok {
		return PlaybackJob{}, ErrNotFound
	}
	return j, nil
}

func (m *Memory) UpdatePlaybackJob(ctx context.Context, job PlaybackJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.playJobs[job.ID]; !ok {
		return ErrNotFound
	}
	job.UpdatedAt = time.Now().UTC()
	m.playJobs[job.ID] = job
	return nil
}

func (m *Memory) LeaseNextPlaybackJob(ctx context.Context, owner string) (PlaybackJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, j := range m.playJobs {
		if j.State == JobQueued {
			j.State = JobRunning
			j.LeaseOwner = owner
			j.UpdatedAt = time.Now().UTC()
			m.playJobs[id] = j
			return j, nil
		}
	}
	return PlaybackJob{}, ErrNotFound
}

func (m *Memory) AppendAuditEvent(ctx context.Context, ev AuditEvent) (AuditEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditSeq++
	ev.ID = m.auditSeq
	ev.CreatedAt = time.Now().UTC()
	if len(ev.Payload) > 0 {
		ev.Payload = append(json.RawMessage(nil), ev.Payload...)
	}
	m.audits = append(m.audits, ev)
	return ev, nil
}

func (m *Memory) ListAuditEvents(ctx context.Context, sessionID string) ([]AuditEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []AuditEvent
	for _, ev := range m.audits {
		if ev.SessionID == sessionID {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (m *Memory) AppendAnalyticsEvent(ctx context.Context, ev AnalyticsEvent) (AnalyticsEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.analyticsSeq++
	ev.ID = m.analyticsSeq
	if ev.Value == 0 {
		ev.Value = 1
	}
	ev.CreatedAt = time.Now().UTC()
	if len(ev.Dimensions) > 0 {
		ev.Dimensions = append(json.RawMessage(nil), ev.Dimensions...)
	}
	m.analytics = append(m.analytics, ev)
	return ev, nil
}

func (m *Memory) ListAnalyticsEvents(ctx context.Context, sessionID string) ([]AnalyticsEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []AnalyticsEvent
	for _, ev := range m.analytics {
		if ev.SessionID == sessionID {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (m *Memory) CreatePostcallJob(ctx context.Context, job PostcallJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.postJobs {
		if j.SessionID == job.SessionID && (j.State == JobQueued || j.State == JobRunning) {
			return ErrConflict
		}
	}
	if _, ok := m.postJobs[job.ID]; ok {
		return ErrConflict
	}
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now
	m.postJobs[job.ID] = job
	return nil
}

func (m *Memory) GetPostcallJob(ctx context.Context, id string) (PostcallJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.postJobs[id]
	if !ok {
		return PostcallJob{}, ErrNotFound
	}
	return j, nil
}

func (m *Memory) UpdatePostcallJob(ctx context.Context, job PostcallJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.postJobs[job.ID]; !ok {
		return ErrNotFound
	}
	job.UpdatedAt = time.Now().UTC()
	m.postJobs[job.ID] = job
	return nil
}

func (m *Memory) LeaseNextPostcallJob(ctx context.Context, owner string) (PostcallJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, j := range m.postJobs {
		if j.State == JobQueued {
			j.State = JobRunning
			j.LeaseOwner = owner
			j.UpdatedAt = time.Now().UTC()
			m.postJobs[id] = j
			return j, nil
		}
	}
	return PostcallJob{}, ErrNotFound
}

func (m *Memory) AppendTranscriptTurn(ctx context.Context, turn TranscriptTurn) (TranscriptTurn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.transcripts[turn.SessionID]
	turn.Seq = len(list) + 1
	m.transcriptSeq++
	turn.ID = m.transcriptSeq
	turn.CreatedAt = time.Now().UTC()
	m.transcripts[turn.SessionID] = append(list, turn)
	return turn, nil
}

func (m *Memory) ListTranscriptTurns(ctx context.Context, sessionID string) ([]TranscriptTurn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.transcripts[sessionID]
	out := make([]TranscriptTurn, len(list))
	copy(out, list)
	return out, nil
}

func (m *Memory) UpsertSessionDisposition(ctx context.Context, d SessionDisposition) (SessionDisposition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d.Source == "" {
		d.Source = "postcall_worker"
	}
	prev, ok := m.dispositions[d.SessionID]
	if ok && d.Final == "" && prev.Final != "" {
		d.Final = prev.Final
	}
	d.UpdatedAt = time.Now().UTC()
	m.dispositions[d.SessionID] = d
	return d, nil
}

func (m *Memory) GetSessionDisposition(ctx context.Context, sessionID string) (SessionDisposition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.dispositions[sessionID]
	if !ok {
		return SessionDisposition{}, ErrNotFound
	}
	return d, nil
}

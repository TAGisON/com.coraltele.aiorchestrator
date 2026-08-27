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
	mu        sync.Mutex
	profiles  map[string]Profile
	versions  map[string][]ProfileVersion
	sessions  map[string]Session
	kbDocs    map[string]KBDocument
	kbChunks  map[string][]KBChunk // document_id → chunks
	playJobs  map[string]PlaybackJob
	healthy   bool
}

func NewMemory() *Memory {
	return &Memory{
		profiles: make(map[string]Profile),
		versions: make(map[string][]ProfileVersion),
		sessions: make(map[string]Session),
		kbDocs:   make(map[string]KBDocument),
		kbChunks: make(map[string][]KBChunk),
		playJobs: make(map[string]PlaybackJob),
		healthy:  true,
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

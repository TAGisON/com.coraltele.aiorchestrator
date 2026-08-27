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
	mu       sync.Mutex
	profiles map[string]Profile
	versions map[string][]ProfileVersion
	sessions map[string]Session
	healthy  bool
}

func NewMemory() *Memory {
	return &Memory{
		profiles: make(map[string]Profile),
		versions: make(map[string][]ProfileVersion),
		sessions: make(map[string]Session),
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

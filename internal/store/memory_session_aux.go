package store

import (
	"context"
	"sort"
	"time"
)

// sessionAuxMemory holds session-adjacent durable maps (attrs, prefs).
// Not former desk registry/draft/version storage (removed in P1.8).
// Skill/compliance ledgers removed in M-E (DROP M-H).
type sessionAuxMemory struct {
	attrs map[string]map[string]SessionAttribute
	prefs map[string]CallerPreference
}

func (m *Memory) sessionAux() *sessionAuxMemory {
	if m.aux == nil {
		m.aux = &sessionAuxMemory{
			attrs: map[string]map[string]SessionAttribute{},
			prefs: map[string]CallerPreference{},
		}
	}
	if m.aux.prefs == nil {
		m.aux.prefs = map[string]CallerPreference{}
	}
	return m.aux
}

func (m *Memory) UpsertSessionAttributes(ctx context.Context, sessionID string, attrs []SessionAttribute) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.sessionAux()
	cur, ok := st.attrs[sessionID]
	if !ok {
		cur = map[string]SessionAttribute{}
		st.attrs[sessionID] = cur
	}
	now := time.Now().UTC()
	for _, a := range attrs {
		a.SessionID = sessionID
		a.UpdatedAt = now
		cur[a.Key] = a
	}
	return nil
}

func (m *Memory) ListSessionAttributes(ctx context.Context, sessionID string) ([]SessionAttribute, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := m.sessionAux().attrs[sessionID]
	out := make([]SessionAttribute, 0, len(cur))
	for _, a := range cur {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (m *Memory) UpsertCallerPreference(ctx context.Context, p CallerPreference) (CallerPreference, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p.UpdatedAt = time.Now().UTC()
	m.sessionAux().prefs[p.TenantID+"\x00"+p.ANI] = p
	return p, nil
}

func (m *Memory) GetCallerPreference(ctx context.Context, tenantID, ani string) (CallerPreference, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.sessionAux().prefs[tenantID+"\x00"+ani]
	if !ok {
		return CallerPreference{}, ErrNotFound
	}
	return p, nil
}

func (m *Memory) CountActiveSessions(ctx context.Context, tenantID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, s := range m.sessions {
		if s.TenantID != tenantID {
			continue
		}
		switch s.State {
		case StateCreated, StateAttached, StateRunning, StateDraining:
			n++
		}
	}
	return n, nil
}

func (m *Memory) PurgeSessionData(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.sessionAux()
	delete(st.attrs, sessionID)
	delete(m.transcripts, sessionID)
	return nil
}

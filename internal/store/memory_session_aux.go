package store

import (
	"context"
	"sort"
	"time"
)

// sessionAuxMemory holds session-adjacent durable maps (attrs, skills, prefs).
// Not Contact Desk registry/draft/version storage (removed in P1.8).
type sessionAuxMemory struct {
	attrs    map[string]map[string]SessionAttribute
	skillSeq int64
	skills   []SkillInvocation
	piiSeq   int64
	pii      []PIIAccess
	erasures map[string]ErasureRequest
	consents map[string]ConsentRecord
	prefs    map[string]CallerPreference
}

func (m *Memory) sessionAux() *sessionAuxMemory {
	if m.aux == nil {
		m.aux = &sessionAuxMemory{
			attrs:    map[string]map[string]SessionAttribute{},
			erasures: map[string]ErasureRequest{},
			consents: map[string]ConsentRecord{},
			prefs:    map[string]CallerPreference{},
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

func (m *Memory) AppendSkillInvocation(ctx context.Context, inv SkillInvocation) (SkillInvocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.sessionAux()
	if inv.IdempotencyKey != "" {
		for _, e := range st.skills {
			if e.IdempotencyKey == inv.IdempotencyKey {
				return e, nil
			}
		}
	}
	st.skillSeq++
	inv.ID = st.skillSeq
	inv.CreatedAt = time.Now().UTC()
	st.skills = append(st.skills, inv)
	return inv, nil
}

func (m *Memory) ListSkillInvocations(ctx context.Context, sessionID string) ([]SkillInvocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []SkillInvocation
	for _, e := range m.sessionAux().skills {
		if e.SessionID == sessionID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (m *Memory) AppendPIIAccess(ctx context.Context, ev PIIAccess) (PIIAccess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.sessionAux()
	st.piiSeq++
	ev.ID = st.piiSeq
	ev.CreatedAt = time.Now().UTC()
	st.pii = append(st.pii, ev)
	return ev, nil
}

func (m *Memory) ListPIIAccess(ctx context.Context, sessionID string, limit int) ([]PIIAccess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	var out []PIIAccess
	list := m.sessionAux().pii
	for i := len(list) - 1; i >= 0 && len(out) < limit; i-- {
		if sessionID == "" || list[i].SessionID == sessionID {
			out = append(out, list[i])
		}
	}
	return out, nil
}

func (m *Memory) CreateErasureRequest(ctx context.Context, r ErasureRequest) (ErasureRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.RequestedAt = time.Now().UTC()
	if r.Status == "" {
		r.Status = "queued"
	}
	m.sessionAux().erasures[r.ID] = r
	return r, nil
}

func (m *Memory) ListErasureRequests(ctx context.Context, tenantID string) ([]ErasureRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ErasureRequest
	for _, r := range m.sessionAux().erasures {
		if tenantID == "" || r.TenantID == tenantID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RequestedAt.After(out[j].RequestedAt) })
	return out, nil
}

func (m *Memory) CompleteErasureRequest(ctx context.Context, id string) (ErasureRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.sessionAux()
	r, ok := st.erasures[id]
	if !ok {
		return ErasureRequest{}, ErrNotFound
	}
	now := time.Now().UTC()
	r.Status = "completed"
	r.CompletedAt = &now
	st.erasures[id] = r
	return r, nil
}

func (m *Memory) UpsertConsent(ctx context.Context, c ConsentRecord) (ConsentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c.UpdatedAt = time.Now().UTC()
	m.sessionAux().consents[c.TenantID+"\x00"+c.Phone] = c
	return c, nil
}

func (m *Memory) GetConsent(ctx context.Context, tenantID, phone string) (ConsentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.sessionAux().consents[tenantID+"\x00"+phone]
	if !ok {
		return ConsentRecord{}, ErrNotFound
	}
	return c, nil
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
	kept := st.skills[:0]
	for _, e := range st.skills {
		if e.SessionID != sessionID {
			kept = append(kept, e)
		}
	}
	st.skills = kept
	return nil
}

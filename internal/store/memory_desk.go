package store

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type deskMemory struct {
	desks     map[string]Desk
	drafts    map[string]DeskDraft
	versions  map[string][]DeskVersion
	attrs     map[string]map[string]SessionAttribute
	skillSeq  int64
	skills    []SkillInvocation
	piiSeq    int64
	pii       []PIIAccess
	erasures  map[string]ErasureRequest
	consents  map[string]ConsentRecord
	prefs     map[string]CallerPreference
}

func (m *Memory) deskState() *deskMemory {
	if m.desk == nil {
		m.desk = &deskMemory{
			desks:    map[string]Desk{},
			drafts:   map[string]DeskDraft{},
			versions: map[string][]DeskVersion{},
			attrs:    map[string]map[string]SessionAttribute{},
			erasures: map[string]ErasureRequest{},
			consents: map[string]ConsentRecord{},
			prefs:    map[string]CallerPreference{},
		}
	}
	if m.desk.prefs == nil {
		m.desk.prefs = map[string]CallerPreference{}
	}
	return m.desk
}

func (m *Memory) UpsertDesk(ctx context.Context, d Desk) (Desk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.deskState()
	now := time.Now().UTC()
	prev, exists := st.desks[d.ID]
	if exists {
		d.CreatedAt = prev.CreatedAt
		if d.CurrentVersion < prev.CurrentVersion {
			d.CurrentVersion = prev.CurrentVersion
		}
		if d.ProfileID == "" {
			d.ProfileID = prev.ProfileID
		}
	} else {
		d.CreatedAt = now
	}
	d.UpdatedAt = now
	st.desks[d.ID] = d
	return d, nil
}

func (m *Memory) GetDesk(ctx context.Context, id string) (Desk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deskState().desks[id]
	if !ok {
		return Desk{}, ErrNotFound
	}
	return d, nil
}

func (m *Memory) ListDesks(ctx context.Context, tenantID string) ([]Desk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Desk
	for _, d := range m.deskState().desks {
		if tenantID == "" || d.TenantID == tenantID {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *Memory) SaveDeskDraft(ctx context.Context, deskID string, doc json.RawMessage) (DeskDraft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.deskState()
	if _, ok := st.desks[deskID]; !ok {
		return DeskDraft{}, ErrNotFound
	}
	d := DeskDraft{DeskID: deskID, Doc: append(json.RawMessage(nil), doc...), UpdatedAt: time.Now().UTC()}
	st.drafts[deskID] = d
	return d, nil
}

func (m *Memory) GetDeskDraft(ctx context.Context, deskID string) (DeskDraft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deskState().drafts[deskID]
	if !ok {
		return DeskDraft{}, ErrNotFound
	}
	return d, nil
}

func (m *Memory) PublishDeskVersion(ctx context.Context, v DeskVersion) (DeskVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.deskState()
	d, ok := st.desks[v.DeskID]
	if !ok {
		return DeskVersion{}, ErrNotFound
	}
	v.Version = len(st.versions[v.DeskID]) + 1
	v.Doc = append(json.RawMessage(nil), v.Doc...)
	v.PublishedAt = time.Now().UTC()
	st.versions[v.DeskID] = append(st.versions[v.DeskID], v)
	d.CurrentVersion = v.Version
	d.Status = DeskStatusPublished
	if v.ProfileID != "" {
		d.ProfileID = v.ProfileID
	}
	d.UpdatedAt = v.PublishedAt
	st.desks[v.DeskID] = d
	return v, nil
}

func (m *Memory) GetDeskVersion(ctx context.Context, deskID string, version int) (DeskVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.deskState().versions[deskID] {
		if v.Version == version {
			return v, nil
		}
	}
	return DeskVersion{}, ErrNotFound
}

func (m *Memory) ListDeskVersions(ctx context.Context, deskID string) ([]DeskVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.deskState().versions[deskID]
	out := make([]DeskVersion, len(list))
	copy(out, list)
	sort.Slice(out, func(i, j int) bool { return out[i].Version > out[j].Version })
	return out, nil
}

func (m *Memory) UpsertSessionAttributes(ctx context.Context, sessionID string, attrs []SessionAttribute) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.deskState()
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
	cur := m.deskState().attrs[sessionID]
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
	st := m.deskState()
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
	for _, e := range m.deskState().skills {
		if e.SessionID == sessionID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (m *Memory) AppendPIIAccess(ctx context.Context, ev PIIAccess) (PIIAccess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.deskState()
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
	list := m.deskState().pii
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
	m.deskState().erasures[r.ID] = r
	return r, nil
}

func (m *Memory) ListErasureRequests(ctx context.Context, tenantID string) ([]ErasureRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ErasureRequest
	for _, r := range m.deskState().erasures {
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
	st := m.deskState()
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
	m.deskState().consents[c.TenantID+"\x00"+c.Phone] = c
	return c, nil
}

func (m *Memory) GetConsent(ctx context.Context, tenantID, phone string) (ConsentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.deskState().consents[tenantID+"\x00"+phone]
	if !ok {
		return ConsentRecord{}, ErrNotFound
	}
	return c, nil
}

func (m *Memory) UpsertCallerPreference(ctx context.Context, p CallerPreference) (CallerPreference, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p.UpdatedAt = time.Now().UTC()
	m.deskState().prefs[p.TenantID+"\x00"+p.ANI] = p
	return p, nil
}

func (m *Memory) GetCallerPreference(ctx context.Context, tenantID, ani string) (CallerPreference, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.deskState().prefs[tenantID+"\x00"+ani]
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
	st := m.deskState()
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

var _ = strings.TrimSpace

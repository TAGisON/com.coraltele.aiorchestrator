package store

import (
	"context"
	"encoding/json"
	"time"
)

func (m *Memory) ensureFlowMaps() {
	if m.flows == nil {
		m.flows = make(map[string]Flow)
	}
	if m.flowDrafts == nil {
		m.flowDrafts = make(map[string]FlowDraft)
	}
	if m.flowVersions == nil {
		m.flowVersions = make(map[string][]FlowVersion)
	}
	if m.bindings == nil {
		m.bindings = make(map[string]Binding)
	}
}

func (m *Memory) CreateFlow(ctx context.Context, f Flow, draftDoc json.RawMessage) (Flow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureFlowMaps()
	if _, ok := m.flows[f.ID]; ok {
		return Flow{}, ErrConflict
	}
	if draftDoc == nil {
		draftDoc = json.RawMessage(`{}`)
	}
	if f.Direction == "" {
		f.Direction = FlowDirectionInbound
	}
	if f.Status == "" {
		f.Status = FlowStatusDraft
	}
	now := time.Now().UTC()
	f.CurrentVersion = 0
	f.CreatedAt = now
	f.UpdatedAt = now
	m.flows[f.ID] = f
	m.flowDrafts[f.ID] = FlowDraft{FlowID: f.ID, Doc: append(json.RawMessage(nil), draftDoc...), UpdatedAt: now}
	return f, nil
}

func (m *Memory) GetFlow(ctx context.Context, id string) (Flow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureFlowMaps()
	f, ok := m.flows[id]
	if !ok {
		return Flow{}, ErrNotFound
	}
	return f, nil
}

func (m *Memory) ListFlows(ctx context.Context, tenantID string, limit int) ([]Flow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureFlowMaps()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]Flow, 0)
	for _, f := range m.flows {
		if f.TenantID == tenantID {
			out = append(out, f)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *Memory) UpsertFlowDraft(ctx context.Context, flowID string, doc json.RawMessage) (FlowDraft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureFlowMaps()
	if _, ok := m.flows[flowID]; !ok {
		return FlowDraft{}, ErrNotFound
	}
	if doc == nil {
		doc = json.RawMessage(`{}`)
	}
	d := FlowDraft{FlowID: flowID, Doc: append(json.RawMessage(nil), doc...), UpdatedAt: time.Now().UTC()}
	m.flowDrafts[flowID] = d
	return d, nil
}

func (m *Memory) GetFlowDraft(ctx context.Context, flowID string) (FlowDraft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureFlowMaps()
	d, ok := m.flowDrafts[flowID]
	if !ok {
		return FlowDraft{}, ErrNotFound
	}
	return d, nil
}

func (m *Memory) PublishFlowVersion(ctx context.Context, flowID string, doc json.RawMessage, contentHash, publishedBy string) (FlowVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureFlowMaps()
	f, ok := m.flows[flowID]
	if !ok {
		return FlowVersion{}, ErrNotFound
	}
	if doc == nil {
		doc = json.RawMessage(`{}`)
	}
	next := f.CurrentVersion + 1
	fv := FlowVersion{
		FlowID: flowID, Version: next, Doc: append(json.RawMessage(nil), doc...),
		ContentHash: contentHash, PublishedBy: publishedBy, PublishedAt: time.Now().UTC(),
	}
	m.flowVersions[flowID] = append(m.flowVersions[flowID], fv)
	f.CurrentVersion = next
	f.Status = FlowStatusPublished
	f.UpdatedAt = time.Now().UTC()
	m.flows[flowID] = f
	m.flowDrafts[flowID] = FlowDraft{FlowID: flowID, Doc: append(json.RawMessage(nil), doc...), UpdatedAt: f.UpdatedAt}
	return fv, nil
}

func (m *Memory) GetFlowVersion(ctx context.Context, flowID string, version int) (FlowVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureFlowMaps()
	for _, fv := range m.flowVersions[flowID] {
		if fv.Version == version {
			return fv, nil
		}
	}
	return FlowVersion{}, ErrNotFound
}

func (m *Memory) GetLatestFlowVersion(ctx context.Context, flowID string) (FlowVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureFlowMaps()
	vers := m.flowVersions[flowID]
	if len(vers) == 0 {
		return FlowVersion{}, ErrNotFound
	}
	return vers[len(vers)-1], nil
}

func (m *Memory) UpsertBinding(ctx context.Context, b Binding) (Binding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureFlowMaps()
	if b.Config == nil {
		b.Config = json.RawMessage(`{}`)
	}
	if b.Status == "" {
		b.Status = BindingStatusActive
	}
	now := time.Now().UTC()
	if prev, ok := m.bindings[b.ID]; ok {
		b.CreatedAt = prev.CreatedAt
	} else {
		b.CreatedAt = now
	}
	b.UpdatedAt = now
	m.bindings[b.ID] = b
	return b, nil
}

func (m *Memory) GetBinding(ctx context.Context, id string) (Binding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureFlowMaps()
	b, ok := m.bindings[id]
	if !ok {
		return Binding{}, ErrNotFound
	}
	return b, nil
}

func (m *Memory) ListBindings(ctx context.Context, tenantID, kind string, limit int) ([]Binding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureFlowMaps()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]Binding, 0)
	for _, b := range m.bindings {
		if b.TenantID != tenantID {
			continue
		}
		if kind != "" && b.Kind != kind {
			continue
		}
		out = append(out, b)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

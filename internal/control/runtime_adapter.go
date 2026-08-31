package control

import (
	"context"
	"fmt"
	"sync"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/composer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/observe"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// SessionRuntime adapts session.Manager to control.Runtime.
type SessionRuntime struct {
	Mgr  *session.Manager
	Repo store.Repository // optional; persists language on lock/switch + observe

	mu    sync.Mutex
	talks map[string]*composer.Talk
}

func (r *SessionRuntime) StartSession(ctx context.Context, p RuntimeStart) error {
	a, err := r.Mgr.Start(ctx, session.StartParams{
		SessionID:      p.SessionID,
		TenantID:       p.TenantID,
		Clock:          p.Clock,
		SampleRate:     p.SampleRate,
		Profile:        p.Profile,
		ProfileRaw:     p.Document,
		GatewayBinding: p.GatewayBinding,
	})
	if err != nil {
		return err
	}
	if r.Repo != nil && a != nil {
		sid := a.ID
		repo := r.Repo
		a.LanguagePersist = func(detected, active string) {
			_, _ = repo.UpdateSessionLanguages(context.Background(), sid, detected, active)
		}
	}
	return nil
}

func (r *SessionRuntime) StopSession(ctx context.Context, sessionID, reason string) (string, error) {
	r.mu.Lock()
	delete(r.talks, sessionID)
	r.mu.Unlock()
	return r.Mgr.Stop(ctx, sessionID, reason)
}

func (r *SessionRuntime) SwitchLanguage(sessionID, primary string) error {
	if r == nil || r.Mgr == nil {
		return fmt.Errorf("runtime not configured")
	}
	a, ok := r.Mgr.Get(sessionID)
	if !ok {
		return fmt.Errorf("session actor not running")
	}
	a.SwitchActiveLanguage(primary)
	_ = a.ConsumeListenFlush() // mark flush for edge/Listen restart consumers
	return nil
}

func (r *SessionRuntime) Languages(sessionID string) (detected, active string, ok bool) {
	if r == nil || r.Mgr == nil {
		return "", "", false
	}
	a, ok := r.Mgr.Get(sessionID)
	if !ok {
		return "", "", false
	}
	return a.DetectedLanguage(), a.ActiveLanguage(), true
}

// InjectText runs composer.Talk.InjectFinal for lab multi-turn / clip smoke.
func (r *SessionRuntime) InjectText(ctx context.Context, sessionID, text string) error {
	if r == nil || r.Mgr == nil {
		return fmt.Errorf("runtime not configured")
	}
	a, ok := r.Mgr.Get(sessionID)
	if !ok {
		return fmt.Errorf("session actor not running")
	}
	talk, err := r.talkFor(a)
	if err != nil {
		return err
	}
	return talk.InjectFinal(ctx, text)
}

// AnswerCall speaks the profile opening without Think / user text.
func (r *SessionRuntime) AnswerCall(ctx context.Context, sessionID string) (string, error) {
	if r == nil || r.Mgr == nil {
		return "", fmt.Errorf("runtime not configured")
	}
	a, ok := r.Mgr.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("session actor not running")
	}
	talk, err := r.talkFor(a)
	if err != nil {
		return "", err
	}
	return talk.AnswerCall(ctx)
}

func (r *SessionRuntime) talkFor(a *session.Actor) (*composer.Talk, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.talks == nil {
		r.talks = make(map[string]*composer.Talk)
	}
	if t, ok := r.talks[a.ID]; ok {
		return t, nil
	}
	talk, err := composer.NewTalk(a.Profile, a.Reg, a.Bus, a.Memory, a.ClockKind, port.SessionID(a.ID))
	if err != nil {
		return nil, err
	}
	talk.BindActor(a)
	if err := bindThinkFromGateway(talk, a); err != nil {
		return nil, err
	}
	if r.Repo != nil {
		meta := observe.SessionMeta{
			SessionID: a.ID,
			TenantID:  a.TenantID,
			ProfileID: a.Profile.ID,
			Clock:     a.ClockKind,
		}
		if sess, err := r.Repo.GetSession(context.Background(), a.ID); err == nil {
			meta.ProfileVersion = sess.ProfileVersion
			meta.RecordingRef = sess.RecordingRef
		}
		talk.Obs = &observe.Observer{Repo: r.Repo, Meta: meta}
	}
	r.talks[a.ID] = talk
	return talk, nil
}

// bindThinkFromGateway fills Think when CC profiles omit routers.think.providers.
func bindThinkFromGateway(talk *composer.Talk, a *session.Actor) error {
	if talk == nil || talk.Path == nil || talk.Path.Deps.Think != nil {
		return nil
	}
	if a == nil || a.GatewayBinding == nil {
		return nil
	}
	id := a.GatewayBinding.Think
	if id == "" || a.Reg == nil {
		return nil
	}
	rec, ok := a.Reg.Get(port.GatewayID(id))
	if !ok {
		return fmt.Errorf("gateway_binding.think %s not registered", id)
	}
	th, ok := rec.Instance.(port.Think)
	if !ok {
		return fmt.Errorf("gateway_binding.think %s is not a Think instance", id)
	}
	talk.Path.Deps.Think = th
	return nil
}

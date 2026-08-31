package control

import (
	"context"
	"fmt"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// SessionRuntime adapts session.Manager to control.Runtime.
type SessionRuntime struct {
	Mgr  *session.Manager
	Repo store.Repository // optional; persists language on lock/switch
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

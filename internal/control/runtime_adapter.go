package control

import (
	"context"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
)

// SessionRuntime adapts session.Manager to control.Runtime.
type SessionRuntime struct {
	Mgr *session.Manager
}

func (r *SessionRuntime) StartSession(ctx context.Context, p RuntimeStart) error {
	_, err := r.Mgr.Start(ctx, session.StartParams{
		SessionID:      p.SessionID,
		TenantID:       p.TenantID,
		Clock:          p.Clock,
		SampleRate:     p.SampleRate,
		Profile:        p.Profile,
		ProfileRaw:     p.Document,
		GatewayBinding: p.GatewayBinding,
	})
	return err
}

func (r *SessionRuntime) StopSession(ctx context.Context, sessionID, reason string) (string, error) {
	return r.Mgr.Stop(ctx, sessionID, reason)
}

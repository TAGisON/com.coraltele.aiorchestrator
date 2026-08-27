package control

import (
	"context"
	"fmt"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/modaudiostream"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/token"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// EdgeBinder binds FS WSS connections to session actors.
type EdgeBinder struct {
	Repo store.Repository
	Mgr  *session.Manager
}

func (b *EdgeBinder) BindEdge(claims token.Claims, peerRate port.SampleRateHz) (port.SampleRateHz, int, func(), error) {
	if b.Mgr == nil {
		return 0, 0, nil, fmt.Errorf("runtime manager required")
	}
	a, ok := b.Mgr.Get(claims.SessionID)
	if !ok {
		return 0, 0, nil, fmt.Errorf("session actor not found")
	}
	if claims.TenantID != "" && a.TenantID != "" && claims.TenantID != a.TenantID {
		return 0, 0, nil, fmt.Errorf("tenant mismatch")
	}
	onGone := func() {
		_, _ = b.Mgr.Stop(context.Background(), claims.SessionID, "feeder_gone")
		if b.Repo != nil {
			_, _ = b.Repo.UpdateSessionState(context.Background(), claims.SessionID, store.StateCancelled)
		}
	}
	return a.SampleRate, a.FrameMs, onGone, nil
}

func (b *EdgeBinder) AttachConn(sessionID string, conn *modaudiostream.Conn) error {
	a, ok := b.Mgr.Get(sessionID)
	if !ok {
		return fmt.Errorf("session actor not found")
	}
	ctx := context.Background()
	a.AttachFeeder(ctx, conn, "fs-feeder")
	a.AttachSink(conn, "fs-sink")
	if b.Repo != nil {
		_, _ = b.Repo.UpdateSessionState(ctx, sessionID, store.StateAttached)
	}
	return nil
}

// MountEdge registers GET /edge/fs on the control mux (Bearer auth skipped for /edge/).
func (s *Server) MountEdge(secret []byte, mgr *session.Manager) {
	if len(secret) == 0 || mgr == nil {
		return
	}
	binder := &EdgeBinder{Repo: s.repo, Mgr: mgr}
	h := modaudiostream.NewHandler(secret, binder)
	s.mux.Handle("GET /edge/fs", h)
}

package control

import (
	"context"
	"fmt"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/modaudiostream"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/token"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

// LiveTalkStarter starts Listen→Talk when an edge attaches (browser or FreeSWITCH).
type LiveTalkStarter interface {
	StartLiveTalk(ctx context.Context, sessionID string) error
}

// EdgeBinder binds FS WSS connections to session actors.
type EdgeBinder struct {
	Repo store.Repository
	Mgr  *session.Manager
	Live LiveTalkStarter
	// OnTerminal runs after durable Cancelled from feeder-gone (same hook as stop API).
	OnTerminal func(ctx context.Context, sess store.Session, terminal string)
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
		ctx := context.Background()
		_, _ = b.Mgr.Stop(ctx, claims.SessionID, "feeder_gone")
		if b.Repo == nil {
			return
		}
		sess, err := b.Repo.UpdateSessionState(ctx, claims.SessionID, store.StateCancelled)
		if err != nil {
			return
		}
		if b.OnTerminal != nil {
			b.OnTerminal(ctx, sess, store.StateCancelled)
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
	if b.Live != nil {
		if err := b.Live.StartLiveTalk(ctx, sessionID); err != nil {
			// Keep edge attached so Speak can still reach the sink; Listen may recover on retry.
			applog.Warn("start live talk", "session", sessionID, "err", err)
		}
	}
	return nil
}

// NewEdgeBinder returns an EdgeBinder that routes feeder-gone Terminal through onSessionTerminal.
func (s *Server) NewEdgeBinder(mgr *session.Manager) *EdgeBinder {
	b := &EdgeBinder{Repo: s.repo, Mgr: mgr, OnTerminal: s.onSessionTerminal}
	if live, ok := s.rt.(LiveTalkStarter); ok {
		b.Live = live
	}
	return b
}

// MountEdge registers GET /edge/fs on the control mux (Bearer auth skipped for /edge/).
func (s *Server) MountEdge(secret []byte, mgr *session.Manager) {
	if len(secret) == 0 || mgr == nil {
		return
	}
	h := modaudiostream.NewHandler(secret, s.NewEdgeBinder(mgr))
	s.mux.Handle("GET /edge/fs", h)
}

package modaudiostream

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/token"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // lab; production should tighten
}

// SessionBinder resolves a live session and wires the edge connection.
type SessionBinder interface {
	// BindEdge validates session exists and is bindable; returns canonical sample rate Hz and frame ms.
	// onGone is called when the WS disconnects (feeder gone).
	BindEdge(claims token.Claims, peerRate port.SampleRateHz) (canonicalRate port.SampleRateHz, frameMs int, onGone func(), err error)
	// AttachConn wires feeder frames onto the session bus (and optional sink).
	AttachConn(sessionID string, conn *Conn) error
}

// Handler serves GET /edge/fs?token=…
type Handler struct {
	Secret []byte
	Binder SessionBinder
	// OptionalIPAllowlist when non-nil rejects remote addrs not in the set (config hook; empty = allow all).
	OptionalIPAllowlist map[string]struct{}

	mu    sync.Mutex
	bound map[string]struct{} // session_id → one WS
}

// NewHandler constructs the FS edge HTTP handler.
func NewHandler(secret []byte, binder SessionBinder) *Handler {
	return &Handler{
		Secret: secret,
		Binder: binder,
		bound:  make(map[string]struct{}),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if len(h.OptionalIPAllowlist) > 0 {
		host := r.RemoteAddr
		if _, ok := h.OptionalIPAllowlist[host]; !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}
	raw := r.URL.Query().Get("token")
	claims, err := token.Validate(h.Secret, raw, time.Now())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.SessionID == "" {
		http.Error(w, "session_id required", http.StatusUnauthorized)
		return
	}

	h.mu.Lock()
	if _, ok := h.bound[claims.SessionID]; ok {
		h.mu.Unlock()
		http.Error(w, "session already bound", http.StatusConflict)
		return
	}
	h.bound[claims.SessionID] = struct{}{}
	h.mu.Unlock()

	peerRate := port.SampleRateHz(8000)
	if v := r.URL.Query().Get("rate"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 8000 && n <= 48000 {
			peerRate = port.SampleRateHz(n)
		}
	}

	canonical, frameMs, onGone, err := h.Binder.BindEdge(claims, peerRate)
	if err != nil {
		h.unbind(claims.SessionID)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.unbind(claims.SessionID)
		if onGone != nil {
			onGone()
		}
		return
	}

	meta := port.FeederMeta{
		PeerID:    r.URL.Query().Get("call_uuid"),
		PeerRate:  peerRate,
		SessionID: port.SessionID(claims.SessionID),
	}
	conn := newConn(ws, meta, canonical, frameMs)
	conn.start()
	if err := h.Binder.AttachConn(claims.SessionID, conn); err != nil {
		_ = ws.Close()
		h.unbind(claims.SessionID)
		if onGone != nil {
			onGone()
		}
		return
	}

	go func() {
		<-conn.done
		h.unbind(claims.SessionID)
		if onGone != nil {
			onGone()
		}
	}()
}

func (h *Handler) unbind(sessionID string) {
	h.mu.Lock()
	delete(h.bound, sessionID)
	h.mu.Unlock()
}

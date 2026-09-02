package modaudiostream

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/gorilla/websocket"
)

const GatewayID port.GatewayID = "modaudiostream"

const (
	// writeTimeout bounds a single WebSocket write. mod_audio_stream reads and
	// writes on one libevent thread, so a stalled write there also stalls its
	// downlink; we must fail fast rather than sit on the socket.
	writeTimeout = 2 * time.Second

	// maxWriteFailures is how many consecutive write errors we tolerate before
	// declaring the leg dead. Transient errors happen; a wedged socket must not
	// silently swallow the rest of the call's audio.
	maxWriteFailures = 5

	// maxQueuedFrames caps unplayed downlink audio (~60 s at 20 ms). Speak emits
	// a whole utterance at once, so the queue is expected to be deep; this only
	// guards against unbounded growth if the socket stops draining entirely.
	maxQueuedFrames = 3000

	// warnEvery throttles the per-connection drop/error warnings.
	warnEvery = 5 * time.Second
)

// Conn is one FS WSS attachment implementing port.Feeder, port.Sink and
// port.CallControl.
type Conn struct {
	meta          port.FeederMeta
	canonicalRate port.SampleRateHz
	frameMs       int

	conn *websocket.Conn

	frames chan port.PCMFrame
	events chan port.FeederEvent
	done   chan struct{}

	closeOnce sync.Once

	mu       sync.Mutex
	queue    [][]byte // peer-rate PCM awaiting inject
	flushGen uint64
	seq      uint64
	closed   bool

	writeMu sync.Mutex // serializes websocket writes (flushOne + Flush + control)

	markMu   sync.Mutex
	markCond *sync.Cond
	pending  int // queued+inflight frames waiting mark

	writeCadence time.Duration

	// actionOnce guards call control: the module rejects a second armed action,
	// and re-sending one would be a bug on our side, not a recoverable state.
	actionOnce sync.Once

	// Counters are read by Stats() for the end-of-session record.
	uplinkDropped   atomic.Int64
	downlinkDropped atomic.Int64
	writeFailures   atomic.Int64
	consecutiveFail atomic.Int64
	lastUplinkWarn  atomic.Int64
	lastWriteWarn   atomic.Int64
}

var (
	_ port.Feeder      = (*Conn)(nil)
	_ port.Sink        = (*Conn)(nil)
	_ port.CallControl = (*Conn)(nil)
)

// Meta returns attach metadata.
func (c *Conn) Meta() port.FeederMeta { return c.meta }

func (c *Conn) ID() port.GatewayID { return GatewayID }

func (c *Conn) Frames() <-chan port.PCMFrame    { return c.frames }
func (c *Conn) Events() <-chan port.FeederEvent { return c.events }

// Stats is a point-in-time snapshot for session records and diagnostics.
type Stats struct {
	UplinkDropped   int64 `json:"uplink_dropped_frames"`
	DownlinkDropped int64 `json:"downlink_dropped_frames"`
	WriteFailures   int64 `json:"write_failures"`
	QueueDepth      int   `json:"queue_depth"`
}

func (c *Conn) Stats() Stats {
	c.mu.Lock()
	depth := len(c.queue)
	c.mu.Unlock()
	return Stats{
		UplinkDropped:   c.uplinkDropped.Load(),
		DownlinkDropped: c.downlinkDropped.Load(),
		WriteFailures:   c.writeFailures.Load(),
		QueueDepth:      depth,
	}
}

func newConn(ws *websocket.Conn, meta port.FeederMeta, canonicalRate port.SampleRateHz, frameMs int) *Conn {
	if frameMs <= 0 {
		frameMs = 20
	}
	if meta.PeerRate == 0 {
		meta.PeerRate = 8000
	}
	c := &Conn{
		meta:          meta,
		canonicalRate: canonicalRate,
		frameMs:       frameMs,
		conn:          ws,
		frames:        make(chan port.PCMFrame, 64),
		events:        make(chan port.FeederEvent, 16),
		done:          make(chan struct{}),
		writeCadence:  time.Duration(frameMs) * time.Millisecond,
	}
	c.markCond = sync.NewCond(&c.markMu)
	return c
}

func (c *Conn) start() {
	go c.readLoop()
	go c.writeLoop()
}

func (c *Conn) sessionID() string { return string(c.meta.SessionID) }

// throttled reports whether enough time has passed since the last warning
// recorded in slot, and claims the slot when it has.
func throttled(slot *atomic.Int64) bool {
	now := time.Now().UnixNano()
	prev := slot.Load()
	if now-prev < int64(warnEvery) {
		return false
	}
	return slot.CompareAndSwap(prev, now)
}

func (c *Conn) readLoop() {
	defer func() {
		c.signalClose()
		close(c.frames)
		close(c.events)
	}()
	for {
		mt, data, err := c.conn.ReadMessage()
		if err != nil {
			c.emitEvent(port.FeederEvent{Kind: "error", Data: err.Error()})
			return
		}
		switch mt {
		case websocket.BinaryMessage:
			c.onPCM(data)
		case websocket.TextMessage:
			if kind, d, ok := parseInboundEvent(data); ok {
				c.emitEvent(port.FeederEvent{Kind: kind, Data: d})
				if kind == "stop" {
					return
				}
			}
		}
	}
}

func (c *Conn) emitEvent(ev port.FeederEvent) {
	select {
	case <-c.done:
		return
	default:
	}
	select {
	case <-c.done:
	case c.events <- ev:
	default:
	}
}

func (c *Conn) onPCM(peerPCM []byte) {
	canonical := ResamplePCM(peerPCM, int(c.meta.PeerRate), int(c.canonicalRate))
	seq := atomic.AddUint64(&c.seq, 1)
	fr := port.PCMFrame{
		Data:       canonical,
		SampleRate: c.canonicalRate,
		Seq:        seq,
		At:         time.Now(),
	}
	select {
	case <-c.done:
		return
	case c.frames <- fr:
		return
	default:
	}
	// The Listen consumer is behind. Drop this frame rather than block: this
	// goroutine is the only reader of the socket, and mod_audio_stream services
	// send and receive on a single libevent thread. Blocking here stops us
	// reading, which back-pressures the module's uplink send, which stalls its
	// event thread, which stalls TTS delivery — the whole call freezes and then
	// floods. Dropping one 20 ms uplink frame is strictly cheaper.
	n := c.uplinkDropped.Add(1)
	if throttled(&c.lastUplinkWarn) {
		applog.Warn("edge uplink frame dropped (listen consumer behind)",
			"session", c.sessionID(), "dropped_total", n)
	}
}

func (c *Conn) writeLoop() {
	ticker := time.NewTicker(c.writeCadence)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.flushOne()
		}
	}
}

// markDone releases n frames from the WaitMark accounting. It must be called for
// every frame removed from the queue, successfully written or not, otherwise
// WaitMark blocks until its context expires.
func (c *Conn) markDone(n int) {
	if n <= 0 {
		return
	}
	c.markMu.Lock()
	c.pending -= n
	if c.pending < 0 {
		c.pending = 0
	}
	if c.pending == 0 {
		c.markCond.Broadcast()
	}
	c.markMu.Unlock()
}

func (c *Conn) flushOne() {
	c.mu.Lock()
	if c.closed || len(c.queue) == 0 {
		c.mu.Unlock()
		return
	}
	chunk := c.queue[0]
	c.queue = c.queue[1:]
	gen := c.flushGen
	c.mu.Unlock()

	msg, err := encodeStreamAudio(chunk, int(c.meta.PeerRate))
	if err != nil {
		// Encoding cannot fail for well-formed input; count it as a lost frame
		// rather than leaving the mark accounting stuck.
		c.downlinkDropped.Add(1)
		c.markDone(1)
		applog.Error("edge encode streamAudio", "session", c.sessionID(), "err", err)
		return
	}

	if err := c.writeAudio(msg, gen); err != nil {
		c.downlinkDropped.Add(1)
		c.markDone(1)
		return
	}
	c.consecutiveFail.Store(0)
	c.markDone(1)
}

// writeAudio writes one media payload belonging to flush generation gen. A
// payload from a superseded generation is discarded rather than played after a
// barge-in. Control verbs must use writeRaw — they are never flush-scoped.
func (c *Conn) writeAudio(payload []byte, gen uint64) error {
	c.mu.Lock()
	cur := c.flushGen
	c.mu.Unlock()
	if gen != cur {
		return errors.New("superseded by flush")
	}
	return c.writeRaw(websocket.TextMessage, payload)
}

// writeRaw serializes every socket write and applies the failure policy.
func (c *Conn) writeRaw(msgType int, payload []byte) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return errors.New("edge closed")
	}

	c.writeMu.Lock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	err := c.conn.WriteMessage(msgType, payload)
	c.writeMu.Unlock()
	if err == nil {
		return nil
	}

	total := c.writeFailures.Add(1)
	streak := c.consecutiveFail.Add(1)
	if throttled(&c.lastWriteWarn) {
		applog.Warn("edge write failed", "session", c.sessionID(),
			"err", err, "failures_total", total, "consecutive", streak)
	}
	if streak >= maxWriteFailures {
		applog.Error("edge write failed repeatedly; closing leg",
			"session", c.sessionID(), "consecutive", streak, "err", err)
		c.signalClose()
	}
	return err
}

// WritePCM accepts canonical PCM and queues peer-rate inject frames.
func (c *Conn) WritePCM(ctx context.Context, frame port.PCMFrame) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return context.Canceled
	default:
	}
	srcRate := int(frame.SampleRate)
	if srcRate == 0 {
		srcRate = int(c.canonicalRate)
	}
	peer := ResamplePCM(frame.Data, srcRate, int(c.meta.PeerRate))
	n := FrameBytes(int(c.meta.PeerRate), c.frameMs)
	if n <= 0 {
		return errors.New("invalid frame size")
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return context.Canceled
	}
	var queued, dropped int
	for off := 0; off < len(peer); off += n {
		if len(c.queue) >= maxQueuedFrames {
			dropped++
			continue
		}
		end := off + n
		if end > len(peer) {
			end = len(peer)
		}
		chunk := make([]byte, n)
		copy(chunk, peer[off:end])
		c.queue = append(c.queue, chunk)
		queued++
	}
	c.mu.Unlock()

	if queued > 0 {
		c.markMu.Lock()
		c.pending += queued
		c.markMu.Unlock()
	}
	if dropped > 0 {
		c.downlinkDropped.Add(int64(dropped))
		applog.Warn("edge downlink queue full; audio discarded",
			"session", c.sessionID(), "frames", dropped, "cap", maxQueuedFrames)
	}
	return nil
}

// Flush drops unplayed queue locally and tells FS mod_audio_stream to clear inject (barge-in).
func (c *Conn) Flush(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	c.mu.Lock()
	c.queue = nil
	c.flushGen++
	closed := c.closed
	c.mu.Unlock()
	c.markMu.Lock()
	c.pending = 0
	c.markCond.Broadcast()
	c.markMu.Unlock()
	if closed {
		return context.Canceled
	}
	return c.writeRaw(websocket.TextMessage, encodeFlush())
}

// WaitMark blocks until queued playout is drained (or ctx/done).
func (c *Conn) WaitMark(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		c.markMu.Lock()
		for c.pending > 0 {
			c.markCond.Wait()
		}
		c.markMu.Unlock()
		close(done)
	}()
	select {
	case <-ctx.Done():
		c.markMu.Lock()
		c.markCond.Broadcast()
		c.markMu.Unlock()
		return ctx.Err()
	case <-c.done:
		return context.Canceled
	case <-done:
		return nil
	}
}

// Hangup asks the edge to end the call once queued playout drains.
// Implements port.CallControl.
func (c *Conn) Hangup(ctx context.Context, cause string) error {
	if cause == "" {
		cause = "NORMAL_CLEARING"
	}
	return c.sendAction(ctx, func() error {
		msg, err := encodeHangup(cause, int(defaultDrain.Milliseconds()))
		if err != nil {
			return err
		}
		applog.Info("edge hangup requested", "session", c.sessionID(), "cause", cause)
		return c.writeRaw(websocket.TextMessage, msg)
	})
}

// Transfer hands the caller leg to another extension once playout drains.
// Implements port.CallControl.
func (c *Conn) Transfer(ctx context.Context, req port.TransferRequest) error {
	dest := req.Destination
	if dest == "" {
		return errors.New("transfer: destination required")
	}
	dialplan := req.Dialplan
	if dialplan == "" {
		dialplan = "XML"
	}
	callCtx := req.Context
	if callCtx == "" {
		callCtx = "calltransfer"
	}
	drain := req.DrainMs
	if drain <= 0 {
		drain = int(defaultDrain.Milliseconds())
	}
	return c.sendAction(ctx, func() error {
		msg, err := encodeTransfer(dest, dialplan, callCtx, drain)
		if err != nil {
			return err
		}
		applog.Info("edge transfer requested", "session", c.sessionID(),
			"dest", dest, "dialplan", dialplan, "context", callCtx, "reason", req.Reason)
		return c.writeRaw(websocket.TextMessage, msg)
	})
}

// defaultDrain is how long the edge may hold a hangup/transfer waiting for the
// last of the queued prompt to play. It matches the module's own ceiling.
const defaultDrain = 15 * time.Second

// sendAction runs send exactly once per connection. A second call is a caller
// bug (the module refuses to retarget an armed action) and returns an error
// rather than racing the first one.
func (c *Conn) sendAction(ctx context.Context, send func() error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return context.Canceled
	default:
	}
	var (
		ran bool
		err error
	)
	c.actionOnce.Do(func() {
		ran = true
		err = send()
	})
	if !ran {
		return errors.New("call control action already requested for this session")
	}
	return err
}

func (c *Conn) Close(ctx context.Context) error {
	c.signalClose()
	return nil
}

func (c *Conn) signalClose() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
		close(c.done)
		_ = c.conn.Close()
		c.markMu.Lock()
		c.pending = 0
		c.markCond.Broadcast()
		c.markMu.Unlock()
	})
}

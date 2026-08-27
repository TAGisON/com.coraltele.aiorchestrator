package modaudiostream

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/gorilla/websocket"
)

const GatewayID port.GatewayID = "modaudiostream"

// Conn is one FS WSS attachment implementing port.Feeder and port.Sink.
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

	markMu   sync.Mutex
	markCond *sync.Cond
	pending  int // queued+inflight frames waiting mark

	writeCadence time.Duration
}

// Meta returns attach metadata.
func (c *Conn) Meta() port.FeederMeta { return c.meta }

func (c *Conn) ID() port.GatewayID { return GatewayID }

func (c *Conn) Frames() <-chan port.PCMFrame    { return c.frames }
func (c *Conn) Events() <-chan port.FeederEvent { return c.events }

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
	case c.frames <- fr:
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

func (c *Conn) flushOne() {
	c.mu.Lock()
	if c.closed || len(c.queue) == 0 {
		c.mu.Unlock()
		return
	}
	chunk := c.queue[0]
	c.queue = c.queue[1:]
	c.mu.Unlock()

	msg, err := encodeStreamAudio(chunk, int(c.meta.PeerRate))
	if err != nil {
		return
	}
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		return
	}
	c.markMu.Lock()
	if c.pending > 0 {
		c.pending--
	}
	if c.pending == 0 {
		c.markCond.Broadcast()
	}
	c.markMu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return context.Canceled
	}
	for off := 0; off < len(peer); off += n {
		end := off + n
		if end > len(peer) {
			end = len(peer)
		}
		chunk := make([]byte, n)
		copy(chunk, peer[off:end])
		c.queue = append(c.queue, chunk)
		c.markMu.Lock()
		c.pending++
		c.markMu.Unlock()
	}
	return nil
}

// Flush drops unplayed queue (barge-in).
func (c *Conn) Flush(ctx context.Context) error {
	c.mu.Lock()
	c.queue = nil
	c.flushGen++
	c.mu.Unlock()
	c.markMu.Lock()
	c.pending = 0
	c.markCond.Broadcast()
	c.markMu.Unlock()
	return nil
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

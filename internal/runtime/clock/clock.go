// Package clock provides live and playback session schedulers.
package clock

import (
	"context"
	"time"
)

// Kind is the session clock declared at create.
type Kind string

const (
	Live     Kind = "live"
	Playback Kind = "playback"
)

// Scheduler paces feeder/sink work and declares VAD policy.
type Scheduler interface {
	Kind() Kind
	// VADEnabled is true for live Talk; false for playback unless simulating.
	VADEnabled() bool
	// FrameDuration is the canonical frame period (default 20 ms).
	FrameDuration() time.Duration
	// Pace waits until the next frame tick should be emitted.
	// Live waits wall-clock frame duration; playback may return immediately (faster than realtime).
	Pace(ctx context.Context) error
}

// LiveClock paces near realtime and enables VAD.
type LiveClock struct {
	frame time.Duration
}

// NewLive returns a live scheduler. frameMs defaults to 20.
func NewLive(frameMs int) *LiveClock {
	if frameMs <= 0 {
		frameMs = 20
	}
	return &LiveClock{frame: time.Duration(frameMs) * time.Millisecond}
}

func (c *LiveClock) Kind() Kind                   { return Live }
func (c *LiveClock) VADEnabled() bool             { return true }
func (c *LiveClock) FrameDuration() time.Duration { return c.frame }

func (c *LiveClock) Pace(ctx context.Context) error {
	t := time.NewTimer(c.frame)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// PlaybackClock feeds at our pace (faster than realtime allowed) and disables VAD by default.
type PlaybackClock struct {
	frame     time.Duration
	simulate  bool // if true, VADEnabled returns true (test/sim)
	paceDelay time.Duration
}

// NewPlayback returns a playback scheduler. frameMs defaults to 20.
// Optional paceDelay > 0 throttles between frames; 0 means as-fast-as-possible.
func NewPlayback(frameMs int, paceDelay time.Duration) *PlaybackClock {
	if frameMs <= 0 {
		frameMs = 20
	}
	return &PlaybackClock{frame: time.Duration(frameMs) * time.Millisecond, paceDelay: paceDelay}
}

// WithSimulateVAD enables VAD policy for playback simulation tests.
func (c *PlaybackClock) WithSimulateVAD(on bool) *PlaybackClock {
	c.simulate = on
	return c
}

func (c *PlaybackClock) Kind() Kind                   { return Playback }
func (c *PlaybackClock) VADEnabled() bool             { return c.simulate }
func (c *PlaybackClock) FrameDuration() time.Duration { return c.frame }

func (c *PlaybackClock) Pace(ctx context.Context) error {
	if c.paceDelay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	t := time.NewTimer(c.paceDelay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// New returns a scheduler for the named clock kind.
func New(kind string, frameMs int) Scheduler {
	switch Kind(kind) {
	case Playback:
		return NewPlayback(frameMs, 0)
	default:
		return NewLive(frameMs)
	}
}

// FrameBytes returns mono s16le bytes for one frame at rateHz and frameMs.
func FrameBytes(rateHz, frameMs int) int {
	if rateHz <= 0 {
		rateHz = 16000
	}
	if frameMs <= 0 {
		frameMs = 20
	}
	return rateHz * 2 * frameMs / 1000
}

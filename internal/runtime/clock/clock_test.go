package clock_test

import (
	"context"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/clock"
)

func TestLive_VsPlayback_Policy(t *testing.T) {
	live := clock.NewLive(20)
	pb := clock.NewPlayback(20, 0)
	if live.Kind() != clock.Live || pb.Kind() != clock.Playback {
		t.Fatalf("kinds live=%s pb=%s", live.Kind(), pb.Kind())
	}
	if !live.VADEnabled() {
		t.Fatal("live VAD should be on")
	}
	if pb.VADEnabled() {
		t.Fatal("playback VAD should be off")
	}
	if live.FrameDuration() != 20*time.Millisecond {
		t.Fatalf("frame %v", live.FrameDuration())
	}
}

func TestPlayback_PaceImmediate(t *testing.T) {
	pb := clock.NewPlayback(20, 0)
	ctx := context.Background()
	start := time.Now()
	for i := 0; i < 50; i++ {
		if err := pb.Pace(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Fatalf("playback pacing too slow: %v", time.Since(start))
	}
}

func TestLive_PaceWaits(t *testing.T) {
	live := clock.NewLive(5)
	ctx := context.Background()
	start := time.Now()
	if err := live.Pace(ctx); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 4*time.Millisecond {
		t.Fatalf("live pace too fast: %v", time.Since(start))
	}
}

func TestFrameBytes(t *testing.T) {
	// 16000 Hz * 2 * 20/1000 = 640
	if n := clock.FrameBytes(16000, 20); n != 640 {
		t.Fatalf("got %d", n)
	}
}

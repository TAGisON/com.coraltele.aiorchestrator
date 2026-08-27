// Package file implements a local-file Feeder for playback sessions (PLATFORM_FIRST Phase D).
package file

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/clock"
)

const GatewayID port.GatewayID = "file"

// Feeder reads mono s16le PCM or WAV and emits canonical frames.
type Feeder struct {
	id         port.GatewayID
	frames     chan port.PCMFrame
	events     chan port.FeederEvent
	closeOnce  sync.Once
	cancel     context.CancelFunc
}

// Open starts a feeder that reads path (file:// or plain path), resamples to canonicalRate, paces via sched.
func Open(ctx context.Context, path string, fileRate, canonicalRate port.SampleRateHz, frameMs int, sched clock.Scheduler) (*Feeder, error) {
	path = strings.TrimPrefix(path, "file://")
	path = filepath.Clean(path)
	pcm, rate, err := readAudioFile(path, fileRate)
	if err != nil {
		return nil, err
	}
	if canonicalRate == 0 {
		canonicalRate = rate
	}
	if rate != canonicalRate {
		pcm = resampleLinear(pcm, int(rate), int(canonicalRate))
	}
	if frameMs <= 0 {
		frameMs = 20
	}
	if sched == nil {
		sched = clock.NewPlayback(frameMs, 0)
	}
	runCtx, cancel := context.WithCancel(ctx)
	f := &Feeder{
		id:     GatewayID,
		frames: make(chan port.PCMFrame, 64),
		events: make(chan port.FeederEvent, 4),
		cancel: cancel,
	}
	go f.run(runCtx, pcm, canonicalRate, frameMs, sched)
	return f, nil
}

func (f *Feeder) ID() port.GatewayID              { return f.id }
func (f *Feeder) Frames() <-chan port.PCMFrame    { return f.frames }
func (f *Feeder) Events() <-chan port.FeederEvent { return f.events }

func (f *Feeder) Close(ctx context.Context) error {
	f.closeOnce.Do(func() {
		if f.cancel != nil {
			f.cancel()
		}
	})
	return nil
}

func (f *Feeder) run(ctx context.Context, pcm []byte, rate port.SampleRateHz, frameMs int, sched clock.Scheduler) {
	defer close(f.frames)
	defer close(f.events)
	n := clock.FrameBytes(int(rate), frameMs)
	var seq uint64
	for off := 0; off < len(pcm); off += n {
		select {
		case <-ctx.Done():
			return
		default:
		}
		end := off + n
		if end > len(pcm) {
			end = len(pcm)
		}
		chunk := make([]byte, n)
		copy(chunk, pcm[off:end])
		seq++
		select {
		case f.frames <- port.PCMFrame{Data: chunk, SampleRate: rate, Seq: seq, At: time.Now()}:
		case <-ctx.Done():
			return
		}
		if err := sched.Pace(ctx); err != nil {
			return
		}
	}
	select {
	case f.events <- port.FeederEvent{Kind: "stop", Data: "eof"}:
	default:
	}
}

func readAudioFile(path string, hintRate port.SampleRateHz) ([]byte, port.SampleRateHz, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, 0, err
	}
	if len(raw) >= 12 && string(raw[0:4]) == "RIFF" && string(raw[8:12]) == "WAVE" {
		pcm, rate, err := parseWAV(raw)
		if err != nil {
			return nil, 0, err
		}
		return pcm, port.SampleRateHz(rate), nil
	}
	rate := hintRate
	if rate == 0 {
		rate = 16000
	}
	return raw, rate, nil
}

func parseWAV(raw []byte) ([]byte, int, error) {
	if len(raw) < 44 {
		return nil, 0, fmt.Errorf("wav too short")
	}
	// Find fmt and data chunks
	rate := int(binary.LittleEndian.Uint32(raw[24:28]))
	off := 12
	var data []byte
	for off+8 <= len(raw) {
		id := string(raw[off : off+4])
		size := int(binary.LittleEndian.Uint32(raw[off+4 : off+8]))
		off += 8
		if off+size > len(raw) {
			break
		}
		chunk := raw[off : off+size]
		switch id {
		case "fmt ":
			if size >= 16 {
				rate = int(binary.LittleEndian.Uint32(chunk[4:8]))
			}
		case "data":
			data = append([]byte(nil), chunk...)
		}
		off += size
		if size%2 == 1 {
			off++
		}
	}
	if data == nil {
		return nil, 0, fmt.Errorf("wav missing data chunk")
	}
	if rate <= 0 {
		rate = 16000
	}
	return data, rate, nil
}

func resampleLinear(src []byte, srcRate, dstRate int) []byte {
	if srcRate == dstRate || len(src) < 2 {
		out := make([]byte, len(src))
		copy(out, src)
		return out
	}
	srcN := len(src) / 2
	dstN := srcN * dstRate / srcRate
	if dstN < 1 {
		dstN = 1
	}
	out := make([]byte, dstN*2)
	for i := 0; i < dstN; i++ {
		pos := i * srcRate / dstRate
		if pos >= srcN {
			pos = srcN - 1
		}
		copy(out[i*2:], src[pos*2:pos*2+2])
	}
	return out
}

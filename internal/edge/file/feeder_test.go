package file_test

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/file"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/clock"
)

func TestFeeder_RawPCM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tone.pcm")
	// 3 frames @ 16k 20ms
	n := clock.FrameBytes(16000, 20)
	blob := make([]byte, n*3)
	for i := 0; i < len(blob); i += 2 {
		binary.LittleEndian.PutUint16(blob[i:], 500)
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	f, err := file.Open(ctx, path, 16000, 16000, 20, clock.NewPlayback(20, 0))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close(ctx)

	var got int
	for {
		select {
		case fr, ok := <-f.Frames():
			if !ok {
				if got != 3 {
					t.Fatalf("frames %d want 3", got)
				}
				return
			}
			if fr.SampleRate != 16000 {
				t.Fatalf("rate %d", fr.SampleRate)
			}
			got++
		case <-ctx.Done():
			t.Fatalf("timeout after %d frames", got)
		}
	}
}

func TestFeeder_WAV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tone.wav")
	pcm := make([]byte, 320) // 20ms @ 8k
	wav := writeWAV(pcm, 8000)
	if err := os.WriteFile(path, wav, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	f, err := file.Open(ctx, "file://"+path, 0, port.SampleRateHz(8000), 20, clock.NewPlayback(20, 0))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close(ctx)
	fr, ok := <-f.Frames()
	if !ok || len(fr.Data) == 0 {
		t.Fatal("expected frame")
	}
}

func writeWAV(pcm []byte, rate int) []byte {
	dataSize := len(pcm)
	out := make([]byte, 44+dataSize)
	copy(out[0:], []byte("RIFF"))
	binary.LittleEndian.PutUint32(out[4:], uint32(36+dataSize))
	copy(out[8:], []byte("WAVE"))
	copy(out[12:], []byte("fmt "))
	binary.LittleEndian.PutUint32(out[16:], 16)
	binary.LittleEndian.PutUint16(out[20:], 1) // PCM
	binary.LittleEndian.PutUint16(out[22:], 1) // mono
	binary.LittleEndian.PutUint32(out[24:], uint32(rate))
	binary.LittleEndian.PutUint32(out[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(out[32:], 2)
	binary.LittleEndian.PutUint16(out[34:], 16)
	copy(out[36:], []byte("data"))
	binary.LittleEndian.PutUint32(out[40:], uint32(dataSize))
	copy(out[44:], pcm)
	return out
}

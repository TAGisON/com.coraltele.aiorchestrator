package record

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDisabledReturnsNilRecorder(t *testing.T) {
	r, err := New(Config{Enabled: false}, Meta{SessionID: "s1"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if r != nil {
		t.Fatalf("expected nil recorder when disabled")
	}
	// Every method must tolerate nil.
	r.WriteCaller([]byte{1, 2})
	r.WriteAgent([]byte{1, 2})
	r.Close(Summary{})
	if r.Path() != "" || r.Dir() != "" {
		t.Fatalf("nil recorder should report empty path/dir")
	}
}

func TestRecorderWritesPlayableStereoWav(t *testing.T) {
	root := t.TempDir()
	r, err := New(Config{Enabled: true, Root: root}, Meta{
		SessionID:  "sess-abc",
		CallID:     "call-123",
		SampleRate: 8000,
		FrameMs:    20,
		ProfileID:  "coral-tfn",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if r == nil {
		t.Fatal("expected recorder")
	}

	// 200 ms on each leg: caller a constant +1000, agent a constant -1000, so we
	// can prove the channel mapping is left=caller / right=agent.
	frame := 8000 * 2 / 1000 * 20 // 320 bytes
	caller := make([]byte, frame*10)
	agent := make([]byte, frame*10)
	callerTone, agentTone := int16(1000), int16(-1000)
	for i := 0; i+1 < len(caller); i += 2 {
		binary.LittleEndian.PutUint16(caller[i:], uint16(callerTone))
		binary.LittleEndian.PutUint16(agent[i:], uint16(agentTone))
	}
	r.WriteCaller(caller)
	r.WriteAgent(agent)

	// Let the mux run long enough to consume both legs.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		empty := len(r.caller) == 0 && len(r.agent) == 0
		r.mu.Unlock()
		if empty {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	r.Close(Summary{Disposition: "resolved", EndReason: "hangup"})

	// Directory name must be "<session>-<call>" under a date folder.
	if base := filepath.Base(r.Dir()); base != "sess-abc-call-123" {
		t.Fatalf("dir name = %q, want sess-abc-call-123", base)
	}

	b, err := os.ReadFile(r.Path())
	if err != nil {
		t.Fatalf("read wav: %v", err)
	}
	if len(b) <= wavHeaderBytes {
		t.Fatalf("wav has no audio data (%d bytes)", len(b))
	}
	if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		t.Fatalf("not a RIFF/WAVE file")
	}
	if ch := binary.LittleEndian.Uint16(b[22:24]); ch != 2 {
		t.Fatalf("channels = %d, want 2", ch)
	}
	if rate := binary.LittleEndian.Uint32(b[24:28]); rate != 8000 {
		t.Fatalf("rate = %d, want 8000", rate)
	}
	if bits := binary.LittleEndian.Uint16(b[34:36]); bits != 16 {
		t.Fatalf("bits = %d, want 16", bits)
	}
	// Header sizes must match the bytes actually on disk, or players truncate.
	dataLen := binary.LittleEndian.Uint32(b[40:44])
	if int(dataLen) != len(b)-wavHeaderBytes {
		t.Fatalf("data chunk = %d, file payload = %d", dataLen, len(b)-wavHeaderBytes)
	}
	if riff := binary.LittleEndian.Uint32(b[4:8]); int(riff) != len(b)-8 {
		t.Fatalf("riff size = %d, want %d", riff, len(b)-8)
	}

	// First stereo sample: left = caller (+1000), right = agent (-1000).
	left := int16(binary.LittleEndian.Uint16(b[wavHeaderBytes : wavHeaderBytes+2]))
	right := int16(binary.LittleEndian.Uint16(b[wavHeaderBytes+2 : wavHeaderBytes+4]))
	if left != 1000 {
		t.Fatalf("left sample = %d, want 1000 (caller leg)", left)
	}
	if right != -1000 {
		t.Fatalf("right sample = %d, want -1000 (agent leg)", right)
	}

	// meta.json must be present, complete and valid JSON.
	mb, err := os.ReadFile(filepath.Join(r.Dir(), "meta.json"))
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	var m metaFile
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatalf("meta json: %v", err)
	}
	if m.SessionID != "sess-abc" || m.CallID != "call-123" {
		t.Fatalf("meta identity wrong: %+v", m)
	}
	if m.ChannelMap != "left=caller,right=agent" {
		t.Fatalf("channel map = %q", m.ChannelMap)
	}
	if m.Disposition != "resolved" || m.EndReason != "hangup" {
		t.Fatalf("meta summary not recorded: %+v", m)
	}
	if m.DurationSec <= 0 {
		t.Fatalf("duration not recorded: %v", m.DurationSec)
	}
	if _, err := os.Stat(filepath.Join(r.Dir(), ".meta.json.tmp")); !os.IsNotExist(err) {
		t.Fatalf("temp meta file was left behind")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	r, err := New(Config{Enabled: true, Root: t.TempDir()}, Meta{SessionID: "s", SampleRate: 8000, FrameMs: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r.Close(Summary{})
	r.Close(Summary{}) // must not panic on a closed channel or closed file
}

func TestSanitizeRejectsPathTraversal(t *testing.T) {
	got := dirName(Meta{SessionID: "../../etc", CallID: "a/b"})
	if strings.Contains(got, "/") || strings.Contains(got, "..") {
		t.Fatalf("dirName leaked path separators: %q", got)
	}
}

func TestSweepRemovesOldDaysOnly(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, time.Now().AddDate(0, 0, -10).Format("2006-01-02"))
	recent := filepath.Join(root, time.Now().AddDate(0, 0, -1).Format("2006-01-02"))
	today := filepath.Join(root, time.Now().Format("2006-01-02"))
	notOurs := filepath.Join(root, "keep-me")
	for _, d := range []string{old, recent, today, notOurs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := Sweep(root, 7)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old day should be gone")
	}
	for _, d := range []string{recent, today, notOurs} {
		if _, err := os.Stat(d); err != nil {
			t.Fatalf("%s should be kept: %v", d, err)
		}
	}
}

func TestSweepDisabledAndMissingRoot(t *testing.T) {
	if n, err := Sweep(t.TempDir(), 0); err != nil || n != 0 {
		t.Fatalf("retention 0 should be a no-op, got %d %v", n, err)
	}
	if n, err := Sweep(filepath.Join(t.TempDir(), "nope"), 7); err != nil || n != 0 {
		t.Fatalf("missing root should be a no-op, got %d %v", n, err)
	}
}

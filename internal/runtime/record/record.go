// Package record writes a per-session stereo call recording to local disk.
//
// Layout (identical on every OS; on Windows a leading "/" resolves against the
// current drive, so the same configured root works unchanged):
//
//	<root>/<YYYY-MM-DD>/<session-id>[-<call-id>]/
//	    session.wav   stereo s16le — left = caller, right = agent
//	    meta.json     session identity, timing and counters
//
// The two legs are muxed on a fixed frame cadence driven by the wall clock, so
// the file reflects what the caller actually heard: agent audio appears where it
// was played out, not where it was synthesised, and silence is written for any
// slot with no audio.
//
// Recording is always best effort. Every failure degrades to "no recording" and
// is logged once — it must never interfere with a live call.
package record

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
)

// DefaultRoot is the on-disk root for recordings.
const DefaultRoot = "/usr/local/recordings/ai-orchestrator"

const (
	// maxLegBacklog bounds per-leg buffering (~30 s at 8 kHz mono s16le). A leg
	// that runs further ahead than this is being produced faster than realtime;
	// we drop rather than grow without limit.
	maxLegBacklogBytes = 30 * 8000 * 2

	// headerPatchEvery re-writes the RIFF sizes periodically so a recording left
	// behind by a crash is still playable.
	headerPatchEvery = 5 * time.Second

	wavHeaderBytes = 44
	channels       = 2
	bytesPerSample = 2
)

// Config controls recording for the process.
type Config struct {
	// Enabled turns recording on. When false New returns a nil Recorder.
	Enabled bool
	// Root is the recordings root; DefaultRoot when empty.
	Root string
	// RetentionDays removes date directories older than this. Zero disables.
	RetentionDays int
}

// Meta identifies the call being recorded. Only SessionID is required.
type Meta struct {
	SessionID string
	// CallID is the telephony call identifier (FreeSWITCH channel UUID). When
	// present it is appended to the directory name.
	CallID     string
	SIPCallID  string
	TenantID   string
	ProfileID  string
	CallerANI  string
	Dest       string
	SampleRate int
	FrameMs    int
}

// Summary is written to meta.json when the recording closes.
type Summary struct {
	Disposition string         `json:"disposition,omitempty"`
	EndReason   string         `json:"end_reason,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

// Recorder muxes the caller and agent legs into one stereo WAV.
//
// All methods are safe on a nil *Recorder, so callers never need a guard.
type Recorder struct {
	dir  string
	path string
	meta Meta

	rate       int
	frameBytes int // per leg, per tick
	cadence    time.Duration

	mu     sync.Mutex
	caller []byte
	agent  []byte

	// f, dataBytes and writeFails are owned by the mux goroutine until it exits;
	// Close waits on muxDone before touching them.
	f          *os.File
	dataBytes  int64
	startedAt  time.Time
	closeOnce  sync.Once
	done       chan struct{}
	muxDone    chan struct{}
	writeFails int

	droppedCaller int64
	droppedAgent  int64
}

// New prepares a recording for one session. It returns (nil, nil) when recording
// is disabled — that is not an error, and every Recorder method tolerates nil.
func New(cfg Config, meta Meta) (*Recorder, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(meta.SessionID) == "" {
		return nil, errors.New("record: session id required")
	}
	rate := meta.SampleRate
	if rate <= 0 {
		rate = 8000
	}
	frameMs := meta.FrameMs
	if frameMs <= 0 {
		frameMs = 20
	}
	root := strings.TrimSpace(cfg.Root)
	if root == "" {
		root = DefaultRoot
	}

	now := time.Now()
	dir := filepath.Join(root, now.Format("2006-01-02"), dirName(meta))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("record: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, "session.wav")
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("record: create %s: %w", path, err)
	}
	if _, err := f.Write(make([]byte, wavHeaderBytes)); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("record: reserve header: %w", err)
	}

	r := &Recorder{
		dir:        dir,
		path:       path,
		meta:       meta,
		rate:       rate,
		frameBytes: rate * bytesPerSample / 1000 * frameMs,
		cadence:    time.Duration(frameMs) * time.Millisecond,
		f:          f,
		startedAt:  now,
		done:       make(chan struct{}),
		muxDone:    make(chan struct{}),
	}
	if r.frameBytes <= 0 {
		_ = f.Close()
		return nil, errors.New("record: invalid frame size")
	}
	go r.mux()
	applog.Info("session recording started", "session", meta.SessionID, "path", path)
	return r, nil
}

// dirName is "<session-id>" or "<session-id>-<call-id>" with anything that is
// not filesystem-safe stripped, so a hostile SIP header cannot escape the root.
func dirName(m Meta) string {
	name := sanitize(m.SessionID)
	if id := sanitize(m.CallID); id != "" {
		name += "-" + id
	}
	return name
}

func sanitize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 96 {
			break
		}
	}
	return b.String()
}

// Path is the recording file path, or "" when not recording. It is what gets
// stored as the session's recording_ref.
func (r *Recorder) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

// Dir is the recording directory, or "" when not recording.
func (r *Recorder) Dir() string {
	if r == nil {
		return ""
	}
	return r.dir
}

// WriteCaller appends inbound (caller → us) canonical PCM.
func (r *Recorder) WriteCaller(pcm []byte) { r.write(true, pcm) }

// WriteAgent appends outbound (us → caller) canonical PCM.
func (r *Recorder) WriteAgent(pcm []byte) { r.write(false, pcm) }

func (r *Recorder) write(callerLeg bool, pcm []byte) {
	if r == nil || len(pcm) == 0 {
		return
	}
	select {
	case <-r.done:
		return
	default:
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if callerLeg {
		if len(r.caller)+len(pcm) > maxLegBacklogBytes {
			r.droppedCaller += int64(len(pcm))
			return
		}
		r.caller = append(r.caller, pcm...)
		return
	}
	if len(r.agent)+len(pcm) > maxLegBacklogBytes {
		r.droppedAgent += int64(len(pcm))
		return
	}
	r.agent = append(r.agent, pcm...)
}

// mux writes one stereo frame per cadence tick. The number of frames due is
// derived from elapsed wall time rather than from tick count, so a scheduling
// hiccup shifts nothing: the recording stays aligned to real time.
func (r *Recorder) mux() {
	defer close(r.muxDone)
	ticker := time.NewTicker(r.cadence)
	defer ticker.Stop()
	lastPatch := time.Now()
	var written int64 // frames

	for {
		select {
		case <-r.done:
			return
		case <-ticker.C:
		}

		due := int64(time.Since(r.startedAt) / r.cadence)
		if due <= written {
			continue
		}
		// Bound catch-up so a long stall cannot produce a huge burst of writes.
		if due-written > 250 {
			due = written + 250
		}
		for ; written < due; written++ {
			if !r.writeFrame() {
				return
			}
		}
		if time.Since(lastPatch) >= headerPatchEvery {
			lastPatch = time.Now()
			r.patchHeader()
		}
	}
}

// writeFrame emits one interleaved stereo frame, padding either leg with
// silence. Returns false when the recording has been shut down by an I/O error.
func (r *Recorder) writeFrame() bool {
	n := r.frameBytes

	r.mu.Lock()
	left := takeFront(&r.caller, n)
	right := takeFront(&r.agent, n)
	r.mu.Unlock()

	out := make([]byte, 0, n*channels)
	for i := 0; i+1 < n; i += 2 {
		out = append(out, left[i], left[i+1], right[i], right[i+1])
	}

	if _, err := r.f.Write(out); err != nil {
		r.writeFails++
		if r.writeFails == 1 {
			applog.Error("session recording write failed; stopping recorder",
				"session", r.meta.SessionID, "path", r.path, "err", err)
		}
		// Disk full or unlinked: stop rather than spin on every tick.
		r.closeOnce.Do(func() { close(r.done) })
		return false
	}
	r.dataBytes += int64(len(out))
	return true
}

// takeFront removes and returns the first n bytes of buf, zero-padded when buf
// holds fewer. The returned slice is always exactly n bytes.
func takeFront(buf *[]byte, n int) []byte {
	out := make([]byte, n)
	b := *buf
	if len(b) == 0 {
		return out
	}
	c := copy(out, b)
	if c >= len(b) {
		*buf = b[:0]
	} else {
		*buf = append(b[:0], b[c:]...)
	}
	return out
}

// Close finalises the WAV header and writes meta.json. Safe on nil and safe to
// call more than once.
func (r *Recorder) Close(sum Summary) {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() { close(r.done) })

	// Wait for the mux goroutine to stop before touching the file: from here on
	// this goroutine is the sole owner of f/dataBytes/writeFails.
	select {
	case <-r.muxDone:
	case <-time.After(2 * time.Second):
		applog.Warn("session recording mux did not stop in time", "session", r.meta.SessionID)
		return
	}

	// One last drain so trailing audio already buffered is not lost.
	r.mu.Lock()
	tail := len(r.caller) > 0 || len(r.agent) > 0
	r.mu.Unlock()
	if tail && r.writeFails == 0 {
		for i := 0; i < 500; i++ {
			r.mu.Lock()
			empty := len(r.caller) == 0 && len(r.agent) == 0
			r.mu.Unlock()
			if empty || !r.writeFrame() {
				break
			}
		}
	}

	r.patchHeader()
	if err := r.f.Close(); err != nil {
		applog.Warn("session recording close", "session", r.meta.SessionID, "err", err)
	}
	r.writeMeta(sum)
	applog.Info("session recording finished",
		"session", r.meta.SessionID, "path", r.path,
		"duration_s", r.durationSeconds(),
		"dropped_caller_bytes", r.droppedCaller,
		"dropped_agent_bytes", r.droppedAgent)
}

func (r *Recorder) durationSeconds() float64 {
	perSec := float64(r.rate * bytesPerSample * channels)
	if perSec == 0 {
		return 0
	}
	return float64(r.dataBytes) / perSec
}

// patchHeader rewrites the RIFF/data sizes in place for the bytes written so far.
func (r *Recorder) patchHeader() {
	h := wavHeader(r.rate, r.dataBytes)
	if _, err := r.f.WriteAt(h, 0); err != nil {
		applog.Warn("session recording header patch", "session", r.meta.SessionID, "err", err)
		return
	}
	// Keep the append position at the end of the data we have written.
	if _, err := r.f.Seek(0, 2); err != nil {
		applog.Warn("session recording seek", "session", r.meta.SessionID, "err", err)
	}
}

// wavHeader builds a canonical 44-byte PCM WAVE header for stereo s16le.
func wavHeader(rate int, dataBytes int64) []byte {
	h := make([]byte, wavHeaderBytes)
	blockAlign := channels * bytesPerSample
	byteRate := rate * blockAlign

	copy(h[0:4], "RIFF")
	binary.LittleEndian.PutUint32(h[4:8], uint32(36+dataBytes))
	copy(h[8:12], "WAVE")
	copy(h[12:16], "fmt ")
	binary.LittleEndian.PutUint32(h[16:20], 16) // PCM fmt chunk size
	binary.LittleEndian.PutUint16(h[20:22], 1)  // PCM
	binary.LittleEndian.PutUint16(h[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(h[24:28], uint32(rate))
	binary.LittleEndian.PutUint32(h[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(h[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(h[34:36], uint16(bytesPerSample*8))
	copy(h[36:40], "data")
	binary.LittleEndian.PutUint32(h[40:44], uint32(dataBytes))
	return h
}

type metaFile struct {
	SessionID     string         `json:"session_id"`
	CallID        string         `json:"call_id,omitempty"`
	SIPCallID     string         `json:"sip_call_id,omitempty"`
	TenantID      string         `json:"tenant_id,omitempty"`
	ProfileID     string         `json:"profile_id,omitempty"`
	CallerANI     string         `json:"caller_ani,omitempty"`
	Destination   string         `json:"destination,omitempty"`
	File          string         `json:"file"`
	Format        string         `json:"format"`
	SampleRateHz  int            `json:"sample_rate_hz"`
	Channels      int            `json:"channels"`
	ChannelMap    string         `json:"channel_map"`
	StartedAt     time.Time      `json:"started_at"`
	EndedAt       time.Time      `json:"ended_at"`
	DurationSec   float64        `json:"duration_seconds"`
	Disposition   string         `json:"disposition,omitempty"`
	EndReason     string         `json:"end_reason,omitempty"`
	DroppedCaller int64          `json:"dropped_caller_bytes"`
	DroppedAgent  int64          `json:"dropped_agent_bytes"`
	Extra         map[string]any `json:"extra,omitempty"`
}

func (r *Recorder) writeMeta(sum Summary) {
	m := metaFile{
		SessionID:     r.meta.SessionID,
		CallID:        r.meta.CallID,
		SIPCallID:     r.meta.SIPCallID,
		TenantID:      r.meta.TenantID,
		ProfileID:     r.meta.ProfileID,
		CallerANI:     r.meta.CallerANI,
		Destination:   r.meta.Dest,
		File:          filepath.Base(r.path),
		Format:        "wav/pcm_s16le",
		SampleRateHz:  r.rate,
		Channels:      channels,
		ChannelMap:    "left=caller,right=agent",
		StartedAt:     r.startedAt,
		EndedAt:       time.Now(),
		DurationSec:   r.durationSeconds(),
		Disposition:   sum.Disposition,
		EndReason:     sum.EndReason,
		DroppedCaller: r.droppedCaller,
		DroppedAgent:  r.droppedAgent,
		Extra:         sum.Extra,
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		applog.Warn("session recording meta encode", "session", r.meta.SessionID, "err", err)
		return
	}
	// Write via a temp file so a reader never sees a half-written meta.json.
	tmp := filepath.Join(r.dir, ".meta.json.tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		applog.Warn("session recording meta write", "session", r.meta.SessionID, "err", err)
		return
	}
	if err := os.Rename(tmp, filepath.Join(r.dir, "meta.json")); err != nil {
		applog.Warn("session recording meta rename", "session", r.meta.SessionID, "err", err)
	}
}

// Sweep removes date directories under root older than retentionDays. It is
// safe to call on a schedule; it never touches today's directory.
func Sweep(root string, retentionDays int) (removed int, err error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	root = strings.TrimSpace(root)
	if root == "" {
		root = DefaultRoot
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	today := time.Now().Format("2006-01-02")
	for _, e := range entries {
		if !e.IsDir() || e.Name() == today {
			continue
		}
		day, perr := time.Parse("2006-01-02", e.Name())
		if perr != nil {
			continue // not one of ours
		}
		if day.After(cutoff) {
			continue
		}
		p := filepath.Join(root, e.Name())
		if rerr := os.RemoveAll(p); rerr != nil {
			applog.Warn("recording retention sweep", "dir", p, "err", rerr)
			continue
		}
		removed++
		applog.Info("recording retention removed", "dir", p, "retention_days", retentionDays)
	}
	return removed, nil
}

// Package fallback stores the operator-uploaded prompts played when the AI
// pipeline cannot serve a call — vendor outage, exhausted credits, timeouts, or
// any unhandled error. The prompt is played to the caller and the call is then
// released, so the caller always hears something intelligible instead of dead
// air or an abrupt drop.
//
// Prompts are system-level assets, not per-profile content: they must work when
// the profile pipeline itself is the thing that is broken.
//
// Layout:
//
//	<root>/<tenant>/<scenario>.wav     tenant override
//	<root>/_default/<scenario>.wav     applies to every tenant
//
// Resolution falls back tenant→default and scenario→generic, so a single
// uploaded generic.wav is enough to cover every failure mode.
package fallback

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/audio"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

// DefaultRoot is where fallback prompts live on disk.
const DefaultRoot = "/usr/local/share/ai-orchestrator/fallback"

// defaultTenant is the directory used for prompts that apply to every tenant.
const defaultTenant = "_default"

const (
	// MaxUploadBytes caps an uploaded prompt (10 MB ≈ 10 min of 8 kHz mono).
	MaxUploadBytes = 10 << 20
	// MaxDurationMs rejects prompts too long to be a failure announcement.
	//
	// It must not exceed the edge's downlink queue depth (60 s), or the tail of a
	// prompt would be silently discarded at playout rather than rejected here —
	// exactly the kind of surprise this store exists to prevent.
	MaxDurationMs = 60_000
)

// Scenario is the closed set of failure modes a prompt can be bound to.
type Scenario string

const (
	// ScenarioAIUnavailable — a required engine (STT/LLM/TTS) is down or unroutable.
	ScenarioAIUnavailable Scenario = "ai_unavailable"
	// ScenarioCreditsExhausted — vendor rejected us for auth/quota/billing reasons.
	ScenarioCreditsExhausted Scenario = "credits_exhausted"
	// ScenarioTimeout — an engine accepted the request but did not answer in time.
	ScenarioTimeout Scenario = "timeout"
	// ScenarioSystemBusy — we refused the work ourselves (capacity, rate limit).
	ScenarioSystemBusy Scenario = "system_busy"
	// ScenarioInternalError — a bug or an error we did not classify.
	ScenarioInternalError Scenario = "internal_error"
	// ScenarioGeneric — the catch-all used when no specific prompt is uploaded.
	ScenarioGeneric Scenario = "generic"
)

// Scenarios is every valid scenario, in a stable order for listings and docs.
var Scenarios = []Scenario{
	ScenarioAIUnavailable,
	ScenarioCreditsExhausted,
	ScenarioTimeout,
	ScenarioSystemBusy,
	ScenarioInternalError,
	ScenarioGeneric,
}

// ParseScenario validates s against the closed set.
func ParseScenario(s string) (Scenario, error) {
	want := Scenario(strings.ToLower(strings.TrimSpace(s)))
	for _, v := range Scenarios {
		if v == want {
			return v, nil
		}
	}
	return "", fmt.Errorf("fallback: unknown scenario %q (valid: %s)", s, joinScenarios())
}

func joinScenarios() string {
	parts := make([]string, 0, len(Scenarios))
	for _, s := range Scenarios {
		parts = append(parts, string(s))
	}
	return strings.Join(parts, ", ")
}

// Classify maps a pipeline error to the scenario whose prompt should be played.
// Unclassified errors deliberately land on ScenarioInternalError rather than
// being silently treated as a normal end of call.
func Classify(err error) Scenario {
	if err == nil {
		return ScenarioInternalError
	}
	ge, ok := port.AsGatewayError(err)
	if !ok {
		return ScenarioInternalError
	}
	switch ge.Code {
	case port.CodeAuth, port.CodeRateLimit:
		// Both are how vendors report "you have run out" — expired key, no
		// credit, quota tripped. The caller-facing outcome is the same.
		return ScenarioCreditsExhausted
	case port.CodeUnavailable:
		return ScenarioAIUnavailable
	case port.CodeTimeout:
		return ScenarioTimeout
	case port.CodeUnsupported, port.CodeBadRequest, port.CodeBadAudio:
		return ScenarioSystemBusy
	default:
		return ScenarioInternalError
	}
}

// Asset is a decoded prompt ready to stream.
type Asset struct {
	Scenario   Scenario `json:"scenario"`
	TenantID   string   `json:"tenant_id,omitempty"`
	Path       string   `json:"path"`
	SampleRate int      `json:"sample_rate_hz"`
	DurationMs int      `json:"duration_ms"`
	Bytes      int      `json:"bytes"`
	UpdatedAt  string   `json:"updated_at"`

	// PCM is mono s16le at SampleRate. Nil in listings.
	PCM []byte `json:"-"`
}

// Store is the on-disk prompt store with an in-process decode cache.
type Store struct {
	root string

	mu    sync.RWMutex
	cache map[string]cached
}

type cached struct {
	modTime time.Time
	size    int64
	wav     audio.WAV
}

// NewStore opens (and creates) the prompt root.
func NewStore(root string) (*Store, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = DefaultRoot
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("fallback: mkdir %s: %w", root, err)
	}
	return &Store{root: root, cache: make(map[string]cached)}, nil
}

// Root is the configured prompt root.
func (s *Store) Root() string { return s.root }

// tenantDir maps a tenant id to its directory, guarding against traversal.
func tenantDir(tenantID string) string {
	t := strings.TrimSpace(tenantID)
	if t == "" {
		return defaultTenant
	}
	var b strings.Builder
	for _, r := range t {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 64 {
			break
		}
	}
	if b.Len() == 0 {
		return defaultTenant
	}
	return b.String()
}

func (s *Store) path(tenantID string, sc Scenario) string {
	return filepath.Join(s.root, tenantDir(tenantID), string(sc)+".wav")
}

// Put validates and stores a prompt. data must be an uncompressed 16-bit PCM
// WAVE file; it is normalised to mono on the way in so playback never has to
// guess. The write is atomic, so a concurrent call never reads a partial file.
func (s *Store) Put(tenantID string, sc Scenario, data []byte) (Asset, error) {
	if len(data) == 0 {
		return Asset{}, errors.New("fallback: empty upload")
	}
	if len(data) > MaxUploadBytes {
		return Asset{}, fmt.Errorf("fallback: upload is %d bytes, limit is %d", len(data), MaxUploadBytes)
	}
	w, err := audio.DecodeWAV(data)
	if err != nil {
		if errors.Is(err, audio.ErrNotWAV) {
			return Asset{}, errors.New("fallback: upload must be a RIFF/WAVE file")
		}
		return Asset{}, err
	}
	if w.DurationMs > MaxDurationMs {
		return Asset{}, fmt.Errorf("fallback: prompt is %d ms, limit is %d ms", w.DurationMs, MaxDurationMs)
	}

	// Store the normalised mono form so playback is deterministic regardless of
	// what the operator uploaded.
	normalised := audio.EncodeWAV(w.PCM, w.SampleRate)

	dst := s.path(tenantID, sc)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return Asset{}, fmt.Errorf("fallback: mkdir: %w", err)
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, normalised, 0o644); err != nil {
		return Asset{}, fmt.Errorf("fallback: write: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return Asset{}, fmt.Errorf("fallback: install: %w", err)
	}

	s.mu.Lock()
	delete(s.cache, dst)
	s.mu.Unlock()

	return s.describe(tenantID, sc, dst, w), nil
}

// Delete removes a prompt. Missing is not an error.
func (s *Store) Delete(tenantID string, sc Scenario) error {
	p := s.path(tenantID, sc)
	s.mu.Lock()
	delete(s.cache, p)
	s.mu.Unlock()
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// List returns the prompts visible to tenantID: its own overrides plus any
// defaults it inherits.
func (s *Store) List(tenantID string) []Asset {
	seen := make(map[Scenario]Asset)
	for _, dir := range []string{defaultTenant, tenantDir(tenantID)} {
		for _, sc := range Scenarios {
			p := filepath.Join(s.root, dir, string(sc)+".wav")
			w, err := s.load(p)
			if err != nil {
				continue
			}
			owner := ""
			if dir != defaultTenant {
				owner = tenantID
			}
			seen[sc] = s.describe(owner, sc, p, w)
		}
	}
	out := make([]Asset, 0, len(seen))
	for _, sc := range Scenarios {
		if a, ok := seen[sc]; ok {
			out = append(out, a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Scenario < out[j].Scenario })
	return out
}

// Resolve finds the prompt to play for (tenant, scenario), widening to the
// generic prompt and then to the default tenant. Returns ok=false when the
// operator has uploaded nothing usable — the caller must then still end the
// call cleanly, just without an announcement.
func (s *Store) Resolve(tenantID string, sc Scenario) (Asset, bool) {
	type candidate struct {
		tenant string
		sc     Scenario
	}
	candidates := []candidate{
		{tenantID, sc},
		{tenantID, ScenarioGeneric},
		{"", sc},
		{"", ScenarioGeneric},
	}
	for _, c := range candidates {
		p := s.path(c.tenant, c.sc)
		w, err := s.load(p)
		if err != nil {
			continue
		}
		a := s.describe(c.tenant, c.sc, p, w)
		a.PCM = w.PCM
		// Report the scenario that was asked for; the file that served it is in Path.
		a.Scenario = sc
		return a, true
	}
	return Asset{}, false
}

func (s *Store) describe(tenantID string, sc Scenario, path string, w audio.WAV) Asset {
	a := Asset{
		Scenario:   sc,
		TenantID:   tenantID,
		Path:       path,
		SampleRate: w.SampleRate,
		DurationMs: w.DurationMs,
		Bytes:      len(w.PCM),
	}
	if st, err := os.Stat(path); err == nil {
		a.UpdatedAt = st.ModTime().UTC().Format(time.RFC3339)
	}
	return a
}

// load reads and decodes a prompt, caching the result until the file changes.
func (s *Store) load(path string) (audio.WAV, error) {
	st, err := os.Stat(path)
	if err != nil {
		return audio.WAV{}, err
	}
	if st.IsDir() {
		return audio.WAV{}, errors.New("fallback: not a file")
	}

	s.mu.RLock()
	c, ok := s.cache[path]
	s.mu.RUnlock()
	if ok && c.modTime.Equal(st.ModTime()) && c.size == st.Size() {
		return c.wav, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return audio.WAV{}, err
	}
	w, err := audio.DecodeWAV(raw)
	if err != nil {
		return audio.WAV{}, err
	}

	s.mu.Lock()
	s.cache[path] = cached{modTime: st.ModTime(), size: st.Size(), wav: w}
	s.mu.Unlock()
	return w, nil
}

// Raw returns the stored WAV bytes for download/inspection.
func (s *Store) Raw(tenantID string, sc Scenario) ([]byte, error) {
	return os.ReadFile(s.path(tenantID, sc))
}

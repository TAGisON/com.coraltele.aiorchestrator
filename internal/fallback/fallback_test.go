package fallback

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/audio"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

// wav builds a valid mono PCM WAVE of roughly durMs at rate.
func wav(t *testing.T, rate, durMs int) []byte {
	t.Helper()
	n := rate * 2 * durMs / 1000
	if n%2 == 1 {
		n++
	}
	return audio.EncodeWAV(make([]byte, n), rate)
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestParseScenario(t *testing.T) {
	if _, err := ParseScenario("credits_exhausted"); err != nil {
		t.Fatalf("valid scenario rejected: %v", err)
	}
	if _, err := ParseScenario("  AI_UNAVAILABLE "); err != nil {
		t.Fatalf("case/space should be tolerated: %v", err)
	}
	if _, err := ParseScenario("../../etc/passwd"); err == nil {
		t.Fatal("path-like scenario must be rejected")
	}
	if _, err := ParseScenario("nonsense"); err == nil {
		t.Fatal("unknown scenario must be rejected")
	}
}

// Classification drives which prompt a caller hears, so each vendor failure
// mode must land on the right one.
func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want Scenario
	}{
		{"auth is out-of-credit", &port.GatewayError{Code: port.CodeAuth}, ScenarioCreditsExhausted},
		{"rate limit is out-of-credit", &port.GatewayError{Code: port.CodeRateLimit}, ScenarioCreditsExhausted},
		{"unavailable", &port.GatewayError{Code: port.CodeUnavailable}, ScenarioAIUnavailable},
		{"timeout", &port.GatewayError{Code: port.CodeTimeout}, ScenarioTimeout},
		{"bad request", &port.GatewayError{Code: port.CodeBadRequest}, ScenarioSystemBusy},
		{"internal", &port.GatewayError{Code: port.CodeInternal}, ScenarioInternalError},
		{"plain error", errors.New("boom"), ScenarioInternalError},
		{"nil", nil, ScenarioInternalError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.err); got != tc.want {
				t.Fatalf("Classify = %s, want %s", got, tc.want)
			}
		})
	}
}

// The composer wraps engine errors before they reach us ("answer speak: %w"),
// so classification has to see through the wrapping or every outage would be
// misreported as an internal error.
func TestClassifyUnwrapsWrappedGatewayError(t *testing.T) {
	inner := &port.GatewayError{Code: port.CodeAuth, Message: "no credits available"}
	wrapped := fmt.Errorf("answer speak: %w", inner)
	if got := Classify(wrapped); got != ScenarioCreditsExhausted {
		t.Fatalf("Classify(wrapped) = %s, want %s", got, ScenarioCreditsExhausted)
	}
}

func TestPutRejectsNonWavAndOversize(t *testing.T) {
	s := newStore(t)
	if _, err := s.Put("t1", ScenarioGeneric, []byte("this is not audio")); err == nil {
		t.Fatal("non-WAV upload must be rejected")
	}
	if _, err := s.Put("t1", ScenarioGeneric, nil); err == nil {
		t.Fatal("empty upload must be rejected")
	}
	big := make([]byte, MaxUploadBytes+1)
	if _, err := s.Put("t1", ScenarioGeneric, big); err == nil {
		t.Fatal("oversize upload must be rejected")
	}
}

func TestPutRejectsUnsupportedFormats(t *testing.T) {
	s := newStore(t)
	// 8-bit PCM: valid RIFF, unusable for us.
	bad := audio.EncodeWAV(make([]byte, 320), 8000)
	bad[34] = 8 // bitsPerSample = 8
	if _, err := s.Put("t1", ScenarioGeneric, bad); err == nil {
		t.Fatal("8-bit PCM must be rejected")
	}
	// Sample rate below the supported range.
	slow := audio.EncodeWAV(make([]byte, 320), 4000)
	if _, err := s.Put("t1", ScenarioGeneric, slow); err == nil {
		t.Fatal("out-of-range sample rate must be rejected")
	}
}

func TestPutAndResolve(t *testing.T) {
	s := newStore(t)
	if _, err := s.Put("t1", ScenarioCreditsExhausted, wav(t, 8000, 500)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	a, ok := s.Resolve("t1", ScenarioCreditsExhausted)
	if !ok {
		t.Fatal("expected the uploaded prompt to resolve")
	}
	if len(a.PCM) == 0 {
		t.Fatal("resolved prompt has no audio")
	}
	if a.SampleRate != 8000 {
		t.Fatalf("rate = %d", a.SampleRate)
	}
	if a.Scenario != ScenarioCreditsExhausted {
		t.Fatalf("scenario = %s", a.Scenario)
	}
}

// One uploaded generic.wav must cover every scenario — that is the whole point
// of the widening rules.
func TestResolveWidensToGenericThenDefaultTenant(t *testing.T) {
	s := newStore(t)

	// Nothing uploaded yet.
	if _, ok := s.Resolve("t1", ScenarioTimeout); ok {
		t.Fatal("must not resolve with an empty store")
	}

	// A default-tenant generic covers an unrelated tenant and scenario.
	if _, err := s.Put("", ScenarioGeneric, wav(t, 8000, 200)); err != nil {
		t.Fatalf("Put default generic: %v", err)
	}
	if _, ok := s.Resolve("t1", ScenarioTimeout); !ok {
		t.Fatal("default generic must cover any tenant/scenario")
	}

	// A tenant-specific prompt wins over the default.
	if _, err := s.Put("t1", ScenarioTimeout, wav(t, 16000, 300)); err != nil {
		t.Fatalf("Put tenant timeout: %v", err)
	}
	a, ok := s.Resolve("t1", ScenarioTimeout)
	if !ok {
		t.Fatal("tenant prompt must resolve")
	}
	if a.SampleRate != 16000 {
		t.Fatalf("tenant override not preferred, rate = %d", a.SampleRate)
	}
}

func TestStereoUploadIsNormalisedToMono(t *testing.T) {
	s := newStore(t)
	// Hand-build a stereo WAV: 100 frames × 2 ch × 2 bytes.
	stereo := audio.EncodeWAV(make([]byte, 400), 8000)
	stereo[22] = 2                                       // channels = 2
	stereo[32] = 4                                       // block align = 4
	a, err := s.Put("t1", ScenarioGeneric, stereo)
	if err != nil {
		t.Fatalf("stereo upload rejected: %v", err)
	}
	// 400 stereo bytes → 200 mono bytes.
	if a.Bytes != 200 {
		t.Fatalf("expected downmix to 200 mono bytes, got %d", a.Bytes)
	}
}

func TestDeleteAndList(t *testing.T) {
	s := newStore(t)
	if _, err := s.Put("t1", ScenarioGeneric, wav(t, 8000, 100)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Put("t1", ScenarioAIUnavailable, wav(t, 8000, 100)); err != nil {
		t.Fatal(err)
	}
	if got := len(s.List("t1")); got != 2 {
		t.Fatalf("List = %d prompts, want 2", got)
	}
	if err := s.Delete("t1", ScenarioGeneric); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := len(s.List("t1")); got != 1 {
		t.Fatalf("after delete List = %d, want 1", got)
	}
	// Deleting something absent is not an error.
	if err := s.Delete("t1", ScenarioTimeout); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestPutIsAtomicAndCacheInvalidates(t *testing.T) {
	s := newStore(t)
	if _, err := s.Put("t1", ScenarioGeneric, wav(t, 8000, 100)); err != nil {
		t.Fatal(err)
	}
	first, _ := s.Resolve("t1", ScenarioGeneric)

	// Replace with a longer prompt; the cache must notice.
	if _, err := s.Put("t1", ScenarioGeneric, wav(t, 8000, 400)); err != nil {
		t.Fatal(err)
	}
	second, _ := s.Resolve("t1", ScenarioGeneric)
	if len(second.PCM) <= len(first.PCM) {
		t.Fatalf("cache not invalidated: %d then %d bytes", len(first.PCM), len(second.PCM))
	}

	// No temp files left behind.
	matches, _ := filepath.Glob(filepath.Join(s.Root(), "*", "*.tmp"))
	if len(matches) != 0 {
		t.Fatalf("temp files left behind: %v", matches)
	}
}

func TestTenantIdCannotEscapeRoot(t *testing.T) {
	s := newStore(t)
	if _, err := s.Put("../../evil", ScenarioGeneric, wav(t, 8000, 100)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Everything must stay under the root.
	var found bool
	_ = filepath.Walk(s.Root(), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(p) == ".wav" {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatal("prompt was written outside the store root")
	}
}

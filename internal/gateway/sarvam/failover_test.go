package sarvam_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvam"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvamstt"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
)

func TestLoadConfigFromSecretsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.local.json")
	content := `{
  "sarvam": {
    "api_key": "from-file",
    "stt_rest_url": "https://example.test/stt",
    "chat_url": "https://example.test/chat"
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIORCH_SECRETS_FILE", path)
	t.Setenv("SARVAM_API_KEY", "")
	cfg, err := sarvam.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "from-file" {
		t.Fatalf("key=%q", cfg.APIKey)
	}
	if cfg.STTRestURL != "https://example.test/stt" {
		t.Fatalf("stt url=%q", cfg.STTRestURL)
	}
	if cfg.ChatURL != "https://example.test/chat" {
		t.Fatalf("chat url=%q", cfg.ChatURL)
	}
}

func TestEnvOverridesSecretsKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.local.json")
	if err := os.WriteFile(path, []byte(`{"sarvam":{"api_key":"from-file"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIORCH_SECRETS_FILE", path)
	t.Setenv("SARVAM_API_KEY", "from-env")
	cfg, err := sarvam.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "from-env" {
		t.Fatalf("key=%q", cfg.APIKey)
	}
}

func TestProfileFailoverSkipsUnhealthySarvam(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	// Register sarvam-stt with unhealthy probe (simulates missing key / down vendor).
	stt := &sarvamstt.Gateway{Cfg: sarvam.Config{APIKey: "present-but-unhealthy"}}
	if err := reg.Register(port.Registration{
		ID:           sarvamstt.ID,
		Port:         port.PortListen,
		Capabilities: stt.Capabilities(),
		Instance:     stt,
		Probe: func(ctx context.Context) port.Health {
			return port.Health{Healthy: false, LastError: "simulated unhealthy"}
		},
	}); err != nil {
		t.Fatal(err)
	}

	rec, err := router.Select(reg,
		[]port.GatewayID{sarvamstt.ID, fake.IDListen},
		port.PortListen,
		router.SelectOptions{Clock: "live"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != fake.IDListen {
		t.Fatalf("want fake-listen got %s", rec.ID)
	}
}

func TestProfileFailoverSkipsUnregisteredSarvam(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	rec, err := router.Select(reg,
		[]port.GatewayID{"sarvam-stt", "sarvam-llm", "sarvam-tts", fake.IDListen},
		port.PortListen,
		router.SelectOptions{Clock: "live"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != fake.IDListen {
		t.Fatalf("want fake-listen got %s", rec.ID)
	}
	_ = time.Now()
}

func TestMapHTTPStatus(t *testing.T) {
	ge := sarvam.MapHTTPStatus(401, "nope")
	if ge.Code != port.CodeAuth {
		t.Fatalf("code=%s", ge.Code)
	}
	ge = sarvam.MapHTTPStatus(429, "slow")
	if ge.Code != port.CodeRateLimit || !ge.Retryable {
		t.Fatalf("%+v", ge)
	}
}

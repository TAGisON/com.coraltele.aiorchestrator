// Package sarvam holds shared Sarvam lab config (API key + endpoint URLs).
// Secrets come from env / .agent/secrets.local.json only — never log the key.
package sarvam

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

const (
	DefaultSTTRestURL = "https://api.sarvam.ai/speech-to-text"
	DefaultSTTWSURL   = "wss://api.sarvam.ai/speech-to-text/ws"
	DefaultChatURL    = "https://api.sarvam.ai/v1/chat/completions"
	DefaultTTSURL     = "https://api.sarvam.ai/text-to-speech"

	HeaderAPIKey = "api-subscription-key"

	// DefaultChatModel is Sarvam's recommended model for realtime conversational / voice-agent workloads
	// (docs.sarvam.ai chat completions — sarvam-105b-conversations on /v1).
	DefaultChatModel = "sarvam-105b-conversations"

	DefaultSTTModel    = "saaras:v3"
	DefaultSTTLanguage = "en-IN" // TTS / explicit callers only — STT empty/auto uses unknown (LANGUAGE_POLICY)
	DefaultSTTMode     = "transcribe"
	// STTLanguageUnknown is Sarvam auto-detect when LanguageHint is empty or "auto".
	STTLanguageUnknown = "unknown"

	DefaultTTSModel   = "bulbul:v3"
	DefaultTTSSpeaker = "shubh" // Bulbul v3 default voice per Sarvam docs
)

// Config is shared by sarvam-stt / sarvam-llm / sarvam-tts.
type Config struct {
	APIKey     string
	STTRestURL string
	STTWSURL   string
	ChatURL    string
	TTSURL     string
}

// Configured reports whether a non-empty API key is present.
func (c Config) Configured() bool {
	return strings.TrimSpace(c.APIKey) != ""
}

// STTLanguageCode maps Listen LanguageHint to Sarvam language_code / language-code.
// Empty or case-insensitive "auto" → unknown (auto-detect). Concrete BCP-47 passes through.
// Does not substitute DefaultSTTLanguage (en-IN).
func STTLanguageCode(hint string) string {
	h := strings.TrimSpace(hint)
	if h == "" || strings.EqualFold(h, "auto") {
		return STTLanguageUnknown
	}
	return h
}

// LoadConfig reads API key from KeyProvider (DB), then env, then secrets.local.json.
// Missing key is not an error — Configured() is false.
func LoadConfig() (Config, error) {
	cfg := Config{
		APIKey:     "",
		STTRestURL: DefaultSTTRestURL,
		STTWSURL:   DefaultSTTWSURL,
		ChatURL:    DefaultChatURL,
		TTSURL:     DefaultTTSURL,
	}
	if keyProvider != nil {
		if k, err := keyProvider(context.Background()); err == nil {
			cfg.APIKey = strings.TrimSpace(k)
		}
	}
	if cfg.APIKey == "" {
		cfg.APIKey = strings.TrimSpace(os.Getenv("SARVAM_API_KEY"))
	}
	path := secretsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	var root struct {
		Sarvam *struct {
			APIKey     string `json:"api_key"`
			STTRestURL string `json:"stt_rest_url"`
			STTWSURL   string `json:"stt_ws_url"`
			ChatURL    string `json:"chat_url"`
			TTSURL     string `json:"tts_url"`
		} `json:"sarvam"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return cfg, err
	}
	if root.Sarvam == nil {
		return cfg, nil
	}
	if cfg.APIKey == "" {
		cfg.APIKey = strings.TrimSpace(root.Sarvam.APIKey)
	}
	if v := strings.TrimSpace(root.Sarvam.STTRestURL); v != "" {
		cfg.STTRestURL = v
	}
	if v := strings.TrimSpace(root.Sarvam.STTWSURL); v != "" {
		cfg.STTWSURL = v
	}
	if v := strings.TrimSpace(root.Sarvam.ChatURL); v != "" {
		cfg.ChatURL = v
	}
	if v := strings.TrimSpace(root.Sarvam.TTSURL); v != "" {
		cfg.TTSURL = v
	}
	return cfg, nil
}

// KeyProvider supplies the Sarvam API key at call time (typically from DB).
type KeyProvider func(ctx context.Context) (string, error)

var keyProvider KeyProvider

// SetKeyProvider installs a runtime key source (DB). Nil clears.
func SetKeyProvider(p KeyProvider) {
	keyProvider = p
}

func secretsPath() string {
	if v := strings.TrimSpace(os.Getenv("AIORCH_SECRETS_FILE")); v != "" {
		return v
	}
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join(".agent", "secrets.local.json")
	}
	dir := wd
	for {
		p := filepath.Join(dir, ".agent", "secrets.local.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(wd, ".agent", "secrets.local.json")
}

// MapHTTPStatus maps Sarvam HTTP status codes to GatewayError.
func MapHTTPStatus(status int, body string) *port.GatewayError {
	msg := truncate(body, 200)
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &port.GatewayError{Code: port.CodeAuth, Message: msg, Retryable: false}
	case status == http.StatusTooManyRequests:
		return &port.GatewayError{Code: port.CodeRateLimit, Message: msg, Retryable: true}
	case status == http.StatusBadRequest:
		return &port.GatewayError{Code: port.CodeBadRequest, Message: msg, Retryable: false}
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		return &port.GatewayError{Code: port.CodeTimeout, Message: msg, Retryable: true}
	case status >= 500:
		return &port.GatewayError{Code: port.CodeUnavailable, Message: msg, Retryable: true}
	default:
		return &port.GatewayError{Code: port.CodeInternal, Message: msg, Retryable: false}
	}
}

// MapDialError maps network / context errors to GatewayError (no secrets in Message).
func MapDialError(err error) *port.GatewayError {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		code := port.CodeTimeout
		if errors.Is(err, context.Canceled) {
			code = port.CodeCancelled
		}
		return &port.GatewayError{Code: code, Message: "request ended", Retryable: code == port.CodeTimeout, Cause: err}
	}
	return &port.GatewayError{Code: port.CodeUnavailable, Message: "transport error", Retryable: true, Cause: err}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// DefaultHTTPClient is a short-timeout client for lab gateways.
func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

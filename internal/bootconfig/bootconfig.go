// Package bootconfig loads committed production properties (boot-only).
package bootconfig

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/fallback"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/record"
)

// Config is process bootstrap configuration (not vendor secrets or tenant engines).
type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	RequireDatabase bool
	LogLevel        string
	LogFormat       string
	LogConfig       string
	LogBase         string
	LogAppName      string
	LogMaxSizeMB    int
	LogMaxBackups   int
	OwnerInstance   string
	EdgeBaseURL     string
	EdgeTokenSecret string
	PropertiesPath  string

	// RecordingEnabled turns per-session call recording on.
	RecordingEnabled bool
	// RecordingRoot is the recordings root. A leading "/" resolves against the
	// current drive on Windows, so the same value works on every OS.
	RecordingRoot string
	// RecordingRetentionDays removes recording day-directories older than this.
	// Zero keeps recordings forever.
	RecordingRetentionDays int
	// FallbackRoot holds operator-uploaded failure prompts.
	FallbackRoot string
	// TransferDialplan / TransferContext are the defaults handed to the edge for
	// `uuid_transfer <uuid> <dest> <dialplan> <context>`.
	TransferDialplan string
	TransferContext  string
}

// Default returns process defaults (port 8011). No vendor or engine presets.
func Default() Config {
	return Config{
		HTTPAddr:        ":8011",
		RequireDatabase: false,
		LogLevel:        "info",
		LogFormat:       "json",
		LogConfig:       "conf/logging.xml",
		LogBase:         "logs/aiorchestrator",
		LogAppName:      "aiorchestrator",
		LogMaxSizeMB:    10,
		LogMaxBackups:   5,
		OwnerInstance:   "local",
		EdgeBaseURL:     "wss://localhost/edge/fs",
		EdgeTokenSecret: "lab-edge-hmac-change-me",

		RecordingEnabled:       true,
		RecordingRoot:          record.DefaultRoot,
		RecordingRetentionDays: 30,
		FallbackRoot:           fallback.DefaultRoot,
		TransferDialplan:       "XML",
		TransferContext:        "calltransfer",
	}
}

// Load reads propertiesPath (if non-empty and exists), then applies env overrides.
func Load(propertiesPath string) (Config, error) {
	cfg := Default()
	path := strings.TrimSpace(propertiesPath)
	if path == "" {
		path = findProperties()
	}
	cfg.PropertiesPath = path
	if path != "" {
		if err := mergeFile(&cfg, path); err != nil && !os.IsNotExist(err) {
			return cfg, err
		}
	}
	applyEnv(&cfg)
	return cfg, nil
}

func findProperties() string {
	if v := strings.TrimSpace(os.Getenv("AIORCH_PROPERTIES")); v != "" {
		return v
	}
	candidates := []string{
		"conf/aiorchestrator.properties",
		filepath.Join("conf", "aiorchestrator.properties"),
	}
	wd, err := os.Getwd()
	if err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			candidates = append(candidates, filepath.Join(dir, "conf", "aiorchestrator.properties"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return "conf/aiorchestrator.properties"
}

func mergeFile(cfg *Config, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "http.addr":
			cfg.HTTPAddr = v
		case "database.url":
			cfg.DatabaseURL = v
		case "database.require":
			cfg.RequireDatabase = truthy(v)
		case "log.level":
			cfg.LogLevel = v
		case "log.format":
			cfg.LogFormat = v
		case "log.config":
			cfg.LogConfig = v
		case "log.base":
			cfg.LogBase = v
		case "log.app_name":
			cfg.LogAppName = v
		case "log.max_size_mb":
			if n, err := parseInt(v); err == nil {
				cfg.LogMaxSizeMB = n
			}
		case "log.max_backups":
			if n, err := parseInt(v); err == nil {
				cfg.LogMaxBackups = n
			}
		case "owner.instance":
			cfg.OwnerInstance = v
		case "edge.base_url":
			cfg.EdgeBaseURL = v
		case "edge.token_secret":
			cfg.EdgeTokenSecret = v
		case "recording.enabled":
			cfg.RecordingEnabled = truthy(v)
		case "recording.root":
			cfg.RecordingRoot = v
		case "recording.retention_days":
			if n, err := parseInt(v); err == nil {
				cfg.RecordingRetentionDays = n
			}
		case "fallback.root":
			cfg.FallbackRoot = v
		case "transfer.dialplan":
			cfg.TransferDialplan = v
		case "transfer.context":
			cfg.TransferContext = v
		}
	}
	return sc.Err()
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("REQUIRE_DATABASE"); v != "" {
		cfg.RequireDatabase = truthy(v)
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.LogFormat = v
	}
	if v := os.Getenv("OWNER_INSTANCE"); v != "" {
		cfg.OwnerInstance = v
	}
	if v := os.Getenv("EDGE_BASE_URL"); v != "" {
		cfg.EdgeBaseURL = v
	}
	if v := os.Getenv("EDGE_TOKEN_SECRET"); v != "" {
		cfg.EdgeTokenSecret = v
	}
	if v := os.Getenv("RECORDING_ENABLED"); v != "" {
		cfg.RecordingEnabled = truthy(v)
	}
	if v := os.Getenv("RECORDING_ROOT"); v != "" {
		cfg.RecordingRoot = v
	}
	if v := os.Getenv("RECORDING_RETENTION_DAYS"); v != "" {
		if n, err := parseInt(v); err == nil {
			cfg.RecordingRetentionDays = n
		}
	}
	if v := os.Getenv("FALLBACK_ROOT"); v != "" {
		cfg.FallbackRoot = v
	}
	if v := os.Getenv("TRANSFER_DIALPLAN"); v != "" {
		cfg.TransferDialplan = v
	}
	if v := os.Getenv("TRANSFER_CONTEXT"); v != "" {
		cfg.TransferContext = v
	}
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseInt(v string) (int, error) {
	n := 0
	for _, c := range strings.TrimSpace(v) {
		if c < '0' || c > '9' {
			return 0, os.ErrInvalid
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

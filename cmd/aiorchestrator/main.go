package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/bootconfig"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/coralcrm"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/coraltransfer"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/ingest"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvam"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvamllm"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvamstt"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/sarvamtts"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
	"github.com/coraltele/com.coraltele.aiorchestrator/web"
)

func main() {
	boot, err := bootconfig.Load("")
	if err != nil {
		applog.Configure("info", "json", "")
		applog.Error("boot properties failed", "err", err)
		os.Exit(1)
	}
	logDir := filepath.Join(boot.LogBase)
	applog.ConfigureRolling(boot.LogLevel, boot.LogFormat, logDir, boot.LogMaxSizeMB, boot.LogMaxBackups)
	applog.Info("boot config loaded",
		"properties", boot.PropertiesPath,
		"http_addr", boot.HTTPAddr,
		"log_dir", logDir,
	)

	control.EngineDefaults = store.GatewayBinding{
		Listen: boot.EnginesListen,
		Think:  boot.EnginesThink,
		Speak:  boot.EnginesSpeak,
	}

	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		applog.Error("register fakes failed", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	databaseURL := strings.TrimSpace(boot.DatabaseURL)
	requireDB := boot.RequireDatabase

	var repo store.Repository
	var closer func()
	storeBackend := "memory"
	if databaseURL == "" {
		if requireDB {
			applog.Error("database.url required (database.require=true)")
			os.Exit(1)
		}
		applog.Warn("database.url unset: using in-memory store (lab only)")
		repo = store.NewMemory()
		closer = func() {}
	} else {
		st, err := store.Open(ctx, databaseURL)
		if err != nil {
			applog.Error("store open failed", "err", err)
			os.Exit(1)
		}
		repo = st
		closer = st.Close
		storeBackend = "postgres"
		applog.Info("postgres connected", "migrations", "applied")
	}
	defer closer()

	// Runtime Sarvam key from DB (tenant default); env/secrets remain lab fallback inside LoadConfig.
	sarvam.SetKeyProvider(func(ctx context.Context) (string, error) {
		return lookupSarvamKey(ctx, repo, "default")
	})

	ing := ingest.New(repo)
	if err := ingest.Register(reg, ing); err != nil {
		applog.Error("register ingest failed", "err", err)
		os.Exit(1)
	}

	coralBase := os.Getenv("CORAL_BASE_URL")
	if st, err := repo.GetSystemSetting(ctx, "default", "coral.base_url"); err == nil {
		coralBase = st.Value
	}
	if err := coraltransfer.Register(reg, &coraltransfer.Gateway{BaseURL: coralBase}); err != nil {
		applog.Error("register coral-transfer failed", "err", err)
		os.Exit(1)
	}
	if err := coralcrm.Register(reg, &coralcrm.Gateway{BaseURL: coralBase}); err != nil {
		applog.Error("register coral-crm failed", "err", err)
		os.Exit(1)
	}

	// Always register Sarvam adapters; they error until DB (or lab env) has a key.
	if err := sarvamstt.Register(reg, nil); err != nil {
		applog.Error("register sarvam-stt failed", "err", err)
		os.Exit(1)
	}
	if err := sarvamllm.Register(reg, nil); err != nil {
		applog.Error("register sarvam-llm failed", "err", err)
		os.Exit(1)
	}
	if err := sarvamtts.Register(reg, nil); err != nil {
		applog.Error("register sarvam-tts failed", "err", err)
		os.Exit(1)
	}
	sarvamConfigured := false
	if cfg, err := sarvam.LoadConfig(); err == nil && cfg.Configured() {
		sarvamConfigured = true
		applog.Info("sarvam credentials available", "source", "db_or_env")
	} else {
		applog.Info("sarvam credentials not set — PUT /v1/tenant/credentials/sarvam with api_key")
	}

	authToken := os.Getenv("CONTROL_AUTH_TOKEN")
	if st, err := repo.GetSystemSetting(ctx, "default", "control.auth_token"); err == nil && st.Value != "" {
		authToken = st.Value
	}

	cfg := control.Config{
		AuthToken:       authToken,
		OwnerInstance:   boot.OwnerInstance,
		EdgeBaseURL:     boot.EdgeBaseURL,
		EdgeTokenSecret: []byte(boot.EdgeTokenSecret),
	}
	mgr := session.NewManager(reg)
	rt := &control.SessionRuntime{Mgr: mgr, Repo: repo}
	srv := control.NewWithRuntime(repo, reg, rt, cfg, web.LabFS)
	srv.SetLabExtras(control.LabExtras{
		StoreBackend:     storeBackend,
		SarvamConfigured: sarvamConfigured,
		HTTPAddr:         boot.HTTPAddr,
	})
	srv.MountEdge(srv.EdgeTokenSecret(), mgr)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	_ = srv.StartPlaybackWorker(workerCtx, mgr)
	_ = srv.StartPostcallWorker(workerCtx)

	addr := boot.HTTPAddr
	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}

	go func() {
		applog.Info("control listening", "addr", addr, "lab_ui", "http://127.0.0.1"+addr+"/lab/", "store", storeBackend)
		kinds := []port.PortKind{
			port.PortListen, port.PortSpeak, port.PortThink,
			port.PortTranslate, port.PortKnowledge, port.PortSkill,
		}
		for _, k := range kinds {
			applog.Info("port gateways", "port", string(k), "count", len(reg.List(k)))
		}
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			applog.Error("http failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	applog.Info("shutdown requested")
	workerCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}

func lookupSarvamKey(ctx context.Context, repo store.Repository, tenantID string) (string, error) {
	ids := []string{"sarvam", "sarvam-stt", "sarvam-llm", "sarvam-tts"}
	for _, id := range ids {
		c, err := repo.GetGatewayCredential(ctx, tenantID, id)
		if err == nil && strings.TrimSpace(c.APIKey) != "" {
			return c.APIKey, nil
		}
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return "", err
		}
	}
	return "", nil
}

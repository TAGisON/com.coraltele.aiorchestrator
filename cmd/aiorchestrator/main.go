package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/applog"
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
	applog.Configure(os.Getenv("LOG_LEVEL"), os.Getenv("LOG_FORMAT"))

	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		applog.Error("register fakes failed", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	databaseURL := envOr("DATABASE_URL", "")
	requireDB := envOr("REQUIRE_DATABASE", "0") == "1" || envOr("REQUIRE_DATABASE", "") == "true"

	var repo store.Repository
	var closer func()
	storeBackend := "memory"
	if databaseURL == "" {
		if requireDB {
			applog.Error("DATABASE_URL required (REQUIRE_DATABASE=1)")
			os.Exit(1)
		}
		applog.Warn("DATABASE_URL unset: using in-memory store (lab only)")
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

	ing := ingest.New(repo)
	if err := ingest.Register(reg, ing); err != nil {
		applog.Error("register ingest failed", "err", err)
		os.Exit(1)
	}
	if err := coraltransfer.Register(reg, &coraltransfer.Gateway{BaseURL: os.Getenv("CORAL_BASE_URL")}); err != nil {
		applog.Error("register coral-transfer failed", "err", err)
		os.Exit(1)
	}
	if err := coralcrm.Register(reg, &coralcrm.Gateway{BaseURL: os.Getenv("CORAL_BASE_URL")}); err != nil {
		applog.Error("register coral-crm failed", "err", err)
		os.Exit(1)
	}

	sarvamConfigured := false
	if sarvamCfg, err := sarvam.LoadConfig(); err != nil {
		applog.Error("sarvam config failed", "err", err)
		os.Exit(1)
	} else if sarvamCfg.Configured() {
		if err := sarvamstt.Register(reg, sarvamstt.New(sarvamCfg)); err != nil {
			applog.Error("register sarvam-stt failed", "err", err)
			os.Exit(1)
		}
		if err := sarvamllm.Register(reg, sarvamllm.New(sarvamCfg)); err != nil {
			applog.Error("register sarvam-llm failed", "err", err)
			os.Exit(1)
		}
		if err := sarvamtts.Register(reg, sarvamtts.New(sarvamCfg)); err != nil {
			applog.Error("register sarvam-tts failed", "err", err)
			os.Exit(1)
		}
		sarvamConfigured = true
		applog.Info("sarvam gateways registered", "ids", "sarvam-stt,sarvam-llm,sarvam-tts")
	} else {
		applog.Info("sarvam not configured (set SARVAM_API_KEY to enable)")
	}

	cfg := control.Config{
		AuthToken:       os.Getenv("CONTROL_AUTH_TOKEN"),
		OwnerInstance:   envOr("OWNER_INSTANCE", "local"),
		EdgeBaseURL:     envOr("EDGE_BASE_URL", "wss://localhost/edge/fs"),
		EdgeTokenSecret: []byte(envOr("EDGE_TOKEN_SECRET", "lab-edge-hmac-change-me")),
	}
	mgr := session.NewManager(reg)
	rt := &control.SessionRuntime{Mgr: mgr}
	srv := control.NewWithRuntime(repo, reg, rt, cfg, web.LabFS)
	srv.SetLabExtras(control.LabExtras{
		StoreBackend:     storeBackend,
		SarvamConfigured: sarvamConfigured,
		HTTPAddr:         envOr("HTTP_ADDR", ":8080"),
	})
	srv.MountEdge(srv.EdgeTokenSecret(), mgr)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	_ = srv.StartPlaybackWorker(workerCtx, mgr)
	_ = srv.StartPostcallWorker(workerCtx)

	addr := envOr("HTTP_ADDR", ":8080")
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

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

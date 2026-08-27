package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/control"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

func main() {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		fmt.Fprintf(os.Stderr, "register: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	databaseURL := envOr("DATABASE_URL", "")
	var repo store.Repository
	var closer func()
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL unset: using in-memory store (lab only)")
		repo = store.NewMemory()
		closer = func() {}
	} else {
		st, err := store.Open(ctx, databaseURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "store: %v\n", err)
			os.Exit(1)
		}
		repo = st
		closer = st.Close
	}
	defer closer()

	cfg := control.Config{
		AuthToken:     os.Getenv("CONTROL_AUTH_TOKEN"),
		OwnerInstance: envOr("OWNER_INSTANCE", "local"),
		EdgeBaseURL:   envOr("EDGE_BASE_URL", "wss://localhost/edge/fs"),
	}
	rt := &control.SessionRuntime{Mgr: session.NewManager(reg)}
	srv := control.NewWithRuntime(repo, reg, rt, cfg)
	addr := envOr("HTTP_ADDR", ":8080")
	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}

	go func() {
		fmt.Printf("aiorchestrator control listening on %s\n", addr)
		kinds := []port.PortKind{
			port.PortListen, port.PortSpeak, port.PortThink,
			port.PortTranslate, port.PortKnowledge, port.PortSkill,
		}
		for _, k := range kinds {
			fmt.Printf("  port %s: %d gateway(s)\n", k, len(reg.List(k)))
		}
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "http: %v\n", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
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

package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/juajotagon/rcon-ui/internal/api"
	"github.com/juajotagon/rcon-ui/internal/config"
	"github.com/juajotagon/rcon-ui/internal/event"
	"github.com/juajotagon/rcon-ui/internal/secret"
	"github.com/juajotagon/rcon-ui/internal/session"
	"github.com/juajotagon/rcon-ui/internal/store"
	"github.com/juajotagon/rcon-ui/internal/webui"
)

// staticFS is the built frontend, or nil when this build has no UI embedded --
// in which case the API is served alone, which is all the curl/SSE checks need.
var staticFS, hasUI = webui.FS()

func cmdServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := flags.String("config", config.DefaultConfigPath(), "path to config file")
	addr := flags.String("addr", "", "listen address (overrides config)")
	dataDir := flags.String("data-dir", "", "data directory (overrides config)")
	token := flags.String("token", "", "require this bearer token (overrides config)")

	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	// Flags win over file and environment.
	if *addr != "" {
		cfg.Addr = *addr
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if *token != "" {
		cfg.Token = *token
	}

	log := newLogger(cfg.LogLevel)

	key, fromEnv, err := secret.LoadOrCreateKey(cfg.DataDir)
	if err != nil {
		return err
	}
	if !fromEnv {
		// Worth saying out loud: with the key beside the database, anyone who
		// obtains the data directory obtains the passwords too. Fine for a
		// local desktop install, not for a server or a shared volume.
		log.Warn("sealing key stored beside the database; set "+secret.KeyEnvVar+" for deployments",
			"dataDir", cfg.DataDir)
	}

	sealer, err := secret.NewFromKey(key)
	if err != nil {
		return err
	}

	st, err := store.Open(cfg.DBPath(), sealer)
	if err != nil {
		return err
	}
	defer st.Close()

	hub := event.NewHub()
	mgr := session.NewManager(st, hub, log)
	defer mgr.Close()

	if !config.IsLoopback(cfg.Addr) && cfg.Token == "" {
		// Not fatal, because a deployment may sit behind an authenticating
		// proxy -- but silence here would mean handing out RCON access.
		log.Warn("listening beyond loopback with no token; anyone who can reach this port controls your servers",
			"addr", cfg.Addr)
	}

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: api.New(st, hub, mgr, log, cfg.Token, staticFS).Handler(),

		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: it would cut off the SSE stream, which is meant to
		// stay open indefinitely. Idle connections are bounded by IdleTimeout
		// and by the stream's own heartbeat.
		IdleTimeout: 120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr, "dataDir", cfg.DataDir, "ui", hasUI)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l}))
}

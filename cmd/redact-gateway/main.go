package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"redact-gateway/internal/admin"
	"redact-gateway/internal/config"
	"redact-gateway/internal/gateway"
	"redact-gateway/internal/store"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("redact-gateway %s (%s, %s, %s/%s)\n", version, commit, buildTime, runtime.GOOS, runtime.GOARCH)
		return
	}
	if len(os.Args) > 1 && os.Args[1] != "serve" {
		fmt.Fprintf(os.Stderr, "usage: %s [serve|version]\n", os.Args[0])
		os.Exit(2)
	}
	if err := run(); err != nil {
		slog.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	eventStore, err := store.Open(cfg.DataDir, cfg.LogRetentionDays)
	if err != nil {
		return err
	}
	defer eventStore.Close()

	proxy := gateway.NewProxy(cfg, eventStore, logger, version)
	adminServer := admin.NewServer(cfg.AdminToken, proxy, eventStore)
	dataHTTP := &http.Server{
		Addr: cfg.ListenAddr, Handler: proxy,
		ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second,
	}
	adminHTTP := &http.Server{
		Addr: cfg.AdminAddr, Handler: adminServer.Handler(),
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}

	logger.Info("starting redact gateway",
		"version", version, "proxy_addr", cfg.ListenAddr, "admin_addr", cfg.AdminAddr,
		"data_dir", cfg.DataDir, "allowed_hosts", len(cfg.AllowedHosts),
		"allow_private_upstreams", cfg.AllowPrivateHosts,
	)
	if os.Getenv("REDACT_ADMIN_TOKEN") == "" {
		logger.Info("control plane token", "token", cfg.AdminToken)
	}

	errCh := make(chan error, 2)
	go func() {
		if serveErr := dataHTTP.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- fmt.Errorf("data plane: %w", serveErr)
		}
	}()
	go func() {
		if serveErr := adminHTTP.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- fmt.Errorf("control plane: %w", serveErr)
		}
	}()

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case serveErr := <-errCh:
		return serveErr
	case <-signalContext.Done():
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Info("shutting down redact gateway")
	if err := adminHTTP.Shutdown(shutdownContext); err != nil {
		logger.Warn("control plane shutdown", "error", err)
	}
	if err := dataHTTP.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("data plane shutdown: %w", err)
	}
	return nil
}

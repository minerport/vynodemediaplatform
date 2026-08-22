package main

import (
	"context"
	"errors"
	"github.com/vynode/media/server/internal/app"
	"github.com/vynode/media/server/internal/config"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(2)
	}
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		_, port, _ := net.SplitHostPort(cfg.HTTPAddress)
		response, err := http.Get("http://127.0.0.1:" + port + "/health")
		if err != nil || response.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		_ = response.Body.Close()
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	runtime, err := app.Initialize(context.Background(), cfg, logger)
	if err != nil {
		logger.Error("initialization failed", "error", err)
		os.Exit(1)
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", "address", cfg.HTTPAddress, "database_type", cfg.DatabaseType)
		errCh <- runtime.Server.ListenAndServe()
	}()
	signals, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-signals.Done():
		logger.Info("shutdown requested")
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := runtime.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}

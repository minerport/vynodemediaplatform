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
	if handled, err := runWindowsService(); handled {
		if err != nil {
			slog.Error("Windows service failed", "error", err)
			os.Exit(1)
		}
		return
	}
	os.Exit(run(context.Background()))
}

func run(parent context.Context) int {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		return 2
	}
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		_, port, _ := net.SplitHostPort(cfg.HTTPAddress)
		response, err := http.Get("http://127.0.0.1:" + port + "/health")
		if err != nil || response.StatusCode != http.StatusOK {
			return 1
		}
		_ = response.Body.Close()
		return 0
	}
	logWriter, closeLog, err := windowsLogWriter(cfg.ConfigDir)
	if err != nil {
		slog.Error("log initialization failed", "error", err)
		return 1
	}
	defer closeLog()
	logger := slog.New(slog.NewJSONHandler(logWriter, &slog.HandlerOptions{Level: cfg.LogLevel}))
	runtime, err := app.Initialize(context.Background(), cfg, logger)
	if err != nil {
		logger.Error("initialization failed", "error", err)
		return 1
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", "address", cfg.HTTPAddress, "database_type", cfg.DatabaseType)
		errCh <- runtime.Server.ListenAndServe()
	}()
	signals, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-signals.Done():
		logger.Info("shutdown requested")
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
			return 1
		}
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := runtime.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		return 1
	}
	logger.Info("server stopped")
	return 0
}

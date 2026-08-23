package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/vynode/media/connect/internal/account"
	"github.com/vynode/media/connect/internal/httpapi"
	"github.com/vynode/media/connect/internal/registry"
	"github.com/vynode/media/connect/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		client := http.Client{Timeout: 3 * time.Second}
		response, err := client.Get("http://127.0.0.1:8090/health/ready")
		if err != nil || response.StatusCode != 200 {
			os.Exit(1)
		}
		_ = response.Body.Close()
		return
	}
	dir := env("VYNODE_CONNECT_DATA_DIR", "./data/connect")
	issuer := env("VYNODE_CONNECT_ISSUER", "http://localhost:8090")
	address := env("VYNODE_CONNECT_LISTEN", ":8090")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := store.Open(ctx, dir)
	if err != nil {
		slog.Error("database startup failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	accounts, err := account.New(db.DB, dir, issuer)
	if err != nil {
		slog.Error("account startup failed", "error", err)
		os.Exit(1)
	}
	registryService := registry.New(db.DB, dir, issuer)
	_, _, err = registryService.LoadSigningKeyForStartup()
	if err != nil {
		slog.Error("signing key startup failed", "error", err)
		os.Exit(1)
	}
	server := &http.Server{Addr: address, Handler: httpapi.New(accounts, registryService, db, env("VYNODE_CONNECT_ALLOWED_ORIGIN", ""), env("VYNODE_CONNECT_ALLOW_INSECURE_ENDPOINTS", "") == "true"), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	slog.Info("VyNode Connect listening", "address", address)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

package app

import (
	"context"
	"fmt"
	"github.com/vynode/media/server/internal/buildinfo"
	"github.com/vynode/media/server/internal/config"
	"github.com/vynode/media/server/internal/database"
	httpserver "github.com/vynode/media/server/internal/http"
	"github.com/vynode/media/server/internal/identity"
	"log/slog"
	"net/http"
	"runtime"
	"time"
)

type Runtime struct {
	Server *http.Server
	store  *database.Store
}

func Initialize(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Runtime, error) {
	store, err := database.Open(ctx, cfg.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	fail := func(err error) (*Runtime, error) { _ = store.Close(); return nil, err }
	if err := store.Migrate(ctx); err != nil {
		return fail(fmt.Errorf("run migrations: %w", err))
	}
	instanceID, err := identity.LoadOrCreate(ctx, store.DB)
	if err != nil {
		return fail(err)
	}
	info := httpserver.SystemInfo{Version: buildinfo.Version, Commit: buildinfo.Commit, OS: runtime.GOOS, Architecture: runtime.GOARCH, InstanceID: instanceID, ServerName: cfg.ServerName, DatabaseType: cfg.DatabaseType, StartedAt: time.Now()}
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: httpserver.NewHandler(logger, store, info), ReadHeaderTimeout: cfg.ReadHeaderTimeout, IdleTimeout: cfg.IdleTimeout}
	return &Runtime{Server: server, store: store}, nil
}
func (r *Runtime) Shutdown(ctx context.Context) error {
	serverErr := r.Server.Shutdown(ctx)
	dbErr := r.store.Close()
	if serverErr != nil {
		return serverErr
	}
	return dbErr
}

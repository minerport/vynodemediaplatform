package app

import (
	"context"
	"fmt"
	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/buildinfo"
	"github.com/vynode/media/server/internal/config"
	"github.com/vynode/media/server/internal/database"
	httpserver "github.com/vynode/media/server/internal/http"
	"github.com/vynode/media/server/internal/identity"
	"github.com/vynode/media/server/internal/media"
	"log/slog"
	"net/http"
	"os"
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
	serverName := cfg.ServerName
	_ = store.DB.QueryRowContext(ctx, "SELECT value FROM server_settings WHERE key='server_name'").Scan(&serverName)
	info := httpserver.SystemInfo{Version: buildinfo.Version, Commit: buildinfo.Commit, OS: runtime.GOOS, Architecture: runtime.GOARCH, InstanceID: instanceID, ServerName: serverName, DatabaseType: cfg.DatabaseType, WebDir: cfg.WebDir, StartedAt: time.Now()}
	authService, err := auth.New(store.DB, cfg.ConfigDir, instanceID, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	if err != nil {
		return fail(fmt.Errorf("initialize authentication: %w", err))
	}
	probe := media.NewFFprobe(cfg.FFprobePath, cfg.ProbeConcurrency)
	mediaService := media.New(store.DB, probe, cfg.ConfigDir, os.Getenv("VYNODE_TRANSCODE_DIR"))
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: httpserver.NewHandler(logger, store, info, authService, mediaService, cfg.AllowedOrigin), ReadHeaderTimeout: cfg.ReadHeaderTimeout, IdleTimeout: cfg.IdleTimeout}
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

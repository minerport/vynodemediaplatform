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
	"github.com/vynode/media/server/internal/intelligence"
	"github.com/vynode/media/server/internal/media"
	"github.com/vynode/media/server/internal/metadata"
	"github.com/vynode/media/server/internal/playback"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"
)

type Runtime struct {
	Server       *http.Server
	store        *database.Store
	playback     *playback.Service
	intelligence *intelligence.Service
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
	providerBase := os.Getenv("VYNODE_TMDB_BASE_URL")
	if providerBase != "" && os.Getenv("VYNODE_METADATA_ALLOW_INSECURE_TEST_PROVIDER") != "true" {
		providerBase = ""
	}
	provider := metadata.NewTMDb(providerBase, metadata.LoadToken(cfg.ConfigDir), buildinfo.Version)
	metadataService := metadata.New(store.DB, cfg.ConfigDir, provider, os.Getenv("VYNODE_TMDB_IMAGE_BASE_URL"), os.Getenv("VYNODE_METADATA_ALLOW_INSECURE_TEST_PROVIDER"))
	pipeline := playback.NewFFmpeg(cfg.FFmpegPath, cfg.PlaybackPipelines)
	playbackService := playback.New(store.DB, pipeline)
	playbackService.ConfigureVideo(playback.NewHLS(cfg.FFmpegPath, cfg.TranscodeDir, cfg.VideoTranscodes), cfg.RemoteBitrate)
	intelligenceService := intelligence.New(store.DB, cfg.FFmpegPath, cfg.OptimizedDir)
	intelligenceService.StartScheduler()
	metadataService.ConfigureAutomation(intelligenceService.HandleEvent)
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: httpserver.NewHandler(logger, store, info, authService, mediaService, metadataService, playbackService, cfg.AllowedOrigin, intelligenceService), ReadHeaderTimeout: cfg.ReadHeaderTimeout, IdleTimeout: cfg.IdleTimeout}
	return &Runtime{Server: server, store: store, playback: playbackService, intelligence: intelligenceService}, nil
}
func (r *Runtime) Shutdown(ctx context.Context) error {
	if r.playback != nil {
		r.playback.Close()
	}
	if r.intelligence != nil {
		r.intelligence.Close()
	}
	serverErr := r.Server.Shutdown(ctx)
	dbErr := r.store.Close()
	if serverErr != nil {
		return serverErr
	}
	return dbErr
}

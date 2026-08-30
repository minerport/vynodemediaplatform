package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddress       string
	LogLevel          slog.Level
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	ConfigDir         string
	ServerName        string
	DatabaseType      string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	AllowedOrigin     string
	WebDir            string
	FFprobePath       string
	FFprobeSource     string
	ProbeConcurrency  int
	FFmpegPath        string
	FFmpegSource      string
	PlaybackPipelines int
	TranscodeDir      string
	VideoTranscodes   int
	RemoteBitrate     int64
	OptimizedDir      string
	DownloadsDir      string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddress:       envOr("VYNODE_HTTP_ADDRESS", "127.0.0.1:8096"),
		ShutdownTimeout:   10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		ConfigDir:         envOr("VYNODE_CONFIG_DIR", "./data"),
		ServerName:        envOr("VYNODE_SERVER_NAME", "VyNode Media"),
		DatabaseType:      strings.ToLower(envOr("VYNODE_DATABASE_TYPE", "sqlite")),
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   30 * 24 * time.Hour,
		AllowedOrigin:     envOr("VYNODE_ALLOWED_ORIGIN", ""),
		WebDir:            envOr("VYNODE_WEB_DIR", ""),
		FFprobePath:       envOr("VYNODE_FFPROBE_PATH", ""),
		FFprobeSource:     mediaToolSource("VYNODE_FFPROBE_PATH", "VYNODE_FFPROBE_SOURCE"),
		ProbeConcurrency:  2,
		FFmpegPath:        envOr("VYNODE_FFMPEG_PATH", ""),
		FFmpegSource:      mediaToolSource("VYNODE_FFMPEG_PATH", "VYNODE_FFMPEG_SOURCE"),
		PlaybackPipelines: 2,
		TranscodeDir:      envOr("VYNODE_TRANSCODE_DIR", "./data/transcode"),
		VideoTranscodes:   1,
		RemoteBitrate:     20_000_000,
		OptimizedDir:      envOr("VYNODE_OPTIMIZED_DIR", filepath.Join(envOr("VYNODE_CONFIG_DIR", "./data"), "optimized")),
		DownloadsDir:      envOr("VYNODE_DOWNLOADS_DIR", filepath.Join(envOr("VYNODE_CONFIG_DIR", "./data"), "downloads")),
	}
	if strings.TrimSpace(cfg.ConfigDir) == "" {
		return Config{}, fmt.Errorf("VYNODE_CONFIG_DIR must not be empty")
	}
	if strings.TrimSpace(cfg.ServerName) == "" {
		return Config{}, fmt.Errorf("VYNODE_SERVER_NAME must not be empty")
	}
	if cfg.DatabaseType != "sqlite" {
		return Config{}, fmt.Errorf("VYNODE_DATABASE_TYPE currently supports only sqlite")
	}
	if cfg.AllowedOrigin != "" {
		parsed, err := url.Parse(cfg.AllowedOrigin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" || strings.Contains(cfg.AllowedOrigin, "*") {
			return Config{}, fmt.Errorf("VYNODE_ALLOWED_ORIGIN must be one absolute origin without wildcards")
		}
	}

	if _, _, err := net.SplitHostPort(cfg.HTTPAddress); err != nil {
		return Config{}, fmt.Errorf("VYNODE_HTTP_ADDRESS must be host:port: %w", err)
	}

	level := strings.ToLower(envOr("VYNODE_LOG_LEVEL", "info"))
	levels := map[string]slog.Level{"debug": slog.LevelDebug, "info": slog.LevelInfo, "warn": slog.LevelWarn, "error": slog.LevelError}
	var ok bool
	if cfg.LogLevel, ok = levels[level]; !ok {
		return Config{}, fmt.Errorf("VYNODE_LOG_LEVEL must be debug, info, warn, or error")
	}

	if raw := os.Getenv("VYNODE_SHUTDOWN_TIMEOUT"); raw != "" {
		duration, err := time.ParseDuration(raw)
		if err != nil || duration <= 0 {
			return Config{}, fmt.Errorf("VYNODE_SHUTDOWN_TIMEOUT must be a positive duration")
		}
		cfg.ShutdownTimeout = duration
	}
	if raw := os.Getenv("VYNODE_PROBE_CONCURRENCY"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 8 {
			return Config{}, fmt.Errorf("VYNODE_PROBE_CONCURRENCY must be between 1 and 8")
		}
		cfg.ProbeConcurrency = value
	}
	if raw := os.Getenv("VYNODE_PLAYBACK_PIPELINES"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 16 {
			return Config{}, fmt.Errorf("VYNODE_PLAYBACK_PIPELINES must be between 1 and 16")
		}
		cfg.PlaybackPipelines = value
	}
	if raw := os.Getenv("VYNODE_VIDEO_TRANSCODES"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 8 {
			return Config{}, fmt.Errorf("VYNODE_VIDEO_TRANSCODES must be between 1 and 8")
		}
		cfg.VideoTranscodes = value
	}
	if raw := os.Getenv("VYNODE_REMOTE_BITRATE"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 500_000 || value > 200_000_000 {
			return Config{}, fmt.Errorf("VYNODE_REMOTE_BITRATE must be between 500000 and 200000000")
		}
		cfg.RemoteBitrate = value
	}
	return cfg, nil
}

func mediaToolSource(pathVariable, sourceVariable string) string {
	if source := strings.ToLower(strings.TrimSpace(os.Getenv(sourceVariable))); source == "managed" || source == "custom" {
		return source
	}
	if strings.TrimSpace(os.Getenv(pathVariable)) != "" {
		return "custom"
	}
	return "unavailable"
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

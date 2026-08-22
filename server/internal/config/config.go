package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
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
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

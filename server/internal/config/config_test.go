package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("VYNODE_HTTP_ADDRESS", "")
	t.Setenv("VYNODE_LOG_LEVEL", "")
	t.Setenv("VYNODE_SHUTDOWN_TIMEOUT", "")
	t.Setenv("VYNODE_CONFIG_DIR", "")
	t.Setenv("VYNODE_SERVER_NAME", "")
	t.Setenv("VYNODE_DATABASE_TYPE", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddress != "127.0.0.1:8096" {
		t.Fatalf("unexpected address %q", cfg.HTTPAddress)
	}
}

func TestLoadRejectsUnsupportedDatabase(t *testing.T) {
	t.Setenv("VYNODE_DATABASE_TYPE", "postgres")
	if _, err := Load(); err == nil {
		t.Fatal("expected database validation error")
	}
}

func TestLoadRejectsInvalidAddress(t *testing.T) {
	t.Setenv("VYNODE_HTTP_ADDRESS", "invalid")
	if _, err := Load(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestMediaToolSourceTracksManagedAndCustomPaths(t *testing.T) {
	t.Setenv("VYNODE_FFMPEG_PATH", `C:\Program Files\VyNode\Media Server\tools\ffmpeg\ffmpeg.exe`)
	t.Setenv("VYNODE_FFMPEG_SOURCE", "managed")
	t.Setenv("VYNODE_FFPROBE_PATH", `D:\Admin Tools\ffprobe.exe`)
	t.Setenv("VYNODE_FFPROBE_SOURCE", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FFmpegSource != "managed" || cfg.FFprobeSource != "custom" {
		t.Fatalf("unexpected media-tool sources: ffmpeg=%q ffprobe=%q", cfg.FFmpegSource, cfg.FFprobeSource)
	}
}

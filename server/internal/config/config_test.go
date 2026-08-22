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

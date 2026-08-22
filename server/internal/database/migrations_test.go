package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrationStartup(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "config"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrations must be idempotent: %v", err)
	}
	for _, table := range []string{"server_settings", "users", "devices", "sessions", "audit_events", "movies", "shows", "seasons", "episodes", "media_associations", "external_ids", "genres", "artwork", "metadata_jobs", "client_capabilities", "playback_sessions", "user_media_progress", "playback_history", "playback_pipeline_instances", "sidecar_subtitles", "transcode_sessions", "transcode_backend_status", "user_quality_preferences"} {
		var name string
		if err := store.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}
	var integrity string
	if err := store.DB.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity=%q err=%v", integrity, err)
	}
	var violations int
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign key violations=%d err=%v", violations, err)
	}
}

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
	for _, table := range []string{"server_settings", "users", "devices", "sessions", "audit_events"} {
		var name string
		if err := store.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}
}

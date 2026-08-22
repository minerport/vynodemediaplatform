package identity

import (
	"context"
	"github.com/vynode/media/server/internal/database"
	"path/filepath"
	"testing"
)

func TestIdentityPersists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config")
	store, err := database.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	first, err := LoadOrCreate(context.Background(), store.DB)
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = database.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	second, err := LoadOrCreate(context.Background(), store.DB)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("identity changed: %s != %s", first, second)
	}
}

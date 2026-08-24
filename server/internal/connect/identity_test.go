package connect

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/vynode/media/server/internal/database"
)

func TestIdentityPersistsAndFailsSafe(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := database.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	first, err := LoadOrCreateIdentity(ctx, store.DB, dir, "server-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateIdentity(ctx, store.DB, dir, "server-a")
	if err != nil {
		t.Fatal(err)
	}
	if first.PublicKey != second.PublicKey || first.Path != second.Path {
		t.Fatal("identity changed on restart")
	}
	if err = os.Remove(first.Path); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadOrCreateIdentity(ctx, store.DB, dir, "server-a"); !errors.Is(err, ErrIdentityUnsafe) {
		t.Fatalf("missing key error=%v", err)
	}
}
func TestIdentityRejectsMalformedAndMismatchedMaterial(t *testing.T) {
	for _, tc := range []struct{ name, contents string }{{"truncated", "AAAA"}, {"copied-key", "FJxGqDJqsZ7zz7tyk5dHGv_7SpM50BTJSZesShVDdv29lvMxDd66oTGpbjaYBoGW1B9ptcHOZnbsNeK9AzPsqg"}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			store, err := database.Open(ctx, dir)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if err = store.Migrate(ctx); err != nil {
				t.Fatal(err)
			}
			identity, err := LoadOrCreateIdentity(ctx, store.DB, dir, "server-a")
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(identity.Path, []byte(tc.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err = LoadOrCreateIdentity(ctx, store.DB, dir, "server-a"); !errors.Is(err, ErrIdentityUnsafe) {
				t.Fatalf("unsafe material error=%v", err)
			}
		})
	}
}

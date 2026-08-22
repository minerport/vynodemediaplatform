package app

import (
	"context"
	"github.com/vynode/media/server/internal/config"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestInitializationFailureIsReturned(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{ConfigDir: file, DatabaseType: "sqlite"}
	if runtime, err := Initialize(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil || runtime != nil {
		t.Fatal("expected clean initialization failure")
	}
}

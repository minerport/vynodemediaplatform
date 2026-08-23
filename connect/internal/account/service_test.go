package account

import (
	"context"
	"errors"
	"testing"

	"github.com/vynode/media/connect/internal/store"
)

func TestRefreshReuseRevokesRotatedFamily(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err := New(db.DB, dir, "https://connect.test")
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Register(ctx, "alice", "Alice", "CorrectHorseBatteryStaple!", DeviceInput{Name: "Phone", Platform: "ANDROID"}, "one")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := service.Refresh(ctx, first.RefreshToken, "two")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Refresh(ctx, first.RefreshToken, "replay"); !errors.Is(err, ErrRevoked) {
		t.Fatalf("old refresh replay error=%v", err)
	}
	if _, err = service.Refresh(ctx, rotated.RefreshToken, "after-replay"); !errors.Is(err, ErrRevoked) {
		t.Fatalf("rotated family remained active: %v", err)
	}
}
func TestDeviceRevocationInvalidatesGlobalSession(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err := New(db.DB, dir, "https://connect.test")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := service.Register(ctx, "alice", "Alice", "CorrectHorseBatteryStaple!", DeviceInput{Name: "TV", Platform: "ANDROID_TV"}, "one")
	if err != nil {
		t.Fatal(err)
	}
	p, err := service.Authenticate(tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.RevokeDevice(ctx, p, p.DeviceID, "revoke"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Authenticate(tokens.AccessToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("revoked access accepted: %v", err)
	}
	if _, err = service.Refresh(ctx, tokens.RefreshToken, "refresh"); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked refresh accepted: %v", err)
	}
}

package registry

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	"github.com/vynode/media/connect/internal/account"
	"github.com/vynode/media/connect/internal/store"
)

func TestInvitationAndDeviceCodeAreBoundAndSingleUse(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	accounts, err := account.New(db.DB, dir, "https://connect.test")
	if err != nil {
		t.Fatal(err)
	}
	ownerTokens, err := accounts.Register(ctx, "owner", "Owner", "CorrectHorseBatteryStaple!", account.DeviceInput{Name: "Web", Platform: "WEB"}, "owner")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := accounts.Authenticate(ownerTokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	service := New(db.DB, dir, "https://connect.test")
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub := base64.RawURLEncoding.EncodeToString(public)
	signature := base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, canonicalRegister("server-a", "Server A", pub)))
	if _, err = service.RegisterServer(ctx, "server-a", "Server A", pub, "14", signature); err != nil {
		t.Fatal(err)
	}
	claimSig := base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, []byte("vynode-connect-claim-request-v1\nserver-a")))
	challenge, err := service.CreateClaim(ctx, "server-a", claimSig)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CompleteClaim(ctx, owner, challenge); err != nil {
		t.Fatal(err)
	}
	invite, err := service.CreateInvitation(ctx, owner, "server-a", "bob", "local-bob", 0)
	if err != nil {
		t.Fatal(err)
	}
	bobTokens, err := accounts.Register(ctx, "bob", "Bob", "AnotherCorrectHorseBattery!", account.DeviceInput{Name: "Phone", Platform: "ANDROID"}, "bob")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := accounts.Authenticate(bobTokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.AcceptInvitation(ctx, bob, invite.Token); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AcceptInvitation(ctx, bob, invite.Token); !errors.Is(err, ErrGone) {
		t.Fatalf("invite reuse error=%v", err)
	}
	code, err := service.CreateDeviceCode(ctx, account.DeviceInput{Name: "TV", Platform: "ANDROID_TV"})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ApproveDeviceCode(ctx, bob, code.UserCode); err != nil {
		t.Fatal(err)
	}
	aid, _, err := service.ExchangeDeviceCode(ctx, code.DeviceCode)
	if err != nil || aid != bob.AccountID {
		t.Fatalf("device exchange account=%s error=%v", aid, err)
	}
	if _, _, err = service.ExchangeDeviceCode(ctx, code.DeviceCode); !errors.Is(err, ErrGone) {
		t.Fatalf("device-code reuse error=%v", err)
	}
	denied, err := service.CreateDeviceCode(ctx, account.DeviceInput{Name: "Other TV", Platform: "ANDROID_TV"})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.DenyDeviceCode(ctx, bob, denied.UserCode); err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.ExchangeDeviceCode(ctx, denied.DeviceCode); !errors.Is(err, ErrGone) {
		t.Fatalf("denied code error=%v", err)
	}
	if err = accounts.RevokeDevice(ctx, bob, bob.DeviceID, "revoke"); err != nil {
		t.Fatal(err)
	}
	bucket := service.Now().UTC().Unix() / 60
	revocationSig := base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, []byte(fmt.Sprintf("vynode-connect-revocations-v1\nserver-a\n%d", bucket))))
	revocations, err := service.DeviceRevocations(ctx, "server-a", revocationSig)
	if err != nil || len(revocations) != 1 || revocations[0].DeviceID != bob.DeviceID || revocations[0].GlobalAccountID != bob.AccountID {
		t.Fatalf("device revocations=%+v error=%v", revocations, err)
	}
}

package connect

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/database"
)

func TestAssertionExchangeSecurity(t *testing.T) {
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
	a, err := auth.New(store.DB, dir, "server-a", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := a.Bootstrap(ctx, "owner", "Owner", "CorrectHorseBatteryStaple!", "Server", "request", "127.0.0.1", auth.DeviceInput{Name: "setup"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Authenticate(tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kid": "test", "kty": "OKP", "crv": "Ed25519", "x": base64.RawURLEncoding.EncodeToString(public)}}})
	_, err = store.DB.Exec(`UPDATE connect_settings SET enabled=1,connect_issuer=?,connect_signing_keys_json=? WHERE id=1`, "https://connect.test", string(keys))
	if err != nil {
		t.Fatal(err)
	}
	s := New(store.DB, a, "server-a")
	request, err := s.CreateLinkRequest(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	grant := signTestLinkGrant(t, private, "server-a", "global-a", request.State)
	otherTokens, err := a.IssueSession(ctx, tokens.User, auth.DeviceInput{Name: "Other browser"}, "127.0.0.1", "other")
	if err != nil {
		t.Fatal(err)
	}
	other, err := a.Authenticate(otherTokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteLink(ctx, other, request.State, grant); err != ErrInvalid {
		t.Fatalf("wrong local session linked: %v", err)
	}
	if err = s.CompleteLink(ctx, p, request.State, grant); err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteLink(ctx, p, request.State, grant); err != ErrInvalid {
		t.Fatalf("link replay error=%v", err)
	}
	assertion := signTestAssertion(t, private, "server-a", "global-a", "nonce-a")
	exchanged, err := s.Exchange(ctx, assertion, auth.DeviceInput{Name: "Phone", Platform: "ANDROID"}, "127.0.0.1", "request")
	if err != nil {
		t.Fatal(err)
	}
	var mapped int
	if err = store.DB.QueryRow("SELECT COUNT(*) FROM connect_global_device_sessions WHERE global_device_id='device-a' AND global_account_id='global-a' AND local_session_id=?", strings.SplitN(exchanged.RefreshToken, ".", 2)[0]).Scan(&mapped); err != nil || mapped != 1 {
		t.Fatalf("mapped global device session=%d error=%v", mapped, err)
	}
	if _, err = s.Exchange(ctx, assertion, auth.DeviceInput{Name: "Phone"}, "127.0.0.1", "request"); err != ErrInvalid {
		t.Fatalf("replay error=%v", err)
	}
	wrong := signTestAssertion(t, private, "server-b", "global-a", "nonce-b")
	if _, err = s.Exchange(ctx, wrong, auth.DeviceInput{}, "127.0.0.1", "request"); err != ErrInvalid {
		t.Fatalf("audience error=%v", err)
	}
}
func signTestLinkGrant(t *testing.T, key ed25519.PrivateKey, aud, sub, state string) string {
	t.Helper()
	now := time.Now()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "kid": "test"})
	claims, _ := json.Marshal(map[string]any{"iss": "https://connect.test", "aud": aud, "sub": sub, "did": "global-device", "purpose": "account-link", "state": state, "jti": "grant-a", "iat": now.Unix(), "exp": now.Add(2 * time.Minute).Unix()})
	message := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	return message + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(message)))
}
func signTestAssertion(t *testing.T, key ed25519.PrivateKey, aud, sub, nonce string) string {
	t.Helper()
	now := time.Now()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "kid": "test"})
	claims, _ := json.Marshal(map[string]any{"iss": "https://connect.test", "aud": aud, "sub": sub, "did": "device-a", "nonce": nonce, "iat": now.Unix(), "exp": now.Add(2 * time.Minute).Unix()})
	message := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	return message + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(message)))
}

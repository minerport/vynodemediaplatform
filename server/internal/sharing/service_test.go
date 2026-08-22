package sharing

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/database"
)

func fixture(t *testing.T) (*Service, auth.Principal) {
	t.Helper()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "config")
	store, e := database.Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { store.Close() })
	if e = store.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	a, e := auth.New(store.DB, dir, "server", 15*time.Minute, 30*24*time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	hash, _ := auth.HashPassword("correct horse battery staple")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, e = store.DB.Exec(`INSERT INTO users(id,username,password_hash,role,display_name,status,created_at) VALUES('owner','owner',?,'OWNER','Owner','ACTIVE',?),('user','user',?,'USER','User','ACTIVE',?);INSERT INTO libraries(id,name,type,enabled,created_at,updated_at) VALUES('a','Library A','MOVIES',1,?,?),('b','Library B','MOVIES',1,?,?)`, hash, now, hash, now, now, now, now, now)
	if e != nil {
		t.Fatal(e)
	}
	return New(store.DB, a, "server", "Test Server", "test"), auth.Principal{UserID: "owner", Role: auth.RoleOwner}
}

func TestInvitationOneTimeAndHashed(t *testing.T) {
	s, p := fixture(t)
	ctx := context.Background()
	inv, token, e := s.CreateInvite(ctx, p, "guest", auth.RoleUser, []Grant{{LibraryID: "a", Permissions: []string{"VIEW", "PLAY"}}}, time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(inv.ID, token) {
		t.Fatal("raw token leaked")
	}
	listed, e := s.Invitations(ctx)
	if e != nil || len(listed) != 1 || len(listed[0].Libraries) != 1 || listed[0].Libraries[0].LibraryID != "a" {
		t.Fatalf("invitation list=%+v err=%v", listed, e)
	}
	var stored string
	_ = s.db.QueryRow("SELECT token_hash FROM user_invitations WHERE id=?", inv.ID).Scan(&stored)
	if stored == token || stored != digest(token) {
		t.Fatal("token was not stored as a digest")
	}
	tokens, e := s.AcceptInvite(ctx, token, "guest", "Guest", "correct horse battery staple", "127.0.0.1", "r", auth.DeviceInput{Name: "Browser", ClientName: "test", Platform: "web"})
	if e != nil {
		t.Fatal(e)
	}
	if tokens.RefreshToken == "" || tokens.User.Role != auth.RoleUser {
		t.Fatal("normal session was not issued")
	}
	if _, e = s.AcceptInvite(ctx, token, "guest2", "Guest 2", "correct horse battery staple", "127.0.0.1", "r", auth.DeviceInput{Name: "Browser", ClientName: "test", Platform: "web"}); e == nil {
		t.Fatal("invite reused")
	}
	if !s.Has(ctx, auth.Principal{UserID: tokens.User.ID, Role: auth.RoleUser}, "a", "PLAY") || s.Has(ctx, auth.Principal{UserID: tokens.User.ID, Role: auth.RoleUser}, "b", "VIEW") {
		t.Fatal("grant isolation failed")
	}
}

func TestInvitationConcurrentAcceptance(t *testing.T) {
	s, p := fixture(t)
	ctx := context.Background()
	_, token, e := s.CreateInvite(ctx, p, "", auth.RoleUser, nil, time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	success := make(chan bool, 2)
	for _, name := range []string{"guest1", "guest2"} {
		go func(n string) {
			defer wg.Done()
			_, e := s.AcceptInvite(ctx, token, n, n, "correct horse battery staple", "127.0.0.1", "r", auth.DeviceInput{Name: "Browser", ClientName: "test", Platform: "web"})
			success <- e == nil
		}(name)
	}
	wg.Wait()
	close(success)
	n := 0
	for ok := range success {
		if ok {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("successful accepts=%d", n)
	}
}

func TestPairingBootstrapsRevocableSession(t *testing.T) {
	s, _ := fixture(t)
	ctx := context.Background()
	x, e := s.CreatePairing(ctx, auth.DeviceInput{Name: "Living Room TV", ClientName: "native-test", Platform: "tvOS"})
	if e != nil {
		t.Fatal(e)
	}
	if x.Code == "" || x.Challenge == "" {
		t.Fatal("missing bootstrap material")
	}
	var codeHash, challengeHash string
	_ = s.db.QueryRow("SELECT code_hash,challenge_hash FROM pairing_requests WHERE id=?", x.ID).Scan(&codeHash, &challengeHash)
	if codeHash == x.Code || challengeHash == x.Challenge {
		t.Fatal("pairing secret stored in plaintext")
	}
	found, e := s.PairingByCode(ctx, x.Code, "192.0.2.1")
	if e != nil || found.ID != x.ID {
		t.Fatal("lookup failed")
	}
	if e = s.DecidePairing(ctx, x.ID, "user", true); e != nil {
		t.Fatal(e)
	}
	tokens, e := s.ExchangePairing(ctx, x.ID, x.Challenge, "192.0.2.1", "r")
	if e != nil {
		t.Fatal(e)
	}
	sid := strings.SplitN(tokens.RefreshToken, ".", 2)[0]
	var kind string
	_ = s.db.QueryRow("SELECT d.authorization_type FROM devices d JOIN sessions s ON s.device_id=d.id WHERE s.id=?", sid).Scan(&kind)
	if kind != "PAIRED" {
		t.Fatalf("authorization=%s", kind)
	}
	_, _ = s.db.Exec("UPDATE sessions SET revoked_at=? WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), sid)
	if _, e = s.auth.Refresh(ctx, tokens.RefreshToken, "r"); e == nil {
		t.Fatal("revoked paired session refreshed")
	}
}

func TestRemoteValidationAndCIDRs(t *testing.T) {
	bad := []string{"file:///tmp/x", "https://user:pass@example.com", "ftp://example.com", "http://example.com", "https://example.com?q=x"}
	for _, v := range bad {
		if _, e := ValidateRemoteURL(v, false); e == nil {
			t.Fatalf("accepted %s", v)
		}
	}
	if _, e := ValidateRemoteURL("https://media.example.com/vynode", false); e != nil {
		t.Fatal(e)
	}
	v, e := ValidateCIDRs([]string{"192.168.1.12/24", "2001:db8::1/64"})
	if e != nil || len(v) != 2 {
		t.Fatal("CIDR validation failed")
	}
	if _, e = ValidateCIDRs([]string{"not-a-cidr"}); e == nil {
		t.Fatal("bad CIDR accepted")
	}
}

func TestPairingRateLimit(t *testing.T) {
	s, _ := fixture(t)
	for i := 0; i < 8; i++ {
		_, _ = s.PairingByCode(context.Background(), "WRNG-CODE", "198.51.100.1")
	}
	if _, e := s.PairingByCode(context.Background(), "WRNG-CODE", "198.51.100.1"); e != ErrLimited {
		t.Fatalf("expected rate limit, got %v", e)
	}
}

func TestForwardedHeadersRequireTrustedProxy(t *testing.T) {
	s, _ := fixture(t)
	ctx := context.Background()
	_, _ = s.db.Exec("INSERT INTO trusted_proxy_cidrs(cidr,created_at) VALUES('10.0.0.0/8',CURRENT_TIMESTAMP);INSERT INTO local_network_cidrs(cidr,created_at) VALUES('192.168.50.0/24',CURRENT_TIMESTAMP)")
	untrusted := s.ResolveConnection(ctx, "203.0.113.9:443", "192.168.50.4", "https", false)
	if untrusted.Address != "203.0.113.9" || untrusted.Local || untrusted.Secure {
		t.Fatalf("spoofed forwarding trusted: %+v", untrusted)
	}
	trusted := s.ResolveConnection(ctx, "10.1.2.3:8080", "192.168.50.4", "https", false)
	if trusted.Address != "192.168.50.4" || !trusted.Local || !trusted.Secure || !trusted.TrustedProxy {
		t.Fatalf("trusted forwarding ignored: %+v", trusted)
	}
	v6 := s.ResolveConnection(ctx, "[2001:db8::1]:443", "::1", "https", true)
	if v6.Address != "2001:db8::1" || v6.Local {
		t.Fatalf("untrusted IPv6 forwarding accepted: %+v", v6)
	}
}

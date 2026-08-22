package auth

import (
	"context"
	"encoding/json"
	"github.com/vynode/media/server/internal/database"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testService(t *testing.T) (*Service, func()) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "config")
	store, err := database.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	service, err := New(store.DB, dir, "test-server", time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return service, func() { store.Close() }
}
func TestPasswordArgon2id(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") || strings.Contains(hash, "correct horse") {
		t.Fatal("unsafe hash")
	}
	ok, err := VerifyPassword(hash, "correct horse battery staple")
	if err != nil || !ok {
		t.Fatal("verification failed")
	}
	ok, _ = VerifyPassword(hash, "wrong password")
	if ok {
		t.Fatal("wrong password accepted")
	}
	if _, err = VerifyPassword("broken", "x"); err == nil {
		t.Fatal("malformed hash accepted")
	}
}
func TestSetupLoginRotationReuseAndPersistence(t *testing.T) {
	s, close := testService(t)
	defer close()
	ctx := context.Background()
	required, _ := s.SetupRequired(ctx)
	if !required {
		t.Fatal("fresh setup not required")
	}
	first, err := s.Bootstrap(ctx, "Owner_1", "Owner", "a long secure passphrase", "My Server", "req", "local", DeviceInput{Name: "Browser"})
	if err != nil {
		t.Fatal(err)
	}
	required, _ = s.SetupRequired(ctx)
	if required {
		t.Fatal("setup remained required")
	}
	if first.User.Role != RoleOwner {
		t.Fatal("owner role missing")
	}
	if _, err = s.Bootstrap(ctx, "second", "Second", "another secure password", "", "req", "", DeviceInput{}); err != ErrSetupComplete {
		t.Fatalf("second setup: %v", err)
	}
	login, err := s.Login(ctx, "owner_1", "a long secure passphrase", "req", "local", DeviceInput{Name: "Phone"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Authenticate(login.AccessToken)
	if err != nil || p.UserID != first.User.ID {
		t.Fatal("access token invalid")
	}
	rotated, err := s.Refresh(ctx, login.RefreshToken, "req")
	if err != nil {
		t.Fatal(err)
	}
	if rotated.RefreshToken == login.RefreshToken {
		t.Fatal("refresh did not rotate")
	}
	newest, err := s.Refresh(ctx, rotated.RefreshToken, "req")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Refresh(ctx, login.RefreshToken, "req"); err != ErrSessionRevoked {
		t.Fatalf("reuse not detected: %v", err)
	}
	if _, err = s.Refresh(ctx, newest.RefreshToken, "req"); err != ErrSessionRevoked {
		t.Fatal("family not revoked")
	}
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM users WHERE id=? AND password_hash NOT LIKE ?", first.User.ID, "%a long secure passphrase%").Scan(&count)
	if count != 1 {
		t.Fatal("plaintext stored")
	}
}
func TestConcurrentBootstrapCreatesOneOwner(t *testing.T) {
	s, close := testService(t)
	defer close()
	var wg sync.WaitGroup
	success := 0
	var mu sync.Mutex
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Bootstrap(context.Background(), "owner", "Owner", "a long secure passphrase", "", "r", "", DeviceInput{}); err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("got %d owners", success)
	}
}
func TestRBAC(t *testing.T) {
	if !Allowed(RoleOwner, CapServerManage) || Allowed(RoleAdmin, CapServerManage) || Allowed(RoleUser, CapUsersManage) {
		t.Fatal("invalid grants")
	}
}
func TestTokenExpiryLogoutOthersAndPassword(t *testing.T) {
	s, close := testService(t)
	defer close()
	ctx := context.Background()
	owner, err := s.Bootstrap(ctx, "owner", "Owner", "initial secure passphrase", "", "r", "", DeviceInput{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Login(ctx, "owner", "initial secure passphrase", "r", "", DeviceInput{Name: "Second"})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.Authenticate(owner.AccessToken)
	other, done := testService(t)
	defer done()
	if _, err = other.Authenticate(owner.AccessToken); err != ErrUnauthorized {
		t.Fatal("wrong signing key accepted")
	}
	s.Now = func() time.Time { return time.Now().Add(20 * time.Minute) }
	if _, err = s.Authenticate(owner.AccessToken); err != ErrUnauthorized {
		t.Fatal("expired access accepted")
	}
	s.Now = time.Now
	if err = s.RevokeOthers(ctx, p, "r"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Refresh(ctx, second.RefreshToken, "r"); err != ErrSessionRevoked {
		t.Fatal("logout-others failed")
	}
	if err = s.ChangePassword(ctx, p, "initial secure passphrase", "replacement secure passphrase", "r"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Login(ctx, "owner", "initial secure passphrase", "r", "", DeviceInput{}); err != ErrInvalidCredentials {
		t.Fatal("old password accepted")
	}
	if _, err = s.Login(ctx, "owner", "replacement secure passphrase", "r", "", DeviceInput{}); err != nil {
		t.Fatal("new password rejected")
	}
}
func TestDisableAndAuditOmitCredentials(t *testing.T) {
	s, close := testService(t)
	defer close()
	ctx := context.Background()
	owner, err := s.Bootstrap(ctx, "owner", "Owner", "owner secure passphrase", "", "req", "", DeviceInput{})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.Authenticate(owner.AccessToken)
	u, err := s.CreateUser(ctx, p, "member", "Member", "VYNODE-DO-NOT-LOG-PASSWORD-TEST", RoleUser, "req")
	if err != nil {
		t.Fatal(err)
	}
	login, err := s.Login(ctx, "member", "VYNODE-DO-NOT-LOG-PASSWORD-TEST", "req", "", DeviceInput{})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetEnabled(ctx, p, u.ID, false, "req"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Refresh(ctx, login.RefreshToken, "req"); err != ErrSessionRevoked {
		t.Fatal("disabled refresh accepted")
	}
	rows, _ := s.AuditPage(ctx, 100, 0)
	raw, _ := json.Marshal(rows)
	if strings.Contains(string(raw), "VYNODE-DO-NOT-LOG-PASSWORD-TEST") || strings.Contains(string(raw), login.RefreshToken) {
		t.Fatal("credential found in audit")
	}
}

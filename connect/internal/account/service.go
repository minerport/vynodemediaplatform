package account

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	ErrValidation   = errors.New("validation failed")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("authentication required")
	ErrRevoked      = errors.New("session revoked")
	usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)
)

type DeviceInput struct{ Name, Platform, ClientName, ClientVersion, PlatformVersion string }
type Account struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}
type Tokens struct {
	AccessToken  string  `json:"accessToken"`
	RefreshToken string  `json:"refreshToken,omitempty"`
	TokenType    string  `json:"tokenType"`
	ExpiresIn    int64   `json:"expiresIn"`
	Account      Account `json:"account"`
}
type Principal struct{ AccountID, SessionID, DeviceID string }
type Device struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Platform          string `json:"platform"`
	ClientName        string `json:"clientName"`
	ClientVersion     string `json:"clientVersion"`
	FirstAuthorizedAt string `json:"firstAuthorizedAt"`
	LastSeenAt        string `json:"lastSeenAt"`
	Current           bool   `json:"current"`
}
type Service struct {
	DB                    *sql.DB
	key                   []byte
	Issuer                string
	AccessTTL, RefreshTTL time.Duration
	Now                   func() time.Time
}

func New(db *sql.DB, dir, issuer string) (*Service, error) {
	path := filepath.Join(dir, "connect-access.key")
	key, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		key = make([]byte, 32)
		_, err = rand.Read(key)
		if err == nil {
			err = os.WriteFile(path, key, 0o600)
		}
	}
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("connect access key: %w", err)
	}
	return &Service{DB: db, key: key, Issuer: issuer, AccessTTL: 10 * time.Minute, RefreshTTL: 30 * 24 * time.Hour, Now: time.Now}, nil
}
func NormalizeUsername(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !usernamePattern.MatchString(value) {
		return "", ErrValidation
	}
	return value, nil
}
func ID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", s[:8], s[8:12], s[12:16], s[16:20], s[20:])
}
func digest(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
func newRefresh(sid string) (string, string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	v := sid + "." + base64.RawURLEncoding.EncodeToString(b)
	return v, digest(v)
}
func normalizeDevice(d DeviceInput) DeviceInput {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		d.Name = "VyNode device"
	}
	d.Platform = strings.ToUpper(strings.TrimSpace(d.Platform))
	if d.Platform == "" {
		d.Platform = "UNKNOWN"
	}
	if d.ClientName == "" {
		d.ClientName = "VyNode"
	}
	return d
}

func (s *Service) Register(ctx context.Context, username, displayName, password string, d DeviceInput, requestID string) (Tokens, error) {
	username, err := NormalizeUsername(username)
	displayName = strings.TrimSpace(displayName)
	if err != nil || len(displayName) < 1 || len(displayName) > 100 {
		return Tokens{}, ErrValidation
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Tokens{}, err
	}
	now := s.Now().UTC().Format(time.RFC3339Nano)
	a := Account{ID: ID(), Username: username, DisplayName: displayName, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now}
	_, err = s.DB.ExecContext(ctx, "INSERT INTO accounts(id,username,display_name,password_hash,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?)", a.ID, a.Username, a.DisplayName, hash, a.Status, now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Tokens{}, ErrConflict
		}
		return Tokens{}, err
	}
	_ = s.audit(ctx, "ACCOUNT_CREATED", &a.ID, "account", a.ID, requestID, map[string]any{})
	return s.issue(ctx, a, d, requestID)
}
func (s *Service) Login(ctx context.Context, username, password string, d DeviceInput, requestID string) (Tokens, error) {
	username, _ = NormalizeUsername(username)
	var a Account
	var hash string
	err := s.DB.QueryRowContext(ctx, "SELECT id,username,display_name,status,created_at,updated_at,password_hash FROM accounts WHERE username=?", username).Scan(&a.ID, &a.Username, &a.DisplayName, &a.Status, &a.CreatedAt, &a.UpdatedAt, &hash)
	ok := false
	if err == nil && a.Status == "ACTIVE" {
		ok, _ = VerifyPassword(hash, password)
	}
	if !ok {
		return Tokens{}, ErrUnauthorized
	}
	_ = s.audit(ctx, "LOGIN", &a.ID, "account", a.ID, requestID, map[string]any{})
	return s.issue(ctx, a, d, requestID)
}
func (s *Service) issue(ctx context.Context, a Account, d DeviceInput, requestID string) (Tokens, error) {
	d = normalizeDevice(d)
	did, sid, family := ID(), ID(), ID()
	refresh, hash := newRefresh(sid)
	now := s.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, "INSERT INTO global_devices(id,account_id,name,platform,client_name,client_version,platform_version,first_authorized_at,last_seen_at) VALUES(?,?,?,?,?,?,?,?,?)", did, a.ID, d.Name, d.Platform, d.ClientName, d.ClientVersion, d.PlatformVersion, stamp, stamp)
	if err == nil {
		_, err = tx.ExecContext(ctx, "INSERT INTO global_sessions(id,account_id,device_id,refresh_token_hash,token_family_id,expires_at,created_at,last_activity_at) VALUES(?,?,?,?,?,?,?,?)", sid, a.ID, did, hash, family, now.Add(s.RefreshTTL).Format(time.RFC3339Nano), stamp, stamp)
	}
	if err != nil {
		return Tokens{}, err
	}
	if err = tx.Commit(); err != nil {
		return Tokens{}, err
	}
	_ = s.audit(ctx, "DEVICE_ADDED", &a.ID, "device", did, requestID, map[string]any{"platform": d.Platform})
	access, err := s.sign(a.ID, sid, did, now)
	return Tokens{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: int64(s.AccessTTL.Seconds()), Account: a}, err
}
func (s *Service) sign(aid, sid, did string, now time.Time) (string, error) {
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	raw, _ := json.Marshal(map[string]any{"sub": aid, "sid": sid, "did": did, "iss": s.Issuer, "iat": now.Unix(), "exp": now.Add(s.AccessTTL).Unix()})
	body := base64.RawURLEncoding.EncodeToString(raw)
	msg := head + "." + body
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(msg))
	return msg + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (s *Service) IssueForAccount(ctx context.Context, accountID string, d DeviceInput, requestID string) (Tokens, error) {
	var a Account
	err := s.DB.QueryRowContext(ctx, "SELECT id,username,display_name,status,created_at,updated_at FROM accounts WHERE id=? AND status='ACTIVE'", accountID).Scan(&a.ID, &a.Username, &a.DisplayName, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return Tokens{}, ErrUnauthorized
	}
	return s.issue(ctx, a, d, requestID)
}
func (s *Service) Authenticate(token string) (Principal, error) {
	p := strings.Split(token, ".")
	if len(p) != 3 {
		return Principal{}, ErrUnauthorized
	}
	sig, err := base64.RawURLEncoding.DecodeString(p[2])
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(p[0] + "." + p[1]))
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return Principal{}, ErrUnauthorized
	}
	raw, err := base64.RawURLEncoding.DecodeString(p[1])
	var c struct {
		Sub string `json:"sub"`
		SID string `json:"sid"`
		DID string `json:"did"`
		Iss string `json:"iss"`
		Exp int64  `json:"exp"`
	}
	if err != nil || json.Unmarshal(raw, &c) != nil || c.Iss != s.Issuer || c.Exp <= s.Now().Unix() {
		return Principal{}, ErrUnauthorized
	}
	var active int
	_ = s.DB.QueryRow("SELECT COUNT(*) FROM global_sessions s JOIN accounts a ON a.id=s.account_id JOIN global_devices d ON d.id=s.device_id WHERE s.id=? AND s.account_id=? AND s.device_id=? AND s.revoked_at IS NULL AND d.revoked_at IS NULL AND a.status='ACTIVE' AND s.expires_at>?", c.SID, c.Sub, c.DID, s.Now().UTC().Format(time.RFC3339Nano)).Scan(&active)
	if active != 1 {
		return Principal{}, ErrUnauthorized
	}
	return Principal{c.Sub, c.SID, c.DID}, nil
}
func (s *Service) Refresh(ctx context.Context, token, requestID string) (Tokens, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return Tokens{}, ErrRevoked
	}
	hash := digest(token)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback()
	var a Account
	var did, current string
	var previous, revoked, deviceRevoked sql.NullString
	var expires string
	err = tx.QueryRowContext(ctx, `SELECT a.id,a.username,a.display_name,a.status,a.created_at,a.updated_at,s.device_id,s.refresh_token_hash,s.previous_refresh_hash,s.revoked_at,s.expires_at,d.revoked_at FROM global_sessions s JOIN accounts a ON a.id=s.account_id JOIN global_devices d ON d.id=s.device_id WHERE s.id=?`, parts[0]).Scan(&a.ID, &a.Username, &a.DisplayName, &a.Status, &a.CreatedAt, &a.UpdatedAt, &did, &current, &previous, &revoked, &expires, &deviceRevoked)
	if err != nil {
		return Tokens{}, ErrRevoked
	}
	var used int
	_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM refresh_history WHERE session_id=? AND token_hash=?", parts[0], hash).Scan(&used)
	if used > 0 || (previous.Valid && hmac.Equal([]byte(previous.String), []byte(hash))) {
		now := s.Now().UTC().Format(time.RFC3339Nano)
		_, _ = tx.ExecContext(ctx, "UPDATE global_sessions SET revoked_at=? WHERE token_family_id=(SELECT token_family_id FROM global_sessions WHERE id=?)", now, parts[0])
		_, _ = tx.ExecContext(ctx, "UPDATE global_devices SET revoked_at=?,revocation_version=revocation_version+1 WHERE id=?", now, did)
		_ = tx.Commit()
		_ = s.audit(ctx, "REFRESH_REUSE_DETECTED", &a.ID, "device", did, requestID, map[string]any{"sessionId": parts[0]})
		return Tokens{}, ErrRevoked
	}
	expiry, _ := time.Parse(time.RFC3339Nano, expires)
	if revoked.Valid || deviceRevoked.Valid || a.Status != "ACTIVE" || s.Now().After(expiry) || !hmac.Equal([]byte(current), []byte(hash)) {
		return Tokens{}, ErrRevoked
	}
	next, nextHash := newRefresh(parts[0])
	now := s.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, "INSERT INTO refresh_history(session_id,token_hash,rotated_at) VALUES(?,?,?)", parts[0], current, stamp)
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE global_sessions SET previous_refresh_hash=refresh_token_hash,refresh_token_hash=?,last_activity_at=? WHERE id=?", nextHash, stamp, parts[0])
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE global_devices SET last_seen_at=? WHERE id=?", stamp, did)
	}
	if err != nil {
		return Tokens{}, err
	}
	if err = tx.Commit(); err != nil {
		return Tokens{}, err
	}
	access, err := s.sign(a.ID, parts[0], did, now)
	return Tokens{AccessToken: access, RefreshToken: next, TokenType: "Bearer", ExpiresIn: int64(s.AccessTTL.Seconds()), Account: a}, err
}
func (s *Service) Me(ctx context.Context, p Principal) (Account, error) {
	var a Account
	err := s.DB.QueryRowContext(ctx, "SELECT id,username,display_name,status,created_at,updated_at FROM accounts WHERE id=?", p.AccountID).Scan(&a.ID, &a.Username, &a.DisplayName, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}
func (s *Service) Devices(ctx context.Context, p Principal) ([]Device, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id,name,platform,client_name,client_version,first_authorized_at,last_seen_at FROM global_devices WHERE account_id=? AND revoked_at IS NULL ORDER BY last_seen_at DESC", p.AccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		var d Device
		if err = rows.Scan(&d.ID, &d.Name, &d.Platform, &d.ClientName, &d.ClientVersion, &d.FirstAuthorizedAt, &d.LastSeenAt); err != nil {
			return nil, err
		}
		d.Current = d.ID == p.DeviceID
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Service) RevokeDevice(ctx context.Context, p Principal, id, requestID string) error {
	now := s.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, "UPDATE global_devices SET revoked_at=?,revocation_version=revocation_version+1 WHERE id=? AND account_id=? AND revoked_at IS NULL", now, id, p.AccountID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrUnauthorized
	}
	_, err = tx.ExecContext(ctx, "UPDATE global_sessions SET revoked_at=? WHERE device_id=? AND revoked_at IS NULL", now, id)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.audit(ctx, "DEVICE_REVOKED", &p.AccountID, "device", id, requestID, map[string]any{})
}
func (s *Service) Logout(ctx context.Context, p Principal) error {
	_, err := s.DB.ExecContext(ctx, "UPDATE global_sessions SET revoked_at=? WHERE id=? AND account_id=?", s.Now().UTC().Format(time.RFC3339Nano), p.SessionID, p.AccountID)
	return err
}
func (s *Service) audit(ctx context.Context, event string, actor *string, targetType, targetID, requestID string, metadata map[string]any) error {
	raw, _ := json.Marshal(metadata)
	_, err := s.DB.ExecContext(ctx, "INSERT INTO audit_events(id,event_type,actor_account_id,target_type,target_id,request_id,metadata_json,created_at) VALUES(?,?,?,?,?,?,?,?)", ID(), event, actor, targetType, targetID, requestID, string(raw), s.Now().UTC().Format(time.RFC3339Nano))
	return err
}

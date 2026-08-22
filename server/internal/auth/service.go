package auth

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

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{2,63}$`)

type Service struct {
	DB                    *sql.DB
	key                   []byte
	issuer                string
	AccessTTL, RefreshTTL time.Duration
	Now                   func() time.Time
}
type DeviceInput struct {
	Name            string `json:"name"`
	ClientName      string `json:"clientName"`
	ClientVersion   string `json:"clientVersion"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platformVersion"`
}
type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        Role   `json:"role"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
}
type Tokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int64  `json:"expiresIn"`
	User         User   `json:"user"`
}
type Principal struct {
	UserID, SessionID string
	Role              Role
}
type Session struct {
	ID             string `json:"id"`
	DeviceName     string `json:"deviceName"`
	ClientName     string `json:"clientName"`
	Platform       string `json:"platform"`
	CreatedAt      string `json:"createdAt"`
	LastActivityAt string `json:"lastActivityAt"`
	Current        bool   `json:"current"`
}

func New(db *sql.DB, dir, issuer string, access, refresh time.Duration) (*Service, error) {
	path := filepath.Join(dir, "auth-signing.key")
	key, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		key = make([]byte, 32)
		_, err = rand.Read(key)
		if err == nil {
			err = os.WriteFile(path, key, 0o600)
		}
	}
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("authentication key invalid: %w", err)
	}
	return &Service{DB: db, key: key, issuer: issuer, AccessTTL: access, RefreshTTL: refresh, Now: time.Now}, nil
}
func ID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", s[:8], s[8:12], s[12:16], s[16:20], s[20:])
}
func username(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	if !usernamePattern.MatchString(v) {
		return "", ErrValidation
	}
	return v, nil
}
func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	var completed sql.NullString
	err := s.DB.QueryRowContext(ctx, "SELECT completed_at FROM setup_state WHERE id=1").Scan(&completed)
	return !completed.Valid, err
}
func (s *Service) Bootstrap(ctx context.Context, user, name, password, serverName, requestID, remote string, device DeviceInput) (Tokens, error) {
	user, err := username(user)
	if err != nil || len(strings.TrimSpace(name)) < 1 || len(name) > 100 {
		return Tokens{}, ErrValidation
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Tokens{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback()
	var complete sql.NullString
	if err = tx.QueryRowContext(ctx, "SELECT completed_at FROM setup_state WHERE id=1").Scan(&complete); err != nil {
		return Tokens{}, err
	}
	if complete.Valid {
		return Tokens{}, ErrSetupComplete
	}
	u := User{ID: ID(), Username: user, DisplayName: strings.TrimSpace(name), Role: RoleOwner, Status: "ACTIVE", CreatedAt: s.Now().UTC().Format(time.RFC3339Nano)}
	_, err = tx.ExecContext(ctx, "INSERT INTO users(id,username,password_hash,role,display_name,status,created_at) VALUES(?,?,?,?,?,?,?)", u.ID, u.Username, hash, u.Role, u.DisplayName, u.Status, u.CreatedAt)
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE setup_state SET completed_at=? WHERE id=1 AND completed_at IS NULL", u.CreatedAt)
	}
	if err == nil && strings.TrimSpace(serverName) != "" {
		_, err = tx.ExecContext(ctx, "INSERT INTO server_settings(key,value) VALUES('server_name',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", strings.TrimSpace(serverName))
	}
	if err == nil {
		err = audit(ctx, tx, "SERVER_SETUP_COMPLETED", &u.ID, "server", s.issuer, requestID, map[string]any{"outcome": "success"})
	}
	if err != nil {
		return Tokens{}, mapDB(err)
	}
	if err = tx.Commit(); err != nil {
		return Tokens{}, err
	}
	return s.createSession(ctx, u, device, remote, requestID)
}
func (s *Service) Login(ctx context.Context, user, password, requestID, remote string, device DeviceInput) (Tokens, error) {
	user, _ = username(user)
	var u User
	var hash string
	err := s.DB.QueryRowContext(ctx, "SELECT id,username,display_name,role,status,created_at,password_hash FROM users WHERE username=?", user).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &u.CreatedAt, &hash)
	ok := false
	if err == nil && u.Status == "ACTIVE" {
		ok, _ = VerifyPassword(hash, password)
	}
	if !ok {
		_ = s.Audit(ctx, "USER_LOGIN_FAILED", nil, "user", "", requestID, map[string]any{"outcome": "denied"})
		return Tokens{}, ErrInvalidCredentials
	}
	_ = s.Audit(ctx, "USER_LOGIN_SUCCEEDED", &u.ID, "user", u.ID, requestID, map[string]any{"outcome": "success"})
	return s.createSession(ctx, u, device, remote, requestID)
}
func (s *Service) createSession(ctx context.Context, u User, d DeviceInput, remote, requestID string) (Tokens, error) {
	if d.Name == "" {
		d.Name = "Web browser"
	}
	if d.Platform == "" {
		d.Platform = "web"
	}
	did, sid, family := ID(), ID(), ID()
	refresh, hash := newRefresh(sid)
	now := s.Now().UTC()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, "INSERT INTO devices(id,user_id,name,platform,client_name,client_version,platform_version,created_at,last_seen_at) VALUES(?,?,?,?,?,?,?,?,?)", did, u.ID, d.Name, d.Platform, d.ClientName, d.ClientVersion, d.PlatformVersion, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err == nil {
		_, err = tx.ExecContext(ctx, "INSERT INTO sessions(id,user_id,device_id,refresh_token_hash,token_family_id,expires_at,created_at,last_activity_at,remote_address) VALUES(?,?,?,?,?,?,?,?,?)", sid, u.ID, did, hash, family, now.Add(s.RefreshTTL).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), remote)
	}
	if err == nil {
		err = audit(ctx, tx, "SESSION_CREATED", &u.ID, "session", sid, requestID, map[string]any{"deviceName": d.Name, "client": d.ClientName, "platform": d.Platform, "outcome": "success"})
	}
	if err != nil {
		return Tokens{}, err
	}
	if err = tx.Commit(); err != nil {
		return Tokens{}, err
	}
	access, err := s.sign(u.ID, sid, u.Role, now)
	return Tokens{access, refresh, "Bearer", int64(s.AccessTTL.Seconds()), u}, err
}
func newRefresh(sid string) (string, string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token := sid + "." + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:])
}
func (s *Service) sign(uid, sid string, role Role, now time.Time) (string, error) {
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	raw, _ := json.Marshal(map[string]any{"sub": uid, "sid": sid, "role": role, "iss": s.issuer, "iat": now.Unix(), "exp": now.Add(s.AccessTTL).Unix()})
	body := base64.RawURLEncoding.EncodeToString(raw)
	msg := head + "." + body
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(msg))
	return msg + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (s *Service) Authenticate(token string) (Principal, error) {
	p := strings.Split(token, ".")
	if len(p) != 3 {
		return Principal{}, ErrUnauthorized
	}
	sig, err := base64.RawURLEncoding.DecodeString(p[2])
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(p[0] + "." + p[1]))
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return Principal{}, ErrUnauthorized
	}
	raw, err := base64.RawURLEncoding.DecodeString(p[1])
	var c struct {
		Sub  string `json:"sub"`
		SID  string `json:"sid"`
		Role Role   `json:"role"`
		Iss  string `json:"iss"`
		Exp  int64  `json:"exp"`
	}
	if err != nil || json.Unmarshal(raw, &c) != nil || c.Iss != s.issuer || c.Exp <= s.Now().Unix() {
		return Principal{}, ErrUnauthorized
	}
	return Principal{c.Sub, c.SID, c.Role}, nil
}
func (s *Service) Refresh(ctx context.Context, token, requestID string) (Tokens, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return Tokens{}, ErrSessionRevoked
	}
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback()
	var u User
	var current string
	var previous, revoked sql.NullString
	var expires string
	err = tx.QueryRowContext(ctx, `SELECT u.id,u.username,u.display_name,u.role,u.status,u.created_at,s.refresh_token_hash,s.previous_refresh_hash,s.revoked_at,s.expires_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.id=?`, parts[0]).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &u.CreatedAt, &current, &previous, &revoked, &expires)
	if err != nil {
		return Tokens{}, ErrSessionRevoked
	}
	var usedCount int
	_ = tx.QueryRowContext(ctx, "SELECT COUNT(1) FROM refresh_token_history WHERE session_id=? AND token_hash=?", parts[0], hash).Scan(&usedCount)
	if usedCount > 0 || (previous.Valid && hmac.Equal([]byte(previous.String), []byte(hash))) {
		now := s.Now().UTC().Format(time.RFC3339Nano)
		_, _ = tx.ExecContext(ctx, "UPDATE sessions SET revoked_at=? WHERE id=?", now, parts[0])
		_ = audit(ctx, tx, "SESSION_REFRESH_REUSE_DETECTED", &u.ID, "session", parts[0], requestID, map[string]any{"outcome": "revoked"})
		_ = tx.Commit()
		return Tokens{}, ErrSessionRevoked
	}
	expiry, _ := time.Parse(time.RFC3339Nano, expires)
	if revoked.Valid || u.Status != "ACTIVE" || s.Now().After(expiry) || !hmac.Equal([]byte(current), []byte(hash)) {
		return Tokens{}, ErrSessionRevoked
	}
	next, nextHash := newRefresh(parts[0])
	now := s.Now().UTC()
	_, err = tx.ExecContext(ctx, "INSERT INTO refresh_token_history(session_id,token_hash,rotated_at) VALUES(?,?,?)", parts[0], current, now.Format(time.RFC3339Nano))
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE sessions SET previous_refresh_hash=refresh_token_hash,refresh_token_hash=?,last_activity_at=? WHERE id=?", nextHash, now.Format(time.RFC3339Nano), parts[0])
	}
	if err == nil {
		err = audit(ctx, tx, "SESSION_REFRESHED", &u.ID, "session", parts[0], requestID, map[string]any{"outcome": "success"})
	}
	if err != nil {
		return Tokens{}, err
	}
	if err = tx.Commit(); err != nil {
		return Tokens{}, err
	}
	access, err := s.sign(u.ID, parts[0], u.Role, now)
	return Tokens{access, next, "Bearer", int64(s.AccessTTL.Seconds()), u}, err
}
func (s *Service) Me(ctx context.Context, p Principal) (User, error) {
	var u User
	err := s.DB.QueryRowContext(ctx, "SELECT id,username,display_name,role,status,created_at FROM users WHERE id=?", p.UserID).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &u.CreatedAt)
	return u, err
}
func (s *Service) Sessions(ctx context.Context, p Principal) ([]Session, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT s.id,d.name,d.client_name,d.platform,s.created_at,s.last_activity_at FROM sessions s JOIN devices d ON d.id=s.device_id WHERE s.user_id=? AND s.revoked_at IS NULL AND s.expires_at>? ORDER BY s.last_activity_at DESC`, p.UserID, s.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var v Session
		if err = rows.Scan(&v.ID, &v.DeviceName, &v.ClientName, &v.Platform, &v.CreatedAt, &v.LastActivityAt); err != nil {
			return nil, err
		}
		v.Current = v.ID == p.SessionID
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) Revoke(ctx context.Context, p Principal, sid, requestID, event string) error {
	r, err := s.DB.ExecContext(ctx, "UPDATE sessions SET revoked_at=? WHERE id=? AND user_id=? AND revoked_at IS NULL", s.Now().UTC().Format(time.RFC3339Nano), sid, p.UserID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrForbidden
	}
	return s.Audit(ctx, event, &p.UserID, "session", sid, requestID, map[string]any{"outcome": "success"})
}
func (s *Service) RevokeOthers(ctx context.Context, p Principal, requestID string) error {
	_, err := s.DB.ExecContext(ctx, "UPDATE sessions SET revoked_at=? WHERE user_id=? AND id<>? AND revoked_at IS NULL", s.Now().UTC().Format(time.RFC3339Nano), p.UserID, p.SessionID)
	return err
}
func (s *Service) ChangePassword(ctx context.Context, p Principal, current, next, requestID string) error {
	var hash string
	if s.DB.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE id=?", p.UserID).Scan(&hash) != nil {
		return ErrInvalidCredentials
	}
	ok, _ := VerifyPassword(hash, current)
	if !ok {
		return ErrInvalidCredentials
	}
	nextHash, err := HashPassword(next)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, "UPDATE users SET password_hash=? WHERE id=?", nextHash, p.UserID)
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE sessions SET revoked_at=? WHERE user_id=? AND id<>? AND revoked_at IS NULL", s.Now().UTC().Format(time.RFC3339Nano), p.UserID, p.SessionID)
	}
	if err == nil {
		err = audit(ctx, tx, "PASSWORD_CHANGED", &p.UserID, "user", p.UserID, requestID, map[string]any{"outcome": "success"})
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id,username,display_name,role,status,created_at FROM users ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func (s *Service) GetUser(ctx context.Context, id string) (User, error) {
	var u User
	err := s.DB.QueryRowContext(ctx, "SELECT id,username,display_name,role,status,created_at FROM users WHERE id=?", id).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &u.CreatedAt)
	return u, err
}
func (s *Service) CreateUser(ctx context.Context, p Principal, user, name, password string, role Role, requestID string) (User, error) {
	if role != RoleAdmin && role != RoleUser {
		return User{}, ErrForbidden
	}
	user, err := username(user)
	if err != nil || len(strings.TrimSpace(name)) < 1 {
		return User{}, ErrValidation
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	u := User{ID: ID(), Username: user, DisplayName: strings.TrimSpace(name), Role: role, Status: "ACTIVE", CreatedAt: s.Now().UTC().Format(time.RFC3339Nano)}
	_, err = s.DB.ExecContext(ctx, "INSERT INTO users(id,username,password_hash,role,display_name,status,created_at) VALUES(?,?,?,?,?,?,?)", u.ID, u.Username, hash, u.Role, u.DisplayName, u.Status, u.CreatedAt)
	if err != nil {
		return User{}, mapDB(err)
	}
	_ = s.Audit(ctx, "ACCOUNT_CREATED", &p.UserID, "user", u.ID, requestID, map[string]any{"role": role, "outcome": "success"})
	return u, nil
}
func (s *Service) SetEnabled(ctx context.Context, p Principal, id string, enabled bool, requestID string) error {
	u, err := s.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if u.Role == RoleOwner {
		return ErrForbidden
	}
	status := "DISABLED"
	var disabled any = s.Now().UTC().Format(time.RFC3339Nano)
	if enabled {
		status = "ACTIVE"
		disabled = nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, "UPDATE users SET status=?,disabled_at=? WHERE id=?", status, disabled, id)
	if err == nil && !enabled {
		_, err = tx.ExecContext(ctx, "UPDATE sessions SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL", disabled, id)
	}
	if err == nil {
		err = audit(ctx, tx, "SECURITY_SETTING_CHANGED", &p.UserID, "user", id, requestID, map[string]any{"status": status, "outcome": "success"})
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) Audit(ctx context.Context, event string, actor *string, targetType, targetID, requestID string, metadata map[string]any) error {
	return audit(ctx, s.DB, event, actor, targetType, targetID, requestID, metadata)
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func audit(ctx context.Context, e execer, event string, actor *string, targetType, targetID, requestID string, metadata map[string]any) error {
	raw, _ := json.Marshal(metadata)
	_, err := e.ExecContext(ctx, "INSERT INTO audit_events(actor_user_id,action,target_type,target_id,request_id,metadata_json,created_at) VALUES(?,?,?,?,?,?,?)", actor, event, targetType, targetID, requestID, string(raw), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func (s *Service) AuditPage(ctx context.Context, limit, offset int) ([]map[string]any, error) {
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT id,action,actor_user_id,target_type,target_id,created_at,metadata_json FROM audit_events ORDER BY id DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var event, created, raw string
		var actor, targetType, targetID sql.NullString
		if err = rows.Scan(&id, &event, &actor, &targetType, &targetID, &created, &raw); err != nil {
			return nil, err
		}
		var meta map[string]any
		_ = json.Unmarshal([]byte(raw), &meta)
		out = append(out, map[string]any{"id": id, "event": event, "actorUserId": actor.String, "targetType": targetType.String, "targetId": targetID.String, "timestamp": created, "metadata": meta})
	}
	return out, rows.Err()
}
func mapDB(err error) error {
	if strings.Contains(strings.ToLower(err.Error()), "unique") {
		return ErrUsernameExists
	}
	return err
}

var _ = errors.Is

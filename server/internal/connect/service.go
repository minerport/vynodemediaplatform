package connect

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vynode/media/server/internal/auth"
)

var (
	ErrInvalid  = errors.New("invalid connect assertion")
	ErrUnlinked = errors.New("global account is not linked")
)

type Service struct {
	db       *sql.DB
	auth     *auth.Service
	serverID string
	now      func() time.Time
	identity Identity
}

func (s *Service) ConfigureIdentity(v Identity) { s.identity = v }

type claims struct {
	Issuer       string `json:"iss"`
	Audience     string `json:"aud"`
	Subject      string `json:"sub"`
	DeviceID     string `json:"did"`
	Nonce        string `json:"nonce"`
	IssuedAt     int64  `json:"iat"`
	ExpiresAt    int64  `json:"exp"`
	Purpose      string `json:"purpose"`
	State        string `json:"state"`
	JTI          string `json:"jti"`
	InvitationID string `json:"invitationId"`
	IntentDigest string `json:"intentDigest"`
	Username     string `json:"username"`
	DisplayName  string `json:"displayName"`
}
type jwks struct {
	Keys []struct {
		Kid     string `json:"kid"`
		KeyType string `json:"kty"`
		Curve   string `json:"crv"`
		X       string `json:"x"`
	} `json:"keys"`
}

func New(db *sql.DB, a *auth.Service, serverID string) *Service {
	return &Service{db: db, auth: a, serverID: serverID, now: time.Now}
}

type Settings struct {
	Enabled       bool            `json:"enabled"`
	ConnectURL    string          `json:"connectUrl"`
	Issuer        string          `json:"issuer"`
	SigningKeys   json.RawMessage `json:"signingKeys"`
	LastContactAt *string         `json:"lastContactAt,omitempty"`
	LastError     string          `json:"lastError"`
}

func (s *Service) Settings(ctx context.Context) (Settings, error) {
	var v Settings
	var enabled int
	var raw string
	var last sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT enabled,connect_url,connect_issuer,connect_signing_keys_json,last_contact_at,last_error FROM connect_settings WHERE id=1").Scan(&enabled, &v.ConnectURL, &v.Issuer, &raw, &last, &v.LastError)
	v.Enabled = enabled == 1
	v.SigningKeys = json.RawMessage(raw)
	if last.Valid {
		v.LastContactAt = &last.String
	}
	return v, err
}
func (s *Service) Configure(ctx context.Context, enabled bool, connectURL, issuer string, keys json.RawMessage) error {
	if enabled && (strings.TrimSpace(connectURL) == "" || strings.TrimSpace(issuer) == "" || len(keys) == 0) {
		return ErrInvalid
	}
	var parsed jwks
	if len(keys) == 0 {
		keys = json.RawMessage(`{"keys":[]}`)
	}
	if json.Unmarshal(keys, &parsed) != nil {
		return ErrInvalid
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, "UPDATE connect_settings SET enabled=?,connect_url=?,connect_issuer=?,connect_signing_keys_json=?,updated_at=? WHERE id=1", boolInt(enabled), strings.TrimSpace(connectURL), strings.TrimSpace(issuer), string(keys), now)
	return err
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

type LinkRequest struct {
	State      string `json:"state"`
	ExpiresAt  string `json:"expiresAt"`
	ServerID   string `json:"serverId"`
	ConnectURL string `json:"connectUrl"`
}

func token(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func hash(v string) string { x := sha256.Sum256([]byte(v)); return fmt.Sprintf("%x", x[:]) }
func (s *Service) CreateLinkRequest(ctx context.Context, p auth.Principal) (LinkRequest, error) {
	settings, err := s.Settings(ctx)
	if err != nil || !settings.Enabled {
		return LinkRequest{}, ErrInvalid
	}
	state := token(32)
	now := s.now().UTC()
	_, err = s.db.ExecContext(ctx, "INSERT INTO connect_link_requests(id,state_hash,user_id,session_id,status,expires_at,created_at) VALUES(?,?,?,?, 'PENDING',?,?)", token(16), hash(state), p.UserID, p.SessionID, now.Add(5*time.Minute).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return LinkRequest{}, err
	}
	return LinkRequest{State: state, ExpiresAt: now.Add(5 * time.Minute).Format(time.RFC3339Nano), ServerID: s.serverID, ConnectURL: settings.ConnectURL}, nil
}
func (s *Service) CompleteLink(ctx context.Context, p auth.Principal, state, grant string) error {
	c, err := s.verifyLinkGrant(ctx, grant)
	if err != nil || c.State != state {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id, userID, sessionID, status, expiry string
	if tx.QueryRowContext(ctx, "SELECT id,user_id,session_id,status,expires_at FROM connect_link_requests WHERE state_hash=?", hash(state)).Scan(&id, &userID, &sessionID, &status, &expiry) != nil {
		return ErrInvalid
	}
	exp, _ := time.Parse(time.RFC3339Nano, expiry)
	if status != "PENDING" || userID != p.UserID || sessionID != p.SessionID || !s.now().Before(exp) {
		return ErrInvalid
	}
	var existingGlobal sql.NullString
	if err = tx.QueryRowContext(ctx, "SELECT global_account_id FROM global_account_links WHERE user_id=? AND status='ACTIVE'", p.UserID).Scan(&existingGlobal); err != nil && err != sql.ErrNoRows {
		return err
	}
	if existingGlobal.Valid && existingGlobal.String != c.Subject {
		return ErrUnlinked
	}
	var existingUser sql.NullString
	if err = tx.QueryRowContext(ctx, "SELECT user_id FROM global_account_links WHERE global_account_id=? AND status='ACTIVE'", c.Subject).Scan(&existingUser); err != nil && err != sql.ErrNoRows {
		return err
	}
	if existingUser.Valid && existingUser.String != p.UserID {
		return ErrUnlinked
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `INSERT INTO global_account_links(global_account_id,user_id,status,linked_by_user_id,created_at,updated_at) VALUES(?,?,'ACTIVE',?,?,?) ON CONFLICT(global_account_id) DO UPDATE SET status='ACTIVE',updated_at=excluded.updated_at`, c.Subject, p.UserID, p.UserID, now, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE server_connect_identity SET claim_status='CLAIMED',updated_at=? WHERE id=1", now); err != nil {
		return err
	}
	r, err := tx.ExecContext(ctx, "UPDATE connect_link_requests SET status='CONSUMED',consumed_at=? WHERE id=? AND status='PENDING'", now, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrInvalid
	}
	if err = tx.Commit(); err == nil {
		s.identity.ClaimStatus = "CLAIMED"
	}
	return err
}
func (s *Service) verifyLinkGrant(ctx context.Context, grant string) (claims, error) {
	parts := strings.Split(grant, ".")
	if len(parts) != 3 {
		return claims{}, ErrInvalid
	}
	hb, e1 := base64.RawURLEncoding.DecodeString(parts[0])
	cb, e2 := base64.RawURLEncoding.DecodeString(parts[1])
	sig, e3 := base64.RawURLEncoding.DecodeString(parts[2])
	var h struct {
		Algorithm string `json:"alg"`
		Kid       string `json:"kid"`
	}
	var c claims
	if e1 != nil || e2 != nil || e3 != nil || json.Unmarshal(hb, &h) != nil || json.Unmarshal(cb, &c) != nil || h.Algorithm != "EdDSA" || c.Purpose != "account-link" || c.Audience != s.serverID || c.State == "" || c.JTI == "" {
		return claims{}, ErrInvalid
	}
	var issuer, raw string
	if s.db.QueryRowContext(ctx, "SELECT connect_issuer,connect_signing_keys_json FROM connect_settings WHERE id=1 AND enabled=1").Scan(&issuer, &raw) != nil || issuer != c.Issuer || c.ExpiresAt <= s.now().Unix() || c.IssuedAt > s.now().Add(time.Minute).Unix() || c.ExpiresAt-c.IssuedAt > 300 {
		return claims{}, ErrInvalid
	}
	var keys jwks
	if json.Unmarshal([]byte(raw), &keys) != nil {
		return claims{}, ErrInvalid
	}
	for _, k := range keys.Keys {
		if k.Kid == h.Kid && k.KeyType == "OKP" && k.Curve == "Ed25519" {
			pub, e := base64.RawURLEncoding.DecodeString(k.X)
			if e == nil && len(pub) == ed25519.PublicKeySize && ed25519.Verify(ed25519.PublicKey(pub), []byte(parts[0]+"."+parts[1]), sig) {
				return c, nil
			}
		}
	}
	return claims{}, ErrInvalid
}
func (s *Service) Unlink(ctx context.Context, p auth.Principal) error {
	now := s.now().UTC().Format(time.RFC3339Nano)
	r, err := s.db.ExecContext(ctx, "UPDATE global_account_links SET status='REVOKED',updated_at=? WHERE user_id=? AND status='ACTIVE'", now, p.UserID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrUnlinked
	}
	return nil
}
func (s *Service) UnlinkServer(ctx context.Context) error {
	now := s.now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE global_account_links SET status='REVOKED',updated_at=? WHERE status='ACTIVE'", now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE connect_settings SET enabled=0,last_error='',updated_at=? WHERE id=1", now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE server_connect_identity SET claim_status='REVOKED',updated_at=? WHERE id=1", now); err != nil {
		return err
	}
	if err = tx.Commit(); err == nil {
		s.identity.ClaimStatus = "REVOKED"
	}
	return err
}
func (s *Service) Exchange(ctx context.Context, assertion string, d auth.DeviceInput, remote, requestID string) (auth.Tokens, error) {
	parts := strings.Split(assertion, ".")
	if len(parts) != 3 {
		return auth.Tokens{}, ErrInvalid
	}
	headerBytes, e1 := base64.RawURLEncoding.DecodeString(parts[0])
	claimBytes, e2 := base64.RawURLEncoding.DecodeString(parts[1])
	signature, e3 := base64.RawURLEncoding.DecodeString(parts[2])
	var header struct {
		Algorithm string `json:"alg"`
		Kid       string `json:"kid"`
	}
	var c claims
	if e1 != nil || e2 != nil || e3 != nil || json.Unmarshal(headerBytes, &header) != nil || json.Unmarshal(claimBytes, &c) != nil || header.Algorithm != "EdDSA" {
		return auth.Tokens{}, ErrInvalid
	}
	var issuer, rawKeys string
	if s.db.QueryRowContext(ctx, "SELECT connect_issuer,connect_signing_keys_json FROM connect_settings WHERE id=1 AND enabled=1").Scan(&issuer, &rawKeys) != nil {
		return auth.Tokens{}, ErrInvalid
	}
	if c.Issuer != issuer || c.Audience != s.serverID || c.Subject == "" || c.DeviceID == "" || c.Nonce == "" || c.ExpiresAt <= s.now().Unix() || c.IssuedAt > s.now().Add(time.Minute).Unix() || c.ExpiresAt-c.IssuedAt > 300 {
		return auth.Tokens{}, ErrInvalid
	}
	var keys jwks
	if json.Unmarshal([]byte(rawKeys), &keys) != nil {
		return auth.Tokens{}, ErrInvalid
	}
	valid := false
	for _, k := range keys.Keys {
		if k.Kid != header.Kid || k.KeyType != "OKP" || k.Curve != "Ed25519" {
			continue
		}
		public, err := base64.RawURLEncoding.DecodeString(k.X)
		if err == nil && len(public) == ed25519.PublicKeySize && ed25519.Verify(ed25519.PublicKey(public), []byte(parts[0]+"."+parts[1]), signature) {
			valid = true
			break
		}
	}
	if !valid {
		return auth.Tokens{}, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.Tokens{}, err
	}
	defer tx.Rollback()
	var userID string
	if tx.QueryRowContext(ctx, "SELECT user_id FROM global_account_links WHERE global_account_id=? AND status='ACTIVE'", c.Subject).Scan(&userID) != nil {
		return auth.Tokens{}, ErrUnlinked
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, "INSERT INTO connect_assertion_nonces(nonce,global_account_id,expires_at,consumed_at) VALUES(?,?,?,?)", c.Nonce, c.Subject, time.Unix(c.ExpiresAt, 0).UTC().Format(time.RFC3339Nano), now); err != nil {
		return auth.Tokens{}, ErrInvalid
	}
	if err = tx.Commit(); err != nil {
		return auth.Tokens{}, err
	}
	u, err := s.auth.GetUser(ctx, userID)
	if err != nil || u.Status != "ACTIVE" {
		return auth.Tokens{}, ErrUnlinked
	}
	tokens, err := s.auth.IssueSession(ctx, u, d, remote, requestID)
	if err != nil {
		return auth.Tokens{}, err
	}
	sessionID := strings.SplitN(tokens.RefreshToken, ".", 2)[0]
	if _, err = s.db.ExecContext(ctx, "INSERT INTO connect_global_device_sessions(global_device_id,global_account_id,local_session_id,created_at) VALUES(?,?,?,?)", c.DeviceID, c.Subject, sessionID, s.now().UTC().Format(time.RFC3339Nano)); err != nil {
		_, _ = s.db.ExecContext(ctx, "UPDATE sessions SET revoked_at=? WHERE id=? AND revoked_at IS NULL", s.now().UTC().Format(time.RFC3339Nano), sessionID)
		return auth.Tokens{}, err
	}
	return tokens, nil
}

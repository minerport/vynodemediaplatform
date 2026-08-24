package registry

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vynode/media/connect/internal/account"
)

var (
	ErrInvalid       = errors.New("invalid request")
	ErrForbidden     = errors.New("forbidden")
	ErrGone          = errors.New("unavailable")
	ErrDeviceDenied  = fmt.Errorf("device authorization denied: %w", ErrGone)
	ErrDeviceExpired = fmt.Errorf("device authorization expired: %w", ErrGone)
	ErrConflict      = errors.New("conflict")
)

type Service struct {
	DB           *sql.DB
	Dir, Issuer  string
	Now          func() time.Time
	AssertionTTL time.Duration
}
type Server struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Relationship string     `json:"relationship"`
	Status       string     `json:"status"`
	PublicKey    string     `json:"publicKey"`
	Version      string     `json:"version"`
	LastSeenAt   *string    `json:"lastSeenAt,omitempty"`
	Endpoints    []Endpoint `json:"endpoints"`
}
type Endpoint struct {
	URL        string  `json:"url"`
	Kind       string  `json:"kind"`
	Secure     bool    `json:"secure"`
	VerifiedAt *string `json:"verifiedAt,omitempty"`
}
type DeviceCode struct {
	ID               string `json:"id"`
	DeviceCode       string `json:"deviceCode"`
	UserCode         string `json:"userCode"`
	VerificationPath string `json:"verificationPath"`
	ExpiresAt        string `json:"expiresAt"`
	PollAfterSeconds int    `json:"pollAfterSeconds"`
}
type Invitation struct {
	ID               string `json:"id"`
	Token            string `json:"token,omitempty"`
	ServerID         string `json:"serverId"`
	ServerName       string `json:"serverName,omitempty"`
	IntendedUsername string `json:"intendedUsername,omitempty"`
	Status           string `json:"status"`
	ExpiresAt        string `json:"expiresAt"`
}
type InvitationAcceptance struct {
	Server       Server `json:"server"`
	InvitationID string `json:"invitationId"`
	Redemption   string `json:"redemption"`
	Assertion    string `json:"assertion"`
}
type DeviceRevocation struct {
	DeviceID        string `json:"deviceId"`
	GlobalAccountID string `json:"globalAccountId"`
	RevokedAt       string `json:"revokedAt"`
}

func New(db *sql.DB, dir, issuer string) *Service {
	return &Service{DB: db, Dir: dir, Issuer: issuer, Now: time.Now, AssertionTTL: 2 * time.Minute}
}
func digest(v string) string { x := sha256.Sum256([]byte(v)); return hex.EncodeToString(x[:]) }
func random(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func canonicalRegister(id, name, key string) []byte {
	return []byte("vynode-connect-register-v1\n" + id + "\n" + strings.TrimSpace(name) + "\n" + key)
}
func canonicalInvitation(serverID, invitationID, username, intentDigest, expiresAt string) []byte {
	return []byte("vynode-connect-invitation-v1\n" + serverID + "\n" + invitationID + "\n" + strings.ToLower(strings.TrimSpace(username)) + "\n" + intentDigest + "\n" + expiresAt)
}
func canonicalInvitationRevoke(serverID, invitationID string) []byte {
	return []byte("vynode-connect-invitation-revoke-v1\n" + serverID + "\n" + invitationID)
}
func decodePublic(v string) (ed25519.PublicKey, error) {
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, ErrInvalid
	}
	return ed25519.PublicKey(b), nil
}
func verify(publicValue, signature string, message []byte) bool {
	key, err := decodePublic(publicValue)
	sig, e := base64.RawURLEncoding.DecodeString(signature)
	return err == nil && e == nil && ed25519.Verify(key, message, sig)
}

func (s *Service) RegisterServer(ctx context.Context, id, name, publicKey, version, signature string) (Server, error) {
	if id == "" || strings.TrimSpace(name) == "" {
		return Server{}, ErrInvalid
	}
	if !verify(publicKey, signature, canonicalRegister(id, name, publicKey)) {
		return Server{}, ErrForbidden
	}
	now := s.Now().UTC().Format(time.RFC3339Nano)
	var existingKey, existingStatus string
	if err := s.DB.QueryRowContext(ctx, "SELECT public_key,status FROM servers WHERE id=?", id).Scan(&existingKey, &existingStatus); err == nil {
		if existingKey != publicKey {
			return Server{}, ErrConflict
		}
		if existingStatus == "REVOKED" {
			_, err = s.DB.ExecContext(ctx, "UPDATE servers SET name=?,status='PENDING',version=?,owner_account_id=NULL,updated_at=? WHERE id=?", strings.TrimSpace(name), version, now, id)
			if err != nil {
				return Server{}, err
			}
			return Server{ID: id, Name: strings.TrimSpace(name), Status: "PENDING", PublicKey: publicKey, Version: version, Endpoints: []Endpoint{}}, nil
		}
		return Server{}, ErrConflict
	}
	_, err := s.DB.ExecContext(ctx, "INSERT INTO servers(id,name,public_key,status,version,last_seen_at,created_at,updated_at) VALUES(?,?,?,'PENDING',?,?,?,?)", id, strings.TrimSpace(name), publicKey, version, now, now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Server{}, ErrConflict
		}
		return Server{}, err
	}
	return Server{ID: id, Name: strings.TrimSpace(name), Status: "PENDING", PublicKey: publicKey, Version: version, LastSeenAt: &now, Endpoints: []Endpoint{}}, nil
}
func (s *Service) CreateClaim(ctx context.Context, serverID, signature string) (string, error) {
	var key, status string
	if s.DB.QueryRowContext(ctx, "SELECT public_key,status FROM servers WHERE id=?", serverID).Scan(&key, &status) != nil || status == "REVOKED" {
		return "", ErrGone
	}
	nonce := random(32)
	intent := "vynode-connect-claim-request-v1\n" + serverID
	if !verify(key, signature, []byte(intent)) {
		return "", ErrForbidden
	}
	id := account.ID()
	now := s.Now().UTC()
	_, err := s.DB.ExecContext(ctx, "INSERT INTO claim_challenges(id,server_id,challenge_hash,expires_at,created_at) VALUES(?,?,?,?,?)", id, serverID, digest(nonce), now.Add(10*time.Minute).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return "", err
	}
	return id + "." + nonce, nil
}
func (s *Service) CompleteClaim(ctx context.Context, p account.Principal, challenge string) (Server, error) {
	parts := strings.SplitN(challenge, ".", 2)
	if len(parts) != 2 {
		return Server{}, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Server{}, err
	}
	defer tx.Rollback()
	var serverID, hash, expiry string
	var consumed sql.NullString
	if tx.QueryRowContext(ctx, "SELECT server_id,challenge_hash,expires_at,consumed_at FROM claim_challenges WHERE id=?", parts[0]).Scan(&serverID, &hash, &expiry, &consumed) != nil {
		return Server{}, ErrGone
	}
	exp, _ := time.Parse(time.RFC3339Nano, expiry)
	if consumed.Valid || s.Now().After(exp) || hash != digest(parts[1]) {
		return Server{}, ErrGone
	}
	var owner sql.NullString
	var name, key, version, status string
	if tx.QueryRowContext(ctx, "SELECT name,public_key,version,status,owner_account_id FROM servers WHERE id=?", serverID).Scan(&name, &key, &version, &status, &owner) != nil {
		return Server{}, ErrGone
	}
	if owner.Valid && owner.String != p.AccountID {
		return Server{}, ErrConflict
	}
	now := s.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, "UPDATE claim_challenges SET account_id=?,consumed_at=? WHERE id=? AND consumed_at IS NULL", p.AccountID, now, parts[0])
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE servers SET owner_account_id=?,status='ACTIVE',updated_at=? WHERE id=? AND (owner_account_id IS NULL OR owner_account_id=?)", p.AccountID, now, serverID, p.AccountID)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, "INSERT INTO server_links(id,server_id,account_id,relationship,status,created_at,updated_at) VALUES(?,?,?,'OWNER','ACTIVE',?,?) ON CONFLICT(server_id,account_id) DO UPDATE SET relationship='OWNER',status='ACTIVE',updated_at=excluded.updated_at", account.ID(), serverID, p.AccountID, now, now)
	}
	if err != nil {
		return Server{}, err
	}
	if err = tx.Commit(); err != nil {
		return Server{}, err
	}
	return Server{ID: serverID, Name: name, Relationship: "OWNER", Status: "ACTIVE", PublicKey: key, Version: version, Endpoints: []Endpoint{}}, nil
}
func (s *Service) Servers(ctx context.Context, p account.Principal) ([]Server, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT x.id,x.name,l.relationship,x.status,x.public_key,x.version,x.last_seen_at FROM servers x JOIN server_links l ON l.server_id=x.id WHERE l.account_id=? AND l.status='ACTIVE' AND x.status='ACTIVE' ORDER BY x.name`, p.AccountID)
	if err != nil {
		return nil, err
	}
	out := []Server{}
	for rows.Next() {
		var v Server
		var last sql.NullString
		if err = rows.Scan(&v.ID, &v.Name, &v.Relationship, &v.Status, &v.PublicKey, &v.Version, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			v.LastSeenAt = &last.String
		}
		out = append(out, v)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Endpoints, err = s.endpoints(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (s *Service) endpoints(ctx context.Context, id string) ([]Endpoint, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT url,kind,secure,verified_at FROM server_endpoints WHERE server_id=? ORDER BY secure DESC,updated_at DESC", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Endpoint{}
	for rows.Next() {
		var e Endpoint
		var secure int
		var verified sql.NullString
		if err = rows.Scan(&e.URL, &e.Kind, &secure, &verified); err != nil {
			return nil, err
		}
		e.Secure = secure == 1
		if verified.Valid {
			e.VerifiedAt = &verified.String
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *Service) UpdateEndpoint(ctx context.Context, serverID, rawURL, kind, signature string, allowInsecure bool) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && !(allowInsecure && parsed.Scheme == "http")) {
		return ErrInvalid
	}
	var key, status string
	if s.DB.QueryRowContext(ctx, "SELECT public_key,status FROM servers WHERE id=?", serverID).Scan(&key, &status) != nil || status != "ACTIVE" {
		return ErrGone
	}
	message := []byte("vynode-connect-endpoint-v1\n" + serverID + "\n" + rawURL + "\n" + kind)
	if !verify(key, signature, message) {
		return ErrForbidden
	}
	now := s.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.DB.ExecContext(ctx, "INSERT INTO server_endpoints(id,server_id,url,kind,secure,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(server_id,url) DO UPDATE SET kind=excluded.kind,secure=excluded.secure,updated_at=excluded.updated_at", account.ID(), serverID, rawURL, kind, boolInt(parsed.Scheme == "https"), now)
	return err
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func (s *Service) Heartbeat(ctx context.Context, serverID, version, signature string) error {
	var key, status string
	if s.DB.QueryRowContext(ctx, "SELECT public_key,status FROM servers WHERE id=?", serverID).Scan(&key, &status) != nil || status != "ACTIVE" {
		return ErrGone
	}
	bucket := s.Now().UTC().Unix() / 60
	message := []byte(fmt.Sprintf("vynode-connect-heartbeat-v1\n%s\n%s\n%d", serverID, version, bucket))
	if !verify(key, signature, message) {
		return ErrForbidden
	}
	now := s.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.DB.ExecContext(ctx, "UPDATE servers SET version=?,last_seen_at=?,updated_at=? WHERE id=?", version, now, now, serverID)
	return err
}

func (s *Service) DeviceRevocations(ctx context.Context, serverID, signature string) ([]DeviceRevocation, error) {
	var key, status string
	if s.DB.QueryRowContext(ctx, "SELECT public_key,status FROM servers WHERE id=?", serverID).Scan(&key, &status) != nil || status != "ACTIVE" {
		return nil, ErrGone
	}
	bucket := s.Now().UTC().Unix() / 60
	message := []byte(fmt.Sprintf("vynode-connect-revocations-v1\n%s\n%d", serverID, bucket))
	if !verify(key, signature, message) {
		return nil, ErrForbidden
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT d.id,d.account_id,d.revoked_at FROM global_devices d JOIN server_links l ON l.account_id=d.account_id WHERE l.server_id=? AND l.status='ACTIVE' AND d.revoked_at IS NOT NULL ORDER BY d.revoked_at,d.id`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DeviceRevocation{}
	for rows.Next() {
		var item DeviceRevocation
		if err = rows.Scan(&item.DeviceID, &item.GlobalAccountID, &item.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) loadSigningKey() (string, ed25519.PrivateKey, error) {
	dir := filepath.Join(s.Dir, "signing")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", nil, err
	}
	kid := "connect-ed25519-1"
	path := filepath.Join(dir, kid+".key")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		_, private, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return "", nil, e
		}
		raw = []byte(base64.RawURLEncoding.EncodeToString(private))
		err = os.WriteFile(path, raw, 0o600)
		if err == nil {
			pub := private.Public().(ed25519.PublicKey)
			now := s.Now().UTC().Format(time.RFC3339Nano)
			_, err = s.DB.Exec("INSERT OR IGNORE INTO signing_keys(kid,public_key,private_key_path,status,created_at) VALUES(?,?,?,'ACTIVE',?)", kid, base64.RawURLEncoding.EncodeToString(pub), path, now)
		}
	}
	if err != nil {
		return "", nil, err
	}
	bytes, err := base64.RawURLEncoding.DecodeString(string(raw))
	if err != nil || len(bytes) != ed25519.PrivateKeySize {
		return "", nil, ErrInvalid
	}
	return kid, ed25519.PrivateKey(bytes), nil
}
func (s *Service) LoadSigningKeyForStartup() (string, ed25519.PrivateKey, error) {
	return s.loadSigningKey()
}
func (s *Service) PublicKeys(ctx context.Context) (map[string]any, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT kid,public_key,status FROM signing_keys WHERE status IN ('ACTIVE','VERIFY_ONLY')")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []map[string]string{}
	for rows.Next() {
		var kid, pub, status string
		if err = rows.Scan(&kid, &pub, &status); err != nil {
			return nil, err
		}
		keys = append(keys, map[string]string{"kid": kid, "kty": "OKP", "crv": "Ed25519", "x": pub, "use": "sig", "alg": "EdDSA"})
	}
	return map[string]any{"keys": keys}, rows.Err()
}
func (s *Service) Assertion(ctx context.Context, p account.Principal, serverID string) (string, error) {
	var linked int
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM server_links l JOIN servers s ON s.id=l.server_id WHERE l.server_id=? AND l.account_id=? AND l.status='ACTIVE' AND s.status='ACTIVE'", serverID, p.AccountID).Scan(&linked)
	if linked != 1 {
		return "", ErrForbidden
	}
	kid, key, err := s.loadSigningKey()
	if err != nil {
		return "", err
	}
	nonce := random(24)
	now := s.Now().UTC()
	header, _ := json.Marshal(map[string]any{"alg": "EdDSA", "typ": "JWT", "kid": kid})
	claims, _ := json.Marshal(map[string]any{"iss": s.Issuer, "aud": serverID, "sub": p.AccountID, "did": p.DeviceID, "nonce": nonce, "iat": now.Unix(), "exp": now.Add(s.AssertionTTL).Unix()})
	head := base64.RawURLEncoding.EncodeToString(header)
	body := base64.RawURLEncoding.EncodeToString(claims)
	message := head + "." + body
	sig := ed25519.Sign(key, []byte(message))
	_, err = s.DB.ExecContext(ctx, "INSERT INTO assertion_nonces(nonce,server_id,account_id,expires_at,created_at) VALUES(?,?,?,?,?)", nonce, serverID, p.AccountID, now.Add(s.AssertionTTL).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return "", err
	}
	return message + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
func (s *Service) LinkGrant(ctx context.Context, p account.Principal, serverID, state string) (string, error) {
	if len(state) < 32 || len(state) > 512 {
		return "", ErrInvalid
	}
	var linked int
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM server_links l JOIN servers x ON x.id=l.server_id WHERE l.server_id=? AND l.account_id=? AND l.status='ACTIVE' AND x.status='ACTIVE'", serverID, p.AccountID).Scan(&linked)
	if linked != 1 {
		return "", ErrForbidden
	}
	kid, key, err := s.loadSigningKey()
	if err != nil {
		return "", err
	}
	now := s.Now().UTC()
	header, _ := json.Marshal(map[string]any{"alg": "EdDSA", "typ": "JWT", "kid": kid})
	claims, _ := json.Marshal(map[string]any{"iss": s.Issuer, "aud": serverID, "sub": p.AccountID, "did": p.DeviceID, "purpose": "account-link", "state": state, "jti": random(24), "iat": now.Unix(), "exp": now.Add(2 * time.Minute).Unix()})
	head := base64.RawURLEncoding.EncodeToString(header)
	body := base64.RawURLEncoding.EncodeToString(claims)
	message := head + "." + body
	return message + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(message))), nil
}
func (s *Service) Unlink(ctx context.Context, p account.Principal, serverID string) error {
	now := s.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, "UPDATE server_links SET status='REVOKED',updated_at=? WHERE server_id=? AND account_id=? AND relationship='OWNER' AND status='ACTIVE'", now, serverID, p.AccountID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrForbidden
	}
	_, err = tx.ExecContext(ctx, "UPDATE servers SET status='REVOKED',updated_at=? WHERE id=? AND owner_account_id=?", now, serverID, p.AccountID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

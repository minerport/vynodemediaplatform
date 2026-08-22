package sharing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vynode/media/server/internal/auth"
)

var (
	ErrInvalid = errors.New("invalid sharing request")
	ErrDenied  = errors.New("sharing request denied")
	ErrGone    = errors.New("sharing request expired, used, or revoked")
	ErrLimited = errors.New("too many attempts")
)

const PairingTTL = 7 * time.Minute

type Service struct {
	db                              *sql.DB
	auth                            *auth.Service
	now                             func() time.Time
	instanceID, serverName, version string
	mu                              sync.Mutex
	attempts                        map[string][]time.Time
	emit                            func(context.Context, string, map[string]any, string)
	runtime                         *Runtime
}

type Grant struct {
	LibraryID   string   `json:"libraryId"`
	LibraryName string   `json:"libraryName,omitempty"`
	Permissions []string `json:"permissions"`
}
type Invitation struct {
	ID         string    `json:"id"`
	Identifier string    `json:"identifier,omitempty"`
	Role       auth.Role `json:"role"`
	Status     string    `json:"status"`
	CreatedBy  string    `json:"createdBy"`
	CreatedAt  string    `json:"createdAt"`
	ExpiresAt  string    `json:"expiresAt"`
	AcceptedAt string    `json:"acceptedAt,omitempty"`
	Libraries  []Grant   `json:"libraries"`
}
type Pairing struct {
	ID              string `json:"id"`
	Code            string `json:"code,omitempty"`
	Challenge       string `json:"challenge,omitempty"`
	Status          string `json:"status"`
	DeviceName      string `json:"deviceName"`
	ClientName      string `json:"clientName"`
	ClientVersion   string `json:"clientVersion,omitempty"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platformVersion,omitempty"`
	RequestedAt     string `json:"requestedAt"`
	ExpiresAt       string `json:"expiresAt"`
}
type RemoteSettings struct {
	DiscoveryEnabled          bool     `json:"discoveryEnabled"`
	PortMappingEnabled        bool     `json:"portMappingEnabled"`
	ReverseProxyEnabled       bool     `json:"reverseProxyEnabled"`
	InsecureRemoteAllowed     bool     `json:"insecureRemoteAllowed"`
	ManualRemoteURL           string   `json:"manualRemoteUrl,omitempty"`
	ManualStatus              string   `json:"manualStatus"`
	TrustedProxyCIDRs         []string `json:"trustedProxyCidrs"`
	LocalNetworkCIDRs         []string `json:"localNetworkCidrs"`
	PortMappingStatus         string   `json:"portMappingStatus"`
	PortMappingProtocol       string   `json:"portMappingProtocol,omitempty"`
	PortMappingExternalPort   int      `json:"portMappingExternalPort,omitempty"`
	PortMappingLeaseExpiresAt string   `json:"portMappingLeaseExpiresAt,omitempty"`
	PortMappingLastError      string   `json:"portMappingLastError,omitempty"`
	DiscoveryStatus           string   `json:"discoveryStatus"`
	DiscoveryLastError        string   `json:"discoveryLastError,omitempty"`
	UpdatedAt                 string   `json:"updatedAt"`
}
type Connection struct {
	Address                     string
	Secure, Local, TrustedProxy bool
}

func New(db *sql.DB, a *auth.Service, instanceID, serverName, version string) *Service {
	return &Service{db: db, auth: a, now: time.Now, instanceID: instanceID, serverName: serverName, version: version, attempts: map[string][]time.Time{}}
}
func (s *Service) ConfigureEvents(fn func(context.Context, string, map[string]any, string)) {
	s.emit = fn
}
func (s *Service) StartRuntime(httpAddress string) error {
	port, err := listenerPort(httpAddress)
	if err != nil {
		return err
	}
	s.runtime = newRuntime(s.db, s.instanceID, s.serverName, port, s.emit)
	s.runtime.Start()
	return nil
}
func (s *Service) Close() {
	if s.runtime != nil {
		s.runtime.Close()
	}
}
func (s *Service) event(ctx context.Context, kind string, payload map[string]any, dedupe string) {
	if s.emit != nil {
		s.emit(ctx, kind, payload, dedupe)
	}
}
func digest(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func secret(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func (s *Service) Has(ctx context.Context, p auth.Principal, libraryID, permission string) bool {
	if p.Role == auth.RoleOwner || p.Role == auth.RoleAdmin {
		return true
	}
	var n int
	return s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_access_grants WHERE user_id=? AND library_id=? AND permission=?", p.UserID, libraryID, permission).Scan(&n) == nil && n > 0
}
func (s *Service) HasLogical(ctx context.Context, p auth.Principal, kind, id, permission string) bool {
	if p.Role == auth.RoleOwner || p.Role == auth.RoleAdmin {
		return true
	}
	q := `SELECT COUNT(*) FROM media_associations a JOIN media_files f ON f.id=a.media_file_id JOIN library_sources src ON src.id=f.source_id JOIN library_access_grants g ON g.library_id=src.library_id AND g.user_id=? AND g.permission=? WHERE f.availability='AVAILABLE' AND a.entity_type=? AND a.entity_id=?`
	if kind == "SHOW" {
		q = `SELECT COUNT(*) FROM seasons se JOIN episodes ep ON ep.season_id=se.id JOIN media_associations a ON a.entity_type='EPISODE' AND a.entity_id=ep.id JOIN media_files f ON f.id=a.media_file_id JOIN library_sources src ON src.id=f.source_id JOIN library_access_grants g ON g.library_id=src.library_id AND g.user_id=? AND g.permission=? WHERE f.availability='AVAILABLE' AND se.show_id=?`
		var n int
		return s.db.QueryRowContext(ctx, q, p.UserID, permission, id).Scan(&n) == nil && n > 0
	}
	var n int
	return s.db.QueryRowContext(ctx, q, p.UserID, permission, kind, id).Scan(&n) == nil && n > 0
}
func (s *Service) HasAssociation(ctx context.Context, p auth.Principal, associationID, permission string) bool {
	if p.Role == auth.RoleOwner || p.Role == auth.RoleAdmin {
		return true
	}
	var n int
	return s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_associations a JOIN media_files f ON f.id=a.media_file_id JOIN library_sources src ON src.id=f.source_id JOIN library_access_grants g ON g.library_id=src.library_id AND g.user_id=? AND g.permission=? WHERE a.id=? AND f.availability='AVAILABLE'`, p.UserID, permission, associationID).Scan(&n) == nil && n > 0
}
func (s *Service) HasArtwork(ctx context.Context, p auth.Principal, artworkID string) bool {
	if p.Role == auth.RoleOwner || p.Role == auth.RoleAdmin {
		return true
	}
	var kind, id string
	if s.db.QueryRowContext(ctx, "SELECT entity_type,entity_id FROM artwork WHERE id=?", artworkID).Scan(&kind, &id) != nil {
		return false
	}
	if kind == "SEASON" {
		_ = s.db.QueryRowContext(ctx, "SELECT show_id FROM seasons WHERE id=?", id).Scan(&id)
		kind = "SHOW"
	}
	if kind == "EPISODE" {
		return s.HasLogical(ctx, p, "EPISODE", id, "VIEW")
	}
	return s.HasLogical(ctx, p, kind, id, "VIEW")
}
func (s *Service) Grants(ctx context.Context, userID string) ([]Grant, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT g.library_id,l.name,g.permission FROM library_access_grants g JOIN libraries l ON l.id=g.library_id WHERE g.user_id=? ORDER BY l.name,g.permission`, userID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	m := map[string]*Grant{}
	order := []string{}
	for rows.Next() {
		var id, n, p string
		if e = rows.Scan(&id, &n, &p); e != nil {
			return nil, e
		}
		if m[id] == nil {
			m[id] = &Grant{LibraryID: id, LibraryName: n}
			order = append(order, id)
		}
		m[id].Permissions = append(m[id].Permissions, p)
	}
	out := []Grant{}
	for _, id := range order {
		out = append(out, *m[id])
	}
	return out, rows.Err()
}
func (s *Service) SetGrants(ctx context.Context, actor, userID string, grants []Grant) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "DELETE FROM library_access_grants WHERE user_id=?", userID); e != nil {
		return e
	}
	now := stamp(s.now())
	for _, g := range grants {
		for _, p := range g.Permissions {
			if p != "VIEW" && p != "PLAY" {
				return ErrInvalid
			}
			if _, e = tx.ExecContext(ctx, "INSERT INTO library_access_grants(user_id,library_id,permission,granted_by,created_at) VALUES(?,?,?,?,?)", userID, g.LibraryID, p, actor, now); e != nil {
				return ErrInvalid
			}
		}
	}
	_, _ = tx.ExecContext(ctx, "UPDATE playback_sessions SET state='STOPPED',completion_reason='LIBRARY_ACCESS_REVOKED',ended_at=?,last_activity_at=? WHERE user_id=? AND state IN ('STARTING','PLAYING','PAUSED') AND NOT EXISTS(SELECT 1 FROM media_files f JOIN library_sources src ON src.id=f.source_id JOIN library_access_grants g ON g.library_id=src.library_id AND g.user_id=playback_sessions.user_id AND g.permission='PLAY' WHERE f.id=playback_sessions.media_file_id)", now, now, userID)
	return tx.Commit()
}

func (s *Service) CreateInvite(ctx context.Context, p auth.Principal, identifier string, role auth.Role, grants []Grant, ttl time.Duration) (Invitation, string, error) {
	if role != auth.RoleUser && role != auth.RoleAdmin {
		return Invitation{}, "", ErrInvalid
	}
	if role == auth.RoleAdmin && p.Role != auth.RoleOwner {
		return Invitation{}, "", ErrDenied
	}
	if ttl < time.Hour || ttl > 7*24*time.Hour {
		return Invitation{}, "", ErrInvalid
	}
	raw := secret(32)
	now := s.now()
	x := Invitation{ID: auth.ID(), Identifier: strings.TrimSpace(identifier), Role: role, Status: "PENDING", CreatedBy: p.UserID, CreatedAt: stamp(now), ExpiresAt: stamp(now.Add(ttl)), Libraries: grants}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return x, "", e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, "INSERT INTO user_invitations(id,identifier,intended_role,token_hash,status,created_by,created_at,expires_at) VALUES(?,?,?,?,?,?,?,?)", x.ID, x.Identifier, x.Role, digest(raw), x.Status, x.CreatedBy, x.CreatedAt, x.ExpiresAt)
	for _, g := range grants {
		for _, perm := range g.Permissions {
			if perm != "VIEW" && perm != "PLAY" {
				return x, "", ErrInvalid
			}
			if e == nil {
				_, e = tx.ExecContext(ctx, "INSERT INTO invitation_library_grants(invitation_id,library_id,permission) VALUES(?,?,?)", x.ID, g.LibraryID, perm)
			}
		}
	}
	if e != nil {
		return x, "", ErrInvalid
	}
	e = tx.Commit()
	return x, raw, e
}
func (s *Service) Invitations(ctx context.Context) ([]Invitation, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id,identifier,intended_role,CASE WHEN status='PENDING' AND expires_at<=? THEN 'EXPIRED' ELSE status END,created_by,created_at,expires_at,COALESCE(accepted_at,'') FROM user_invitations ORDER BY created_at DESC`, stamp(s.now()))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Invitation{}
	for rows.Next() {
		var x Invitation
		if e = rows.Scan(&x.ID, &x.Identifier, &x.Role, &x.Status, &x.CreatedBy, &x.CreatedAt, &x.ExpiresAt, &x.AcceptedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	if e = rows.Close(); e != nil {
		return nil, e
	}
	for i := range out {
		out[i].Libraries, e = s.inviteGrants(ctx, out[i].ID)
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}
func (s *Service) inviteGrants(ctx context.Context, id string) ([]Grant, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT g.library_id,l.name,g.permission FROM invitation_library_grants g JOIN libraries l ON l.id=g.library_id WHERE g.invitation_id=? ORDER BY l.name,g.permission`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	m := map[string]*Grant{}
	for rows.Next() {
		var i, n, p string
		_ = rows.Scan(&i, &n, &p)
		if m[i] == nil {
			m[i] = &Grant{LibraryID: i, LibraryName: n}
		}
		m[i].Permissions = append(m[i].Permissions, p)
	}
	out := []Grant{}
	for _, g := range m {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LibraryName < out[j].LibraryName })
	return out, rows.Err()
}
func (s *Service) InspectInvite(ctx context.Context, token string) (Invitation, error) {
	if len(token) < 32 {
		return Invitation{}, ErrGone
	}
	var x Invitation
	e := s.db.QueryRowContext(ctx, `SELECT id,identifier,intended_role,status,created_by,created_at,expires_at FROM user_invitations WHERE token_hash=?`, digest(token)).Scan(&x.ID, &x.Identifier, &x.Role, &x.Status, &x.CreatedBy, &x.CreatedAt, &x.ExpiresAt)
	if e != nil || x.Status != "PENDING" || x.ExpiresAt <= stamp(s.now()) {
		return Invitation{}, ErrGone
	}
	x.Libraries, _ = s.inviteGrants(ctx, x.ID)
	return x, nil
}
func (s *Service) AcceptInvite(ctx context.Context, token, username, display, password, remote, requestID string, d auth.DeviceInput) (auth.Tokens, error) {
	username, e := auth.NormalizeUsername(username)
	if e != nil {
		return auth.Tokens{}, ErrInvalid
	}
	hash, e := auth.HashPassword(password)
	if e != nil {
		return auth.Tokens{}, ErrInvalid
	}
	now := stamp(s.now())
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return auth.Tokens{}, e
	}
	defer tx.Rollback()
	var inv Invitation
	e = tx.QueryRowContext(ctx, `SELECT id,intended_role,status,expires_at FROM user_invitations WHERE token_hash=?`, digest(token)).Scan(&inv.ID, &inv.Role, &inv.Status, &inv.ExpiresAt)
	if e != nil || inv.Status != "PENDING" || inv.ExpiresAt <= now {
		return auth.Tokens{}, ErrGone
	}
	u := auth.User{ID: auth.ID(), Username: username, DisplayName: strings.TrimSpace(display), Role: inv.Role, Status: "ACTIVE", CreatedAt: now}
	if len(u.Username) < 3 || u.DisplayName == "" {
		return auth.Tokens{}, ErrInvalid
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO users(id,username,password_hash,role,display_name,status,created_at) VALUES(?,?,?,?,?,?,?)", u.ID, u.Username, hash, u.Role, u.DisplayName, u.Status, u.CreatedAt); e != nil {
		return auth.Tokens{}, ErrInvalid
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO library_access_grants(user_id,library_id,permission,granted_by,created_at) SELECT ?,library_id,permission,created_by,? FROM invitation_library_grants JOIN user_invitations ON user_invitations.id=invitation_library_grants.invitation_id WHERE invitation_id=?`, u.ID, now, inv.ID)
	if e == nil {
		var n int
		e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_invitations WHERE id=? AND status='PENDING'", inv.ID).Scan(&n)
		if e == nil && n == 1 {
			_, e = tx.ExecContext(ctx, "UPDATE user_invitations SET status='ACCEPTED',accepted_at=?,accepted_user_id=? WHERE id=? AND status='PENDING'", now, u.ID, inv.ID)
		} else if e == nil {
			e = ErrGone
		}
	}
	if e != nil {
		return auth.Tokens{}, e
	}
	if e = tx.Commit(); e != nil {
		return auth.Tokens{}, e
	}
	s.event(ctx, "INVITATION_ACCEPTED", map[string]any{"invitationId": inv.ID, "userId": u.ID}, "invite-accepted:"+inv.ID)
	return s.auth.IssueSession(ctx, u, d, remote, requestID)
}
func (s *Service) RevokeInvite(ctx context.Context, id string) error {
	r, e := s.db.ExecContext(ctx, "UPDATE user_invitations SET status='REVOKED',revoked_at=? WHERE id=? AND status='PENDING'", stamp(s.now()), id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrGone
	}
	return nil
}

var alphabet = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

func code() string {
	b := make([]byte, 8)
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	for i := range b {
		b[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(b[:4]) + "-" + string(b[4:])
}
func (s *Service) CreatePairing(ctx context.Context, d auth.DeviceInput) (Pairing, error) {
	if len(d.Name) < 1 || len(d.Name) > 100 || d.ClientName == "" || d.Platform == "" {
		return Pairing{}, ErrInvalid
	}
	now := s.now()
	for i := 0; i < 10; i++ {
		c, ch := code(), secret(32)
		x := Pairing{ID: auth.ID(), Code: c, Challenge: ch, Status: "PENDING", DeviceName: d.Name, ClientName: d.ClientName, ClientVersion: d.ClientVersion, Platform: d.Platform, PlatformVersion: d.PlatformVersion, RequestedAt: stamp(now), ExpiresAt: stamp(now.Add(PairingTTL))}
		_, e := s.db.ExecContext(ctx, "INSERT INTO pairing_requests(id,code_hash,challenge_hash,status,device_name,client_name,client_version,platform,platform_version,requested_at,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", x.ID, digest(c), digest(ch), x.Status, x.DeviceName, x.ClientName, x.ClientVersion, x.Platform, x.PlatformVersion, x.RequestedAt, x.ExpiresAt)
		if e == nil {
			return x, nil
		}
	}
	return Pairing{}, errors.New("pairing code generation exhausted")
}
func (s *Service) limited(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cut := s.now().Add(-time.Minute)
	v := s.attempts[key][:0]
	for _, t := range s.attempts[key] {
		if t.After(cut) {
			v = append(v, t)
		}
	}
	if len(v) >= 8 {
		s.attempts[key] = v
		return true
	}
	s.attempts[key] = append(v, s.now())
	return false
}
func (s *Service) PairingByCode(ctx context.Context, codeValue, remote string) (Pairing, error) {
	if s.limited(remote) {
		return Pairing{}, ErrLimited
	}
	var x Pairing
	e := s.db.QueryRowContext(ctx, `SELECT id,status,device_name,client_name,client_version,platform,platform_version,requested_at,expires_at FROM pairing_requests WHERE code_hash=?`, digest(strings.ToUpper(strings.TrimSpace(codeValue)))).Scan(&x.ID, &x.Status, &x.DeviceName, &x.ClientName, &x.ClientVersion, &x.Platform, &x.PlatformVersion, &x.RequestedAt, &x.ExpiresAt)
	if e != nil || x.Status != "PENDING" || x.ExpiresAt <= stamp(s.now()) {
		return Pairing{}, ErrGone
	}
	return x, nil
}
func (s *Service) DecidePairing(ctx context.Context, id, userID string, approve bool) error {
	state, col := "DENIED", "denied_at"
	if approve {
		state, col = "APPROVED", "approved_at"
	}
	q := fmt.Sprintf("UPDATE pairing_requests SET status=?,approved_by=?,%s=? WHERE id=? AND status='PENDING' AND expires_at>?", col)
	r, e := s.db.ExecContext(ctx, q, state, userID, stamp(s.now()), id, stamp(s.now()))
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrGone
	}
	return nil
}
func (s *Service) PairingStatus(ctx context.Context, id string) (Pairing, error) {
	var x Pairing
	e := s.db.QueryRowContext(ctx, `SELECT id,CASE WHEN status='PENDING' AND expires_at<=? THEN 'EXPIRED' ELSE status END,device_name,client_name,client_version,platform,platform_version,requested_at,expires_at FROM pairing_requests WHERE id=?`, stamp(s.now()), id).Scan(&x.ID, &x.Status, &x.DeviceName, &x.ClientName, &x.ClientVersion, &x.Platform, &x.PlatformVersion, &x.RequestedAt, &x.ExpiresAt)
	if e != nil {
		return x, ErrGone
	}
	return x, nil
}
func (s *Service) ExchangePairing(ctx context.Context, id, challenge, remote, requestID string) (auth.Tokens, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return auth.Tokens{}, e
	}
	defer tx.Rollback()
	var x Pairing
	var ch, userID string
	e = tx.QueryRowContext(ctx, `SELECT status,challenge_hash,approved_by,device_name,client_name,client_version,platform,platform_version,expires_at FROM pairing_requests WHERE id=?`, id).Scan(&x.Status, &ch, &userID, &x.DeviceName, &x.ClientName, &x.ClientVersion, &x.Platform, &x.PlatformVersion, &x.ExpiresAt)
	if e != nil || x.Status != "APPROVED" || x.ExpiresAt <= stamp(s.now()) || digest(challenge) != ch {
		return auth.Tokens{}, ErrGone
	}
	var u auth.User
	e = tx.QueryRowContext(ctx, "SELECT id,username,display_name,role,status,created_at FROM users WHERE id=? AND status='ACTIVE'", userID).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &u.CreatedAt)
	if e != nil {
		return auth.Tokens{}, ErrDenied
	}
	if _, e = tx.ExecContext(ctx, "UPDATE pairing_requests SET status='EXCHANGED' WHERE id=? AND status='APPROVED'", id); e != nil {
		return auth.Tokens{}, e
	}
	if e = tx.Commit(); e != nil {
		return auth.Tokens{}, e
	}
	tokens, e := s.auth.IssueSession(ctx, u, auth.DeviceInput{Name: x.DeviceName, ClientName: x.ClientName, ClientVersion: x.ClientVersion, Platform: x.Platform, PlatformVersion: x.PlatformVersion}, remote, requestID)
	if e == nil {
		sid := strings.SplitN(tokens.RefreshToken, ".", 2)[0]
		_, _ = s.db.ExecContext(ctx, "UPDATE devices SET authorization_type='PAIRED',authorized_at=? WHERE id=(SELECT device_id FROM sessions WHERE id=?)", stamp(s.now()), sid)
		_, _ = s.db.ExecContext(ctx, "UPDATE pairing_requests SET session_id=?,device_id=(SELECT device_id FROM sessions WHERE id=?) WHERE id=?", sid, sid, id)
		s.event(ctx, "NEW_DEVICE_PAIRED", map[string]any{"pairingId": id, "userId": u.ID, "deviceName": x.DeviceName}, "paired:"+id)
	}
	return tokens, e
}

func ValidateRemoteURL(raw string, allowHTTP bool) (string, error) {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && !(allowHTTP && u.Scheme == "http")) || u.RawQuery != "" || u.Fragment != "" {
		return "", ErrInvalid
	}
	return strings.TrimRight(u.String(), "/"), nil
}
func ValidateCIDRs(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, v := range values {
		_, n, e := net.ParseCIDR(strings.TrimSpace(v))
		if e != nil {
			return nil, ErrInvalid
		}
		c := n.String()
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out, nil
}
func (s *Service) Remote(ctx context.Context) (RemoteSettings, error) {
	var x RemoteSettings
	var d, p, r, i int
	e := s.db.QueryRowContext(ctx, "SELECT discovery_enabled,port_mapping_enabled,reverse_proxy_enabled,insecure_remote_allowed,port_mapping_external_port,discovery_runtime_status,discovery_last_error,updated_at FROM remote_access_settings WHERE id=1").Scan(&d, &p, &r, &i, &x.PortMappingExternalPort, &x.DiscoveryStatus, &x.DiscoveryLastError, &x.UpdatedAt)
	if e != nil {
		return x, e
	}
	x.DiscoveryEnabled = d == 1
	x.PortMappingEnabled = p == 1
	x.ReverseProxyEnabled = r == 1
	x.InsecureRemoteAllowed = i == 1
	x.ManualStatus = "UNCONFIGURED"
	_ = s.db.QueryRowContext(ctx, "SELECT base_url,verification_status FROM server_endpoints WHERE endpoint_type='MANUAL_REMOTE' AND enabled=1 LIMIT 1").Scan(&x.ManualRemoteURL, &x.ManualStatus)
	x.TrustedProxyCIDRs, _ = listStrings(ctx, s.db, "trusted_proxy_cidrs", "cidr")
	x.LocalNetworkCIDRs, _ = listStrings(ctx, s.db, "local_network_cidrs", "cidr")
	x.PortMappingStatus = "DISABLED"
	_ = s.db.QueryRowContext(ctx, "SELECT protocol,state,COALESCE(external_port,0),COALESCE(lease_expires_at,''),COALESCE(last_error,'') FROM port_mappings WHERE protocol='UPNP' LIMIT 1").Scan(&x.PortMappingProtocol, &x.PortMappingStatus, &x.PortMappingExternalPort, &x.PortMappingLeaseExpiresAt, &x.PortMappingLastError)
	if !x.PortMappingEnabled {
		x.PortMappingStatus = "DISABLED"
	}
	return x, nil
}
func listStrings(ctx context.Context, db *sql.DB, table, col string) ([]string, error) {
	rows, e := db.QueryContext(ctx, "SELECT "+col+" FROM "+table+" ORDER BY "+col)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		_ = rows.Scan(&v)
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) SaveRemote(ctx context.Context, x RemoteSettings) error {
	if x.PortMappingExternalPort < 0 || x.PortMappingExternalPort > 65535 {
		return ErrInvalid
	}
	u, e := ValidateRemoteURL(x.ManualRemoteURL, x.InsecureRemoteAllowed)
	if x.ManualRemoteURL != "" && e != nil {
		return e
	}
	trusted, e := ValidateCIDRs(x.TrustedProxyCIDRs)
	if e != nil {
		return e
	}
	local, e := ValidateCIDRs(x.LocalNetworkCIDRs)
	if e != nil {
		return e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	now := stamp(s.now())
	_, e = tx.ExecContext(ctx, "UPDATE remote_access_settings SET discovery_enabled=?,port_mapping_enabled=?,reverse_proxy_enabled=?,insecure_remote_allowed=?,port_mapping_external_port=?,updated_at=? WHERE id=1", x.DiscoveryEnabled, x.PortMappingEnabled, x.ReverseProxyEnabled, x.InsecureRemoteAllowed, x.PortMappingExternalPort, now)
	if e == nil {
		_, e = tx.ExecContext(ctx, "DELETE FROM trusted_proxy_cidrs")
	}
	for _, v := range trusted {
		if e == nil {
			_, e = tx.ExecContext(ctx, "INSERT INTO trusted_proxy_cidrs(cidr,created_at) VALUES(?,?)", v, now)
		}
	}
	if e == nil {
		_, e = tx.ExecContext(ctx, "DELETE FROM local_network_cidrs")
	}
	for _, v := range local {
		if e == nil {
			_, e = tx.ExecContext(ctx, "INSERT INTO local_network_cidrs(cidr,created_at) VALUES(?,?)", v, now)
		}
	}
	if e == nil {
		_, e = tx.ExecContext(ctx, "DELETE FROM server_endpoints WHERE endpoint_type='MANUAL_REMOTE'")
	}
	if e == nil && u != "" {
		_, e = tx.ExecContext(ctx, "INSERT INTO server_endpoints(id,endpoint_type,base_url,enabled,verification_status,created_at,updated_at) VALUES(?,'MANUAL_REMOTE',?,1,'UNVERIFIED_EXTERNALLY',?,?)", auth.ID(), u, now, now)
	}
	if e != nil {
		return e
	}
	if e = tx.Commit(); e == nil {
		s.event(ctx, "REMOTE_ENDPOINT_CHANGED", map[string]any{"configured": u != "", "secure": strings.HasPrefix(u, "https://")}, "remote-endpoint")
		if s.runtime != nil {
			s.runtime.Wake()
		}
	}
	return e
}
func (s *Service) ConnectionInfo(ctx context.Context, secure bool) map[string]any {
	x, _ := s.Remote(ctx)
	return map[string]any{"serverId": s.instanceID, "serverName": s.serverName, "apiVersion": "v1", "serverVersion": s.version, "secureConnection": secure, "authentication": []string{"password", "pairing"}, "capabilities": []string{"local", "manual-remote", "library-grants"}, "manualRemoteUrl": x.ManualRemoteURL}
}
func (s *Service) ResolveConnection(ctx context.Context, remoteAddr, forwardedFor, forwardedProto string, tls bool) Connection {
	host, _, e := net.SplitHostPort(remoteAddr)
	if e != nil {
		host = remoteAddr
	}
	peer := net.ParseIP(strings.Trim(host, "[]"))
	c := Connection{Address: host, Secure: tls}
	trusted, _ := listStrings(ctx, s.db, "trusted_proxy_cidrs", "cidr")
	for _, raw := range trusted {
		_, n, _ := net.ParseCIDR(raw)
		if peer != nil && n.Contains(peer) {
			c.TrustedProxy = true
			break
		}
	}
	if c.TrustedProxy {
		if first := strings.TrimSpace(strings.Split(forwardedFor, ",")[0]); net.ParseIP(strings.Trim(first, "[]")) != nil {
			c.Address = strings.Trim(first, "[]")
		}
		if forwardedProto == "https" {
			c.Secure = true
		} else if forwardedProto == "http" {
			c.Secure = false
		}
	}
	ip := net.ParseIP(c.Address)
	if ip != nil && ip.IsLoopback() {
		c.Local = true
	}
	local, _ := listStrings(ctx, s.db, "local_network_cidrs", "cidr")
	for _, raw := range local {
		_, n, _ := net.ParseCIDR(raw)
		if ip != nil && n.Contains(ip) {
			c.Local = true
			break
		}
	}
	if !c.TrustedProxy && ip != nil && ip.IsPrivate() {
		c.Local = true
	}
	return c
}

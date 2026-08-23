package registry

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/vynode/media/connect/internal/account"
)

func (s *Service) RegisterInvitation(ctx context.Context, serverID, invitationID, username, intentDigest, expiresAt, signature string) (Invitation, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if invitationID == "" || len(intentDigest) != 64 {
		return Invitation{}, ErrInvalid
	}
	if username != "" {
		if _, err := account.NormalizeUsername(username); err != nil {
			return Invitation{}, ErrInvalid
		}
	}
	expires, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil || !expires.After(s.Now()) || expires.After(s.Now().Add(30*24*time.Hour)) {
		return Invitation{}, ErrInvalid
	}
	var publicKey, ownerID, name, status string
	if s.DB.QueryRowContext(ctx, "SELECT public_key,owner_account_id,name,status FROM servers WHERE id=?", serverID).Scan(&publicKey, &ownerID, &name, &status) != nil || ownerID == "" || status != "ACTIVE" {
		return Invitation{}, ErrForbidden
	}
	if !verify(publicKey, signature, canonicalInvitation(serverID, invitationID, username, intentDigest, expiresAt)) {
		return Invitation{}, ErrForbidden
	}
	token := random(32)
	now := s.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.DB.ExecContext(ctx, `INSERT INTO invitations(id,server_id,token_hash,intended_username,relationship,local_principal_hint,status,created_by_account_id,expires_at,created_at,intent_digest) VALUES(?,?,?,?,'MEMBER',NULL,'PENDING',?,?,?,?)`, invitationID, serverID, digest(token), nullString(username), ownerID, expiresAt, now, intentDigest)
	if err != nil {
		return Invitation{}, ErrConflict
	}
	return Invitation{ID: invitationID, Token: token, ServerID: serverID, ServerName: name, IntendedUsername: username, Status: "PENDING", ExpiresAt: expiresAt}, nil
}

func (s *Service) RevokeRegisteredInvitation(ctx context.Context, serverID, invitationID, signature string) error {
	var publicKey string
	if s.DB.QueryRowContext(ctx, "SELECT public_key FROM servers WHERE id=? AND status='ACTIVE'", serverID).Scan(&publicKey) != nil || !verify(publicKey, signature, canonicalInvitationRevoke(serverID, invitationID)) {
		return ErrForbidden
	}
	now := s.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.DB.ExecContext(ctx, "UPDATE invitations SET status='REVOKED' WHERE id=? AND server_id=? AND status='PENDING'", invitationID, serverID)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return ErrGone
	}
	_, _ = s.DB.ExecContext(ctx, "INSERT INTO audit_events(id,event_type,target_type,target_id,metadata_json,created_at) VALUES(?,?,?,?,?,?)", account.ID(), "INVITATION_REVOKED", "invitation", invitationID, "{}", now)
	return nil
}

func (s *Service) CreateInvitation(ctx context.Context, p account.Principal, serverID, username, hint string, ttl time.Duration) (Invitation, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username != "" {
		if _, err := account.NormalizeUsername(username); err != nil {
			return Invitation{}, ErrInvalid
		}
	}
	if ttl <= 0 || ttl > 30*24*time.Hour {
		ttl = 7 * 24 * time.Hour
	}
	var name string
	var owner int
	_ = s.DB.QueryRowContext(ctx, `SELECT s.name,COUNT(*) FROM servers s JOIN server_links l ON l.server_id=s.id WHERE s.id=? AND l.account_id=? AND l.relationship='OWNER' AND l.status='ACTIVE' AND s.status='ACTIVE'`, serverID, p.AccountID).Scan(&name, &owner)
	if owner != 1 {
		return Invitation{}, ErrForbidden
	}
	token := random(32)
	now := s.Now().UTC()
	id := account.ID()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO invitations(id,server_id,token_hash,intended_username,local_principal_hint,status,created_by_account_id,expires_at,created_at,intent_digest) VALUES(?,?,?,?,?,'PENDING',?,?,?,?)`, id, serverID, digest(token), nullString(username), nullString(hint), p.AccountID, now.Add(ttl).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), digest("legacy:"+id))
	if err != nil {
		return Invitation{}, err
	}
	return Invitation{ID: id, Token: token, ServerID: serverID, ServerName: name, IntendedUsername: username, Status: "PENDING", ExpiresAt: now.Add(ttl).Format(time.RFC3339Nano)}, nil
}

func nullString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.TrimSpace(v)
}

func (s *Service) AcceptInvitation(ctx context.Context, p account.Principal, token string) (Server, error) {
	v, err := s.AcceptInvitationRedemption(ctx, p, token)
	return v.Server, err
}

func (s *Service) AcceptInvitationRedemption(ctx context.Context, p account.Principal, token string) (InvitationAcceptance, error) {
	now := s.Now().UTC()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return InvitationAcceptance{}, err
	}
	defer tx.Rollback()
	var id, serverID, status, expiry, intentDigest string
	var intended sql.NullString
	if tx.QueryRowContext(ctx, `SELECT id,server_id,status,expires_at,intended_username,intent_digest FROM invitations WHERE token_hash=?`, digest(token)).Scan(&id, &serverID, &status, &expiry, &intended, &intentDigest) != nil {
		return InvitationAcceptance{}, ErrGone
	}
	exp, _ := time.Parse(time.RFC3339Nano, expiry)
	if status != "PENDING" || !now.Before(exp) || intentDigest == "" {
		return InvitationAcceptance{}, ErrGone
	}
	var username, displayName string
	if tx.QueryRowContext(ctx, "SELECT username,display_name FROM accounts WHERE id=?", p.AccountID).Scan(&username, &displayName) != nil {
		return InvitationAcceptance{}, ErrForbidden
	}
	if intended.Valid && intended.String != username {
		return InvitationAcceptance{}, ErrForbidden
	}
	stamp := now.Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `INSERT INTO server_links(id,server_id,account_id,relationship,status,created_at,updated_at) VALUES(?,?,?,'MEMBER','ACTIVE',?,?) ON CONFLICT(server_id,account_id) DO UPDATE SET status='ACTIVE',updated_at=excluded.updated_at`, account.ID(), serverID, p.AccountID, stamp, stamp); err != nil {
		return InvitationAcceptance{}, err
	}
	r, err := tx.ExecContext(ctx, `UPDATE invitations SET status='ACCEPTED',accepted_by_account_id=?,accepted_at=? WHERE id=? AND status='PENDING'`, p.AccountID, stamp, id)
	if err != nil {
		return InvitationAcceptance{}, err
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return InvitationAcceptance{}, ErrGone
	}
	if err = tx.Commit(); err != nil {
		return InvitationAcceptance{}, err
	}
	metadata, _ := json.Marshal(map[string]string{"serverId": serverID})
	_, _ = s.DB.ExecContext(ctx, "INSERT INTO audit_events(id,event_type,actor_account_id,target_type,target_id,metadata_json,created_at) VALUES(?,?,?,?,?,?,?)", account.ID(), "INVITATION_ACCEPTED", p.AccountID, "invitation", id, string(metadata), stamp)
	list, err := s.Servers(ctx, p)
	if err != nil {
		return InvitationAcceptance{}, err
	}
	for _, v := range list {
		if v.ID == serverID {
			redemption, signErr := s.invitationRedemption(serverID, id, p.AccountID, username, displayName, intentDigest)
			if signErr != nil {
				return InvitationAcceptance{}, signErr
			}
			assertion, signErr := s.Assertion(ctx, p, serverID)
			if signErr != nil {
				return InvitationAcceptance{}, signErr
			}
			return InvitationAcceptance{Server: v, InvitationID: id, Redemption: redemption, Assertion: assertion}, nil
		}
	}
	return InvitationAcceptance{}, ErrGone
}

func (s *Service) invitationRedemption(serverID, invitationID, accountID, username, displayName, intentDigest string) (string, error) {
	kid, key, err := s.loadSigningKey()
	if err != nil {
		return "", err
	}
	now := s.Now().UTC()
	header, _ := json.Marshal(map[string]any{"alg": "EdDSA", "typ": "JWT", "kid": kid})
	claims, _ := json.Marshal(map[string]any{"iss": s.Issuer, "aud": serverID, "sub": accountID, "purpose": "invitation-redemption", "invitationId": invitationID, "intentDigest": intentDigest, "username": username, "displayName": displayName, "jti": random(24), "iat": now.Unix(), "exp": now.Add(2 * time.Minute).Unix()})
	head := base64.RawURLEncoding.EncodeToString(header)
	body := base64.RawURLEncoding.EncodeToString(claims)
	message := head + "." + body
	return message + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(message))), nil
}

func (s *Service) CreateDeviceCode(ctx context.Context, d account.DeviceInput) (DeviceCode, error) {
	now := s.Now().UTC()
	device := random(32)
	user := strings.ToUpper(random(6)[:8])
	id := account.ID()
	exp := now.Add(10 * time.Minute)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO device_codes(id,device_code_hash,user_code_hash,name,platform,client_name,client_version,platform_version,status,expires_at,created_at) VALUES(?,?,?,?,?,?,?,?,'PENDING',?,?)`, id, digest(device), digest(user), d.Name, d.Platform, d.ClientName, d.ClientVersion, d.PlatformVersion, exp.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	return DeviceCode{ID: id, DeviceCode: device, UserCode: user, VerificationPath: "/activate", ExpiresAt: exp.Format(time.RFC3339Nano), PollAfterSeconds: 5}, err
}

func (s *Service) ApproveDeviceCode(ctx context.Context, p account.Principal, userCode string) error {
	now := s.Now().UTC()
	r, err := s.DB.ExecContext(ctx, `UPDATE device_codes SET status='APPROVED',account_id=?,approved_at=? WHERE user_code_hash=? AND status='PENDING' AND expires_at>?`, p.AccountID, now.Format(time.RFC3339Nano), digest(strings.ToUpper(strings.TrimSpace(userCode))), now.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrGone
	}
	return nil
}
func (s *Service) DenyDeviceCode(ctx context.Context, p account.Principal, userCode string) error {
	now := s.Now().UTC()
	r, err := s.DB.ExecContext(ctx, `UPDATE device_codes SET status='DENIED',account_id=?,approved_at=? WHERE user_code_hash=? AND status='PENDING' AND expires_at>?`, p.AccountID, now.Format(time.RFC3339Nano), digest(strings.ToUpper(strings.TrimSpace(userCode))), now.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrGone
	}
	return nil
}

func (s *Service) ExchangeDeviceCode(ctx context.Context, deviceCode string) (string, account.DeviceInput, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", account.DeviceInput{}, err
	}
	defer tx.Rollback()
	var id, status, expiry string
	var accountID sql.NullString
	var d account.DeviceInput
	err = tx.QueryRowContext(ctx, `SELECT id,status,account_id,expires_at,name,platform,client_name,client_version,platform_version FROM device_codes WHERE device_code_hash=?`, digest(deviceCode)).Scan(&id, &status, &accountID, &expiry, &d.Name, &d.Platform, &d.ClientName, &d.ClientVersion, &d.PlatformVersion)
	if err != nil {
		return "", d, ErrGone
	}
	exp, _ := time.Parse(time.RFC3339Nano, expiry)
	if status == "PENDING" && s.Now().Before(exp) {
		return "", d, ErrConflict
	}
	if status != "APPROVED" || !s.Now().Before(exp) {
		return "", d, ErrGone
	}
	r, err := tx.ExecContext(ctx, "UPDATE device_codes SET status='EXCHANGED' WHERE id=? AND status='APPROVED'", id)
	if err != nil {
		return "", d, err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return "", d, ErrGone
	}
	if err = tx.Commit(); err != nil {
		return "", d, err
	}
	if !accountID.Valid || accountID.String == "" {
		return "", d, ErrGone
	}
	return accountID.String, d, nil
}

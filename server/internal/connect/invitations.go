package connect

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vynode/media/server/internal/auth"
)

type InvitationGrant struct {
	LibraryID   string   `json:"libraryId"`
	Permissions []string `json:"permissions"`
}

type GlobalInvitation struct {
	ID               string            `json:"id"`
	Token            string            `json:"token,omitempty"`
	ServerID         string            `json:"serverId"`
	IntendedUsername string            `json:"intendedUsername,omitempty"`
	Role             auth.Role         `json:"role"`
	Grants           []InvitationGrant `json:"grants"`
	Status           string            `json:"status"`
	ExpiresAt        string            `json:"expiresAt"`
}

func canonicalInvitation(serverID, invitationID, username, digest, expiresAt string) []byte {
	return []byte("vynode-connect-invitation-v1\n" + serverID + "\n" + invitationID + "\n" + strings.ToLower(strings.TrimSpace(username)) + "\n" + digest + "\n" + expiresAt)
}
func canonicalInvitationRevoke(serverID, invitationID string) []byte {
	return []byte("vynode-connect-invitation-revoke-v1\n" + serverID + "\n" + invitationID)
}

func canonicalIntent(role auth.Role, grants []InvitationGrant) (string, error) {
	flat := make([]string, 0)
	for _, grant := range grants {
		if strings.TrimSpace(grant.LibraryID) == "" {
			return "", ErrInvalid
		}
		for _, permission := range grant.Permissions {
			if permission != "VIEW" && permission != "PLAY" && permission != "DOWNLOAD" {
				return "", ErrInvalid
			}
			flat = append(flat, grant.LibraryID+":"+permission)
		}
	}
	sort.Strings(flat)
	return hash(string(role) + "\n" + strings.Join(flat, "\n")), nil
}

func (s *Service) CreateGlobalInvitation(ctx context.Context, p auth.Principal, username, displayName string, role auth.Role, grants []InvitationGrant, ttl time.Duration) (GlobalInvitation, error) {
	if s.identity.ClaimStatus != "CLAIMED" || (role != auth.RoleUser && role != auth.RoleAdmin) || (role == auth.RoleAdmin && p.Role != auth.RoleOwner) || ttl < time.Hour || ttl > 7*24*time.Hour {
		return GlobalInvitation{}, ErrInvalid
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if username != "" {
		if _, err := auth.NormalizeUsername(username); err != nil {
			return GlobalInvitation{}, ErrInvalid
		}
	}
	digest, err := canonicalIntent(role, grants)
	if err != nil {
		return GlobalInvitation{}, err
	}
	now := s.now().UTC()
	id, expiry := auth.ID(), now.Add(ttl).Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GlobalInvitation{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO connect_global_invitation_intents(id,intended_username,display_name,intended_role,intent_digest,status,created_by_user_id,created_at,expires_at) VALUES(?,?,?,?,?,'DRAFT',?,?,?)`, id, username, strings.TrimSpace(displayName), role, digest, p.UserID, now.Format(time.RFC3339Nano), expiry); err != nil {
		return GlobalInvitation{}, err
	}
	for _, grant := range grants {
		var exists int
		if tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM libraries WHERE id=?", grant.LibraryID).Scan(&exists) != nil || exists != 1 {
			return GlobalInvitation{}, ErrInvalid
		}
		for _, permission := range grant.Permissions {
			if _, err = tx.ExecContext(ctx, "INSERT INTO connect_global_invitation_grants(invitation_id,library_id,permission) VALUES(?,?,?)", id, grant.LibraryID, permission); err != nil {
				return GlobalInvitation{}, ErrInvalid
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return GlobalInvitation{}, err
	}
	settings, err := s.Settings(ctx)
	if err != nil {
		return GlobalInvitation{}, err
	}
	base, err := safeConnectURL(settings.ConnectURL)
	if err != nil {
		return GlobalInvitation{}, err
	}
	body := map[string]string{"serverId": s.serverID, "invitationId": id, "intendedUsername": username, "intentDigest": digest, "expiresAt": expiry, "signature": s.identity.Sign(canonicalInvitation(s.serverID, id, username, digest, expiry))}
	status, raw, err := postJSONBody(ctx, base+"/api/v1/servers/invitations/register", body, nil)
	if err != nil || status != 200 {
		return GlobalInvitation{}, fmt.Errorf("Connect invitation registration failed")
	}
	var registered struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(raw, &registered) != nil || registered.Token == "" {
		return GlobalInvitation{}, ErrInvalid
	}
	if _, err = s.db.ExecContext(ctx, "UPDATE connect_global_invitation_intents SET status='PENDING' WHERE id=? AND status='DRAFT'", id); err != nil {
		return GlobalInvitation{}, err
	}
	return GlobalInvitation{ID: id, Token: registered.Token, ServerID: s.serverID, IntendedUsername: username, Role: role, Grants: grants, Status: "PENDING", ExpiresAt: expiry}, nil
}

func (s *Service) verifyRedemption(ctx context.Context, assertion string) (claims, error) {
	parts := strings.Split(assertion, ".")
	if len(parts) != 3 {
		return claims{}, ErrInvalid
	}
	headerRaw, e1 := base64.RawURLEncoding.DecodeString(parts[0])
	claimsRaw, e2 := base64.RawURLEncoding.DecodeString(parts[1])
	sig, e3 := base64.RawURLEncoding.DecodeString(parts[2])
	var header struct {
		Algorithm string `json:"alg"`
		Kid       string `json:"kid"`
	}
	var c claims
	if e1 != nil || e2 != nil || e3 != nil || json.Unmarshal(headerRaw, &header) != nil || json.Unmarshal(claimsRaw, &c) != nil || header.Algorithm != "EdDSA" || c.Purpose != "invitation-redemption" || c.Audience != s.serverID || c.Subject == "" || c.InvitationID == "" || c.IntentDigest == "" || c.JTI == "" {
		return claims{}, ErrInvalid
	}
	var issuer, rawKeys string
	if s.db.QueryRowContext(ctx, "SELECT connect_issuer,connect_signing_keys_json FROM connect_settings WHERE id=1 AND enabled=1").Scan(&issuer, &rawKeys) != nil || issuer != c.Issuer || c.ExpiresAt <= s.now().Unix() || c.IssuedAt > s.now().Add(time.Minute).Unix() || c.ExpiresAt-c.IssuedAt > 300 {
		return claims{}, ErrInvalid
	}
	var keys jwks
	if json.Unmarshal([]byte(rawKeys), &keys) != nil {
		return claims{}, ErrInvalid
	}
	for _, key := range keys.Keys {
		if key.Kid == header.Kid && key.KeyType == "OKP" && key.Curve == "Ed25519" {
			public, err := base64.RawURLEncoding.DecodeString(key.X)
			if err == nil && len(public) == ed25519.PublicKeySize && ed25519.Verify(ed25519.PublicKey(public), []byte(parts[0]+"."+parts[1]), sig) {
				return c, nil
			}
		}
	}
	return claims{}, ErrInvalid
}

func (s *Service) RedeemGlobalInvitation(ctx context.Context, assertion, requestID string) error {
	c, err := s.verifyRedemption(ctx, assertion)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var role auth.Role
	var status, expiry, storedDigest, createdBy, displayName string
	if tx.QueryRowContext(ctx, `SELECT intended_role,status,expires_at,intent_digest,created_by_user_id,display_name FROM connect_global_invitation_intents WHERE id=?`, c.InvitationID).Scan(&role, &status, &expiry, &storedDigest, &createdBy, &displayName) != nil || status != "PENDING" || expiry <= s.now().UTC().Format(time.RFC3339Nano) || storedDigest != c.IntentDigest {
		return ErrInvalid
	}
	rows, err := tx.QueryContext(ctx, "SELECT library_id,permission FROM connect_global_invitation_grants WHERE invitation_id=? ORDER BY library_id,permission", c.InvitationID)
	if err != nil {
		return err
	}
	grantsByLibrary := map[string][]string{}
	for rows.Next() {
		var libraryID, permission string
		if err = rows.Scan(&libraryID, &permission); err != nil {
			rows.Close()
			return err
		}
		grantsByLibrary[libraryID] = append(grantsByLibrary[libraryID], permission)
	}
	rows.Close()
	grants := make([]InvitationGrant, 0, len(grantsByLibrary))
	for libraryID, permissions := range grantsByLibrary {
		grants = append(grants, InvitationGrant{LibraryID: libraryID, Permissions: permissions})
	}
	currentDigest, err := canonicalIntent(role, grants)
	if err != nil || currentDigest != storedDigest {
		return ErrInvalid
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, "INSERT INTO connect_invitation_redemption_nonces(jti,invitation_id,global_account_id,expires_at,consumed_at) VALUES(?,?,?,?,?)", c.JTI, c.InvitationID, c.Subject, time.Unix(c.ExpiresAt, 0).UTC().Format(time.RFC3339Nano), now); err != nil {
		return ErrInvalid
	}
	userID := auth.ID()
	accountSlug := strings.ReplaceAll(c.Subject, "-", "")
	if len(accountSlug) < 16 {
		return ErrInvalid
	}
	username := "global-" + accountSlug[:16]
	if displayName == "" {
		displayName = strings.TrimSpace(c.DisplayName)
	}
	if displayName == "" {
		displayName = "VyNode Account"
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO users(id,username,password_hash,role,display_name,status,created_at,authentication_type) VALUES(?,?,'!GLOBAL_LINKED!',?,?,'ACTIVE',?,'GLOBAL_LINKED')`, userID, username, role, displayName, now); err != nil {
		return ErrInvalid
	}
	for _, grant := range grants {
		for _, permission := range grant.Permissions {
			if _, err = tx.ExecContext(ctx, "INSERT INTO library_access_grants(user_id,library_id,permission,granted_by,created_at) VALUES(?,?,?,?,?)", userID, grant.LibraryID, permission, createdBy, now); err != nil {
				return err
			}
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO global_account_links(global_account_id,user_id,status,linked_by_user_id,created_at,updated_at) VALUES(?,?,'ACTIVE',?,?,?)", c.Subject, userID, createdBy, now, now); err != nil {
		return ErrInvalid
	}
	result, err := tx.ExecContext(ctx, "UPDATE connect_global_invitation_intents SET status='REDEEMED',redeemed_at=?,redeemed_global_account_id=?,redeemed_user_id=? WHERE id=? AND status='PENDING'", now, c.Subject, userID, c.InvitationID)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return ErrInvalid
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for _, event := range []string{"GLOBAL_INVITATION_REDEEMED", "GLOBAL_PRINCIPAL_CREATED", "GLOBAL_ACCOUNT_LINKED", "LIBRARY_ACCESS_GRANTED"} {
		_ = s.auth.Audit(ctx, event, &userID, "global_invitation", c.InvitationID, requestID, map[string]any{"globalAccountId": c.Subject})
	}
	return nil
}

func (s *Service) RevokeGlobalInvitation(ctx context.Context, p auth.Principal, id string) error {
	var status string
	if s.db.QueryRowContext(ctx, "SELECT status FROM connect_global_invitation_intents WHERE id=? AND created_by_user_id=?", id, p.UserID).Scan(&status) != nil || (status != "DRAFT" && status != "PENDING") {
		return ErrInvalid
	}
	if status == "PENDING" {
		settings, err := s.Settings(ctx)
		if err != nil {
			return err
		}
		base, err := safeConnectURL(settings.ConnectURL)
		if err != nil {
			return err
		}
		code, err := postJSON(ctx, base+"/api/v1/servers/invitations/revoke", map[string]string{"serverId": s.serverID, "invitationId": id, "signature": s.identity.Sign(canonicalInvitationRevoke(s.serverID, id))}, nil)
		if err != nil || code != 200 {
			return ErrInvalid
		}
	}
	result, err := s.db.ExecContext(ctx, "UPDATE connect_global_invitation_intents SET status='REVOKED' WHERE id=? AND created_by_user_id=? AND status IN ('DRAFT','PENDING')", id, p.UserID)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return ErrInvalid
	}
	return nil
}

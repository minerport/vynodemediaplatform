package connect

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var ErrIdentityUnsafe = errors.New("server Connect identity is missing, malformed, or inconsistent")

type Identity struct {
	ServerID    string
	PublicKey   string
	private     ed25519.PrivateKey
	Path        string
	ClaimStatus string
}

func LoadOrCreateIdentity(ctx context.Context, db *sql.DB, configDir, serverID string) (Identity, error) {
	var v Identity
	var path string
	err := db.QueryRowContext(ctx, "SELECT server_id,public_key,private_key_path,claim_status FROM server_connect_identity WHERE id=1").Scan(&v.ServerID, &v.PublicKey, &path, &v.ClaimStatus)
	if err == nil {
		return loadIdentityFile(v, path, serverID)
	}
	if err != sql.ErrNoRows {
		return Identity{}, err
	}
	dir := filepath.Join(configDir, "connect")
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return Identity{}, err
	}
	path = filepath.Join(dir, "server-identity.key")
	if _, statErr := os.Stat(path); statErr == nil {
		return Identity{}, fmt.Errorf("%w: untracked key file already exists", ErrIdentityUnsafe)
	} else if !os.IsNotExist(statErr) {
		return Identity{}, statErr
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Identity{}, err
	}
	encoded := base64.RawURLEncoding.EncodeToString(private)
	if err = os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		return Identity{}, err
	}
	_ = os.Chmod(path, 0o600)
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	publicValue := base64.RawURLEncoding.EncodeToString(public)
	if _, err = db.ExecContext(ctx, "INSERT INTO server_connect_identity(id,server_id,public_key,private_key_path,claim_status,updated_at) VALUES(1,?,?,?,'UNREGISTERED',?)", serverID, publicValue, path, stamp); err != nil {
		_ = os.Remove(path)
		return Identity{}, err
	}
	return Identity{ServerID: serverID, PublicKey: publicValue, private: private, Path: path, ClaimStatus: "UNREGISTERED"}, nil
}
func loadIdentityFile(v Identity, path, expectedServerID string) (Identity, error) {
	if v.ServerID != expectedServerID {
		return Identity{}, fmt.Errorf("%w: database belongs to server %s", ErrIdentityUnsafe, v.ServerID)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %v", ErrIdentityUnsafe, err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(string(raw))
	if err != nil || len(decoded) != ed25519.PrivateKeySize {
		return Identity{}, fmt.Errorf("%w: invalid private key", ErrIdentityUnsafe)
	}
	private := ed25519.PrivateKey(decoded)
	actual := base64.RawURLEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	if actual != v.PublicKey {
		return Identity{}, fmt.Errorf("%w: public key mismatch", ErrIdentityUnsafe)
	}
	v.private = private
	v.Path = path
	_ = os.Chmod(path, 0o600)
	return v, nil
}
func (i Identity) Sign(message []byte) string {
	return base64.RawURLEncoding.EncodeToString(ed25519.Sign(i.private, message))
}
func (i Identity) RegistrationMessage(name string) []byte {
	return []byte("vynode-connect-register-v1\n" + i.ServerID + "\n" + name + "\n" + i.PublicKey)
}
func (i Identity) ClaimMessage() []byte {
	return []byte("vynode-connect-claim-request-v1\n" + i.ServerID)
}

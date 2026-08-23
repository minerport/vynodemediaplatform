package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }

func Open(ctx context.Context, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "connect.db")+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{DB: db}
	if err = s.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error                    { return s.DB.Close() }
func (s *Store) Ready(ctx context.Context) error { return s.DB.PingContext(ctx) }

func (s *Store) Migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS accounts(id TEXT PRIMARY KEY,username TEXT NOT NULL UNIQUE,display_name TEXT NOT NULL,password_hash TEXT NOT NULL,status TEXT NOT NULL CHECK(status IN ('ACTIVE','DISABLED')),security_version INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL,updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS global_devices(id TEXT PRIMARY KEY,account_id TEXT NOT NULL,name TEXT NOT NULL,platform TEXT NOT NULL,client_name TEXT NOT NULL,client_version TEXT NOT NULL,platform_version TEXT NOT NULL,first_authorized_at TEXT NOT NULL,last_seen_at TEXT NOT NULL,revoked_at TEXT,revocation_version INTEGER NOT NULL DEFAULT 0,FOREIGN KEY(account_id) REFERENCES accounts(id))`,
		`CREATE TABLE IF NOT EXISTS global_sessions(id TEXT PRIMARY KEY,account_id TEXT NOT NULL,device_id TEXT NOT NULL,refresh_token_hash TEXT NOT NULL,previous_refresh_hash TEXT,token_family_id TEXT NOT NULL,expires_at TEXT NOT NULL,created_at TEXT NOT NULL,last_activity_at TEXT NOT NULL,revoked_at TEXT,FOREIGN KEY(account_id) REFERENCES accounts(id),FOREIGN KEY(device_id) REFERENCES global_devices(id))`,
		`CREATE TABLE IF NOT EXISTS refresh_history(session_id TEXT NOT NULL,token_hash TEXT NOT NULL,rotated_at TEXT NOT NULL,PRIMARY KEY(session_id,token_hash),FOREIGN KEY(session_id) REFERENCES global_sessions(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS servers(id TEXT PRIMARY KEY,name TEXT NOT NULL,public_key TEXT NOT NULL UNIQUE,owner_account_id TEXT,status TEXT NOT NULL CHECK(status IN ('PENDING','ACTIVE','REVOKED','DECOMMISSIONED')),version TEXT NOT NULL,capabilities_json TEXT NOT NULL DEFAULT '{}',last_seen_at TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,FOREIGN KEY(owner_account_id) REFERENCES accounts(id))`,
		`CREATE TABLE IF NOT EXISTS server_endpoints(id TEXT PRIMARY KEY,server_id TEXT NOT NULL,url TEXT NOT NULL,kind TEXT NOT NULL,secure INTEGER NOT NULL,verified_at TEXT,updated_at TEXT NOT NULL,UNIQUE(server_id,url),FOREIGN KEY(server_id) REFERENCES servers(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS server_links(id TEXT PRIMARY KEY,server_id TEXT NOT NULL,account_id TEXT NOT NULL,relationship TEXT NOT NULL CHECK(relationship IN ('OWNER','MEMBER')),status TEXT NOT NULL CHECK(status IN ('ACTIVE','REVOKED')),local_principal_hint TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,UNIQUE(server_id,account_id),FOREIGN KEY(server_id) REFERENCES servers(id),FOREIGN KEY(account_id) REFERENCES accounts(id))`,
		`CREATE TABLE IF NOT EXISTS claim_challenges(id TEXT PRIMARY KEY,server_id TEXT NOT NULL,challenge_hash TEXT NOT NULL,account_id TEXT,expires_at TEXT NOT NULL,consumed_at TEXT,created_at TEXT NOT NULL,FOREIGN KEY(server_id) REFERENCES servers(id),FOREIGN KEY(account_id) REFERENCES accounts(id))`,
		`CREATE TABLE IF NOT EXISTS assertion_nonces(nonce TEXT PRIMARY KEY,server_id TEXT NOT NULL,account_id TEXT NOT NULL,expires_at TEXT NOT NULL,consumed_at TEXT,created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS invitations(id TEXT PRIMARY KEY,server_id TEXT NOT NULL,token_hash TEXT NOT NULL UNIQUE,intended_username TEXT,relationship TEXT NOT NULL DEFAULT 'MEMBER',local_principal_hint TEXT,status TEXT NOT NULL CHECK(status IN ('PENDING','ACCEPTED','REVOKED','EXPIRED')),created_by_account_id TEXT NOT NULL,accepted_by_account_id TEXT,expires_at TEXT NOT NULL,created_at TEXT NOT NULL,accepted_at TEXT,FOREIGN KEY(server_id) REFERENCES servers(id),FOREIGN KEY(created_by_account_id) REFERENCES accounts(id),FOREIGN KEY(accepted_by_account_id) REFERENCES accounts(id))`,
		`CREATE TABLE IF NOT EXISTS device_codes(id TEXT PRIMARY KEY,device_code_hash TEXT NOT NULL UNIQUE,user_code_hash TEXT NOT NULL UNIQUE,name TEXT NOT NULL,platform TEXT NOT NULL,client_name TEXT NOT NULL,client_version TEXT NOT NULL,platform_version TEXT NOT NULL,status TEXT NOT NULL CHECK(status IN ('PENDING','APPROVED','DENIED','EXPIRED','EXCHANGED')),account_id TEXT,expires_at TEXT NOT NULL,created_at TEXT NOT NULL,approved_at TEXT,FOREIGN KEY(account_id) REFERENCES accounts(id))`,
		`CREATE TABLE IF NOT EXISTS signing_keys(kid TEXT PRIMARY KEY,public_key TEXT NOT NULL,private_key_path TEXT NOT NULL,status TEXT NOT NULL CHECK(status IN ('ACTIVE','VERIFY_ONLY','RETIRED')),created_at TEXT NOT NULL,retired_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS audit_events(id TEXT PRIMARY KEY,event_type TEXT NOT NULL,actor_account_id TEXT,target_type TEXT NOT NULL,target_id TEXT NOT NULL,request_id TEXT,metadata_json TEXT NOT NULL DEFAULT '{}',created_at TEXT NOT NULL,FOREIGN KEY(actor_account_id) REFERENCES accounts(id))`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_account ON global_sessions(account_id,revoked_at,expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_links_account ON server_links(account_id,status)`,
		`CREATE INDEX IF NOT EXISTS idx_invites_username ON invitations(intended_username,status)`,
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range statements {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("connect migration: %w", err)
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version,applied_at) VALUES(1,datetime('now'))`)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	var version2 int
	if err = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version=2").Scan(&version2); err != nil || version2 != 0 {
		return err
	}
	tx, err = s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "ALTER TABLE invitations ADD COLUMN intent_digest TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,applied_at) VALUES(2,datetime('now'))`); err != nil {
		return err
	}
	return tx.Commit()
}

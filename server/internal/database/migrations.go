package database

import (
	"context"
	"fmt"
)

type migration struct {
	version   int
	name, sql string
}

var migrations = []migration{{1, "foundation", `
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS server_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS users (id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, role TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, disabled_at TEXT);
CREATE TABLE IF NOT EXISTS devices (id TEXT PRIMARY KEY, user_id TEXT, name TEXT NOT NULL, platform TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, last_seen_at TEXT, FOREIGN KEY(user_id) REFERENCES users(id));
CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, device_id TEXT, refresh_token_hash TEXT NOT NULL, expires_at TEXT NOT NULL, revoked_at TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(user_id) REFERENCES users(id), FOREIGN KEY(device_id) REFERENCES devices(id));
CREATE TABLE IF NOT EXISTS audit_events (id INTEGER PRIMARY KEY AUTOINCREMENT, actor_user_id TEXT, action TEXT NOT NULL, target_type TEXT, target_id TEXT, request_id TEXT, metadata_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(actor_user_id) REFERENCES users(id));
`}, {2, "authentication", `
ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'ACTIVE';
ALTER TABLE devices ADD COLUMN client_name TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN client_version TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN platform_version TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN token_family_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN previous_refresh_hash TEXT;
ALTER TABLE sessions ADD COLUMN last_activity_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE sessions ADD COLUMN remote_address TEXT;
CREATE TABLE setup_state (id INTEGER PRIMARY KEY CHECK(id=1), completed_at TEXT);
INSERT INTO setup_state(id, completed_at) VALUES(1, NULL);
CREATE TABLE refresh_token_history (session_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE, rotated_at TEXT NOT NULL, PRIMARY KEY(session_id, token_hash), FOREIGN KEY(session_id) REFERENCES sessions(id));
CREATE INDEX idx_sessions_user_active ON sessions(user_id, revoked_at, expires_at);
CREATE INDEX idx_audit_created ON audit_events(created_at DESC);
`}}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return err
	}
	for _, item := range migrations {
		var count int
		if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", item.version).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			continue
		}
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, item.sql); err == nil {
			_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, name) VALUES(?, ?)", item.version, item.name)
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d %s: %w", item.version, item.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", item.version, err)
		}
	}
	return nil
}

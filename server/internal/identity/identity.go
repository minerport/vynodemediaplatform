package identity

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
)

func LoadOrCreate(ctx context.Context, db *sql.DB) (string, error) {
	var id string
	err := db.QueryRowContext(ctx, "SELECT value FROM server_settings WHERE key = 'server_instance_id'").Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("read server identity: %w", err)
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate server identity: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(bytes)
	id = fmt.Sprintf("%s-%s-%s-%s-%s", raw[0:8], raw[8:12], raw[12:16], raw[16:20], raw[20:32])
	if _, err := db.ExecContext(ctx, "INSERT INTO server_settings(key, value) VALUES('server_instance_id', ?)", id); err != nil {
		return "", fmt.Errorf("persist server identity: %w", err)
	}
	return id, nil
}

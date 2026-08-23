package connect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func (s *Service) RegisterAndBeginClaim(ctx context.Context, name, version string) (string, error) {
	settings, err := s.Settings(ctx)
	if err != nil || !settings.Enabled || s.identity.private == nil {
		return "", ErrInvalid
	}
	base, err := safeConnectURL(settings.ConnectURL)
	if err != nil {
		return "", err
	}
	body := map[string]string{"serverId": s.serverID, "name": strings.TrimSpace(name), "publicKey": s.identity.PublicKey, "version": version, "signature": s.identity.Sign(s.identity.RegistrationMessage(strings.TrimSpace(name)))}
	status, err := postJSON(ctx, base+"/api/v1/servers/register", body, nil)
	if err != nil {
		return "", fmt.Errorf("Connect registration failed: %w", err)
	}
	if status != 200 && status != 409 {
		return "", fmt.Errorf("Connect registration failed with status %d", status)
	}
	// A 200 means a new or previously revoked registry object is pending a
	// fresh owner claim. A 409 means the same active identity is already
	// registered and can proceed directly to a new claim challenge.
	if status == 200 {
		now := s.now().UTC().Format(time.RFC3339Nano)
		_, _ = s.db.ExecContext(ctx, "UPDATE server_connect_identity SET claim_status='PENDING',updated_at=? WHERE id=1", now)
		s.identity.ClaimStatus = "PENDING"
	}
	body = map[string]string{"serverId": s.serverID, "signature": s.identity.Sign(s.identity.ClaimMessage())}
	status, raw, err := postJSONBody(ctx, base+"/api/v1/servers/claim", body, nil)
	if err != nil {
		return "", fmt.Errorf("Connect claim failed: %w", err)
	}
	if status != 200 {
		return "", fmt.Errorf("Connect claim failed with status %d", status)
	}
	var out struct {
		Challenge string `json:"challenge"`
	}
	if json.Unmarshal(raw, &out) != nil || out.Challenge == "" {
		return "", ErrInvalid
	}
	return out.Challenge, nil
}
func (s *Service) Heartbeat(ctx context.Context, version string) error {
	settings, err := s.Settings(ctx)
	if err != nil || !settings.Enabled || s.identity.private == nil || s.identity.ClaimStatus != "CLAIMED" {
		return ErrInvalid
	}
	base, err := safeConnectURL(settings.ConnectURL)
	if err != nil {
		return err
	}
	bucket := s.now().UTC().Unix() / 60
	message := []byte(fmt.Sprintf("vynode-connect-heartbeat-v1\n%s\n%s\n%d", s.serverID, version, bucket))
	status, err := postJSON(ctx, base+"/api/v1/servers/heartbeat", map[string]string{"serverId": s.serverID, "version": version, "signature": s.identity.Sign(message)}, nil)
	stamp := s.now().UTC().Format(time.RFC3339Nano)
	if err == nil && status == 200 {
		_, _ = s.db.ExecContext(ctx, "UPDATE connect_settings SET last_contact_at=?,last_error='',updated_at=? WHERE id=1", stamp, stamp)
		if err = s.pullDeviceRevocations(ctx, base, bucket); err != nil {
			_, _ = s.db.ExecContext(ctx, "UPDATE connect_settings SET last_error=?,updated_at=? WHERE id=1", err.Error(), stamp)
			return err
		}
		return nil
	}
	messageText := "heartbeat failed"
	if err != nil {
		messageText = err.Error()
	}
	_, _ = s.db.ExecContext(ctx, "UPDATE connect_settings SET last_error=?,updated_at=? WHERE id=1", messageText, stamp)
	return fmt.Errorf("Connect heartbeat: %s", messageText)
}

func (s *Service) pullDeviceRevocations(ctx context.Context, base string, bucket int64) error {
	message := []byte(fmt.Sprintf("vynode-connect-revocations-v1\n%s\n%d", s.serverID, bucket))
	status, raw, err := postJSONBody(ctx, base+"/api/v1/servers/device-revocations", map[string]string{"serverId": s.serverID, "signature": s.identity.Sign(message)}, nil)
	if err != nil {
		return fmt.Errorf("Connect revocation sync failed: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("Connect revocation sync failed with status %d", status)
	}
	var items []struct {
		DeviceID        string `json:"deviceId"`
		GlobalAccountID string `json:"globalAccountId"`
		RevokedAt       string `json:"revokedAt"`
	}
	if json.Unmarshal(raw, &items) != nil {
		return ErrInvalid
	}
	for _, item := range items {
		result, execErr := s.db.ExecContext(ctx, `UPDATE sessions SET revoked_at=? WHERE id IN (SELECT local_session_id FROM connect_global_device_sessions WHERE global_device_id=? AND global_account_id=? AND revoked_at IS NULL) AND revoked_at IS NULL`, item.RevokedAt, item.DeviceID, item.GlobalAccountID)
		if execErr != nil {
			return execErr
		}
		if changed, _ := result.RowsAffected(); changed > 0 {
			_, _ = s.db.ExecContext(ctx, "UPDATE connect_global_device_sessions SET revoked_at=? WHERE global_device_id=? AND global_account_id=? AND revoked_at IS NULL", item.RevokedAt, item.DeviceID, item.GlobalAccountID)
			_ = s.auth.Audit(ctx, "GLOBAL_DEVICE_SESSION_REVOKED", nil, "global_device", item.DeviceID, "connect-revocation-sync", map[string]any{"globalAccountId": item.GlobalAccountID, "sessionsRevoked": changed})
		}
	}
	return nil
}
func (s *Service) PublishEndpoint(ctx context.Context, endpoint, kind string) error {
	settings, err := s.Settings(ctx)
	if err != nil || !settings.Enabled || s.identity.private == nil || s.identity.ClaimStatus != "CLAIMED" {
		return ErrInvalid
	}
	base, err := safeConnectURL(settings.ConnectURL)
	if err != nil {
		return err
	}
	message := []byte("vynode-connect-endpoint-v1\n" + s.serverID + "\n" + endpoint + "\n" + kind)
	status, _, err := sendJSON(ctx, http.MethodPut, base+"/api/v1/servers/endpoint", map[string]string{"serverId": s.serverID, "url": endpoint, "kind": kind, "signature": s.identity.Sign(message)}, nil)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("Connect endpoint update failed with status %d", status)
	}
	return nil
}
func (s *Service) StartHeartbeat(parent context.Context, version string) context.CancelFunc {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.Heartbeat(ctx, version)
			}
		}
	}()
	return cancel
}
func safeConnectURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrInvalid
	}
	host := parsed.Hostname()
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && (host == "localhost" || host == "127.0.0.1" || host == "::1" || os.Getenv("VYNODE_CONNECT_ALLOW_INSECURE_TEST_SERVICE") == "true")) {
		return "", ErrInvalid
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}
func postJSON(ctx context.Context, target string, value any, headers map[string]string) (int, error) {
	status, _, err := postJSONBody(ctx, target, value, headers)
	return status, err
}
func postJSONBody(ctx context.Context, target string, value any, headers map[string]string) (int, []byte, error) {
	return sendJSON(ctx, http.MethodPost, target, value, headers)
}
func sendJSON(ctx context.Context, method, target string, value any, headers map[string]string) (int, []byte, error) {
	raw, _ := json.Marshal(value)
	request, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(raw))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		request.Header.Set(k, v)
	}
	client := http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	return response.StatusCode, body, err
}

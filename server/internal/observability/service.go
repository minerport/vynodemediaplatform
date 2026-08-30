package observability

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrValidation = errors.New("validation failed")
var ErrNotFound = errors.New("not found")

type Resolver func(context.Context, string) ([]net.IPAddr, error)
type Service struct {
	db        *sql.DB
	info      SystemInfo
	paths     Paths
	key       []byte
	client    *http.Client
	resolve   Resolver
	disk      func(string) (uint64, uint64, uint64, error)
	now       func() time.Time
	wake      chan struct{}
	stop      chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
}

func New(db *sql.DB, info SystemInfo, paths Paths, keyDir string) (*Service, error) {
	key, err := loadKey(keyDir)
	if err != nil {
		return nil, err
	}
	s := &Service{db: db, info: info, paths: paths, key: key, resolve: net.DefaultResolver.LookupIPAddr, disk: diskUsage, now: time.Now, wake: make(chan struct{}, 1), stop: make(chan struct{})}
	s.client = &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects disabled") }}
	s.wg.Add(1)
	go s.worker()
	_, _ = s.Emit(context.Background(), "SERVER_STARTED", map[string]any{"version": info.Version}, "server-start")
	return s, nil
}
func (s *Service) Close()      { s.closeOnce.Do(func() { close(s.stop); s.wg.Wait() }) }
func id() string               { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func loadKey(dir string) ([]byte, error) {
	p := filepath.Join(dir, "observability.key")
	if b, e := os.ReadFile(p); e == nil && len(b) == 32 {
		return b, nil
	}
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return nil, e
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	if e := os.WriteFile(p, b, 0600); e != nil {
		return nil, e
	}
	return b, nil
}
func (s *Service) seal(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	b, e := aes.NewCipher(s.key)
	if e != nil {
		return "", e
	}
	g, e := cipher.NewGCM(b)
	if e != nil {
		return "", e
	}
	n := make([]byte, g.NonceSize())
	if _, e = rand.Read(n); e != nil {
		return "", e
	}
	return base64.RawStdEncoding.EncodeToString(g.Seal(n, n, []byte(v), nil)), nil
}
func (s *Service) open(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	raw, e := base64.RawStdEncoding.DecodeString(v)
	if e != nil {
		return "", e
	}
	b, e := aes.NewCipher(s.key)
	if e != nil {
		return "", e
	}
	g, e := cipher.NewGCM(b)
	if e != nil || len(raw) < g.NonceSize() {
		return "", errors.New("invalid secret")
	}
	plain, e := g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
	return string(plain), e
}

func (s *Service) Emit(ctx context.Context, eventType string, payload map[string]any, dedupe string) (Event, error) {
	def, ok := EventCatalog[eventType]
	if !ok {
		return Event{}, ErrValidation
	}
	raw, _ := json.Marshal(payload)
	e := Event{ID: id(), Type: eventType, Category: def.Category, Severity: def.Severity, Payload: payload, CreatedAt: stamp(s.now())}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return e, err
	}
	defer tx.Rollback()
	if dedupe != "" {
		var found int
		_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM operational_events WHERE event_type=? AND dedupe_key=? AND created_at>=?", eventType, dedupe, stamp(s.now().Add(-24*time.Hour))).Scan(&found)
		if found > 0 {
			return e, nil
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO operational_events(id,event_type,category,severity,payload_json,dedupe_key,created_at) VALUES(?,?,?,?,?,?,?)", e.ID, e.Type, e.Category, e.Severity, string(raw), null(dedupe), e.CreatedAt); err != nil {
		return e, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_deliveries(id,event_id,destination_id,status,next_attempt_at,created_at) SELECT lower(hex(randomblob(16))),?,d.id,'PENDING',?,? FROM notification_destinations d JOIN notification_subscriptions s ON s.destination_id=d.id WHERE d.enabled=1 AND s.event_type=?`, e.ID, e.CreatedAt, e.CreatedAt, eventType)
	if err != nil {
		return e, err
	}
	if err = tx.Commit(); err == nil {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
	return e, err
}
func null(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (s *Service) Events(ctx context.Context, limit, offset int) ([]Event, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, e := s.db.QueryContext(ctx, "SELECT id,event_type,category,severity,payload_json,created_at FROM operational_events ORDER BY created_at DESC LIMIT ? OFFSET ?", limit, offset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var x Event
		var raw string
		if e = rows.Scan(&x.ID, &x.Type, &x.Category, &x.Severity, &raw, &x.CreatedAt); e != nil {
			return nil, e
		}
		_ = json.Unmarshal([]byte(raw), &x.Payload)
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) Destinations(ctx context.Context) ([]Destination, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id,name,url,enabled,allow_private_network,allow_insecure_http,secret_ciphertext,max_attempts,created_at,updated_at FROM notification_destinations ORDER BY name`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Destination{}
	for rows.Next() {
		var x Destination
		var en, priv, insecure int
		var secret sql.NullString
		if e = rows.Scan(&x.ID, &x.Name, &x.URL, &en, &priv, &insecure, &secret, &x.MaxAttempts, &x.CreatedAt, &x.UpdatedAt); e != nil {
			return nil, e
		}
		x.Enabled = en == 1
		x.AllowPrivateNetwork = priv == 1
		x.AllowInsecureHTTP = insecure == 1
		x.HasSecret = secret.Valid && secret.String != ""
		sr, _ := s.db.QueryContext(ctx, "SELECT event_type FROM notification_subscriptions WHERE destination_id=? ORDER BY event_type", x.ID)
		for sr.Next() {
			var t string
			_ = sr.Scan(&t)
			x.EventTypes = append(x.EventTypes, t)
		}
		sr.Close()
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) SaveDestination(ctx context.Context, x Destination) (Destination, error) {
	x.Name = strings.TrimSpace(x.Name)
	x.URL = strings.TrimSpace(x.URL)
	if x.Name == "" || x.MaxAttempts < 1 || x.MaxAttempts > 5 {
		return x, ErrValidation
	}
	if err := s.ValidateURL(ctx, x.URL, x.AllowPrivateNetwork, x.AllowInsecureHTTP); err != nil {
		return x, err
	}
	for _, t := range x.EventTypes {
		if _, ok := EventCatalog[t]; !ok {
			return x, ErrValidation
		}
	}
	enc, err := s.seal(x.Secret)
	if err != nil {
		return x, err
	}
	if x.ID == "" {
		x.ID = id()
	}
	now := stamp(s.now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	if x.Secret == "" {
		_, err = tx.ExecContext(ctx, `INSERT INTO notification_destinations(id,name,url,enabled,allow_private_network,allow_insecure_http,max_attempts,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,url=excluded.url,enabled=excluded.enabled,allow_private_network=excluded.allow_private_network,allow_insecure_http=excluded.allow_insecure_http,max_attempts=excluded.max_attempts,updated_at=excluded.updated_at`, x.ID, x.Name, x.URL, x.Enabled, x.AllowPrivateNetwork, x.AllowInsecureHTTP, x.MaxAttempts, now, now)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO notification_destinations(id,name,url,enabled,allow_private_network,allow_insecure_http,secret_ciphertext,max_attempts,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,url=excluded.url,enabled=excluded.enabled,allow_private_network=excluded.allow_private_network,allow_insecure_http=excluded.allow_insecure_http,secret_ciphertext=excluded.secret_ciphertext,max_attempts=excluded.max_attempts,updated_at=excluded.updated_at`, x.ID, x.Name, x.URL, x.Enabled, x.AllowPrivateNetwork, x.AllowInsecureHTTP, enc, x.MaxAttempts, now, now)
	}
	if err != nil {
		return x, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM notification_subscriptions WHERE destination_id=?", x.ID); err != nil {
		return x, err
	}
	for _, t := range x.EventTypes {
		if _, err = tx.ExecContext(ctx, "INSERT INTO notification_subscriptions(destination_id,event_type) VALUES(?,?)", x.ID, t); err != nil {
			return x, err
		}
	}
	if !x.Enabled {
		_, _ = tx.ExecContext(ctx, "UPDATE notification_deliveries SET status='CANCELED',next_attempt_at=NULL WHERE destination_id=? AND status IN ('PENDING','RETRYING')", x.ID)
	}
	err = tx.Commit()
	x.Secret = ""
	x.HasSecret = enc != ""
	return x, err
}
func (s *Service) DeleteDestination(ctx context.Context, id string) error {
	r, e := s.db.ExecContext(ctx, "DELETE FROM notification_destinations WHERE id=?", id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Service) TestDestination(ctx context.Context, destinationID string) error {
	var enabled int
	if e := s.db.QueryRowContext(ctx, "SELECT enabled FROM notification_destinations WHERE id=?", destinationID).Scan(&enabled); e != nil {
		return e
	}
	if enabled != 1 {
		return ErrValidation
	}
	def := EventCatalog["TEST"]
	eventID, now := id(), stamp(s.now())
	raw := `{"test":true,"message":"VyNode webhook test"}`
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "INSERT INTO operational_events(id,event_type,category,severity,payload_json,created_at) VALUES(?,?,?,?,?,?)", eventID, "TEST", def.Category, def.Severity, raw, now); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO notification_deliveries(id,event_id,destination_id,status,next_attempt_at,created_at) VALUES(?,?,?,'PENDING',?,?)", id(), eventID, destinationID, now, now); e != nil {
		return e
	}
	if e = tx.Commit(); e == nil {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
	return e
}
func (s *Service) Deliveries(ctx context.Context, limit int) ([]Delivery, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, e := s.db.QueryContext(ctx, `SELECT d.id,d.event_id,e.event_type,d.destination_id,n.name,d.status,d.attempt_count,COALESCE(d.last_http_status,0),COALESCE(d.last_error,''),COALESCE(d.next_attempt_at,''),COALESCE(d.delivered_at,''),d.created_at FROM notification_deliveries d JOIN operational_events e ON e.id=d.event_id JOIN notification_destinations n ON n.id=d.destination_id ORDER BY d.created_at DESC LIMIT ?`, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Delivery{}
	for rows.Next() {
		var x Delivery
		if e = rows.Scan(&x.ID, &x.EventID, &x.EventType, &x.DestinationID, &x.DestinationName, &x.Status, &x.AttemptCount, &x.LastHTTPStatus, &x.LastError, &x.NextAttemptAt, &x.DeliveredAt, &x.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func prohibited(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}
func (s *Service) ValidateURL(ctx context.Context, raw string, allowPrivate, allowHTTP bool) error {
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || u.User != nil {
		return ErrValidation
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && allowHTTP) {
		return ErrValidation
	}
	ips, e := s.resolve(ctx, u.Hostname())
	if e != nil || len(ips) == 0 {
		return ErrValidation
	}
	for _, x := range ips {
		if prohibited(x.IP) && !allowPrivate {
			return ErrValidation
		}
		if x.IP.IsUnspecified() || x.IP.IsMulticast() || x.IP.IsLinkLocalUnicast() {
			return ErrValidation
		}
	}
	return nil
}
func (s *Service) worker() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
		case <-s.wake:
		}
		s.processOne()
	}
}
func (s *Service) processOne() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var d Delivery
	var endpoint, secret string
	var priv, insecure, enabled, max int
	var raw, eventType, occurred string
	e := s.db.QueryRowContext(ctx, `SELECT x.id,x.event_id,x.destination_id,x.attempt_count,n.url,COALESCE(n.secret_ciphertext,''),n.allow_private_network,n.allow_insecure_http,n.enabled,n.max_attempts,e.payload_json,e.event_type,e.created_at FROM notification_deliveries x JOIN notification_destinations n ON n.id=x.destination_id JOIN operational_events e ON e.id=x.event_id WHERE x.status IN ('PENDING','RETRYING') AND x.next_attempt_at<=? ORDER BY x.created_at LIMIT 1`, stamp(s.now())).Scan(&d.ID, &d.EventID, &d.DestinationID, &d.AttemptCount, &endpoint, &secret, &priv, &insecure, &enabled, &max, &raw, &eventType, &occurred)
	if e != nil {
		return
	}
	if enabled != 1 {
		_, _ = s.db.Exec("UPDATE notification_deliveries SET status='CANCELED',next_attempt_at=NULL WHERE id=?", d.ID)
		return
	}
	if e = s.ValidateURL(ctx, endpoint, priv == 1, insecure == 1); e != nil {
		s.finishFailure(d, max, 0, "destination rejected by network policy", false)
		return
	}
	payload := map[string]any{"schemaVersion": 1, "eventId": d.EventID, "eventType": eventType, "occurredAt": occurred, "serverId": s.info.InstanceID, "payload": json.RawMessage(raw)}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-VyNode-Event-ID", d.EventID)
	if secret != "" {
		plain, er := s.open(secret)
		if er == nil {
			m := hmac.New(sha256.New, []byte(plain))
			m.Write(body)
			req.Header.Set("X-VyNode-Signature", "sha256="+hex.EncodeToString(m.Sum(nil)))
		}
	}
	resp, e := s.client.Do(req)
	if e != nil {
		s.finishFailure(d, max, 0, "delivery timeout or network failure", true)
		return
	}
	_, _ = io.CopyN(io.Discard, resp.Body, 4096)
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = s.db.Exec("UPDATE notification_deliveries SET status='DELIVERED',attempt_count=attempt_count+1,last_http_status=?,last_error=NULL,next_attempt_at=NULL,delivered_at=? WHERE id=?", resp.StatusCode, stamp(s.now()), d.ID)
		return
	}
	retry := resp.StatusCode == 408 || resp.StatusCode == 429 || resp.StatusCode >= 500
	s.finishFailure(d, max, resp.StatusCode, fmt.Sprintf("receiver returned HTTP %d", resp.StatusCode), retry)
}
func (s *Service) finishFailure(d Delivery, max, status int, msg string, retry bool) {
	attempt := d.AttemptCount + 1
	if retry && attempt < max {
		delay := time.Duration(1<<(attempt-1)) * time.Second
		_, _ = s.db.Exec("UPDATE notification_deliveries SET status='RETRYING',attempt_count=?,last_http_status=?,last_error=?,next_attempt_at=? WHERE id=?", attempt, nullInt(status), msg, stamp(s.now().Add(delay)), d.ID)
	} else {
		_, _ = s.db.Exec("UPDATE notification_deliveries SET status='FAILED',attempt_count=?,last_http_status=?,last_error=?,next_attempt_at=NULL WHERE id=?", attempt, nullInt(status), msg, d.ID)
	}
}
func nullInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func commandVersion(path string) string {
	if path == "" {
		return "Unavailable"
	}
	ctx, c := context.WithTimeout(context.Background(), 2*time.Second)
	defer c()
	b, e := exec.CommandContext(ctx, path, "-version").Output()
	if e != nil {
		return "Unavailable"
	}
	line := strings.SplitN(string(b), "\n", 2)[0]
	if len(line) > 160 {
		line = line[:160]
	}
	return line
}
func (s *Service) Metrics(ctx context.Context) Metrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	out := Metrics{UptimeSeconds: int64(time.Since(s.info.StartedAt).Seconds()), OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, GoRoutines: runtime.NumGoroutine(), GoHeapBytes: m.HeapAlloc, GoSystemBytes: m.Sys}
	out.ProcessRSSBytes, out.SystemMemoryTotalBytes, out.SystemMemoryAvailableBytes = platformMemory()
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM playback_sessions WHERE state IN ('STARTING','PLAYING','PAUSED')").Scan(&out.ActivePlaybackSessions)
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM playback_pipeline_instances WHERE state IN ('STARTING','RUNNING')").Scan(&out.ActiveFFmpegProcesses)
	seen := map[string]bool{}
	for label, p := range map[string]string{"Config": s.paths.Config, "Transcode": s.paths.Transcode, "Optimized": s.paths.Optimized, "Downloads": s.paths.Downloads} {
		if p == "" {
			continue
		}
		abs, _ := filepath.Abs(p)
		if seen[abs] {
			continue
		}
		if total, used, avail, e := s.disk(abs); e == nil {
			seen[abs] = true
			out.Disks = append(out.Disks, DiskMetric{label, abs, total, used, avail})
		}
	}
	sort.Slice(out.Disks, func(i, j int) bool { return out.Disks[i].Label < out.Disks[j].Label })
	return out
}

func (s *Service) Analytics(ctx context.Context, from, to time.Time, user string) (Analytics, error) {
	a := Analytics{From: stamp(from), To: stamp(to), Modes: map[string]int{}}
	where := "started_at>=? AND started_at<?"
	args := []any{stamp(from), stamp(to)}
	if user != "" {
		where += " AND user_id=?"
		args = append(args, user)
	}
	q := `SELECT COUNT(*),COUNT(DISTINCT user_id),COALESCE(SUM(CASE WHEN logical_type='MOVIE' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN logical_type='EPISODE' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN state='ERROR' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN state='COMPLETED' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN duration_seconds>0 THEN MIN(position_seconds,duration_seconds) ELSE position_seconds END),0) FROM playback_sessions WHERE ` + where
	if e := s.db.QueryRowContext(ctx, q, args...).Scan(&a.TotalPlays, &a.UniqueUsers, &a.MoviesPlayed, &a.EpisodesPlayed, &a.PlaybackErrors, &a.CompletionCount, &a.PlaybackSeconds); e != nil {
		return a, e
	}
	rows, e := s.db.QueryContext(ctx, "SELECT mode,COUNT(*) FROM playback_sessions WHERE "+where+" GROUP BY mode", args...)
	if e != nil {
		return a, e
	}
	for rows.Next() {
		var k string
		var n int
		_ = rows.Scan(&k, &n)
		a.Modes[k] = n
	}
	rows.Close()
	a.TopMedia, _ = s.breakdown(ctx, "logical_type||':'||logical_id", where, args)
	a.TopUsers, _ = s.breakdown(ctx, "user_id", where, args)
	return a, nil
}
func (s *Service) breakdown(ctx context.Context, expr, where string, args []any) ([]Breakdown, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT "+expr+",COUNT(*) FROM playback_sessions WHERE "+where+" GROUP BY "+expr+" ORDER BY COUNT(*) DESC LIMIT 10", args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Breakdown{}
	for rows.Next() {
		var x Breakdown
		if e = rows.Scan(&x.Key, &x.Count); e != nil {
			return nil, e
		}
		x.Label = x.Key
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) LibraryStats(ctx context.Context) (LibraryStats, error) {
	x := LibraryStats{Resolution: map[string]int{}, VideoCodecs: map[string]int{}, HDR: map[string]int{}}
	queries := []struct {
		q string
		p *int
	}{{"SELECT COUNT(*) FROM movies WHERE orphaned=0", &x.Movies}, {"SELECT COUNT(*) FROM shows WHERE orphaned=0", &x.Shows}, {"SELECT COUNT(*) FROM episodes", &x.Episodes}, {"SELECT COUNT(*) FROM media_files", &x.PhysicalFiles}, {"SELECT COUNT(*) FROM media_files WHERE availability='AVAILABLE'", &x.AvailableFiles}, {"SELECT COUNT(*) FROM media_files WHERE availability='MISSING'", &x.MissingFiles}, {"SELECT COUNT(*) FROM media_files f WHERE NOT EXISTS(SELECT 1 FROM media_associations a WHERE a.media_file_id=f.id)", &x.UnmatchedFiles}, {"SELECT COUNT(*) FROM optimized_media WHERE status='COMPLETED'", &x.OptimizedVersions}}
	for _, q := range queries {
		if e := s.db.QueryRowContext(ctx, q.q).Scan(q.p); e != nil {
			return x, e
		}
	}
	for q, target := range map[string]map[string]int{"SELECT COALESCE(resolution_class,'Unknown'),COUNT(*) FROM media_files GROUP BY resolution_class": x.Resolution, "SELECT COALESCE(codec,'Unknown'),COUNT(*) FROM media_streams WHERE stream_type='video' GROUP BY codec": x.VideoCodecs, "SELECT COALESCE(hdr_class,'SDR'),COUNT(*) FROM media_files GROUP BY hdr_class": x.HDR} {
		rows, e := s.db.QueryContext(ctx, q)
		if e != nil {
			return x, e
		}
		for rows.Next() {
			var k string
			var n int
			_ = rows.Scan(&k, &n)
			target[k] = n
		}
		rows.Close()
	}
	return x, nil
}

func (s *Service) Jobs(ctx context.Context, limit int) ([]Job, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	q := `SELECT id,'BACKGROUND',target_type||':'||target_id,state,progress,priority,created_at,COALESCE(started_at,''),COALESCE(completed_at,''),COALESCE(error_summary,'') FROM background_jobs UNION ALL SELECT id,'SCAN',library_id,state,CASE WHEN candidates_found>0 THEN CAST(files_probed AS REAL)/candidates_found ELSE 0 END,0,created_at,COALESCE(started_at,''),COALESCE(completed_at,''),COALESCE(error_summary,'') FROM scan_jobs UNION ALL SELECT id,'METADATA',COALESCE(library_id,entity_type||':'||entity_id),state,CASE WHEN total_files>0 THEN CAST(processed AS REAL)/total_files ELSE 0 END,0,created_at,COALESCE(started_at,''),COALESCE(completed_at,''),COALESCE(error_summary,'') FROM metadata_jobs UNION ALL SELECT id,'OFFLINE_DOWNLOAD',asset_id,state,progress,priority,created_at,COALESCE(started_at,''),COALESCE(completed_at,''),COALESCE(last_error,'') FROM download_jobs ORDER BY created_at DESC LIMIT ?`
	rows, e := s.db.QueryContext(ctx, q, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		var x Job
		if e = rows.Scan(&x.ID, &x.Type, &x.Target, &x.State, &x.Progress, &x.Priority, &x.CreatedAt, &x.StartedAt, &x.CompletedAt, &x.Error); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) EvaluateHealth(ctx context.Context) ([]HealthIssue, error) {
	now := stamp(s.now())
	type detected struct{ category, severity, rt, rid, desc string }
	found := []detected{}
	rows, e := s.db.QueryContext(ctx, `SELECT s.id,l.name,s.configured_path,s.last_scan_status,COALESCE(s.last_scan_error,'') FROM library_sources s JOIN libraries l ON l.id=s.library_id WHERE s.enabled=1`)
	if e != nil {
		return nil, e
	}
	offline := map[string]bool{}
	for rows.Next() {
		var sid, name, path, state, msg string
		_ = rows.Scan(&sid, &name, &path, &state, &msg)
		if state == "SOURCE_UNAVAILABLE" || state == "FAILED" {
			offline[sid] = true
			found = append(found, detected{"SOURCE_UNAVAILABLE", "ERROR", "SOURCE", sid, "Source for " + name + " is unavailable. " + bounded(msg, 240)})
		}
	}
	rows.Close()
	rows, e = s.db.QueryContext(ctx, `SELECT f.id,f.file_name,f.source_id FROM media_files f WHERE f.availability='MISSING'`)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var fid, name, sid string
		_ = rows.Scan(&fid, &name, &sid)
		if !offline[sid] {
			found = append(found, detected{"MISSING_MEDIA", "WARNING", "MEDIA_FILE", fid, "Media file is missing: " + bounded(name, 160)})
		}
	}
	rows.Close()
	rows, e = s.db.QueryContext(ctx, `SELECT id,file_name FROM media_files WHERE probe_status='FAILED'`)
	if e == nil {
		for rows.Next() {
			var fid, name string
			_ = rows.Scan(&fid, &name)
			found = append(found, detected{"PROBE_FAILURE", "WARNING", "MEDIA_FILE", fid, "Media probe failed: " + bounded(name, 160)})
		}
		rows.Close()
	}
	rows, e = s.db.QueryContext(ctx, `SELECT f.id,f.file_name FROM media_files f WHERE f.availability='AVAILABLE' AND NOT EXISTS(SELECT 1 FROM media_associations a WHERE a.media_file_id=f.id)`)
	if e == nil {
		for rows.Next() {
			var fid, name string
			_ = rows.Scan(&fid, &name)
			found = append(found, detected{"UNMATCHED_MEDIA", "INFO", "MEDIA_FILE", fid, "Media is not matched: " + bounded(name, 160)})
		}
		rows.Close()
	}
	for label, path := range map[string]string{"CONFIG": s.paths.Config, "TRANSCODE": s.paths.Transcode, "OPTIMIZED": s.paths.Optimized, "DOWNLOADS": s.paths.Downloads} {
		if path == "" {
			continue
		}
		total, _, available, err := s.disk(path)
		if err != nil || total == 0 {
			continue
		}
		if available < 5*1024*1024*1024 || available*100/total < 5 {
			severity := "WARNING"
			if available < 1024*1024*1024 {
				severity = "ERROR"
			}
			found = append(found, detected{"STORAGE_LOW", severity, "STORAGE", label, label + " storage is low (" + fmt.Sprintf("%.1f GiB", float64(available)/1073741824) + " available)."})
		}
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	active := map[string]bool{}
	for _, d := range found {
		key := d.category + "|" + d.rt + "|" + d.rid
		active[key] = true
		var old string
		_ = tx.QueryRowContext(ctx, "SELECT status FROM health_issues WHERE category=? AND reference_type=? AND reference_id=?", d.category, d.rt, d.rid).Scan(&old)
		hid := id()
		_, e = tx.ExecContext(ctx, `INSERT INTO health_issues(id,category,severity,reference_type,reference_id,description,status,first_detected_at,last_detected_at) VALUES(?,?,?,?,?,?, 'OPEN',?,?) ON CONFLICT(category,reference_type,reference_id) DO UPDATE SET severity=excluded.severity,description=excluded.description,last_detected_at=excluded.last_detected_at,status=CASE WHEN health_issues.status='IGNORED' THEN 'IGNORED' ELSE 'OPEN' END,resolved_at=NULL`, hid, d.category, d.severity, d.rt, d.rid, d.desc, now, now)
		if e != nil {
			return nil, e
		}
		if old == "" || old == "RESOLVED" {
			_, _ = s.emitTx(ctx, tx, "HEALTH_ERROR_OPENED", map[string]any{"category": d.category, "severity": d.severity, "referenceType": d.rt, "referenceId": d.rid}, d.category+":"+d.rt+":"+d.rid+":open")
			if d.category == "SOURCE_UNAVAILABLE" {
				_, _ = s.emitTx(ctx, tx, "SOURCE_UNAVAILABLE", map[string]any{"sourceId": d.rid}, "source:"+d.rid+":unavailable")
			}
		}
	}
	openRows, e := tx.QueryContext(ctx, "SELECT id,category,reference_type,reference_id,severity FROM health_issues WHERE status='OPEN'")
	if e != nil {
		return nil, e
	}
	for openRows.Next() {
		var hid, cat, rt, rid, sev string
		_ = openRows.Scan(&hid, &cat, &rt, &rid, &sev)
		if !active[cat+"|"+rt+"|"+rid] {
			_, _ = tx.ExecContext(ctx, "UPDATE health_issues SET status='RESOLVED',resolved_at=?,last_detected_at=? WHERE id=?", now, now, hid)
			_, _ = s.emitTx(ctx, tx, "HEALTH_ERROR_RESOLVED", map[string]any{"category": cat, "severity": sev, "referenceType": rt, "referenceId": rid}, cat+":"+rt+":"+rid+":resolved")
			if cat == "SOURCE_UNAVAILABLE" {
				_, _ = s.emitTx(ctx, tx, "SOURCE_RECOVERED", map[string]any{"sourceId": rid}, "source:"+rid+":recovered")
			}
		}
	}
	openRows.Close()
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return s.Health(ctx, "", "")
}
func bounded(v string, n int) string {
	v = strings.TrimSpace(v)
	if len(v) > n {
		return v[:n]
	}
	return v
}
func (s *Service) emitTx(ctx context.Context, tx *sql.Tx, t string, p map[string]any, dedupe string) (Event, error) {
	def := EventCatalog[t]
	raw, _ := json.Marshal(p)
	e := Event{ID: id(), Type: t, Category: def.Category, Severity: def.Severity, Payload: p, CreatedAt: stamp(s.now())}
	var n int
	_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM operational_events WHERE event_type=? AND dedupe_key=? AND created_at>=?", t, dedupe, stamp(s.now().Add(-24*time.Hour))).Scan(&n)
	if n > 0 {
		return e, nil
	}
	_, er := tx.ExecContext(ctx, "INSERT INTO operational_events(id,event_type,category,severity,payload_json,dedupe_key,created_at) VALUES(?,?,?,?,?,?,?)", e.ID, t, e.Category, e.Severity, string(raw), dedupe, e.CreatedAt)
	if er == nil {
		_, er = tx.ExecContext(ctx, `INSERT INTO notification_deliveries(id,event_id,destination_id,status,next_attempt_at,created_at) SELECT lower(hex(randomblob(16))),?,d.id,'PENDING',?,? FROM notification_destinations d JOIN notification_subscriptions n ON n.destination_id=d.id WHERE d.enabled=1 AND n.event_type=?`, e.ID, e.CreatedAt, e.CreatedAt, t)
	}
	return e, er
}
func (s *Service) Health(ctx context.Context, status, severity string) ([]HealthIssue, error) {
	q := `SELECT id,category,severity,reference_type,reference_id,description,status,first_detected_at,last_detected_at,COALESCE(resolved_at,''),COALESCE(ignored_at,'') FROM health_issues WHERE 1=1`
	args := []any{}
	if status != "" {
		q += " AND status=?"
		args = append(args, status)
	}
	if severity != "" {
		q += " AND severity=?"
		args = append(args, severity)
	}
	q += " ORDER BY CASE severity WHEN 'CRITICAL' THEN 0 WHEN 'ERROR' THEN 1 WHEN 'WARNING' THEN 2 ELSE 3 END,last_detected_at DESC LIMIT 500"
	rows, e := s.db.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []HealthIssue{}
	for rows.Next() {
		var x HealthIssue
		if e = rows.Scan(&x.ID, &x.Category, &x.Severity, &x.ReferenceType, &x.ReferenceID, &x.Description, &x.Status, &x.FirstDetectedAt, &x.LastDetectedAt, &x.ResolvedAt, &x.IgnoredAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) SetHealthIgnored(ctx context.Context, id string, ignored bool) error {
	status := "OPEN"
	var when any = nil
	if ignored {
		status = "IGNORED"
		when = stamp(s.now())
	}
	r, e := s.db.ExecContext(ctx, "UPDATE health_issues SET status=?,ignored_at=? WHERE id=? AND status!='RESOLVED'", status, when, id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Service) Cleanup(ctx context.Context) error {
	_, e := s.db.ExecContext(ctx, "DELETE FROM operational_events WHERE created_at<? AND id NOT IN(SELECT event_id FROM notification_deliveries WHERE status IN ('PENDING','RETRYING'))", stamp(s.now().Add(-90*24*time.Hour)))
	if e == nil {
		_, e = s.db.ExecContext(ctx, "DELETE FROM notification_deliveries WHERE created_at<? AND status IN ('DELIVERED','FAILED','CANCELED')", stamp(s.now().Add(-30*24*time.Hour)))
	}
	return e
}
func (s *Service) Dashboard(ctx context.Context) (Dashboard, error) {
	d := Dashboard{Version: s.info.Version, Commit: s.info.Commit, ServerName: s.info.ServerName, InstanceID: s.info.InstanceID, DatabaseType: s.info.DatabaseType, FFmpegVersion: commandVersion(s.info.FFmpeg), FFprobeVersion: commandVersion(s.info.FFprobe), FFmpegPath: s.info.FFmpeg, FFmpegSource: s.info.FFmpegSource, FFprobePath: s.info.FFprobe, FFprobeSource: s.info.FFprobeSource, Metrics: s.Metrics(ctx), Health: map[string]int{}}
	var e error
	if d.Libraries, e = s.LibraryStats(ctx); e != nil {
		return d, e
	}
	rows, e := s.db.QueryContext(ctx, "SELECT severity,COUNT(*) FROM health_issues WHERE status='OPEN' GROUP BY severity")
	if e != nil {
		return d, e
	}
	for rows.Next() {
		var k string
		var n int
		_ = rows.Scan(&k, &n)
		d.Health[k] = n
	}
	rows.Close()
	_ = s.db.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM background_jobs WHERE state='FAILED')+(SELECT COUNT(*) FROM scan_jobs WHERE state='FAILED')+(SELECT COUNT(*) FROM metadata_jobs WHERE state='FAILED')+(SELECT COUNT(*) FROM download_jobs WHERE state IN ('FAILED','INTERRUPTED'))").Scan(&d.FailedJobs)
	d.RecentEvents, _ = s.Events(ctx, 12, 0)
	return d, nil
}

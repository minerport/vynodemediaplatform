package observability

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vynode/media/server/internal/database"
)

func testService(t *testing.T) (*Service, *sql.DB, func()) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	store, e := database.Open(ctx, filepath.Join(dir, "config"))
	if e != nil {
		t.Fatal(e)
	}
	if e = store.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	s, e := New(store.DB, SystemInfo{InstanceID: "server-test", StartedAt: time.Now()}, Paths{Config: dir}, dir)
	if e != nil {
		t.Fatal(e)
	}
	return s, store.DB, func() { s.Close(); store.Close() }
}

func TestSSRFPolicy(t *testing.T) {
	s, _, done := testService(t)
	defer done()
	ctx := context.Background()
	cases := []string{"http://127.0.0.1/x", "https://localhost/x", "https://169.254.169.254/latest", "https://[::1]/x", "https://10.0.0.1/x", "file:///tmp/x"}
	for _, raw := range cases {
		if e := s.ValidateURL(ctx, raw, false, false); e == nil {
			t.Errorf("accepted prohibited %s", raw)
		}
	}
	s.resolve = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}
	if e := s.ValidateURL(ctx, "https://hooks.example.test/vynode", false, false); e != nil {
		t.Fatal(e)
	}
	s.resolve = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("10.1.2.3")}}, nil
	}
	if e := s.ValidateURL(ctx, "https://public-looking.test/vynode", false, false); e == nil {
		t.Fatal("DNS rebind target accepted")
	}
	if e := s.ValidateURL(ctx, "http://private.test/vynode", true, true); e != nil {
		t.Fatalf("explicit private policy rejected: %v", e)
	}
}

func TestWebhookSignatureRetryAndPersistence(t *testing.T) {
	var calls atomic.Int32
	received := make(chan []byte, 1)
	secret := "known-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		m := hmac.New(sha256.New, []byte(secret))
		m.Write(body)
		want := "sha256=" + hex.EncodeToString(m.Sum(nil))
		if !hmac.Equal([]byte(want), []byte(r.Header.Get("X-VyNode-Signature"))) {
			t.Errorf("bad signature")
		}
		if calls.Add(1) < 3 {
			w.WriteHeader(500)
			return
		}
		received <- body
		w.WriteHeader(204)
	}))
	defer server.Close()
	s, db, done := testService(t)
	defer done()
	ctx := context.Background()
	d, e := s.SaveDestination(ctx, Destination{Name: "Harness", URL: server.URL, Enabled: true, AllowPrivateNetwork: true, AllowInsecureHTTP: true, MaxAttempts: 3, Secret: secret, EventTypes: []string{"TEST"}})
	if e != nil {
		t.Fatal(e)
	}
	if d.Secret != "" || !d.HasSecret {
		t.Fatal("secret exposure/state")
	}
	if _, e = s.Emit(ctx, "TEST", map[string]any{"test": true}, ""); e != nil {
		t.Fatal(e)
	}
	select {
	case body := <-received:
		var p map[string]any
		if json.Unmarshal(body, &p) != nil || p["serverId"] != "server-test" {
			t.Fatalf("payload=%s", body)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("delivery did not recover")
	}
	var status string
	var attempts int
	for i := 0; i < 20; i++ {
		e = db.QueryRow("SELECT status,attempt_count FROM notification_deliveries ORDER BY created_at DESC LIMIT 1").Scan(&status, &attempts)
		if status == "DELIVERED" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if e != nil || status != "DELIVERED" || attempts != 3 {
		t.Fatalf("%s %d %v", status, attempts, e)
	}
}

func TestPermanent400DoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(400) }))
	defer server.Close()
	s, db, done := testService(t)
	defer done()
	d, e := s.SaveDestination(context.Background(), Destination{Name: "Bad", URL: server.URL, Enabled: true, AllowPrivateNetwork: true, AllowInsecureHTTP: true, MaxAttempts: 5, EventTypes: []string{"TEST"}})
	_ = d
	if e != nil {
		t.Fatal(e)
	}
	_, _ = s.Emit(context.Background(), "TEST", map[string]any{"test": true}, "")
	time.Sleep(1500 * time.Millisecond)
	var status string
	var attempts int
	_ = db.QueryRow("SELECT status,attempt_count FROM notification_deliveries ORDER BY created_at DESC LIMIT 1").Scan(&status, &attempts)
	if status != "FAILED" || attempts != 1 || calls.Load() != 1 {
		t.Fatalf("%s attempts=%d calls=%d", status, attempts, calls.Load())
	}
}

func TestWebhookTimeoutAndRedirectAreBounded(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(6 * time.Second); w.WriteHeader(204) }))
		defer server.Close()
		s, db, done := testService(t)
		defer done()
		_, e := s.SaveDestination(context.Background(), Destination{Name: "Slow", URL: server.URL, Enabled: true, AllowPrivateNetwork: true, AllowInsecureHTTP: true, MaxAttempts: 1, EventTypes: []string{"TEST"}})
		if e != nil {
			t.Fatal(e)
		}
		started := time.Now()
		_, _ = s.Emit(context.Background(), "TEST", map[string]any{"test": true}, "")
		var status string
		for time.Since(started) < 7*time.Second {
			_ = db.QueryRow("SELECT status FROM notification_deliveries ORDER BY created_at DESC LIMIT 1").Scan(&status)
			if status == "FAILED" {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if status != "FAILED" || time.Since(started) > 7*time.Second {
			t.Fatalf("status=%s elapsed=%v", status, time.Since(started))
		}
	})
	t.Run("redirect", func(t *testing.T) {
		targetCalls := atomic.Int32{}
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls.Add(1); w.WriteHeader(204) }))
		defer target.Close()
		redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
		defer redirect.Close()
		s, db, done := testService(t)
		defer done()
		_, e := s.SaveDestination(context.Background(), Destination{Name: "Redirect", URL: redirect.URL, Enabled: true, AllowPrivateNetwork: true, AllowInsecureHTTP: true, MaxAttempts: 1, EventTypes: []string{"TEST"}})
		if e != nil {
			t.Fatal(e)
		}
		_, _ = s.Emit(context.Background(), "TEST", map[string]any{"test": true}, "")
		time.Sleep(1200 * time.Millisecond)
		var status string
		_ = db.QueryRow("SELECT status FROM notification_deliveries ORDER BY created_at DESC LIMIT 1").Scan(&status)
		if status != "FAILED" || targetCalls.Load() != 0 {
			t.Fatalf("status=%s redirected calls=%d", status, targetCalls.Load())
		}
	})
}

func TestHealthOfflineSuppressionDedupeAndRecovery(t *testing.T) {
	s, db, done := testService(t)
	defer done()
	now := stamp(time.Now())
	_, e := db.Exec(`INSERT INTO libraries(id,name,type,created_at,updated_at) VALUES('l','Movies','MOVIES',?,?); INSERT INTO library_sources(id,library_id,configured_path,normalized_path,created_at,last_scan_status) VALUES('src','l','/media','/media',?,'FAILED'); INSERT INTO media_files(id,source_id,relative_path,file_name,base_name,extension,parent_path,size_bytes,modified_at_ns,availability,probe_status,created_at,updated_at) VALUES('f','src','gone.mkv','gone.mkv','gone','mkv','',1,1,'MISSING','SUCCESS',?,?)`, now, now, now, now, now)
	if e != nil {
		t.Fatal(e)
	}
	issues, e := s.EvaluateHealth(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if len(issues) != 1 || issues[0].Category != "SOURCE_UNAVAILABLE" {
		t.Fatalf("issues=%+v", issues)
	}
	_, _ = s.EvaluateHealth(context.Background())
	var events int
	_ = db.QueryRow("SELECT COUNT(*) FROM operational_events WHERE event_type='HEALTH_ERROR_OPENED'").Scan(&events)
	if events != 1 {
		t.Fatalf("opened events=%d", events)
	}
	_ = db.QueryRow("SELECT COUNT(*) FROM operational_events WHERE event_type='SOURCE_UNAVAILABLE'").Scan(&events)
	if events != 1 {
		t.Fatalf("source unavailable events=%d", events)
	}
	_, _ = db.Exec("UPDATE library_sources SET last_scan_status='COMPLETED' WHERE id='src'")
	_, _ = db.Exec("UPDATE media_files SET availability='AVAILABLE' WHERE id='f'")
	issues, e = s.EvaluateHealth(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	var sourceResolved bool
	for _, x := range issues {
		if x.Category == "SOURCE_UNAVAILABLE" && x.Status == "RESOLVED" {
			sourceResolved = true
		}
		if x.Category == "MISSING_MEDIA" && x.Status == "OPEN" {
			t.Fatal("missing issue remained after recovery")
		}
	}
	if !sourceResolved {
		t.Fatalf("recovery=%+v", issues)
	}
	_ = db.QueryRow("SELECT COUNT(*) FROM operational_events WHERE event_type='SOURCE_RECOVERED'").Scan(&events)
	if events != 1 {
		t.Fatalf("source recovered events=%d", events)
	}
}

func TestAnalyticsUserIsolationAndModes(t *testing.T) {
	s, db, done := testService(t)
	defer done()
	_, _ = db.Exec("PRAGMA foreign_keys=OFF")
	now := time.Now().UTC()
	insert := func(id, user, kind, mode, state string, pos float64) {
		_, e := db.Exec(`INSERT INTO playback_sessions(id,user_id,auth_session_id,capability_id,logical_type,logical_id,mode,state,position_seconds,duration_seconds,started_at,last_activity_at,pipeline_plan_json) VALUES(?,?,?,?,?,?, ?,?,?,100,?,?,'{}')`, id, user, "s", "c", kind, id, mode, state, pos, stamp(now), stamp(now))
		if e != nil {
			t.Fatal(e)
		}
	}
	insert("a", "u1", "MOVIE", "DIRECT_PLAY", "COMPLETED", 100)
	insert("b", "u1", "EPISODE", "VIDEO_TRANSCODE", "ERROR", 20)
	insert("c", "u2", "MOVIE", "DIRECT_STREAM", "STOPPED", 50)
	all, e := s.Analytics(context.Background(), now.Add(-time.Hour), now.Add(time.Hour), "")
	if e != nil {
		t.Fatal(e)
	}
	if all.TotalPlays != 3 || all.UniqueUsers != 2 || all.Modes["DIRECT_PLAY"] != 1 || all.Modes["VIDEO_TRANSCODE"] != 1 {
		t.Fatalf("all=%+v", all)
	}
	own, e := s.Analytics(context.Background(), now.Add(-time.Hour), now.Add(time.Hour), "u1")
	if e != nil || own.TotalPlays != 2 || own.UniqueUsers != 1 {
		t.Fatalf("own=%+v err=%v", own, e)
	}
}

func TestStorageHealthTransition(t *testing.T) {
	s, db, done := testService(t)
	defer done()
	s.paths = Paths{Config: "config"}
	s.disk = func(string) (uint64, uint64, uint64, error) { return 100 << 30, 99 << 30, 1 << 30, nil }
	issues, e := s.EvaluateHealth(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	var opened bool
	for _, x := range issues {
		if x.Category == "STORAGE_LOW" && x.Status == "OPEN" {
			opened = true
		}
	}
	if !opened {
		t.Fatalf("storage issue not opened: %+v", issues)
	}
	s.disk = func(string) (uint64, uint64, uint64, error) { return 100 << 30, 50 << 30, 50 << 30, nil }
	issues, e = s.EvaluateHealth(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	var resolved bool
	for _, x := range issues {
		if x.Category == "STORAGE_LOW" && x.Status == "RESOLVED" {
			resolved = true
		}
	}
	if !resolved {
		t.Fatalf("storage issue not resolved: %+v", issues)
	}
	var events int
	_ = db.QueryRow("SELECT COUNT(*) FROM operational_events WHERE event_type='HEALTH_ERROR_RESOLVED'").Scan(&events)
	if events != 1 {
		t.Fatalf("resolved events=%d", events)
	}
}

func TestPendingQueueResumesAfterRestart(t *testing.T) {
	var calls atomic.Int32
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(204) }))
	defer receiver.Close()
	s, db, done := testService(t)
	defer done()
	d, e := s.SaveDestination(context.Background(), Destination{Name: "Restart", URL: receiver.URL, Enabled: true, AllowPrivateNetwork: true, AllowInsecureHTTP: true, MaxAttempts: 2, EventTypes: []string{"TEST"}})
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	now := stamp(time.Now())
	eventID := id()
	_, e = db.Exec("INSERT INTO operational_events(id,event_type,category,severity,payload_json,created_at) VALUES(?,'TEST','SERVER','INFO','{}',?)", eventID, now)
	if e == nil {
		_, e = db.Exec("INSERT INTO notification_deliveries(id,event_id,destination_id,status,next_attempt_at,created_at) VALUES(?,?,?,'PENDING',?,?)", id(), eventID, d.ID, now, now)
	}
	if e != nil {
		t.Fatal(e)
	}
	s2, e := New(db, SystemInfo{InstanceID: "server-test", StartedAt: time.Now()}, Paths{}, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s2.Close()
	deadline := time.Now().Add(3 * time.Second)
	var status string
	for time.Now().Before(deadline) {
		_ = db.QueryRow("SELECT status FROM notification_deliveries WHERE event_id=?", eventID).Scan(&status)
		if status == "DELIVERED" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if status != "DELIVERED" || calls.Load() != 1 {
		t.Fatalf("status=%s calls=%d", status, calls.Load())
	}
}

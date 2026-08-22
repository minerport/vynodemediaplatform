package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type readyStub struct{ err error }

func (s readyStub) Ready(context.Context) error { return s.err }
func handler(ready error) http.Handler {
	return NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), readyStub{ready}, SystemInfo{Version: "0.1.0", Commit: "test", OS: "testos", Architecture: "testarch", InstanceID: "instance", ServerName: "Test", DatabaseType: "sqlite", StartedAt: time.Now()}, nil, nil, nil, nil, "")
}
func request(t *testing.T, path string, ready error) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	handler(ready).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
	return rr
}
func TestHealth(t *testing.T) {
	if got := request(t, "/health", nil).Code; got != 200 {
		t.Fatalf("got %d", got)
	}
}

func TestSPAHandlerServesAssetsAndProtectedRouteFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<title>VyNode</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log('vynode')"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), readyStub{}, SystemInfo{WebDir: dir, StartedAt: time.Now()}, nil, nil, nil, nil, "")
	for _, path := range []string{"/home", "/account", "/security/sessions"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "VyNode") {
			t.Fatalf("route %s did not serve SPA: status=%d body=%q", path, rr.Code, rr.Body.String())
		}
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "console.log") {
		t.Fatalf("asset not served: status=%d body=%q", rr.Code, rr.Body.String())
	}
}
func TestReadiness(t *testing.T) {
	if got := request(t, "/ready", nil).Code; got != 200 {
		t.Fatalf("got %d", got)
	}
	if got := request(t, "/ready", errors.New("down")).Code; got != 503 {
		t.Fatalf("got %d", got)
	}
}
func TestVersion(t *testing.T) {
	rr := request(t, "/api/v1/system/version", nil)
	var body map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["version"] != "0.1.0" {
		t.Fatalf("unexpected: %v", body)
	}
}
func TestSystemInfoDoesNotExposePaths(t *testing.T) {
	rr := request(t, "/api/v1/system/info", nil)
	var body map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["instanceId"] != "instance" || body["databaseType"] != "sqlite" {
		t.Fatalf("unexpected: %v", body)
	}
	if _, ok := body["configDir"]; ok {
		t.Fatal("system info exposed config directory")
	}
}
func TestErrorFormatAndRequestID(t *testing.T) {
	rr := request(t, "/missing", nil)
	var body errorResponse
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if rr.Code != 404 || body.Error.Code != "not_found" || body.Error.RequestID == "" {
		t.Fatalf("unexpected: %+v", body)
	}
	if rr.Header().Get("X-Request-ID") != body.Error.RequestID {
		t.Fatal("request IDs differ")
	}
}

func TestAcceptsSafeClientRequestID(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("X-Request-ID", "client-request-123")
	handler(nil).ServeHTTP(recorder, request)
	if recorder.Header().Get("X-Request-ID") != "client-request-123" {
		t.Fatal("request ID was not preserved")
	}
}

func TestOriginAndSecurityPolicy(t *testing.T) {
	h := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), readyStub{}, SystemInfo{StartedAt: time.Now()}, nil, nil, nil, nil, "https://media.example")
	foreign := httptest.NewRequest(http.MethodPost, "/missing", nil)
	foreign.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, foreign)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("foreign origin got %d", rr.Code)
	}
	trusted := httptest.NewRequest(http.MethodPost, "/missing", nil)
	trusted.Header.Set("Origin", "https://media.example")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, trusted)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("trusted origin got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://media.example" {
		t.Fatal("trusted CORS origin missing")
	}
	if rr.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("CSP missing")
	}
}
func TestRateLimiterRecovers(t *testing.T) {
	l := &limiter{attempts: map[string][]time.Time{}}
	for i := 0; i < 10; i++ {
		if !l.allow("peer") {
			t.Fatal("limited early")
		}
	}
	if l.allow("peer") {
		t.Fatal("limit not enforced")
	}
	l.attempts["peer"] = []time.Time{time.Now().Add(-2 * time.Minute)}
	if !l.allow("peer") {
		t.Fatal("limit did not recover")
	}
}

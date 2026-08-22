package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type readyStub struct{ err error }

func (s readyStub) Ready(context.Context) error { return s.err }
func handler(ready error) http.Handler {
	return NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), readyStub{ready}, SystemInfo{Version: "0.1.0", Commit: "test", OS: "testos", Architecture: "testarch", InstanceID: "instance", ServerName: "Test", DatabaseType: "sqlite", StartedAt: time.Now()})
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

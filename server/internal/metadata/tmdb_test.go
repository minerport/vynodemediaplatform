package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func tmdbServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Error("missing bearer")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}
func testTMDb(s *httptest.Server) *TMDb {
	t := NewTMDb(s.URL, "token", "test")
	t.client = s.Client()
	t.client.Timeout = 100 * time.Millisecond
	return t
}
func TestTMDbSearch(t *testing.T) {
	s := tmdbServer(t, 200, `{"results":[{"id":1091,"title":"The Thing","release_date":"1982-06-25"}]}`)
	defer s.Close()
	v, e := testTMDb(s).SearchMovies(context.Background(), "The Thing", 1982, "en-US", "US")
	if e != nil || len(v) != 1 || v[0].Year != 1982 {
		t.Fatalf("%+v %v", v, e)
	}
}
func TestTMDbErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		want   error
	}{{401, "{}", ErrUnauthorized}, {429, "{}", ErrRateLimited}, {200, "{", ErrProviderResponse}} {
		s := tmdbServer(t, tc.status, tc.body)
		_, e := testTMDb(s).SearchMovies(context.Background(), "x", 0, "", "")
		s.Close()
		if e != tc.want {
			t.Errorf("status %d: %v", tc.status, e)
		}
	}
}
func TestTMDbBounded(t *testing.T) {
	s := tmdbServer(t, 200, `{"results":[]}`+strings.Repeat(" ", 2<<20))
	defer s.Close()
	_, e := testTMDb(s).SearchMovies(context.Background(), "x", 0, "", "")
	if e != ErrProviderResponse {
		t.Fatalf("expected bounded response error, got %v", e)
	}
}
func TestTMDbTimeout(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(200 * time.Millisecond) }))
	defer s.Close()
	_, e := testTMDb(s).SearchMovies(context.Background(), "x", 0, "", "")
	if e == nil {
		t.Fatal("expected timeout")
	}
}
func TestTMDbNormalizesCreditsAndCompanies(t *testing.T) {
	s := tmdbServer(t, 200, `{"id":603,"title":"The Matrix","release_date":"1999-03-31","production_companies":[{"id":79,"name":"Studio"}],"external_ids":{"imdb_id":"tt0133093"},"credits":{"cast":[{"id":1,"name":"Actor","character":"Lead","order":0}],"crew":[{"id":2,"name":"Director","job":"Director","department":"Directing"},{"id":3,"name":"Writer","job":"Screenplay","department":"Writing"}]}}`)
	defer s.Close()
	v, e := testTMDb(s).Movie(context.Background(), "603", "en-US", "US")
	if e != nil {
		t.Fatal(e)
	}
	if len(v.Credits) != 3 || len(v.Companies) != 1 || v.ExternalIDs["IMDB"] != "tt0133093" {
		t.Fatalf("normalization failed: %+v", v)
	}
}

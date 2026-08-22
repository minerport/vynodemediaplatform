package metadata

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vynode/media/server/internal/database"
)

func artworkFixture(t *testing.T, handler http.HandlerFunc) (*Service, *database.Store, *httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(handler)
	dir := t.TempDir()
	store, e := database.Open(context.Background(), filepath.Join(dir, "config"))
	if e != nil {
		t.Fatal(e)
	}
	if e = store.Migrate(context.Background()); e != nil {
		t.Fatal(e)
	}
	s := New(store.DB, filepath.Join(dir, "config"), fakeArtworkProvider{}, srv.URL, "true")
	id, e := s.AddArtwork(context.Background(), "MOVIE", "movie", ProviderArtwork{Type: "POSTER", Path: "/poster"})
	if e != nil {
		t.Fatal(e)
	}
	return s, store, srv, id
}

type fakeArtworkProvider struct{}

func (fakeArtworkProvider) Name() string               { return "TMDB" }
func (fakeArtworkProvider) Test(context.Context) error { return nil }
func (fakeArtworkProvider) SearchMovies(context.Context, string, int, string, string) ([]Candidate, error) {
	return nil, nil
}
func (fakeArtworkProvider) Movie(context.Context, string, string, string) (MovieDetails, error) {
	return MovieDetails{}, nil
}
func (fakeArtworkProvider) SearchShows(context.Context, string, int, string, string) ([]Candidate, error) {
	return nil, nil
}
func (fakeArtworkProvider) Show(context.Context, string, string, string) (ShowDetails, error) {
	return ShowDetails{}, nil
}
func (fakeArtworkProvider) Season(context.Context, string, int, string, string) (SeasonDetails, error) {
	return SeasonDetails{}, nil
}
func encoded(format string) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.White)
	var b bytes.Buffer
	if format == "png" {
		_ = png.Encode(&b, img)
	} else {
		_ = jpeg.Encode(&b, img, nil)
	}
	return b.Bytes()
}
func TestArtworkValidJPEGAndPNG(t *testing.T) {
	for _, tc := range []struct{ mime, format string }{{"image/jpeg", "jpeg"}, {"image/png", "png"}} {
		t.Run(tc.format, func(t *testing.T) {
			body := encoded(tc.format)
			s, db, srv, id := artworkFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.mime)
				_, _ = w.Write(body)
			})
			defer db.Close()
			defer srv.Close()
			if e := s.CacheArtwork(context.Background(), id); e != nil {
				t.Fatal(e)
			}
			path, mime, etag, e := s.ArtworkFile(context.Background(), id)
			if e != nil || mime != tc.mime || etag == "" {
				t.Fatalf("%s %s %v", path, mime, e)
			}
			if _, e = os.Stat(path); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestArtworkRejectsUnsafeResponses(t *testing.T) {
	cases := []struct {
		name, mime string
		body       []byte
	}{{"mime", "text/plain", encoded("jpeg")}, {"invalid", "image/jpeg", []byte("not an image")}, {"truncated", "image/png", encoded("png")[:12]}, {"oversized", "image/jpeg", bytes.Repeat([]byte{'x'}, maxArtworkBytes+1)}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, db, srv, id := artworkFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.mime)
				_, _ = w.Write(tc.body)
			})
			defer db.Close()
			defer srv.Close()
			if e := s.CacheArtwork(context.Background(), id); e == nil {
				t.Fatal("expected rejection")
			}
			entries, _ := os.ReadDir(filepath.Join(s.configDir, "cache", "artwork"))
			for _, x := range entries {
				if strings.HasSuffix(x.Name(), ".tmp") {
					t.Fatal("partial temp file retained")
				}
			}
		})
	}
}
func TestArtworkTimeoutAndLastKnownGoodRetention(t *testing.T) {
	good := true
	s, db, srv, id := artworkFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if !good {
			time.Sleep(100 * time.Millisecond)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(encoded("jpeg"))
	})
	defer db.Close()
	defer srv.Close()
	s.artworkTimeout = 25 * time.Millisecond
	if e := s.CacheArtwork(context.Background(), id); e != nil {
		t.Fatal(e)
	}
	before, _, _, _ := s.ArtworkFile(context.Background(), id)
	dataBefore, _ := os.ReadFile(before)
	good = false
	if e := s.CacheArtwork(context.Background(), id); e == nil {
		t.Fatal("expected timeout")
	}
	after, _, _, e := s.ArtworkFile(context.Background(), id)
	if e != nil {
		t.Fatal(e)
	}
	dataAfter, _ := os.ReadFile(after)
	if !bytes.Equal(dataBefore, dataAfter) {
		t.Fatal("failed refresh replaced valid cache")
	}
}
func TestArtworkPartialTransferCleansTemporaryFile(t *testing.T) {
	s, db, srv, id := artworkFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write(encoded("png")[:10])
	})
	defer db.Close()
	defer srv.Close()
	if e := s.CacheArtwork(context.Background(), id); e == nil {
		t.Fatal("expected partial transfer failure")
	}
	dir := filepath.Join(s.configDir, "cache", "artwork")
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp") {
			t.Fatal("temporary file retained")
		}
	}
}
func TestArtworkRejectsMalformedProviderPathAndCorruptCache(t *testing.T) {
	s, db, srv, _ := artworkFixture(t, func(w http.ResponseWriter, r *http.Request) {})
	defer db.Close()
	defer srv.Close()
	if _, e := s.AddArtwork(context.Background(), "MOVIE", "movie", ProviderArtwork{Type: "POSTER", Path: "https://evil.invalid/a"}); e != ErrValidation {
		t.Fatalf("got %v", e)
	}
	id, e := s.AddArtwork(context.Background(), "MOVIE", "movie", ProviderArtwork{Type: "BACKDROP", Path: "/backdrop"})
	if e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join(s.configDir, "cache", "artwork")
	_ = os.MkdirAll(dir, 0750)
	_ = os.WriteFile(filepath.Join(dir, "bad.jpg"), []byte("bad"), 0600)
	_, _ = s.db.Exec("UPDATE artwork SET cached_relative_path='cache/artwork/bad.jpg',mime_type='image/jpeg' WHERE id=?", id)
	if _, _, _, e = s.ArtworkFile(context.Background(), id); e != ErrNotFound {
		t.Fatalf("corrupt cache served: %v", e)
	}
}
func TestManualArtworkSelectionPersistsRestart(t *testing.T) {
	body := encoded("png")
	s, db, srv, first := artworkFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	})
	defer db.Close()
	defer srv.Close()
	second, e := s.AddArtwork(context.Background(), "MOVIE", "movie", ProviderArtwork{Type: "POSTER", Path: "/alternate"})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.SelectArtwork(context.Background(), "MOVIE", "movie", second); e != nil {
		t.Fatal(e)
	}
	restarted := New(db.DB, s.configDir, fakeArtworkProvider{}, srv.URL, "true")
	items, e := restarted.Artwork(context.Background(), "MOVIE", "movie")
	if e != nil {
		t.Fatal(e)
	}
	for _, a := range items {
		if a.ID == second && (!a.Selected || !a.ManualSelection || !a.Cached) {
			t.Fatalf("manual selection not persistent: %+v", a)
		}
		if a.ID == first && a.Selected {
			t.Fatal("previous artwork remained selected")
		}
	}
}

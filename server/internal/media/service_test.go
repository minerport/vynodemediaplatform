package media

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/vynode/media/server/internal/database"
)

type fakeProbe struct {
	mu    sync.Mutex
	calls int
	block chan struct{}
}

func (f *fakeProbe) Available() bool                { return true }
func (f *fakeProbe) Version(context.Context) string { return "ffprobe test" }
func (f *fakeProbe) Probe(ctx context.Context, path string) (ProbeResult, error) {
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return ProbeResult{}, ctx.Err()
		}
	}
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return ProbeResult{ContainerFormat: "matroska", Duration: 1, Streams: []Stream{{Index: 0, Type: "VIDEO", Codec: "h264", Width: 1920, Height: 1080}, {Index: 1, Type: "AUDIO", Codec: "aac", Channels: 2}}}, nil
}
func (f *fakeProbe) count() int { f.mu.Lock(); defer f.mu.Unlock(); return f.calls }
func testService(t *testing.T, p MediaProbe) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	store, err := database.Open(context.Background(), filepath.Join(root, "config"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err = store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	mediaDir := filepath.Join(root, "media")
	if err = os.Mkdir(mediaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return New(store.DB, p, filepath.Join(root, "config"), filepath.Join(root, "transcode")), mediaDir
}
func waitJob(t *testing.T, s *Service, libraryID, jobID string) Job {
	t.Helper()
	for i := 0; i < 100; i++ {
		j, err := s.GetJob(context.Background(), libraryID, jobID)
		if err != nil {
			t.Fatal(err)
		}
		if j.State != "QUEUED" && j.State != "RUNNING" {
			return j
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job timeout")
	return Job{}
}

func TestIncrementalScanOfflineSafetyAndDeletionReadOnly(t *testing.T) {
	ctx := context.Background()
	probe := &fakeProbe{}
	s, dir := testService(t, probe)
	movie := filepath.Join(dir, "The.Thing.1982.1080p.mkv")
	if err := os.WriteFile(movie, []byte("synthetic"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib, err := s.CreateLibrary(ctx, "Movies", LibraryMovies, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddSource(ctx, lib.ID, dir); err != nil {
		t.Fatal(err)
	}
	j, _ := s.StartScan(ctx, lib.ID)
	done := waitJob(t, s, lib.ID, j.ID)
	if done.State != "COMPLETED" || done.FilesAdded != 1 || probe.count() != 1 {
		t.Fatalf("first scan: %#v calls=%d", done, probe.count())
	}
	files, _ := s.ListFiles(ctx, lib.ID, 50, 0)
	stableID := files[0].ID
	j, _ = s.StartScan(ctx, lib.ID)
	done = waitJob(t, s, lib.ID, j.ID)
	if done.FilesUnchanged != 1 || probe.count() != 1 {
		t.Fatalf("unchanged reprobed: %#v calls=%d", done, probe.count())
	}
	time.Sleep(time.Millisecond)
	if err = os.WriteFile(movie, []byte("synthetic changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	j, _ = s.StartScan(ctx, lib.ID)
	done = waitJob(t, s, lib.ID, j.ID)
	if done.FilesUpdated != 1 || probe.count() != 2 {
		t.Fatalf("modify: %#v calls=%d", done, probe.count())
	}
	if err = os.Remove(movie); err != nil {
		t.Fatal(err)
	}
	j, _ = s.StartScan(ctx, lib.ID)
	done = waitJob(t, s, lib.ID, j.ID)
	files, _ = s.ListFiles(ctx, lib.ID, 50, 0)
	if done.FilesMissing != 1 || files[0].Availability != "MISSING" {
		t.Fatalf("missing: %#v %#v", done, files)
	}
	if err = os.WriteFile(movie, []byte("synthetic changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	j, _ = s.StartScan(ctx, lib.ID)
	_ = waitJob(t, s, lib.ID, j.ID)
	files, _ = s.ListFiles(ctx, lib.ID, 50, 0)
	if files[0].ID != stableID || files[0].Availability != "AVAILABLE" {
		t.Fatalf("restore lost identity: %#v", files[0])
	}
	offline := dir + "-offline"
	if err = os.Rename(dir, offline); err != nil {
		t.Fatal(err)
	}
	j, _ = s.StartScan(ctx, lib.ID)
	done = waitJob(t, s, lib.ID, j.ID)
	files, _ = s.ListFiles(ctx, lib.ID, 50, 0)
	if done.State != "COMPLETED_WITH_ERRORS" || files[0].Availability != "AVAILABLE" {
		t.Fatalf("offline mass missing: %#v %#v", done, files[0])
	}
	if err = s.DeleteLibrary(ctx, lib.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(offline, filepath.Base(movie))); err != nil {
		t.Fatalf("library deletion touched media: %v", err)
	}
}

func TestValidationDuplicateAndCancellation(t *testing.T) {
	ctx := context.Background()
	p := &fakeProbe{block: make(chan struct{})}
	s, dir := testService(t, p)
	if _, err := s.ValidatePath("relative", ""); err != ErrValidation {
		t.Fatalf("relative path accepted: %v", err)
	}
	lib, _ := s.CreateLibrary(ctx, "TV", LibraryTV, true)
	src, err := s.AddSource(ctx, lib.ID, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddSource(ctx, lib.ID, dir); err != ErrConflict {
		t.Fatalf("duplicate accepted: %v", err)
	}
	if err = os.WriteFile(filepath.Join(dir, "Show.S01E01.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	j, _ := s.StartScan(ctx, lib.ID)
	for i := 0; i < 50; i++ {
		x, _ := s.GetJob(ctx, lib.ID, j.ID)
		if x.State == "RUNNING" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, err = s.StartScan(ctx, lib.ID); err != ErrConflict {
		t.Fatalf("duplicate scan accepted: %v", err)
	}
	if err = s.Cancel(lib.ID, j.ID); err != nil {
		t.Fatal(err)
	}
	close(p.block)
	done := waitJob(t, s, lib.ID, j.ID)
	if done.State != "CANCELED" {
		t.Fatalf("cancel state: %#v", done)
	}
	if err = s.RemoveSource(ctx, lib.ID, src.ID); err != nil {
		t.Fatal(err)
	}
}

package offline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/database"
)

type testFixture struct {
	s      *Service
	p      auth.Principal
	other  auth.Principal
	source string
}

func fixture(t *testing.T) testFixture {
	t.Helper()
	ctx := context.Background()
	base := t.TempDir()
	store, e := database.Open(ctx, filepath.Join(base, "config"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = store.Close() })
	if e = store.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	mediaRoot := filepath.Join(base, "media")
	_ = os.MkdirAll(mediaRoot, 0700)
	sourcePath := filepath.Join(mediaRoot, "sample.mp4")
	if e = os.WriteFile(sourcePath, []byte("synthetic-compatible-mp4-bytes"), 0600); e != nil {
		t.Fatal(e)
	}
	st, _ := os.Stat(sourcePath)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := store.DB.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO users(id,username,password_hash,role,display_name,status,created_at) VALUES('u','user','x','USER','User','ACTIVE',?),('o','other','x','USER','Other','ACTIVE',?)", now, now)
	exec("INSERT INTO devices(id,user_id,name,client_name,platform,created_at,last_seen_at,authorization_type) VALUES('d','u','Phone','native','android',?,?,'PAIRED'),('d2','o','Other','native','android',?,?,'PAIRED')", now, now, now, now)
	exec("INSERT INTO sessions(id,user_id,device_id,refresh_token_hash,created_at,last_activity_at,expires_at) VALUES('s','u','d','x',?,?,?),('s2','o','d2','y',?,?,?)", now, now, future, now, now, future)
	exec("INSERT INTO libraries(id,name,type,created_at,updated_at) VALUES('l','Movies','MOVIES',?,?)", now, now)
	exec("INSERT INTO library_sources(id,library_id,configured_path,normalized_path,created_at) VALUES('ls','l',?,?,?)", mediaRoot, mediaRoot, now)
	exec("INSERT INTO media_files(id,source_id,relative_path,file_name,base_name,extension,parent_path,size_bytes,modified_at_ns,availability,probe_status,container_format,duration_seconds,created_at,updated_at) VALUES('f','ls','sample.mp4','sample.mp4','sample','.mp4','.',?,?,'AVAILABLE','OK','mp4',120,?,?)", st.Size(), st.ModTime().UnixNano(), now, now)
	exec("INSERT INTO media_streams(id,media_file_id,stream_index,stream_type,codec,width,height,is_default) VALUES('v','f',0,'video','h264',1280,720,1)")
	exec("INSERT INTO media_streams(id,media_file_id,stream_index,stream_type,codec,channels,is_default) VALUES('a','f',1,'audio','aac',2,1)")
	exec("INSERT INTO movies(id,title,sort_title,year,metadata_state,created_at,updated_at) VALUES('m','Movie','movie',2026,'MATCHED',?,?)", now, now)
	exec("INSERT INTO media_associations(id,media_file_id,entity_type,entity_id,association_type,created_at) VALUES('ma','f','MOVIE','m','MANUAL',?)", now)
	exec("INSERT INTO library_access_grants(user_id,library_id,permission,granted_by,created_at) VALUES('u','l','DOWNLOAD','u',?)", now)
	s, e := New(store.DB, filepath.Join(base, "downloads"), "")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	return testFixture{s: s, p: auth.Principal{UserID: "u", SessionID: "s", Role: auth.RoleUser}, other: auth.Principal{UserID: "o", SessionID: "s2", Role: auth.RoleUser}, source: sourcePath}
}

func TestOriginalDownloadDedupChecksumAndDeviceBinding(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	plan, e := f.s.Plan(ctx, "MOVIE", "m", "ORIGINAL")
	if e != nil || plan.Mode != "ORIGINAL_COPY" {
		t.Fatalf("plan=%+v err=%v", plan, e)
	}
	a, e := f.s.Create(ctx, f.p, CreateRequest{LogicalType: "MOVIE", LogicalID: "m", ProfileID: "ORIGINAL"})
	if e != nil {
		t.Fatal(e)
	}
	if a.Status != "READY" || a.ChecksumSHA256 == "" {
		t.Fatalf("download=%+v", a)
	}
	raw, _ := os.ReadFile(f.source)
	sum := sha256.Sum256(raw)
	if a.ChecksumSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("checksum mismatch")
	}
	b, e := f.s.Create(ctx, f.p, CreateRequest{LogicalType: "MOVIE", LogicalID: "m", ProfileID: "ORIGINAL"})
	if e != nil || a.AssetID != b.AssetID || a.ID != b.ID {
		t.Fatalf("dedup failed a=%+v b=%+v err=%v", a, b, e)
	}
	if _, _, e = f.s.Path(ctx, f.other, a.ID); e != ErrNotFound {
		t.Fatalf("cross-device=%v", e)
	}
}

func TestOfflineProgressIdempotenceAndNoRegression(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	newer := ProgressEvent{EventID: "e1", SequenceEpoch: "install-1", DeviceSequence: 1, LogicalType: "MOVIE", LogicalID: "m", Position: 70, Duration: 100, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if _, e := f.s.Push(ctx, f.p, Push{Progress: []ProgressEvent{newer}}); e != nil {
		t.Fatal(e)
	}
	older := ProgressEvent{EventID: "e2", SequenceEpoch: "install-1", DeviceSequence: 2, LogicalType: "MOVIE", LogicalID: "m", Position: 40, Duration: 100, OccurredAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)}
	if _, e := f.s.Push(ctx, f.p, Push{Progress: []ProgressEvent{older, older}}); e != nil {
		t.Fatal(e)
	}
	var position float64
	var events int
	_ = f.s.db.QueryRow("SELECT position_seconds FROM user_media_progress WHERE user_id='u' AND logical_id='m'").Scan(&position)
	_ = f.s.db.QueryRow("SELECT COUNT(*) FROM offline_progress_events").Scan(&events)
	if position != 70 || events != 2 {
		t.Fatalf("position=%v events=%d", position, events)
	}
}

func TestWatchedWinsAndManualUnwatched(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	events := []ProgressEvent{{EventID: "w", SequenceEpoch: "x", DeviceSequence: 1, LogicalType: "MOVIE", LogicalID: "m", Position: 99, Duration: 100, Watched: true, ExplicitAction: "WATCHED"}, {EventID: "old", SequenceEpoch: "x", DeviceSequence: 2, LogicalType: "MOVIE", LogicalID: "m", Position: 20, Duration: 100, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano)}}
	if _, e := f.s.Push(ctx, f.p, Push{Progress: events}); e != nil {
		t.Fatal(e)
	}
	var watched bool
	_ = f.s.db.QueryRow("SELECT watched FROM user_media_progress WHERE user_id='u' AND logical_id='m'").Scan(&watched)
	if !watched {
		t.Fatal("watched regressed")
	}
	u := ProgressEvent{EventID: "u", SequenceEpoch: "x", DeviceSequence: 3, LogicalType: "MOVIE", LogicalID: "m", ExplicitAction: "UNWATCHED", OccurredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	_, _ = f.s.Push(ctx, f.p, Push{Progress: []ProgressEvent{u}})
	_ = f.s.db.QueryRow("SELECT watched FROM user_media_progress WHERE user_id='u' AND logical_id='m'").Scan(&watched)
	if watched {
		t.Fatal("manual unwatched ignored")
	}
}

func TestSafeFilename(t *testing.T) {
	for in, want := range map[string]string{"../CON\r\n.mp4": "_CON__.mp4", "CON": "_CON", "a/b:c": "a_b_c"} {
		if got := SafeFilename(in); got != want {
			t.Fatalf("%q => %q want %q", in, got, want)
		}
	}
}

func TestDeviceHeadroomBlocksAssignment(t *testing.T) {
	f := fixture(t)
	_, err := f.s.Push(context.Background(), f.p, Push{Storage: &StorageReport{TotalBytes: 100, AvailableBytes: 30, MinimumFreeBytes: 20}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.s.Create(context.Background(), f.p, CreateRequest{LogicalType: "MOVIE", LogicalID: "m", ProfileID: "ORIGINAL"})
	if err != ErrStorage {
		t.Fatalf("got %v, want storage policy rejection", err)
	}
}

func TestServerCacheQuotaBlocksGeneratedAsset(t *testing.T) {
	f := fixture(t)
	if _, err := f.s.SetSettings(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	_, err := f.s.Create(context.Background(), f.p, CreateRequest{LogicalType: "MOVIE", LogicalID: "m", ProfileID: "LOW"})
	if err != ErrStorage {
		t.Fatalf("got %v, want cache quota rejection", err)
	}
}

func TestGeneratedOfflineAssetUsesOwnedAtomicCache(t *testing.T) {
	f := fixture(t)
	fake := filepath.Join(t.TempDir(), "ffmpeg")
	script := "#!/bin/sh\nin=''\nprev=''\nfor arg in \"$@\"; do\n  if [ \"$prev\" = '-i' ]; then in=\"$arg\"; fi\n  prev=\"$arg\"\n  out=\"$arg\"\ndone\ncp \"$in\" \"$out\"\n"
	if err := os.WriteFile(fake, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	f.s.ffmpeg = fake
	x, err := f.s.Create(context.Background(), f.p, CreateRequest{LogicalType: "MOVIE", LogicalID: "m", ProfileID: "LOW"})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		x, err = f.s.Get(context.Background(), f.p, x.ID)
		if err == nil && x.AssetState == "READY" && x.Status == "READY" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if x.AssetState != "READY" || x.Status != "READY" || x.Mode != "GENERATED_OFFLINE_VERSION" || x.ChecksumSHA256 == "" {
		t.Fatalf("download=%+v err=%v", x, err)
	}
	_, path, err := f.s.Path(context.Background(), f.p, x.ID)
	if err != nil || filepath.Dir(path) != filepath.Join(f.s.root, "assets") || filepath.Ext(path) != ".mp4" {
		t.Fatalf("owned path=%q err=%v", path, err)
	}
	if _, err = os.Stat(path + ".partial.mp4"); !os.IsNotExist(err) {
		t.Fatalf("partial output remained: %v", err)
	}
}

func TestSourceRevisionMakesAssetStaleWithoutMutation(t *testing.T) {
	f := fixture(t)
	x, err := f.s.Create(context.Background(), f.p, CreateRequest{LogicalType: "MOVIE", LogicalID: "m", ProfileID: "ORIGINAL"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.db.Exec("UPDATE media_files SET modified_at_ns=modified_at_ns+1 WHERE id='f'"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = f.s.Path(context.Background(), f.p, x.ID); err != ErrNotReady {
		t.Fatalf("stale path=%v", err)
	}
	y, err := f.s.Create(context.Background(), f.p, CreateRequest{LogicalType: "MOVIE", LogicalID: "m", ProfileID: "ORIGINAL"})
	if err != nil || y.AssetID == x.AssetID {
		t.Fatalf("old=%s new=%s err=%v", x.AssetID, y.AssetID, err)
	}
}

func TestMetadataOnlySyncPreservesMediaAssetRevision(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	x, err := f.s.Create(ctx, f.p, CreateRequest{LogicalType: "MOVIE", LogicalID: "m", ProfileID: "ORIGINAL"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := f.s.Manifest(ctx, f.p, x.ID)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := f.s.Pull(ctx, f.p, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.db.Exec("UPDATE movies SET title='Renamed Movie',overview='Presentation only',updated_at=? WHERE id='m'", stamp(time.Now().Add(time.Second))); err != nil {
		t.Fatal(err)
	}
	delta, err := f.s.Pull(ctx, f.p, initial.Cursor, 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, change := range delta.Changes {
		if change.Type == "METADATA_UPDATED" && change.EntityID == "m" {
			found = true
		}
	}
	if !found {
		t.Fatalf("metadata delta=%+v", delta.Changes)
	}
	afterDownload, err := f.s.Get(ctx, f.p, x.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, err := f.s.Manifest(ctx, f.p, x.ID)
	if err != nil {
		t.Fatal(err)
	}
	var jobs int
	_ = f.s.db.QueryRow("SELECT COUNT(*) FROM download_jobs").Scan(&jobs)
	if afterDownload.AssetID != x.AssetID || afterDownload.ChecksumSHA256 != x.ChecksumSHA256 || afterDownload.SizeBytes != x.SizeBytes || before.PresentationRevision == after.PresentationRevision || jobs != 0 {
		t.Fatalf("before=%+v after=%+v download=%+v jobs=%d", before, after, afterDownload, jobs)
	}
}

func TestNextUnwatchedSubscriptionRemovesAndRefills(t *testing.T) {
	f := fixture(t)
	now := stamp(time.Now())
	args := make([]any, 21)
	for i := range args {
		args[i] = now
	}
	_, err := f.s.db.Exec(`INSERT INTO shows(id,title,sort_title,metadata_state,created_at,updated_at) VALUES('show','Show','show','MATCHED',?,?);
INSERT INTO seasons(id,show_id,season_number,created_at,updated_at) VALUES('season','show',1,?,?);
INSERT INTO episodes(id,season_id,episode_number,title,created_at,updated_at) VALUES('e1','season',1,'One',?,?),('e2','season',2,'Two',?,?),('e3','season',3,'Three',?,?),('e4','season',4,'Four',?,?),('e5','season',5,'Five',?,?);
INSERT INTO media_associations(id,media_file_id,entity_type,entity_id,association_type,created_at) VALUES('ae1','f','EPISODE','e1','MANUAL',?),('ae2','f','EPISODE','e2','MANUAL',?),('ae3','f','EPISODE','e3','MANUAL',?),('ae4','f','EPISODE','e4','MANUAL',?),('ae5','f','EPISODE','e5','MANUAL',?);
INSERT INTO user_media_progress(user_id,logical_type,logical_id,position_seconds,duration_seconds,watched,last_played_at,updated_at) VALUES('u','EPISODE','e1',0,100,1,?,?)`, args...)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.s.CreateSubscription(context.Background(), f.p, SubscriptionRequest{ShowID: "show", ProfileID: "ORIGINAL", DesiredCount: 3, Enabled: true, RemoveWatched: true})
	if err != nil {
		t.Fatal(err)
	}
	var initial int
	_ = f.s.db.QueryRow("SELECT COUNT(*) FROM device_downloads WHERE logical_id IN ('e2','e3','e4') AND status='READY'").Scan(&initial)
	if initial != 3 {
		t.Fatalf("initial next-N count=%d", initial)
	}
	event := ProgressEvent{EventID: "watched-e2", SequenceEpoch: "install", DeviceSequence: 1, LogicalType: "EPISODE", LogicalID: "e2", Duration: 100, Watched: true, ExplicitAction: "WATCHED", OccurredAt: now}
	if _, err = f.s.Push(context.Background(), f.p, Push{Progress: []ProgressEvent{event}}); err != nil {
		t.Fatal(err)
	}
	var removed, replacement int
	_ = f.s.db.QueryRow("SELECT COUNT(*) FROM device_downloads WHERE logical_id='e2' AND status='REMOVAL_REQUESTED'").Scan(&removed)
	_ = f.s.db.QueryRow("SELECT COUNT(*) FROM device_downloads WHERE logical_id='e5' AND status='READY'").Scan(&replacement)
	if removed != 1 || replacement != 1 {
		t.Fatalf("removed=%d replacement=%d", removed, replacement)
	}
}

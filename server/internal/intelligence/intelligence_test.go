package intelligence

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/vynode/media/server/internal/database"
)

func dbFixture(t *testing.T) (*Service, func()) {
	t.Helper()
	ctx := context.Background()
	store, e := database.Open(ctx, filepath.Join(t.TempDir(), "config"))
	if e != nil {
		t.Fatal(e)
	}
	if e = store.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	exec := func(q string, a ...any) {
		if _, e = store.DB.Exec(q, a...); e != nil {
			t.Fatalf("%v: %s", e, q)
		}
	}
	exec("INSERT INTO libraries(id,name,type,created_at,updated_at) VALUES('l','Movies','MOVIES',?,?)", now, now)
	exec("INSERT INTO library_sources(id,library_id,configured_path,normalized_path,created_at) VALUES('src','l','/media','/media',?)", now)
	for i, id := range []string{"m1", "m2", "m3"} {
		fid := "f" + id
		exec("INSERT INTO media_files(id,source_id,relative_path,file_name,base_name,extension,parent_path,size_bytes,modified_at_ns,availability,probe_status,container_format,duration_seconds,resolution_class,hdr_class,created_at,updated_at) VALUES(?, 'src', ?, ?, ?, '.mkv','',100,1,'AVAILABLE','OK','matroska',100,'1080p','SDR',?,?)", fid, id+".mkv", id+".mkv", id, now, now)
		exec("INSERT INTO media_streams(id,media_file_id,stream_index,stream_type,codec,width,height,is_default) VALUES(?,?,0,'video',?,1920,1080,1)", fid+"v", fid, map[bool]string{true: "hevc", false: "h264"}[i < 3])
		exec("INSERT INTO movies(id,title,sort_title,metadata_state,created_at,updated_at) VALUES(?,?,?,'IDENTIFIED',?,?)", id, id, id, now, now)
		exec("INSERT INTO media_associations(id,media_file_id,entity_type,entity_id,association_type,created_at) VALUES(?,?, 'MOVIE',?,'VERSION',?)", "a"+id, fid, id, now)
	}
	s := New(store.DB, "", filepath.Join(t.TempDir(), "optimized"))
	return s, func() { store.Close() }
}

func TestRecurringIntroConfidenceAndNegative(t *testing.T) {
	common := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	a := append([]string{"x", "y"}, common...)
	b := append([]string{"q", "r", "s"}, common...)
	c := append([]string{"z"}, common...)
	start, end, score, ok := DetectRecurring([][]string{a, b, c})
	if !ok || start != 4 || end != 24 || Classify(score) != ConfidenceHigh {
		t.Fatalf("%v %v %v %v", start, end, score, ok)
	}
	for i, episode := range [][]string{a, b, c} {
		offset, matched := MatchStart(episode, common)
		if !matched || offset != []int{2, 3, 1}[i] {
			t.Fatalf("episode %d offset=%d matched=%v", i, offset, matched)
		}
	}
	if _, _, score, ok = DetectRecurring([][]string{{"a", "b", "c", "d", "e"}, {"f", "g", "h", "i", "j"}, {"k", "l", "m", "n", "o"}}); ok || Classify(score) == ConfidenceHigh {
		t.Fatal("false positive")
	}
}
func TestCreditsConservative(t *testing.T) {
	clear := append([]float64{100, 110, 95}, make([]float64, 14)...)
	start, end, score, ok := DetectCredits(clear, 100)
	if !ok || start < 80 || end != 100 || Classify(score) == ConfidenceLow {
		t.Fatalf("%v %v %v %v", start, end, score, ok)
	}
	if _, _, _, ok = DetectCredits([]float64{100, 5, 100, 4, 100, 3, 100, 2, 100, 1, 100, 0}, 100); ok {
		t.Fatal("intermittent darkness detected as credits")
	}
}
func TestManualPrecedenceAndRejectedSuppression(t *testing.T) {
	s, done := dbFixture(t)
	defer done()
	ctx := context.Background()
	now := stamp(time.Now())
	_, e := s.db.Exec("INSERT INTO media_markers(id,logical_type,logical_id,marker_type,start_seconds,end_seconds,source,active,review_state,created_at,updated_at) VALUES('manual','EPISODE','m1','INTRO',10,30,'MANUAL',1,'ACCEPTED',?,?)", now, now)
	if e != nil {
		t.Fatal(e)
	}
	s.persistCandidate(ctx, "EPISODE", "m1", "INTRO", "AUTOMATIC_AUDIO", 12, 31, .95, "same")
	var active int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM media_markers WHERE logical_id='m1' AND source!='MANUAL' AND active=1").Scan(&active)
	if active != 0 {
		t.Fatal("automatic marker overrode manual")
	}
	s.persistCandidate(ctx, "EPISODE", "m2", "INTRO", "AUTOMATIC_AUDIO", 12, 31, .7, "rejected-source")
	var id string
	_ = s.db.QueryRow("SELECT id FROM media_markers WHERE logical_id='m2'").Scan(&id)
	if e = s.Review(ctx, id, "REJECT", nil, nil); e != nil {
		t.Fatal(e)
	}
	s.persistCandidate(ctx, "EPISODE", "m2", "INTRO", "AUTOMATIC_AUDIO", 12, 31, .7, "rejected-source")
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM media_markers WHERE logical_id='m2'").Scan(&count)
	if count != 1 {
		t.Fatalf("rejected candidate returned: %d", count)
	}
}
func TestDryRunNoSideEffectsAndLoopPrevention(t *testing.T) {
	s, done := dbFixture(t)
	defer done()
	ctx := context.Background()
	r := Rule{Name: "HEVC", Enabled: true, Trigger: "MEDIA_IDENTIFIED", Timezone: "UTC", Conditions: []Condition{{"codec", "EQUALS", "hevc"}}, Actions: []Action{{Type: "RUN_MARKER_ANALYSIS"}}}
	dry, e := s.DryRun(ctx, r)
	if e != nil || len(dry.Matches) != 3 || dry.Actions != 0 {
		t.Fatalf("%+v %v", dry, e)
	}
	r, e = s.SaveRule(ctx, r)
	if e != nil {
		t.Fatal(e)
	}
	first, e := s.Execute(ctx, r.ID, "event-1", 0)
	if e != nil || first.Actions != 3 {
		t.Fatalf("%+v %v", first, e)
	}
	second, e := s.Execute(ctx, r.ID, "event-1", 0)
	if e != nil || second.Actions != 0 {
		t.Fatalf("duplicate executed %+v %v", second, e)
	}
	if _, e = s.Execute(ctx, r.ID, "loop", 4); e != ErrValidation {
		t.Fatalf("depth=%v", e)
	}
}
func TestInjectedScheduleExecutesOnce(t *testing.T) {
	s, done := dbFixture(t)
	defer done()
	ctx := context.Background()
	r, e := s.SaveRule(ctx, Rule{Name: "Nightly", Enabled: true, Trigger: "SCHEDULE", Timezone: "America/New_York", Schedule: &Schedule{Hour: 2, Minute: 30}, Actions: []Action{{Type: "RUN_MARKER_ANALYSIS"}}})
	if e != nil {
		t.Fatal(e)
	}
	now := time.Date(2026, 8, 22, 6, 30, 0, 0, time.UTC)
	if e = s.RunDue(ctx, now); e != nil {
		t.Fatal(e)
	}
	if e = s.RunDue(ctx, now); e != nil {
		t.Fatal(e)
	}
	var n int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM automation_executions WHERE rule_id=?", r.ID).Scan(&n)
	if n != 1 {
		t.Fatalf("executions=%d", n)
	}
}
func TestPlaybackPriorityDefersBackgroundWork(t *testing.T) {
	s, done := dbFixture(t)
	defer done()
	now := stamp(time.Now())
	_, _ = s.db.Exec("INSERT INTO users(id,username,password_hash,role,display_name,status,created_at) VALUES('u','u','x','USER','U','ACTIVE',?)", now)
	_, _ = s.db.Exec("INSERT INTO devices(id,user_id,name,platform,created_at) VALUES('d','u','D','web',?)", now)
	_, _ = s.db.Exec("INSERT INTO sessions(id,user_id,device_id,refresh_token_hash,expires_at,created_at,last_activity_at,token_family_id) VALUES('sess','u','d','x',?,?,?,'f')", stamp(time.Now().Add(time.Hour)), now, now)
	_, _ = s.db.Exec("INSERT INTO client_capabilities(id,user_id,auth_session_id,schema_version,client_name,platform,containers_json,video_codecs_json,audio_codecs_json,subtitle_formats_json,hdr_json,direct_play,created_at,updated_at) VALUES('cap','u','sess',2,'web','web','[]','[]','[]','[]','[]',1,?,?)", now, now)
	_, _ = s.db.Exec("INSERT INTO playback_sessions(id,user_id,auth_session_id,capability_id,logical_type,logical_id,mode,state,started_at,last_activity_at) VALUES('play','u','sess','cap','MOVIE','m1','VIDEO_TRANSCODE','PLAYING',?,?)", now, now)
	started := make(chan struct{})
	_, e := s.enqueue(context.Background(), "TEST", "MOVIE", "m1", 30, nil, func(context.Context, string) error { close(started); return nil })
	if e != nil {
		t.Fatal(e)
	}
	select {
	case <-started:
		t.Fatal("background work started during playback")
	case <-time.After(150 * time.Millisecond):
	}
	_, _ = s.db.Exec("UPDATE playback_sessions SET state='STOPPED' WHERE id='play'")
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("background work did not resume")
	}
}

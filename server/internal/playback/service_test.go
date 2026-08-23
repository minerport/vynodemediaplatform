package playback

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vynode/media/server/internal/database"
)

func fixture(t *testing.T) (*Service, func(), string, string) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	media := filepath.Join(root, "movie.mp4")
	if err := os.WriteFile(media, []byte("0123456789abcdef"), 0600); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(media)
	store, err := database.Open(ctx, filepath.Join(root, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	n := stamp(now)
	exec := func(q string, args ...any) {
		if _, e := store.DB.Exec(q, args...); e != nil {
			t.Fatalf("%s: %v", q, e)
		}
	}
	exec("INSERT INTO users(id,username,password_hash,role,display_name,status,created_at) VALUES('u1','one','x','USER','One','ACTIVE',?)", n)
	exec("INSERT INTO users(id,username,password_hash,role,display_name,status,created_at) VALUES('u2','two','x','USER','Two','ACTIVE',?)", n)
	exec("INSERT INTO devices(id,user_id,name,platform,created_at) VALUES('d1','u1','Web','web',?)", n)
	exec("INSERT INTO devices(id,user_id,name,platform,created_at) VALUES('d2','u2','Web','web',?)", n)
	exec("INSERT INTO sessions(id,user_id,device_id,refresh_token_hash,expires_at,created_at,last_activity_at,token_family_id) VALUES('s1','u1','d1','x',?,?,?,'f')", stamp(now.Add(time.Hour)), n, n)
	exec("INSERT INTO sessions(id,user_id,device_id,refresh_token_hash,expires_at,created_at,last_activity_at,token_family_id) VALUES('s2','u2','d2','y',?,?,?,'g')", stamp(now.Add(time.Hour)), n, n)
	exec("INSERT INTO libraries(id,name,type,created_at,updated_at) VALUES('l','Movies','MOVIES',?,?)", n, n)
	exec("INSERT INTO library_access_grants(user_id,library_id,permission,granted_by,created_at) VALUES('u1','l','VIEW','u1',?),('u1','l','PLAY','u1',?),('u2','l','VIEW','u1',?),('u2','l','PLAY','u1',?)", n, n, n, n)
	exec("INSERT INTO library_sources(id,library_id,configured_path,normalized_path,created_at) VALUES('src','l',?,?,?)", root, root, n)
	exec("INSERT INTO media_files(id,source_id,relative_path,file_name,base_name,extension,parent_path,size_bytes,modified_at_ns,availability,probe_status,container_format,duration_seconds,bitrate,resolution_class,hdr_class,created_at,updated_at) VALUES('f','src','movie.mp4','movie.mp4','movie','.mp4','',?,?, 'AVAILABLE','OK','mov,mp4',100,1000,'1080p','SDR',?,?)", st.Size(), st.ModTime().UnixNano(), n, n)
	exec("INSERT INTO media_streams(id,media_file_id,stream_index,stream_type,codec,width,height,is_default,is_forced,hearing_impaired,commentary) VALUES('v','f',0,'video','h264',1920,1080,1,0,0,0)")
	exec("INSERT INTO media_streams(id,media_file_id,stream_index,stream_type,codec,is_default,is_forced,hearing_impaired,commentary) VALUES('a','f',1,'audio','aac',1,0,0,0)")
	exec("INSERT INTO movies(id,title,sort_title,metadata_state,created_at,updated_at) VALUES('m','Movie','movie','IDENTIFIED',?,?)", n, n)
	exec("INSERT INTO media_associations(id,media_file_id,entity_type,entity_id,association_type,created_at) VALUES('mv','f','MOVIE','m','VERSION',?)", n)
	s := &Service{db: store.DB, now: func() time.Time { return now }, inactivity: 45 * time.Minute}
	return s, func() { store.Close() }, media, n
}
func TestMediaAuthorizationStaleAndRevoked(t *testing.T) {
	s, done, path, _ := fixture(t)
	defer done()
	ctx := context.Background()
	x, err := s.Start(ctx, "u1", "s1", StartRequest{LogicalType: "MOVIE", LogicalID: "m", Capabilities: browser()})
	if err != nil {
		t.Fatal(err)
	}
	token := strings.SplitN(x.MediaURL, "?token=", 2)[1]
	a, err := s.AuthorizeMedia(ctx, x.ID, token)
	if err != nil || a.Size != 16 || a.MIME != "video/mp4" {
		t.Fatalf("access=%+v err=%v", a, err)
	}
	if _, err = s.AuthorizeMedia(ctx, x.ID, "wrong"); err != ErrForbidden {
		t.Fatalf("wrong token=%v", err)
	}
	if owner, ownerErr := s.AuthorizeMediaForOwner(ctx, x.ID); ownerErr != nil || owner.Size != 16 {
		t.Fatalf("owner bearer access=%+v err=%v", owner, ownerErr)
	}
	if err = os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AuthorizeMedia(ctx, x.ID, token); err != ErrStale {
		t.Fatalf("changed file=%v", err)
	}
}
func TestSessionResumeMultiUserAndAuthorization(t *testing.T) {
	s, done, _, _ := fixture(t)
	defer done()
	ctx := context.Background()
	request := StartRequest{LogicalType: "MOVIE", LogicalID: "m", Resume: true, Capabilities: browser()}
	one, err := s.Start(ctx, "u1", "s1", request)
	if err != nil || one.Decision.Mode != DirectPlay || one.MediaURL == "" {
		t.Fatalf("start: %+v %v", one, err)
	}
	if err = s.Update(ctx, "u1", one.ID, Progress{State: Stopped, Position: 50, Duration: 100}); err != nil {
		t.Fatal(err)
	}
	two, err := s.Start(ctx, "u1", "s1", request)
	if err != nil || two.ResumePosition != 50 {
		t.Fatalf("resume=%v err=%v", two.ResumePosition, err)
	}
	other, err := s.Start(ctx, "u2", "s2", request)
	if err != nil || other.ResumePosition != 0 {
		t.Fatalf("other resume=%v err=%v", other.ResumePosition, err)
	}
	if err = s.Update(ctx, "u2", two.ID, Progress{State: Paused, Position: 60, Duration: 100}); err != ErrForbidden {
		t.Fatalf("cross-user err=%v", err)
	}
}
func TestWatchedNearRulesAndActive(t *testing.T) {
	s, done, _, _ := fixture(t)
	defer done()
	ctx := context.Background()
	r := StartRequest{LogicalType: "MOVIE", LogicalID: "m", Resume: true, Capabilities: browser()}
	x, _ := s.Start(ctx, "u1", "s1", r)
	if err := s.Update(ctx, "u1", x.ID, Progress{State: Playing, Position: 91, Duration: 100}); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Progress(ctx, "u1", "MOVIE", "m")
	if !p.Watched || p.Position != 91 {
		t.Fatalf("progress=%+v", p)
	}
	if _, err := s.AuthorizeMediaForOwner(ctx, x.ID); err != nil {
		t.Fatalf("near-end inference ended active playback: %v", err)
	}
	if err := s.Update(ctx, "u1", x.ID, Progress{State: Playing, Position: 95, Duration: 100}); err != nil {
		t.Fatalf("seek after near-end inference failed: %v", err)
	}
	if _, err := s.AuthorizeMediaForOwner(ctx, x.ID); err != nil {
		t.Fatalf("media unavailable after seek from near-end: %v", err)
	}
	if err := s.Stop(ctx, "u1", x.ID); err != nil {
		t.Fatal(err)
	}
	next, _ := s.Start(ctx, "u1", "s1", r)
	if next.ResumePosition != 0 {
		t.Fatal(next.ResumePosition)
	}
	items, err := s.Active(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("active=%d err=%v", len(items), err)
	}
	if err = s.AdminStop(ctx, next.ID); err != nil {
		t.Fatal(err)
	}
	items, _ = s.Active(ctx)
	if len(items) != 0 {
		t.Fatal("stopped session remained active")
	}
}

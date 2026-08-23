package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vynode/media/server/internal/auth"
	"github.com/vynode/media/server/internal/database"
	"github.com/vynode/media/server/internal/offline"
	"github.com/vynode/media/server/internal/sharing"
)

func TestAuthenticatedOfflineRangeReconstruction(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := database.Open(ctx, filepath.Join(root, "config"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	mediaRoot := filepath.Join(root, "media")
	if err = os.MkdirAll(mediaRoot, 0700); err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte("vynode-range-reconstruction-"), 25000)
	source := filepath.Join(mediaRoot, "range.mp4")
	if err = os.WriteFile(source, payload, 0600); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(source)
	queries := []struct {
		q    string
		args []any
	}{
		{"INSERT INTO users(id,username,password_hash,role,display_name,status,created_at) VALUES('u','user','x','USER','User','ACTIVE',?),('other','other','x','USER','Other','ACTIVE',?)", []any{now, now}},
		{"INSERT INTO libraries(id,name,type,created_at,updated_at) VALUES('lib','Movies','MOVIES',?,?)", []any{now, now}},
		{"INSERT INTO library_sources(id,library_id,configured_path,normalized_path,created_at) VALUES('src','lib',?,?,?)", []any{mediaRoot, mediaRoot, now}},
		{"INSERT INTO media_files(id,source_id,relative_path,file_name,base_name,extension,parent_path,size_bytes,modified_at_ns,availability,probe_status,container_format,duration_seconds,created_at,updated_at) VALUES('file','src','range.mp4','range.mp4','range','.mp4','.',?,?,'AVAILABLE','OK','mp4',30,?,?)", []any{st.Size(), st.ModTime().UnixNano(), now, now}},
		{"INSERT INTO media_streams(id,media_file_id,stream_index,stream_type,codec,width,height,is_default) VALUES('video','file',0,'video','h264',1280,720,1)", nil},
		{"INSERT INTO media_streams(id,media_file_id,stream_index,stream_type,codec,channels,is_default) VALUES('audio','file',1,'audio','aac',2,1)", nil},
		{"INSERT INTO movies(id,title,sort_title,metadata_state,created_at,updated_at) VALUES('movie','Range Movie','range movie','MATCHED',?,?)", []any{now, now}},
		{"INSERT INTO media_associations(id,media_file_id,entity_type,entity_id,association_type,created_at) VALUES('assoc','file','MOVIE','movie','MANUAL',?)", []any{now}},
		{"INSERT INTO library_access_grants(user_id,library_id,permission,granted_by,created_at) VALUES('u','lib','DOWNLOAD','u',?)", []any{now}},
	}
	for _, q := range queries {
		if _, err = store.DB.Exec(q.q, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	authService, err := auth.New(store.DB, filepath.Join(root, "config"), "test-instance", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	issue := func(user auth.User, name string) (auth.Tokens, auth.Principal) {
		tok, issueErr := authService.IssueSession(ctx, user, auth.DeviceInput{Name: name, ClientName: "native-test", Platform: "android"}, "127.0.0.1", "test")
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		sid := strings.SplitN(tok.RefreshToken, ".", 2)[0]
		if _, issueErr = store.DB.Exec("UPDATE devices SET authorization_type='PAIRED' WHERE id=(SELECT device_id FROM sessions WHERE id=?)", sid); issueErr != nil {
			t.Fatal(issueErr)
		}
		principal, issueErr := authService.Authenticate(tok.AccessToken)
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		return tok, principal
	}
	user := auth.User{ID: "u", Username: "user", DisplayName: "User", Role: auth.RoleUser, Status: "ACTIVE", CreatedAt: now}
	token, principal := issue(user, "Device A")
	wrongDevice, _ := issue(user, "Device B")
	otherToken, _ := issue(auth.User{ID: "other", Username: "other", DisplayName: "Other", Role: auth.RoleUser, Status: "ACTIVE", CreatedAt: now}, "Other device")
	offlineService, err := offline.New(store.DB, filepath.Join(root, "downloads"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(offlineService.Close)
	download, err := offlineService.Create(ctx, principal, offline.CreateRequest{LogicalType: "MOVIE", LogicalID: "movie", ProfileID: "ORIGINAL"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), store, SystemInfo{StartedAt: time.Now()}, authService, nil, nil, nil, "", offlineService)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	request := func(access, byteRange string) *http.Response {
		req, requestErr := http.NewRequest(http.MethodGet, server.URL+"/api/v1/downloads/"+download.ID+"/file", nil)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		if access != "" {
			req.Header.Set("Authorization", "Bearer "+access)
		}
		if byteRange != "" {
			req.Header.Set("Range", byteRange)
		}
		resp, requestErr := http.DefaultClient.Do(req)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		return resp
	}
	for label, access := range map[string]string{"unauthenticated": "", "wrong-device": wrongDevice.AccessToken, "wrong-user": otherToken.AccessToken} {
		resp := request(access, "bytes=0-9")
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s status=%d", label, resp.StatusCode)
		}
	}
	first := request(token.AccessToken, "bytes=0-199999")
	firstBytes, _ := io.ReadAll(first.Body)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusPartialContent || first.Header.Get("Content-Range") != "bytes 0-199999/"+stringInt(int64(len(payload))) {
		t.Fatalf("first status=%d range=%q", first.StatusCode, first.Header.Get("Content-Range"))
	}
	second := request(token.AccessToken, "bytes=200000-")
	secondBytes, _ := io.ReadAll(second.Body)
	_ = second.Body.Close()
	wantRange := "bytes 200000-" + stringInt(int64(len(payload)-1)) + "/" + stringInt(int64(len(payload)))
	if second.StatusCode != http.StatusPartialContent || second.Header.Get("Content-Range") != wantRange {
		t.Fatalf("resume status=%d range=%q want=%q", second.StatusCode, second.Header.Get("Content-Range"), wantRange)
	}
	rebuilt := append(firstBytes, secondBytes...)
	want, got := sha256.Sum256(payload), sha256.Sum256(rebuilt)
	if len(rebuilt) != len(payload) || want != got || download.ChecksumSHA256 == "" {
		t.Fatalf("size=%d/%d digestMatch=%v", len(rebuilt), len(payload), want == got)
	}
	if err = authService.Revoke(ctx, principal, principal.SessionID, "test", "DEVICE_REVOKED"); err != nil {
		t.Fatal(err)
	}
	revoked := request(token.AccessToken, "bytes=200000-")
	_ = revoked.Body.Close()
	if revoked.StatusCode != http.StatusUnauthorized && revoked.StatusCode != http.StatusForbidden {
		t.Fatalf("revoked device status=%d", revoked.StatusCode)
	}
	grantToken, grantPrincipal := issue(user, "Grant device")
	grantDownload, err := offlineService.Create(ctx, grantPrincipal, offline.CreateRequest{LogicalType: "MOVIE", LogicalID: "movie", ProfileID: "ORIGINAL"})
	if err != nil {
		t.Fatal(err)
	}
	download = grantDownload
	sharingService := sharing.New(store.DB, authService, "test-instance", "Test", "test")
	if err = sharingService.SetGrants(ctx, "u", "u", nil); err != nil {
		t.Fatal(err)
	}
	denied := request(grantToken.AccessToken, "bytes=0-9")
	_ = denied.Body.Close()
	if denied.StatusCode != http.StatusForbidden {
		t.Fatalf("grant-revoked status=%d", denied.StatusCode)
	}
	var revokedAssignments, removalChanges int
	_ = store.DB.QueryRow("SELECT COUNT(*) FROM device_downloads WHERE id=? AND status='REVOKED'", grantDownload.ID).Scan(&revokedAssignments)
	_ = store.DB.QueryRow("SELECT COUNT(*) FROM sync_changes WHERE entity_id=? AND change_type='DOWNLOAD_REVOKED'", grantDownload.ID).Scan(&removalChanges)
	if revokedAssignments != 1 || removalChanges != 1 {
		t.Fatalf("assignment=%d changes=%d", revokedAssignments, removalChanges)
	}
}

func stringInt(v int64) string { return fmt.Sprintf("%d", v) }

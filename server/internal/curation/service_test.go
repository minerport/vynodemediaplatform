package curation

import (
	"context"
	"fmt"
	"github.com/vynode/media/server/internal/database"
	"path/filepath"
	"testing"
	"time"
)

func fixture(t *testing.T) (*Service, func()) {
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
	exec("INSERT INTO users(id,username,password_hash,role,display_name,status,created_at) VALUES('u1','one','x','USER','One','ACTIVE',?),('u2','two','x','USER','Two','ACTIVE',?)", now, now)
	for _, m := range []struct {
		id, title string
		year      int
		rating    float64
	}{{"m1", "Alpha", 2001, 8.5}, {"m2", "Beta", 1999, 7}, {"m3", "Gamma", 2024, 9.1}} {
		exec("INSERT INTO movies(id,title,sort_title,year,rating_value,metadata_state,orphaned,created_at,updated_at) VALUES(?,?,?,?,?,'IDENTIFIED',0,?,?)", m.id, m.title, m.title, m.year, m.rating, now, now)
	}
	exec("INSERT INTO shows(id,title,sort_title,year,metadata_state,orphaned,created_at,updated_at) VALUES('s1','Show','Show',2020,'IDENTIFIED',0,?,?)", now, now)
	exec("INSERT INTO seasons(id,show_id,season_number,created_at,updated_at) VALUES('se','s1',1,?,?)", now, now)
	exec("INSERT INTO episodes(id,season_id,episode_number,title,created_at,updated_at) VALUES('e1','se',1,'Pilot',?,?),('e2','se',2,'Second',?,?)", now, now, now, now)
	return New(store.DB), func() { store.Close() }
}

func TestCollectionLifecycleDeleteSafetyAndNoDuplicates(t *testing.T) {
	s, done := fixture(t)
	defer done()
	ctx := context.Background()
	c, e := s.SaveCollection(ctx, "u1", true, Collection{Name: "Mixed", Scope: "SERVER_SHARED", Ordering: "CUSTOM"})
	if e != nil {
		t.Fatal(e)
	}
	items := []Item{{Type: "MOVIE", ID: "m1"}, {Type: "SHOW", ID: "s1"}, {Type: "MOVIE", ID: "m1"}}
	if e = s.AddCollectionItems(ctx, "u1", true, c.ID, items); e != nil {
		t.Fatal(e)
	}
	c, e = s.Collection(ctx, "u2", c.ID)
	if e != nil || len(c.Items) != 2 {
		t.Fatalf("%+v %v", c, e)
	}
	if e = s.ReorderCollection(ctx, "u1", true, c.ID, []string{"SHOW:s1", "MOVIE:m1"}); e != nil {
		t.Fatal(e)
	}
	c, _ = s.Collection(ctx, "u1", c.ID)
	if c.Items[0].ID != "s1" {
		t.Fatal("custom order not persisted")
	}
	if e = s.DeleteCollection(ctx, "u1", true, c.ID); e != nil {
		t.Fatal(e)
	}
	var movies, shows int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM movies").Scan(&movies)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM shows").Scan(&shows)
	if movies != 3 || shows != 1 {
		t.Fatal("logical media changed when collection deleted")
	}
}
func TestPersonalIsolationAndWatchlistDoesNotTouchProgress(t *testing.T) {
	s, done := fixture(t)
	defer done()
	ctx := context.Background()
	_, _ = s.db.Exec("INSERT INTO user_media_progress(user_id,logical_type,logical_id,position_seconds,duration_seconds,watched,last_played_at,updated_at) VALUES('u1','MOVIE','m1',50,100,0,'x','x')")
	if e := s.TogglePersonal(ctx, "u1", "WATCHLIST", "MOVIE", "m1", true); e != nil {
		t.Fatal(e)
	}
	a, _ := s.Personal(ctx, "u1", "WATCHLIST", 10)
	b, _ := s.Personal(ctx, "u2", "WATCHLIST", 10)
	if len(a) != 1 || len(b) != 0 {
		t.Fatal("watchlist privacy failure")
	}
	if e := s.TogglePersonal(ctx, "u1", "FAVORITE", "MOVIE", "m1", true); e != nil {
		t.Fatal(e)
	}
	f, _ := s.Personal(ctx, "u2", "FAVORITE", 10)
	if len(f) != 0 {
		t.Fatal("favorite privacy failure")
	}
	var pos float64
	_ = s.db.QueryRow("SELECT position_seconds FROM user_media_progress WHERE user_id='u1' AND logical_id='m1'").Scan(&pos)
	if pos != 50 {
		t.Fatal("curation changed progress")
	}
}
func TestPlaylistMixedOrderingDuplicatesAndNavigation(t *testing.T) {
	s, done := fixture(t)
	defer done()
	ctx := context.Background()
	p, e := s.SavePlaylist(ctx, "u1", Playlist{Name: "Road trip"})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := s.AddPlaylistItem(ctx, "u1", p.ID, "MOVIE", "m1")
	b, _ := s.AddPlaylistItem(ctx, "u1", p.ID, "EPISODE", "e1")
	c, _ := s.AddPlaylistItem(ctx, "u1", p.ID, "MOVIE", "m1")
	p, _ = s.Playlist(ctx, "u1", p.ID)
	if len(p.Items) != 3 {
		t.Fatal("playlist duplicates were collapsed")
	}
	prev, next, e := s.PlaylistNavigation(ctx, "u1", p.ID, b.ArtworkID)
	if e != nil || prev.ArtworkID != a.ArtworkID || next.ArtworkID != c.ArtworkID {
		t.Fatalf("%+v %+v %v", prev, next, e)
	}
	if _, e = s.Playlist(ctx, "u2", p.ID); e != ErrNotFound {
		t.Fatal("playlist leaked")
	}
}
func TestSmartNestedDynamicAndSQLSafety(t *testing.T) {
	s, done := fixture(t)
	defer done()
	ctx := context.Background()
	x := SmartCollection{Name: "Modern favorites", Scope: "USER_PRIVATE", RuleSchemaVersion: 1, SortField: "rating", SortDirection: "DESC", Limit: 25, Rule: RuleNode{Logic: "ALL", Children: []RuleNode{{Field: "year", Operator: "GTE", Value: 2000}, {Logic: "ANY", Children: []RuleNode{{Field: "title", Operator: "CONTAINS", Value: "a"}, {Field: "rating", Operator: "GT", Value: 9}}}}}}
	items, e := s.PreviewSmart(ctx, "u1", x)
	if e != nil || len(items) != 2 {
		t.Fatalf("%+v %v", items, e)
	}
	saved, e := s.SaveSmart(ctx, "u1", false, x)
	if e != nil {
		t.Fatal(e)
	}
	_, _ = s.db.Exec("UPDATE movies SET year=1980 WHERE id='m1'")
	saved, e = s.Smart(ctx, "u1", saved.ID)
	if e != nil || len(saved.Items) != 1 || saved.Items[0].ID != "m3" {
		t.Fatalf("dynamic=%+v %v", saved.Items, e)
	}
	x.Rule = RuleNode{Field: "title; DROP TABLE movies", Operator: "EQUALS", Value: "x"}
	if _, e = s.PreviewSmart(ctx, "u1", x); e != ErrValidation {
		t.Fatalf("unsafe field accepted: %v", e)
	}
	var n int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM movies").Scan(&n)
	if n != 3 {
		t.Fatal("movies table changed")
	}
}
func TestHomeLayoutsIndependentOrderedAndSources(t *testing.T) {
	s, done := fixture(t)
	defer done()
	ctx := context.Background()
	a, _ := s.HomeRows(ctx, "u1")
	b, _ := s.HomeRows(ctx, "u2")
	if len(a) != 3 || len(b) != 3 {
		t.Fatal("defaults missing")
	}
	a[0].Enabled = false
	if _, e := s.SaveHomeRow(ctx, "u1", a[0]); e != nil {
		t.Fatal(e)
	}
	if e := s.ReorderHome(ctx, "u1", []string{a[2].ID, a[1].ID, a[0].ID}); e != nil {
		t.Fatal(e)
	}
	again, _ := New(s.db).HomeRows(ctx, "u1")
	other, _ := New(s.db).HomeRows(ctx, "u2")
	if again[0].ID != a[2].ID || !other[0].Enabled {
		t.Fatal("home order/privacy not persisted")
	}
	c, _ := s.SaveCollection(ctx, "u1", true, Collection{Name: "Featured", Scope: "SERVER_SHARED", Ordering: "CUSTOM"})
	_ = s.AddCollectionItems(ctx, "u1", true, c.ID, []Item{{Type: "MOVIE", ID: "m1"}})
	_, e := s.SaveHomeRow(ctx, "u1", HomeRow{Type: "COLLECTION", Title: "Featured", SourceID: c.ID, Enabled: true, Limit: 10})
	if e != nil {
		t.Fatal(e)
	}
	home, e := s.Home(ctx, "u1")
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, r := range home.Rows {
		if r.Type == "COLLECTION" && len(r.Items) == 1 && r.SeeAll == "/collections/"+c.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("collection row did not resolve")
	}
}
func TestAutomationMembershipIdempotentSharedOnly(t *testing.T) {
	s, done := fixture(t)
	defer done()
	ctx := context.Background()
	c, _ := s.SaveCollection(ctx, "u1", true, Collection{Name: "4K", Scope: "SERVER_SHARED", Ordering: "CUSTOM"})
	for i := 0; i < 2; i++ {
		if e := s.AutomationMembership(ctx, c.ID, "ADD_TO_COLLECTION", "MOVIE", "m1"); e != nil {
			t.Fatal(e)
		}
	}
	var n int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM collection_items WHERE collection_id=?", c.ID).Scan(&n)
	if n != 1 {
		t.Fatalf("duplicates=%d", n)
	}
	private, _ := s.SaveCollection(ctx, "u1", false, Collection{Name: "Mine", Scope: "USER_PRIVATE", Ordering: "CUSTOM"})
	if e := s.AutomationMembership(ctx, private.ID, "ADD_TO_COLLECTION", "MOVIE", "m1"); e != ErrValidation {
		t.Fatal("automation modified private collection")
	}
}

func TestSmartQueryTenThousandLogicalRows(t *testing.T) {
	s, done := fixture(t)
	defer done()
	ctx := context.Background()
	tx, e := s.db.Begin()
	if e != nil {
		t.Fatal(e)
	}
	now := stamp(time.Now())
	stmt, e := tx.Prepare("INSERT INTO movies(id,title,sort_title,year,rating_value,metadata_state,orphaned,created_at,updated_at) VALUES(?,?,?,?,?,'IDENTIFIED',0,?,?)")
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 10000; i++ {
		if _, e = stmt.Exec(fmt.Sprintf("bulk-%d", i), fmt.Sprintf("Movie %05d", i), fmt.Sprintf("Movie %05d", i), 1980+i%50, float64(i%100)/10, now, now); e != nil {
			t.Fatal(e)
		}
	}
	stmt.Close()
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	start := time.Now()
	items, e := s.PreviewSmart(ctx, "u1", SmartCollection{RuleSchemaVersion: 1, SortField: "year", SortDirection: "DESC", Limit: 100, Rule: RuleNode{Field: "year", Operator: "GTE", Value: 2020}})
	if e != nil || len(items) != 100 {
		t.Fatalf("items=%d err=%v", len(items), e)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("10k query too slow: %v", time.Since(start))
	}
}

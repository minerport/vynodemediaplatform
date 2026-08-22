package curation

import (
	"context"
	"database/sql"
	"strings"
)

type rowResolver interface {
	Resolve(context.Context, string, HomeRow) ([]Item, string, error)
}
type resolverFunc func(context.Context, string, HomeRow) ([]Item, string, error)

func (f resolverFunc) Resolve(c context.Context, u string, r HomeRow) ([]Item, string, error) {
	return f(c, u, r)
}
func (s *Service) resolvers() map[string]rowResolver {
	return map[string]rowResolver{
		"CONTINUE_WATCHING": resolverFunc(s.resolveContinue), "RECENTLY_ADDED_MOVIES": resolverFunc(s.resolveRecent), "RECENTLY_ADDED_SHOWS": resolverFunc(s.resolveRecent), "WATCHLIST": resolverFunc(s.resolvePersonal), "FAVORITES": resolverFunc(s.resolvePersonal), "COLLECTION": resolverFunc(s.resolveCollection), "SMART_COLLECTION": resolverFunc(s.resolveSmart), "PLAYLIST": resolverFunc(s.resolvePlaylist),
	}
}
func (s *Service) ensureLayout(ctx context.Context, user string) error {
	now := stamp(s.now())
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	r, e := tx.ExecContext(ctx, "INSERT OR IGNORE INTO home_layouts(user_id,created_at,updated_at) VALUES(?,?,?)", user, now, now)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 1 {
		defaults := []struct {
			t, title string
			limit    int
		}{{"CONTINUE_WATCHING", "Continue Watching", 20}, {"RECENTLY_ADDED_MOVIES", "Recently Added Movies", 20}, {"RECENTLY_ADDED_SHOWS", "Recently Added Shows", 20}}
		for p, x := range defaults {
			if _, e = tx.ExecContext(ctx, "INSERT INTO home_rows(id,user_id,row_type,title,enabled,position,item_limit,created_at,updated_at) VALUES(?,?,?,?,1,?,?,?,?)", id(), user, x.t, x.title, p, x.limit, now, now); e != nil {
				return e
			}
		}
	}
	return tx.Commit()
}
func (s *Service) HomeRows(ctx context.Context, user string) ([]HomeRow, error) {
	if e := s.ensureLayout(ctx, user); e != nil {
		return nil, e
	}
	rows, e := s.db.QueryContext(ctx, "SELECT id,row_type,title,COALESCE(source_id,''),enabled,position,item_limit FROM home_rows WHERE user_id=? ORDER BY position", user)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []HomeRow{}
	for rows.Next() {
		var x HomeRow
		if e = rows.Scan(&x.ID, &x.Type, &x.Title, &x.SourceID, &x.Enabled, &x.Position, &x.Limit); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) Home(ctx context.Context, user string) (Home, error) {
	rows, e := s.HomeRows(ctx, user)
	if e != nil {
		return Home{}, e
	}
	resolvers := s.resolvers()
	out := Home{Rows: []HomeRow{}}
	for _, r := range rows {
		if !r.Enabled {
			continue
		}
		resolver := resolvers[r.Type]
		if resolver == nil {
			continue
		}
		r.Items, r.SeeAll, e = resolver.Resolve(ctx, user, r)
		if e == ErrNotFound {
			_, _ = s.db.ExecContext(ctx, "DELETE FROM home_rows WHERE id=? AND user_id=?", r.ID, user)
			continue
		}
		if e != nil {
			return out, e
		}
		if len(r.Items) > 0 {
			out.Rows = append(out.Rows, r)
		}
	}
	return out, nil
}
func (s *Service) SaveHomeRow(ctx context.Context, user string, r HomeRow) (HomeRow, error) {
	if _, ok := s.resolvers()[r.Type]; !ok || strings.TrimSpace(r.Title) == "" || r.Limit < 1 || r.Limit > 50 {
		return r, ErrValidation
	}
	requires := validType(r.Type, "COLLECTION", "SMART_COLLECTION", "PLAYLIST")
	if requires != (r.SourceID != "") {
		return r, ErrValidation
	}
	if r.Type == "COLLECTION" {
		if _, e := s.Collection(ctx, user, r.SourceID); e != nil {
			return r, e
		}
	}
	if r.Type == "SMART_COLLECTION" {
		if _, e := s.Smart(ctx, user, r.SourceID); e != nil {
			return r, e
		}
	}
	if r.Type == "PLAYLIST" {
		if _, e := s.Playlist(ctx, user, r.SourceID); e != nil {
			return r, e
		}
	}
	if e := s.ensureLayout(ctx, user); e != nil {
		return r, e
	}
	now := stamp(s.now())
	if r.ID == "" {
		var count int
		_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM home_rows WHERE user_id=?", user).Scan(&count)
		if count >= 50 {
			return r, ErrValidation
		}
		r.ID = id()
		r.Position = count
	}
	_, e := s.db.ExecContext(ctx, `INSERT INTO home_rows(id,user_id,row_type,title,source_id,enabled,position,item_limit,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET title=excluded.title,enabled=excluded.enabled,item_limit=excluded.item_limit,updated_at=excluded.updated_at WHERE home_rows.user_id=?`, r.ID, user, r.Type, r.Title, null(r.SourceID), r.Enabled, r.Position, r.Limit, now, now, user)
	return r, e
}
func null(x string) any {
	if x == "" {
		return nil
	}
	return x
}
func (s *Service) DeleteHomeRow(ctx context.Context, user, rid string) error {
	r, e := s.db.ExecContext(ctx, "DELETE FROM home_rows WHERE id=? AND user_id=?", rid, user)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return s.compact(ctx, "home_rows", "user_id", user)
}
func (s *Service) ReorderHome(ctx context.Context, user string, ids []string) error {
	return s.reorder(ctx, "home_rows", "user_id", user, ids)
}

func (s *Service) resolveRecent(ctx context.Context, user string, r HomeRow) ([]Item, string, error) {
	_ = user
	table, typ, route := "movies", "MOVIE", "/movies"
	if r.Type == "RECENTLY_ADDED_SHOWS" {
		table, typ, route = "shows", "SHOW", "/shows"
	}
	rows, e := s.db.QueryContext(ctx, "SELECT id,title,COALESCE(year,0) FROM "+table+" WHERE orphaned=0 ORDER BY created_at DESC LIMIT ?", r.Limit)
	if e != nil {
		return nil, "", e
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var x Item
		x.Type = typ
		if e = rows.Scan(&x.ID, &x.Title, &x.Year); e != nil {
			return nil, "", e
		}
		out = append(out, x)
	}
	return out, route, rows.Err()
}
func (s *Service) resolvePersonal(ctx context.Context, user string, r HomeRow) ([]Item, string, error) {
	kind, route := "WATCHLIST", "/watchlist"
	if r.Type == "FAVORITES" {
		kind, route = "FAVORITE", "/favorites"
	}
	x, e := s.Personal(ctx, user, kind, r.Limit)
	return x, route, e
}
func (s *Service) resolveCollection(ctx context.Context, user string, r HomeRow) ([]Item, string, error) {
	x, e := s.Collection(ctx, user, r.SourceID)
	if e != nil {
		return nil, "", e
	}
	if len(x.Items) > r.Limit {
		x.Items = x.Items[:r.Limit]
	}
	return x.Items, "/collections/" + x.ID, nil
}
func (s *Service) resolveSmart(ctx context.Context, user string, r HomeRow) ([]Item, string, error) {
	x, e := s.Smart(ctx, user, r.SourceID)
	if e != nil {
		return nil, "", e
	}
	if len(x.Items) > r.Limit {
		x.Items = x.Items[:r.Limit]
	}
	return x.Items, "/collections/" + x.ID, nil
}
func (s *Service) resolvePlaylist(ctx context.Context, user string, r HomeRow) ([]Item, string, error) {
	x, e := s.Playlist(ctx, user, r.SourceID)
	if e != nil {
		return nil, "", e
	}
	if len(x.Items) > r.Limit {
		x.Items = x.Items[:r.Limit]
	}
	return x.Items, "/playlists/" + x.ID, nil
}
func (s *Service) resolveContinue(ctx context.Context, user string, r HomeRow) ([]Item, string, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT p.logical_type,p.logical_id,CASE p.logical_type WHEN 'MOVIE' THEN COALESCE(m.title,'') ELSE COALESCE(ep.title,'') END,CAST(100*p.position_seconds/NULLIF(p.duration_seconds,0) AS INTEGER) FROM user_media_progress p LEFT JOIN movies m ON p.logical_type='MOVIE' AND m.id=p.logical_id LEFT JOIN episodes ep ON p.logical_type='EPISODE' AND ep.id=p.logical_id LEFT JOIN continue_watching_dismissals d ON d.user_id=p.user_id AND d.logical_type=p.logical_type AND d.logical_id=p.logical_id WHERE p.user_id=? AND p.watched=0 AND p.position_seconds>0 AND d.user_id IS NULL ORDER BY p.last_played_at DESC LIMIT ?`, user, r.Limit)
	if e != nil {
		return nil, "", e
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var x Item
		if e = rows.Scan(&x.Type, &x.ID, &x.Title, &x.Position); e != nil {
			return nil, "", e
		}
		out = append(out, x)
	}
	return out, "", rows.Err()
}

var _ = sql.ErrNoRows

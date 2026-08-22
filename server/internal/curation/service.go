package curation

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	db  *sql.DB
	now func() time.Time
}

func New(db *sql.DB) *Service  { return &Service{db: db, now: time.Now} }
func id() string               { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func validType(t string, types ...string) bool {
	for _, x := range types {
		if t == x {
			return true
		}
	}
	return false
}
func visible(scope, owner, user string) bool {
	return scope == "SERVER_SHARED" || (scope == "USER_PRIVATE" && owner == user)
}

func (s *Service) ensureItem(ctx context.Context, t, itemID string) error {
	var n int
	table := ""
	if t == "MOVIE" {
		table = "movies"
	} else if t == "SHOW" {
		table = "shows"
	} else if t == "EPISODE" {
		table = "episodes"
	} else {
		return ErrValidation
	}
	if e := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE id=?", itemID).Scan(&n); e != nil {
		return e
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Collections(ctx context.Context, user string) ([]Collection, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT c.id,c.name,c.description,COALESCE(c.sort_title,''),c.scope,COALESCE(c.owner_user_id,''),c.ordering,c.created_at,c.updated_at,COALESCE((SELECT item_type FROM collection_items WHERE collection_id=c.id ORDER BY position LIMIT 1),''),COALESCE((SELECT item_id FROM collection_items WHERE collection_id=c.id ORDER BY position LIMIT 1),'') FROM collections c WHERE c.scope='SERVER_SHARED' OR c.owner_user_id=? ORDER BY COALESCE(c.sort_title,c.name)", user)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Collection{}
	for rows.Next() {
		var x Collection
		if e = rows.Scan(&x.ID, &x.Name, &x.Description, &x.SortTitle, &x.Scope, &x.OwnerUserID, &x.Ordering, &x.CreatedAt, &x.UpdatedAt, &x.ArtworkItemType, &x.ArtworkItemID); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) Collection(ctx context.Context, user, cid string) (Collection, error) {
	var x Collection
	e := s.db.QueryRowContext(ctx, "SELECT id,name,description,COALESCE(sort_title,''),scope,COALESCE(owner_user_id,''),ordering,created_at,updated_at FROM collections WHERE id=?", cid).Scan(&x.ID, &x.Name, &x.Description, &x.SortTitle, &x.Scope, &x.OwnerUserID, &x.Ordering, &x.CreatedAt, &x.UpdatedAt)
	if e == sql.ErrNoRows {
		return x, ErrNotFound
	}
	if e != nil {
		return x, e
	}
	if !visible(x.Scope, x.OwnerUserID, user) {
		return x, ErrNotFound
	}
	x.Items, e = s.collectionItems(ctx, cid, x.Ordering, 500)
	if len(x.Items) > 0 {
		x.ArtworkItemType = x.Items[0].Type
		x.ArtworkItemID = x.Items[0].ID
	}
	return x, e
}
func (s *Service) SaveCollection(ctx context.Context, user string, admin bool, x Collection) (Collection, error) {
	x.Name = strings.TrimSpace(x.Name)
	if x.Name == "" || !validType(x.Scope, "SERVER_SHARED", "USER_PRIVATE") || !validType(x.Ordering, "CUSTOM", "TITLE", "YEAR", "DATE_ADDED", "RELEASE_DATE", "RATING") {
		return x, ErrValidation
	}
	if x.Scope == "SERVER_SHARED" {
		if !admin {
			return x, ErrForbidden
		}
		x.OwnerUserID = ""
	} else {
		x.OwnerUserID = user
	}
	now := stamp(s.now())
	if x.ID == "" {
		x.ID = id()
		x.CreatedAt = now
	}
	x.UpdatedAt = now
	owner := any(x.OwnerUserID)
	if x.OwnerUserID == "" {
		owner = nil
	}
	if x.SortTitle == "" {
		x.SortTitle = x.Name
	}
	_, e := s.db.ExecContext(ctx, `INSERT INTO collections(id,name,description,sort_title,scope,owner_user_id,ordering,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,description=excluded.description,sort_title=excluded.sort_title,ordering=excluded.ordering,updated_at=excluded.updated_at WHERE collections.scope='USER_PRIVATE' AND collections.owner_user_id=? OR ?`, x.ID, x.Name, x.Description, x.SortTitle, x.Scope, owner, x.Ordering, x.CreatedAt, x.UpdatedAt, user, admin)
	if e != nil {
		return x, e
	}
	return s.Collection(ctx, user, x.ID)
}
func (s *Service) DeleteCollection(ctx context.Context, user string, admin bool, cid string) error {
	x, e := s.Collection(ctx, user, cid)
	if e != nil {
		return e
	}
	if x.Scope == "SERVER_SHARED" && !admin {
		return ErrForbidden
	}
	if x.Scope == "USER_PRIVATE" && x.OwnerUserID != user {
		return ErrNotFound
	}
	r, e := s.db.ExecContext(ctx, "DELETE FROM collections WHERE id=?", cid)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	_, _ = s.db.ExecContext(ctx, "DELETE FROM home_rows WHERE row_type='COLLECTION' AND source_id=?", cid)
	return nil
}
func (s *Service) AddCollectionItems(ctx context.Context, user string, admin bool, cid string, items []Item) error {
	x, e := s.Collection(ctx, user, cid)
	if e != nil {
		return e
	}
	if x.Scope == "SERVER_SHARED" && !admin {
		return ErrForbidden
	}
	for _, it := range items {
		if !validType(it.Type, "MOVIE", "SHOW") {
			return ErrValidation
		}
		if e = s.ensureItem(ctx, it.Type, it.ID); e != nil {
			return e
		}
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var pos int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(position),-1)+1 FROM collection_items WHERE collection_id=?", cid).Scan(&pos)
	for _, it := range items {
		_, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO collection_items(collection_id,item_type,item_id,position,added_at) VALUES(?,?,?,?,?)", cid, it.Type, it.ID, pos, stamp(s.now()))
		if e != nil {
			return e
		}
		pos++
	}
	return tx.Commit()
}
func (s *Service) RemoveCollectionItem(ctx context.Context, user string, admin bool, cid, t, itemID string) error {
	x, e := s.Collection(ctx, user, cid)
	if e != nil {
		return e
	}
	if x.Scope == "SERVER_SHARED" && !admin {
		return ErrForbidden
	}
	_, e = s.db.ExecContext(ctx, "DELETE FROM collection_items WHERE collection_id=? AND item_type=? AND item_id=?", cid, t, itemID)
	if e == nil {
		e = s.compact(ctx, "collection_items", "collection_id", cid)
	}
	return e
}
func (s *Service) ReorderCollection(ctx context.Context, user string, admin bool, cid string, ids []string) error {
	x, e := s.Collection(ctx, user, cid)
	if e != nil {
		return e
	}
	if x.Scope == "SERVER_SHARED" && !admin {
		return ErrForbidden
	}
	if len(ids) > 5000 {
		return ErrValidation
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var n int
	_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM collection_items WHERE collection_id=?", cid).Scan(&n)
	if n != len(ids) {
		return ErrValidation
	}
	for p, key := range ids {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			return ErrValidation
		}
		r, e := tx.ExecContext(ctx, "UPDATE collection_items SET position=? WHERE collection_id=? AND item_type=? AND item_id=?", -p-1, cid, parts[0], parts[1])
		if e != nil {
			return e
		}
		changed, _ := r.RowsAffected()
		if changed != 1 {
			return ErrValidation
		}
	}
	if _, e = tx.ExecContext(ctx, "UPDATE collection_items SET position=-position-1 WHERE collection_id=?", cid); e != nil {
		return e
	}
	return tx.Commit()
}

// AutomationMembership is deliberately limited to server-shared manual collections.
func (s *Service) AutomationMembership(ctx context.Context, collectionID, action, itemType, itemID string) error {
	var scope string
	if e := s.db.QueryRowContext(ctx, "SELECT scope FROM collections WHERE id=?", collectionID).Scan(&scope); e == sql.ErrNoRows {
		return ErrNotFound
	} else if e != nil {
		return e
	}
	if scope != "SERVER_SHARED" || !validType(itemType, "MOVIE", "SHOW") {
		return ErrValidation
	}
	if action == "REMOVE_FROM_COLLECTION" {
		_, e := s.db.ExecContext(ctx, "DELETE FROM collection_items WHERE collection_id=? AND item_type=? AND item_id=?", collectionID, itemType, itemID)
		return e
	}
	if action != "ADD_TO_COLLECTION" {
		return ErrValidation
	}
	var pos int
	_ = s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(position),-1)+1 FROM collection_items WHERE collection_id=?", collectionID).Scan(&pos)
	_, e := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO collection_items(collection_id,item_type,item_id,position,added_at) VALUES(?,?,?,?,?)", collectionID, itemType, itemID, pos, stamp(s.now()))
	return e
}
func (s *Service) compact(ctx context.Context, table, parent, id string) error {
	_, e := s.db.ExecContext(ctx, fmt.Sprintf("WITH ranked AS (SELECT rowid,ROW_NUMBER() OVER(ORDER BY position)-1 p FROM %s WHERE %s=?) UPDATE %s SET position=(SELECT p FROM ranked WHERE ranked.rowid=%s.rowid) WHERE %s=?", table, parent, table, table, parent), id, id)
	return e
}
func (s *Service) reorder(ctx context.Context, table, parent, pid string, ids []string) error {
	if len(ids) > 5000 {
		return ErrValidation
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var n int
	_ = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s=?", table, parent), pid).Scan(&n)
	if n != len(ids) {
		return ErrValidation
	}
	for p, eid := range ids {
		r, e := tx.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET position=? WHERE %s=? AND id=?", table, parent), -p-1, pid, eid)
		if e != nil {
			return e
		}
		changed, _ := r.RowsAffected()
		if changed != 1 {
			return ErrValidation
		}
	}
	_, e = tx.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET position=-position-1 WHERE %s=?", table, parent), pid)
	if e != nil {
		return e
	}
	return tx.Commit()
}

func (s *Service) collectionItems(ctx context.Context, cid, ordering string, limit int) ([]Item, error) {
	order := map[string]string{"CUSTOM": "ci.position", "TITLE": "title COLLATE NOCASE", "YEAR": "year DESC,title", "DATE_ADDED": "ci.added_at DESC", "RELEASE_DATE": "release_date DESC", "RATING": "rating DESC,title"}[ordering]
	q := `SELECT ci.item_type,ci.item_id,CASE ci.item_type WHEN 'MOVIE' THEN m.title ELSE sh.title END title,COALESCE(CASE ci.item_type WHEN 'MOVIE' THEN m.year ELSE sh.year END,0),COALESCE(CASE ci.item_type WHEN 'MOVIE' THEN m.rating_value ELSE sh.rating_value END,0),COALESCE(CASE ci.item_type WHEN 'MOVIE' THEN m.release_date ELSE sh.first_air_date END,''),ci.position,CASE WHEN EXISTS(SELECT 1 FROM logical_library_memberships lm WHERE lm.entity_type=ci.item_type AND lm.entity_id=ci.item_id) THEN 'AVAILABLE' ELSE 'UNAVAILABLE' END FROM collection_items ci LEFT JOIN movies m ON ci.item_type='MOVIE' AND m.id=ci.item_id LEFT JOIN shows sh ON ci.item_type='SHOW' AND sh.id=ci.item_id WHERE ci.collection_id=? ORDER BY ` + order + ` LIMIT ?`
	rows, e := s.db.QueryContext(ctx, q, cid, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var x Item
		var release string
		if e = rows.Scan(&x.Type, &x.ID, &x.Title, &x.Year, &x.Rating, &release, &x.Position, &x.Availability); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) TogglePersonal(ctx context.Context, user, kind, t, itemID string, on bool) error {
	table := ""
	if kind == "WATCHLIST" {
		table = "watchlist_items"
	} else if kind == "FAVORITE" {
		table = "favorite_items"
	} else {
		return ErrValidation
	}
	if !validType(t, "MOVIE", "SHOW") {
		return ErrValidation
	}
	if e := s.ensureItem(ctx, t, itemID); e != nil {
		return e
	}
	if on {
		_, e := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO "+table+"(user_id,item_type,item_id,added_at) VALUES(?,?,?,?)", user, t, itemID, stamp(s.now()))
		return e
	}
	_, e := s.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE user_id=? AND item_type=? AND item_id=?", user, t, itemID)
	return e
}
func (s *Service) Personal(ctx context.Context, user, kind string, limit int) ([]Item, error) {
	table := ""
	if kind == "WATCHLIST" {
		table = "watchlist_items"
	} else if kind == "FAVORITE" {
		table = "favorite_items"
	} else {
		return nil, ErrValidation
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	q := `SELECT x.item_type,x.item_id,CASE x.item_type WHEN 'MOVIE' THEN m.title ELSE sh.title END,COALESCE(CASE x.item_type WHEN 'MOVIE' THEN m.year ELSE sh.year END,0),0,x.added_at FROM ` + table + ` x LEFT JOIN movies m ON x.item_type='MOVIE' AND m.id=x.item_id LEFT JOIN shows sh ON x.item_type='SHOW' AND sh.id=x.item_id WHERE x.user_id=? ORDER BY x.added_at DESC LIMIT ?`
	rows, e := s.db.QueryContext(ctx, q, user, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var x Item
		var added string
		if e = rows.Scan(&x.Type, &x.ID, &x.Title, &x.Year, &x.Rating, &added); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) Playlists(ctx context.Context, user string) ([]Playlist, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,owner_user_id,name,description,created_at,updated_at FROM playlists WHERE owner_user_id=? ORDER BY updated_at DESC", user)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Playlist{}
	for rows.Next() {
		var x Playlist
		if e = rows.Scan(&x.ID, &x.OwnerUserID, &x.Name, &x.Description, &x.CreatedAt, &x.UpdatedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) Playlist(ctx context.Context, user, pid string) (Playlist, error) {
	var x Playlist
	e := s.db.QueryRowContext(ctx, "SELECT id,owner_user_id,name,description,created_at,updated_at FROM playlists WHERE id=? AND owner_user_id=?", pid, user).Scan(&x.ID, &x.OwnerUserID, &x.Name, &x.Description, &x.CreatedAt, &x.UpdatedAt)
	if e == sql.ErrNoRows {
		return x, ErrNotFound
	}
	if e != nil {
		return x, e
	}
	rows, e := s.db.QueryContext(ctx, `SELECT pi.id,pi.item_type,pi.item_id,CASE pi.item_type WHEN 'MOVIE' THEN m.title ELSE ep.title END,COALESCE(CASE pi.item_type WHEN 'MOVIE' THEN m.year ELSE CAST(substr(ep.air_date,1,4) AS INTEGER) END,0),pi.position,COALESCE(sh.title,'') FROM playlist_items pi LEFT JOIN movies m ON pi.item_type='MOVIE' AND m.id=pi.item_id LEFT JOIN episodes ep ON pi.item_type='EPISODE' AND ep.id=pi.item_id LEFT JOIN seasons se ON ep.season_id=se.id LEFT JOIN shows sh ON se.show_id=sh.id WHERE pi.playlist_id=? ORDER BY pi.position`, pid)
	if e != nil {
		return x, e
	}
	defer rows.Close()
	for rows.Next() {
		var it Item
		if e = rows.Scan(&it.ArtworkID, &it.Type, &it.ID, &it.Title, &it.Year, &it.Position, &it.Subtitle); e != nil {
			return x, e
		}
		x.Items = append(x.Items, it)
	}
	return x, rows.Err()
}
func (s *Service) SavePlaylist(ctx context.Context, user string, x Playlist) (Playlist, error) {
	x.Name = strings.TrimSpace(x.Name)
	if x.Name == "" {
		return x, ErrValidation
	}
	now := stamp(s.now())
	if x.ID == "" {
		x.ID = id()
		x.CreatedAt = now
	}
	x.OwnerUserID = user
	x.UpdatedAt = now
	_, e := s.db.ExecContext(ctx, `INSERT INTO playlists(id,owner_user_id,name,description,created_at,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,description=excluded.description,updated_at=excluded.updated_at WHERE playlists.owner_user_id=?`, x.ID, user, x.Name, x.Description, x.CreatedAt, x.UpdatedAt, user)
	if e != nil {
		return x, e
	}
	return s.Playlist(ctx, user, x.ID)
}
func (s *Service) DeletePlaylist(ctx context.Context, user, pid string) error {
	r, e := s.db.ExecContext(ctx, "DELETE FROM playlists WHERE id=? AND owner_user_id=?", pid, user)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	_, _ = s.db.ExecContext(ctx, "DELETE FROM home_rows WHERE user_id=? AND row_type='PLAYLIST' AND source_id=?", user, pid)
	return nil
}
func (s *Service) AddPlaylistItem(ctx context.Context, user, pid, t, itemID string) (Item, error) {
	if !validType(t, "MOVIE", "EPISODE") {
		return Item{}, ErrValidation
	}
	if _, e := s.Playlist(ctx, user, pid); e != nil {
		return Item{}, e
	}
	if e := s.ensureItem(ctx, t, itemID); e != nil {
		return Item{}, e
	}
	var pos int
	_ = s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(position),-1)+1 FROM playlist_items WHERE playlist_id=?", pid).Scan(&pos)
	eid := id()
	_, e := s.db.ExecContext(ctx, "INSERT INTO playlist_items(id,playlist_id,item_type,item_id,position,added_at) VALUES(?,?,?,?,?,?)", eid, pid, t, itemID, pos, stamp(s.now()))
	return Item{ArtworkID: eid, Type: t, ID: itemID, Position: pos}, e
}
func (s *Service) RemovePlaylistItem(ctx context.Context, user, pid, eid string) error {
	if _, e := s.Playlist(ctx, user, pid); e != nil {
		return e
	}
	_, e := s.db.ExecContext(ctx, "DELETE FROM playlist_items WHERE id=? AND playlist_id=?", eid, pid)
	if e == nil {
		e = s.compact(ctx, "playlist_items", "playlist_id", pid)
	}
	return e
}
func (s *Service) ReorderPlaylist(ctx context.Context, user, pid string, ids []string) error {
	if _, e := s.Playlist(ctx, user, pid); e != nil {
		return e
	}
	return s.reorder(ctx, "playlist_items", "playlist_id", pid, ids)
}
func (s *Service) PlaylistNavigation(ctx context.Context, user, pid, eid string) (*Item, *Item, error) {
	p, e := s.Playlist(ctx, user, pid)
	if e != nil {
		return nil, nil, e
	}
	for i, x := range p.Items {
		if x.ArtworkID == eid {
			var prev, next *Item
			if i > 0 {
				v := p.Items[i-1]
				prev = &v
			}
			if i+1 < len(p.Items) {
				v := p.Items[i+1]
				next = &v
			}
			return prev, next, nil
		}
	}
	return nil, nil, ErrNotFound
}

func marshalRule(r RuleNode) string { b, _ := json.Marshal(r); return string(b) }

package curation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type queryBuild struct {
	args  []any
	count int
}

func usesUserState(n RuleNode) bool {
	if validType(n.Field, "watched", "favorite", "progressState") {
		return true
	}
	for _, c := range n.Children {
		if usesUserState(c) {
			return true
		}
	}
	return false
}

func (b *queryBuild) node(n RuleNode, depth int) (string, error) {
	if depth > 5 || b.count >= 50 {
		return "", ErrValidation
	}
	if n.Logic != "" {
		if !validType(n.Logic, "ALL", "ANY", "NOT") || len(n.Children) == 0 || n.Logic == "NOT" && len(n.Children) != 1 {
			return "", ErrValidation
		}
		parts := []string{}
		for _, c := range n.Children {
			x, e := b.node(c, depth+1)
			if e != nil {
				return "", e
			}
			parts = append(parts, x)
		}
		join := " AND "
		if n.Logic == "ANY" {
			join = " OR "
		}
		out := "(" + strings.Join(parts, join) + ")"
		if n.Logic == "NOT" {
			out = "NOT " + out
		}
		return out, nil
	}
	b.count++
	field := map[string]string{"title": "q.title", "year": "q.year", "rating": "q.rating", "contentRating": "q.content_rating", "releaseDate": "q.release_date", "dateAdded": "q.created_at", "availability": "q.availability", "resolution": "q.resolution", "videoCodec": "q.video_codec", "hdr": "q.hdr", "audioCodec": "q.audio_codec", "audioChannels": "q.audio_channels", "optimized": "q.optimized", "watched": "q.watched", "favorite": "q.favorite", "progressState": "q.progress_state", "genre": "q.genres"}[n.Field]
	if field == "" {
		return "", ErrValidation
	}
	numeric := validType(n.Field, "year", "rating", "audioChannels")
	boolean := validType(n.Field, "optimized", "watched", "favorite")
	if numeric {
		v, e := strconv.ParseFloat(fmt.Sprint(n.Value), 64)
		if e != nil {
			return "", ErrValidation
		}
		n.Value = v
	}
	op := ""
	switch n.Operator {
	case "EQUALS":
		op = "="
	case "NOT_EQUALS":
		op = "!="
	case "CONTAINS":
		if numeric || boolean {
			return "", ErrValidation
		}
		op = "LIKE"
		n.Value = "%" + fmt.Sprint(n.Value) + "%"
	case "STARTS_WITH":
		if numeric || boolean {
			return "", ErrValidation
		}
		op = "LIKE"
		n.Value = fmt.Sprint(n.Value) + "%"
	case "GT":
		if !numeric {
			return "", ErrValidation
		}
		op = ">"
	case "GTE":
		if !numeric {
			return "", ErrValidation
		}
		op = ">="
	case "LT":
		if !numeric {
			return "", ErrValidation
		}
		op = "<"
	case "LTE":
		if !numeric {
			return "", ErrValidation
		}
		op = "<="
	case "IS_TRUE":
		if !boolean {
			return "", ErrValidation
		}
		op = "="
		n.Value = 1
	case "IS_FALSE":
		if !boolean {
			return "", ErrValidation
		}
		op = "="
		n.Value = 0
	default:
		return "", ErrValidation
	}
	b.args = append(b.args, n.Value)
	return field + " " + op + " ?", nil
}

func mediaQuery(user string) string {
	return `WITH q AS (
SELECT 'MOVIE' item_type,m.id,m.title,COALESCE(m.year,0) year,COALESCE(m.rating_value,0) rating,COALESCE(m.content_rating,'') content_rating,COALESCE(m.release_date,'') release_date,m.created_at,
CASE WHEN EXISTS(SELECT 1 FROM media_associations a JOIN media_files f ON f.id=a.media_file_id WHERE a.entity_type='MOVIE' AND a.entity_id=m.id AND f.availability='AVAILABLE') THEN 'AVAILABLE' ELSE 'UNAVAILABLE' END availability,
COALESCE((SELECT f.resolution_class FROM media_associations a JOIN media_files f ON f.id=a.media_file_id WHERE a.entity_type='MOVIE' AND a.entity_id=m.id ORDER BY f.size_bytes DESC LIMIT 1),'' ) resolution,
COALESCE((SELECT ms.codec FROM media_associations a JOIN media_streams ms ON ms.media_file_id=a.media_file_id AND ms.stream_type='video' WHERE a.entity_type='MOVIE' AND a.entity_id=m.id LIMIT 1),'') video_codec,
COALESCE((SELECT f.hdr_class FROM media_associations a JOIN media_files f ON f.id=a.media_file_id WHERE a.entity_type='MOVIE' AND a.entity_id=m.id LIMIT 1),'') hdr,
COALESCE((SELECT ms.codec FROM media_associations a JOIN media_streams ms ON ms.media_file_id=a.media_file_id AND ms.stream_type='audio' WHERE a.entity_type='MOVIE' AND a.entity_id=m.id LIMIT 1),'') audio_codec,
COALESCE((SELECT ms.channels FROM media_associations a JOIN media_streams ms ON ms.media_file_id=a.media_file_id AND ms.stream_type='audio' WHERE a.entity_type='MOVIE' AND a.entity_id=m.id LIMIT 1),0) audio_channels,
EXISTS(SELECT 1 FROM optimized_media o WHERE o.logical_type='MOVIE' AND o.logical_id=m.id AND o.status='COMPLETED') optimized,
EXISTS(SELECT 1 FROM user_media_progress p WHERE p.user_id=? AND p.logical_type='MOVIE' AND p.logical_id=m.id AND p.watched=1) watched,
EXISTS(SELECT 1 FROM favorite_items f WHERE f.user_id=? AND f.item_type='MOVIE' AND f.item_id=m.id) favorite,
CASE WHEN EXISTS(SELECT 1 FROM user_media_progress p WHERE p.user_id=? AND p.logical_type='MOVIE' AND p.logical_id=m.id AND p.watched=1) THEN 'WATCHED' WHEN EXISTS(SELECT 1 FROM user_media_progress p WHERE p.user_id=? AND p.logical_type='MOVIE' AND p.logical_id=m.id AND p.position_seconds>0) THEN 'PARTIAL' ELSE 'UNWATCHED' END progress_state,
COALESCE((SELECT group_concat(g.name,'|') FROM media_genres mg JOIN genres g ON g.id=mg.genre_id WHERE mg.entity_type='MOVIE' AND mg.entity_id=m.id),'') genres
FROM movies m WHERE m.orphaned=0
UNION ALL
SELECT 'SHOW',s.id,s.title,COALESCE(s.year,0),COALESCE(s.rating_value,0),'',COALESCE(s.first_air_date,''),s.created_at,
CASE WHEN EXISTS(SELECT 1 FROM seasons se JOIN episodes e ON e.season_id=se.id JOIN media_associations a ON a.entity_type='EPISODE' AND a.entity_id=e.id JOIN media_files f ON f.id=a.media_file_id WHERE se.show_id=s.id AND f.availability='AVAILABLE') THEN 'AVAILABLE' ELSE 'UNAVAILABLE' END,'','','','',0,0,
EXISTS(SELECT 1 FROM user_media_progress p JOIN episodes e ON e.id=p.logical_id JOIN seasons se ON se.id=e.season_id WHERE p.user_id=? AND p.logical_type='EPISODE' AND se.show_id=s.id GROUP BY se.show_id HAVING MIN(p.watched)=1),
EXISTS(SELECT 1 FROM favorite_items f WHERE f.user_id=? AND f.item_type='SHOW' AND f.item_id=s.id),
'UNWATCHED',COALESCE((SELECT group_concat(g.name,'|') FROM media_genres mg JOIN genres g ON g.id=mg.genre_id WHERE mg.entity_type='SHOW' AND mg.entity_id=s.id),'') FROM shows s WHERE s.orphaned=0)`
}

func (s *Service) evaluate(ctx context.Context, user string, rule RuleNode, sort, direction string, limit int) ([]Item, error) {
	if limit < 1 || limit > 500 {
		return nil, ErrValidation
	}
	b := &queryBuild{}
	where, e := b.node(rule, 0)
	if e != nil {
		return nil, e
	}
	order := map[string]string{"title": "title COLLATE NOCASE", "year": "year", "rating": "rating", "dateAdded": "created_at", "releaseDate": "release_date"}[sort]
	if order == "" || !validType(direction, "ASC", "DESC") {
		return nil, ErrValidation
	}
	args := []any{user, user, user, user, user, user}
	args = append(args, b.args...)
	args = append(args, limit)
	rows, e := s.db.QueryContext(ctx, mediaQuery(user)+" SELECT item_type,id,title,year,rating,availability FROM q WHERE "+where+" ORDER BY "+order+" "+direction+",title LIMIT ?", args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var x Item
		if e = rows.Scan(&x.Type, &x.ID, &x.Title, &x.Year, &x.Rating, &x.Availability); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) PreviewSmart(ctx context.Context, user string, x SmartCollection) ([]Item, error) {
	return s.evaluate(ctx, user, x.Rule, x.SortField, x.SortDirection, min(x.Limit, 100))
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func (s *Service) SaveSmart(ctx context.Context, user string, admin bool, x SmartCollection) (SmartCollection, error) {
	x.Name = strings.TrimSpace(x.Name)
	if x.Name == "" || !validType(x.Scope, "SERVER_SHARED", "USER_PRIVATE") || x.RuleSchemaVersion != 1 {
		return x, ErrValidation
	}
	if x.Scope == "SERVER_SHARED" && usesUserState(x.Rule) {
		return x, ErrValidation
	}
	if _, e := s.evaluate(ctx, user, x.Rule, x.SortField, x.SortDirection, min(x.Limit, 1)); e != nil {
		return x, e
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
	_, e := s.db.ExecContext(ctx, `INSERT INTO smart_collections(id,name,description,scope,owner_user_id,rule_schema_version,rule_json,sort_field,sort_direction,item_limit,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,description=excluded.description,rule_json=excluded.rule_json,sort_field=excluded.sort_field,sort_direction=excluded.sort_direction,item_limit=excluded.item_limit,updated_at=excluded.updated_at WHERE smart_collections.owner_user_id=? OR ?`, x.ID, x.Name, x.Description, x.Scope, owner, 1, marshalRule(x.Rule), x.SortField, x.SortDirection, x.Limit, x.CreatedAt, x.UpdatedAt, user, admin)
	if e != nil {
		return x, e
	}
	return s.Smart(ctx, user, x.ID)
}
func (s *Service) Smarts(ctx context.Context, user string) ([]SmartCollection, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,name,description,scope,COALESCE(owner_user_id,''),rule_schema_version,rule_json,sort_field,sort_direction,item_limit,created_at,updated_at FROM smart_collections WHERE scope='SERVER_SHARED' OR owner_user_id=? ORDER BY name", user)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []SmartCollection{}
	for rows.Next() {
		var x SmartCollection
		var rule string
		if e = rows.Scan(&x.ID, &x.Name, &x.Description, &x.Scope, &x.OwnerUserID, &x.RuleSchemaVersion, &rule, &x.SortField, &x.SortDirection, &x.Limit, &x.CreatedAt, &x.UpdatedAt); e != nil {
			return nil, e
		}
		if json.Unmarshal([]byte(rule), &x.Rule) != nil {
			continue
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) Smart(ctx context.Context, user, sid string) (SmartCollection, error) {
	var x SmartCollection
	var rule string
	e := s.db.QueryRowContext(ctx, "SELECT id,name,description,scope,COALESCE(owner_user_id,''),rule_schema_version,rule_json,sort_field,sort_direction,item_limit,created_at,updated_at FROM smart_collections WHERE id=?", sid).Scan(&x.ID, &x.Name, &x.Description, &x.Scope, &x.OwnerUserID, &x.RuleSchemaVersion, &rule, &x.SortField, &x.SortDirection, &x.Limit, &x.CreatedAt, &x.UpdatedAt)
	if e == sql.ErrNoRows {
		return x, ErrNotFound
	}
	if e != nil {
		return x, e
	}
	if !visible(x.Scope, x.OwnerUserID, user) {
		return x, ErrNotFound
	}
	if json.Unmarshal([]byte(rule), &x.Rule) != nil {
		return x, ErrValidation
	}
	x.Items, e = s.evaluate(ctx, user, x.Rule, x.SortField, x.SortDirection, x.Limit)
	if len(x.Items) > 0 {
		x.ArtworkItemType = x.Items[0].Type
		x.ArtworkItemID = x.Items[0].ID
	}
	return x, e
}
func (s *Service) DeleteSmart(ctx context.Context, user string, admin bool, sid string) error {
	x, e := s.Smart(ctx, user, sid)
	if e != nil {
		return e
	}
	if x.Scope == "SERVER_SHARED" && !admin {
		return ErrForbidden
	}
	r, e := s.db.ExecContext(ctx, "DELETE FROM smart_collections WHERE id=?", sid)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	_, _ = s.db.ExecContext(ctx, "DELETE FROM home_rows WHERE row_type='SMART_COLLECTION' AND source_id=?", sid)
	return nil
}

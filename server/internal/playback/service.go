package playback

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrForbidden   = errors.New("forbidden")
	ErrValidation  = errors.New("validation")
	ErrUnavailable = errors.New("media unavailable")
	ErrStale       = errors.New("stale inventory")
	ErrExpired     = errors.New("playback authorization expired")
)

type Service struct {
	db         *sql.DB
	now        func() time.Time
	inactivity time.Duration
}

func New(db *sql.DB) *Service {
	s := &Service{db: db, now: time.Now, inactivity: 45 * time.Minute}
	s.AbandonStale(context.Background())
	return s
}
func id() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	x := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", x[:8], x[8:12], x[12:16], x[16:20], x[20:])
}
func stamp(t time.Time) string   { return t.UTC().Format(time.RFC3339Nano) }
func listJSON(v []string) string { b, _ := json.Marshal(v); return string(b) }

func (s *Service) Start(ctx context.Context, userID, authSessionID string, in StartRequest) (Session, error) {
	if in.LogicalType != "MOVIE" && in.LogicalType != "EPISODE" {
		return Session{}, ErrValidation
	}
	if in.LogicalID == "" || in.Capabilities.SchemaVersion != 1 || in.Capabilities.ClientName == "" || len(in.Capabilities.Containers) > 20 || len(in.Capabilities.VideoCodecs) > 20 || len(in.Capabilities.AudioCodecs) > 30 {
		return Session{}, ErrValidation
	}
	var active int
	if s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users u JOIN sessions s ON s.user_id=u.id WHERE u.id=? AND u.status='ACTIVE' AND s.id=? AND s.revoked_at IS NULL AND s.expires_at>?", userID, authSessionID, stamp(s.now())).Scan(&active) != nil || active == 0 {
		return Session{}, ErrForbidden
	}
	versions, err := s.Versions(ctx, in.LogicalType, in.LogicalID)
	if err != nil {
		return Session{}, err
	}
	selected, decision := Select(versions, in.Capabilities, in.RequestedVersionID)
	now := s.now().UTC()
	capID := id()
	tokenRaw := make([]byte, 32)
	_, _ = rand.Read(tokenRaw)
	token := base64.RawURLEncoding.EncodeToString(tokenRaw)
	sum := sha256.Sum256([]byte(token))
	sessionID := id()
	var duration float64
	if selected.ID != "" {
		_ = s.db.QueryRowContext(ctx, "SELECT COALESCE(duration_seconds,0) FROM media_files WHERE id=?", selected.FileID).Scan(&duration)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO client_capabilities(id,user_id,auth_session_id,schema_version,client_name,client_version,platform,platform_version,device_model,containers_json,video_codecs_json,audio_codecs_json,subtitle_formats_json,max_width,max_height,hdr_json,direct_play,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, capID, userID, authSessionID, in.Capabilities.SchemaVersion, in.Capabilities.ClientName, in.Capabilities.ClientVersion, in.Capabilities.Platform, in.Capabilities.PlatformVersion, in.Capabilities.DeviceModel, listJSON(in.Capabilities.Containers), listJSON(in.Capabilities.VideoCodecs), listJSON(in.Capabilities.AudioCodecs), listJSON(in.Capabilities.SubtitleFormats), in.Capabilities.MaxWidth, in.Capabilities.MaxHeight, listJSON(in.Capabilities.HDR), in.Capabilities.DirectPlay, stamp(now), stamp(now))
	if err != nil {
		return Session{}, err
	}
	resume := 0.0
	var watched bool
	if in.Resume {
		_ = tx.QueryRowContext(ctx, "SELECT position_seconds,watched FROM user_media_progress WHERE user_id=? AND logical_type=? AND logical_id=?", userID, in.LogicalType, in.LogicalID).Scan(&resume, &watched)
		if resume < 30 || watched || (duration > 0 && resume >= duration*.9) {
			resume = 0
		}
	}
	state := Starting
	if decision.Mode == Unsupported {
		state = Error
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO playback_sessions(id,user_id,auth_session_id,capability_id,logical_type,logical_id,media_association_id,media_file_id,mode,state,position_seconds,duration_seconds,started_at,last_activity_at,ended_at,error_code,media_token_hash,media_token_expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, sessionID, userID, authSessionID, capID, in.LogicalType, in.LogicalID, null(selected.ID), null(selected.FileID), decision.Mode, state, resume, duration, stamp(now), stamp(now), func() any {
		if state == Error {
			return stamp(now)
		}
		return nil
	}(), func() any {
		if state == Error && len(decision.Reasons) > 0 {
			return decision.Reasons[0].Code
		}
		return nil
	}(), hex.EncodeToString(sum[:]), stamp(now.Add(time.Hour)))
	if err != nil {
		return Session{}, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO playback_history(playback_session_id,user_id,event_type,position_seconds,created_at) VALUES(?,?,?,?,?)", sessionID, userID, "PLAYBACK_STARTED", resume, stamp(now)); err != nil {
		return Session{}, err
	}
	if err = tx.Commit(); err != nil {
		return Session{}, err
	}
	out := Session{ID: sessionID, UserID: userID, LogicalType: in.LogicalType, LogicalID: in.LogicalID, MediaVersion: selected, Decision: decision, State: state, Position: resume, Duration: duration, ResumePosition: resume, StartedAt: stamp(now), LastActivityAt: stamp(now)}
	if decision.Mode == DirectPlay {
		out.MediaURL = "/api/v1/playback/sessions/" + sessionID + "/media?token=" + token
	}
	return out, nil
}
func null(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (s *Service) Versions(ctx context.Context, kind, logicalID string) ([]Version, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,f.id,f.extension,COALESCE(f.resolution_class,''),COALESCE(f.hdr_class,''),COALESCE(f.bitrate,0),f.availability,COALESCE((SELECT codec FROM media_streams WHERE media_file_id=f.id AND UPPER(stream_type)='VIDEO' ORDER BY is_default DESC,stream_index LIMIT 1),''),COALESCE((SELECT width FROM media_streams WHERE media_file_id=f.id AND UPPER(stream_type)='VIDEO' ORDER BY is_default DESC,stream_index LIMIT 1),0),COALESCE((SELECT height FROM media_streams WHERE media_file_id=f.id AND UPPER(stream_type)='VIDEO' ORDER BY is_default DESC,stream_index LIMIT 1),0),COALESCE(a.version_label,'') FROM media_associations a JOIN media_files f ON f.id=a.media_file_id WHERE a.entity_type=? AND a.entity_id=?`, kind, logicalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Version{}
	for rows.Next() {
		var v Version
		var ext, availability string
		if err = rows.Scan(&v.ID, &v.FileID, &ext, &v.Resolution, &v.HDR, &v.Bitrate, &availability, &v.VideoCodec, &v.Width, &v.Height, &v.Label); err != nil {
			return nil, err
		}
		v.Container = normalizeContainer(ext)
		v.Available = availability == "AVAILABLE"
		out = append(out, v)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range out {
		arows, e := s.db.QueryContext(ctx, "SELECT DISTINCT COALESCE(codec,'') FROM media_streams WHERE media_file_id=? AND UPPER(stream_type)='AUDIO' ORDER BY is_default DESC,stream_index", out[i].FileID)
		if e != nil {
			return nil, e
		}
		for arows.Next() {
			var a string
			_ = arows.Scan(&a)
			if a != "" {
				out[i].AudioCodecs = append(out[i].AudioCodecs, a)
			}
		}
		arows.Close()
	}
	if len(out) == 0 {
		return nil, ErrNotFound
	}
	return out, nil
}
func normalizeContainer(ext string) string {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if ext == "m4v" {
		return "mp4"
	}
	return ext
}

func (s *Service) Update(ctx context.Context, userID, sessionID string, p Progress) error {
	if p.Position < 0 || p.Duration < 0 || p.Position > 7*24*3600 || p.Duration > 7*24*3600 {
		return ErrValidation
	}
	switch p.State {
	case Playing, Paused, Stopped, Completed, Error:
	default:
		return ErrValidation
	}
	var logicalType, logicalID, state string
	var storedDuration float64
	if s.db.QueryRowContext(ctx, "SELECT logical_type,logical_id,state,duration_seconds FROM playback_sessions WHERE id=? AND user_id=?", sessionID, userID).Scan(&logicalType, &logicalID, &state, &storedDuration) != nil {
		return ErrForbidden
	}
	if state == string(Stopped) || state == string(Completed) || state == string(Error) {
		return ErrExpired
	}
	if p.Duration <= 0 {
		p.Duration = storedDuration
	}
	if p.Duration > 0 && p.Position > p.Duration+30 {
		return ErrValidation
	}
	watched := p.State == Completed || (p.Duration > 0 && p.Position/p.Duration >= .9)
	if watched {
		p.State = Completed
		p.Position = 0
	}
	now := s.now().UTC()
	ended := any(nil)
	if p.State == Stopped || p.State == Completed || p.State == Error {
		ended = stamp(now)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, "UPDATE playback_sessions SET state=?,position_seconds=?,duration_seconds=?,last_activity_at=?,ended_at=? WHERE id=? AND user_id=?", p.State, p.Position, p.Duration, stamp(now), ended, sessionID, userID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrForbidden
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_media_progress(user_id,logical_type,logical_id,position_seconds,duration_seconds,watched,last_played_at,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(user_id,logical_type,logical_id) DO UPDATE SET position_seconds=excluded.position_seconds,duration_seconds=excluded.duration_seconds,watched=excluded.watched,last_played_at=excluded.last_played_at,updated_at=excluded.updated_at`, userID, logicalType, logicalID, p.Position, p.Duration, watched, stamp(now), stamp(now))
	if err != nil {
		return err
	}
	event := map[State]string{Playing: "PLAYBACK_RESUMED", Paused: "PLAYBACK_PAUSED", Stopped: "PLAYBACK_STOPPED", Completed: "PLAYBACK_COMPLETED", Error: "PLAYBACK_ERROR"}[p.State]
	_, err = tx.ExecContext(ctx, "INSERT INTO playback_history(playback_session_id,user_id,event_type,position_seconds,created_at) VALUES(?,?,?,?,?)", sessionID, userID, event, p.Position, stamp(now))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) Stop(ctx context.Context, userID, sessionID string) error {
	var position, duration float64
	if s.db.QueryRowContext(ctx, "SELECT position_seconds,duration_seconds FROM playback_sessions WHERE id=? AND user_id=?", sessionID, userID).Scan(&position, &duration) != nil {
		return ErrForbidden
	}
	return s.Update(ctx, userID, sessionID, Progress{State: Stopped, Position: position, Duration: duration})
}
func (s *Service) MarkWatched(ctx context.Context, userID, kind, logicalID string, watched bool) error {
	if kind != "MOVIE" && kind != "EPISODE" {
		return ErrValidation
	}
	now := stamp(s.now())
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_media_progress(user_id,logical_type,logical_id,position_seconds,duration_seconds,watched,last_played_at,updated_at) VALUES(?,?,?,0,0,?,?,?) ON CONFLICT(user_id,logical_type,logical_id) DO UPDATE SET position_seconds=0,watched=excluded.watched,last_played_at=excluded.last_played_at,updated_at=excluded.updated_at`, userID, kind, logicalID, watched, now, now)
	return err
}
func (s *Service) Progress(ctx context.Context, userID, kind, logicalID string) (WatchProgress, error) {
	var p WatchProgress
	p.LogicalType = kind
	p.LogicalID = logicalID
	err := s.db.QueryRowContext(ctx, "SELECT position_seconds,duration_seconds,watched,last_played_at FROM user_media_progress WHERE user_id=? AND logical_type=? AND logical_id=?", userID, kind, logicalID).Scan(&p.Position, &p.Duration, &p.Watched, &p.LastPlayedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	return p, err
}

type MediaAccess struct {
	Path, MIME, ETag, Modified string
	Size                       int64
	SessionID                  string
}

func (s *Service) AuthorizeMedia(ctx context.Context, sessionID, token string) (MediaAccess, error) {
	sum := sha256.Sum256([]byte(token))
	var a MediaAccess
	var root, rel string
	var storedSize, storedMtime int64
	var state, expiry string
	err := s.db.QueryRowContext(ctx, `SELECT src.normalized_path,f.relative_path,f.size_bytes,f.modified_at_ns,p.state,p.media_token_expires_at,f.extension FROM playback_sessions p JOIN media_files f ON f.id=p.media_file_id JOIN library_sources src ON src.id=f.source_id JOIN sessions s ON s.id=p.auth_session_id JOIN users u ON u.id=p.user_id WHERE p.id=? AND p.mode='DIRECT_PLAY' AND p.media_token_hash=? AND f.availability='AVAILABLE' AND s.revoked_at IS NULL AND s.expires_at>? AND u.status='ACTIVE'`, sessionID, hex.EncodeToString(sum[:]), stamp(s.now())).Scan(&root, &rel, &storedSize, &storedMtime, &state, &expiry, &a.MIME)
	if err != nil {
		return a, ErrForbidden
	}
	if state == string(Stopped) || state == string(Completed) || state == string(Error) {
		return a, ErrExpired
	}
	ex, _ := time.Parse(time.RFC3339Nano, expiry)
	if s.now().After(ex) {
		return a, ErrExpired
	}
	clean := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
	rootClean := filepath.Clean(root)
	if clean == rootClean || !strings.HasPrefix(clean, rootClean+string(os.PathSeparator)) {
		return a, ErrForbidden
	}
	st, err := os.Stat(clean)
	if err != nil {
		s.fail(sessionID, "MEDIA_UNAVAILABLE")
		return a, ErrUnavailable
	}
	if st.Size() != storedSize || st.ModTime().UnixNano() != storedMtime {
		s.fail(sessionID, "STALE_INVENTORY")
		return a, ErrStale
	}
	a.Path = clean
	a.Size = st.Size()
	a.Modified = st.ModTime().UTC().Format(time.RFC1123)
	a.ETag = fmt.Sprintf(`"%x-%x"`, storedSize, storedMtime)
	a.MIME = mediaType(a.MIME)
	a.SessionID = sessionID
	_, _ = s.db.ExecContext(ctx, "UPDATE playback_sessions SET state=CASE WHEN state='STARTING' THEN 'PLAYING' ELSE state END,last_activity_at=? WHERE id=?", stamp(s.now()), sessionID)
	return a, nil
}
func mediaType(ext string) string {
	switch normalizeContainer(ext) {
	case "mp4":
		return "video/mp4"
	case "webm":
		return "video/webm"
	case "ogg", "ogv":
		return "video/ogg"
	default:
		return "application/octet-stream"
	}
}
func (s *Service) AddBytes(ctx context.Context, id string, n int64) {
	_, _ = s.db.ExecContext(ctx, "UPDATE playback_sessions SET bytes_served=bytes_served+?,last_activity_at=? WHERE id=?", n, stamp(s.now()), id)
}
func (s *Service) OwnedBy(ctx context.Context, id, userID, authSessionID string) bool {
	var n int
	return s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM playback_sessions WHERE id=? AND user_id=? AND auth_session_id=?", id, userID, authSessionID).Scan(&n) == nil && n == 1
}
func (s *Service) fail(id, code string) {
	now := stamp(s.now())
	_, _ = s.db.Exec("UPDATE playback_sessions SET state='ERROR',error_code=?,ended_at=?,last_activity_at=? WHERE id=?", code, now, now, id)
}
func (s *Service) AbandonStale(ctx context.Context) {
	cut := stamp(s.now().Add(-s.inactivity))
	now := stamp(s.now())
	_, _ = s.db.ExecContext(ctx, "UPDATE playback_sessions SET state='STOPPED',completion_reason='SERVER_RESTART_OR_INACTIVE',ended_at=? WHERE state IN ('STARTING','PLAYING','PAUSED') AND last_activity_at<?", now, cut)
	_, _ = s.db.ExecContext(ctx, "UPDATE playback_sessions SET state='STOPPED',completion_reason='SERVER_RESTART',ended_at=? WHERE state IN ('STARTING','PLAYING','PAUSED')", now)
}
func (s *Service) Active(ctx context.Context) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,p.logical_type,p.logical_id,p.state,p.position_seconds,p.duration_seconds,p.started_at,p.last_activity_at,u.display_name,c.client_name,c.platform,CASE p.logical_type WHEN 'MOVIE' THEN COALESCE((SELECT title FROM movies WHERE id=p.logical_id),'Unknown movie') ELSE COALESCE((SELECT e.title FROM episodes e WHERE e.id=p.logical_id),'Unknown episode') END,COALESCE(f.resolution_class,''),COALESCE((SELECT codec FROM media_streams WHERE media_file_id=f.id AND UPPER(stream_type)='VIDEO' ORDER BY stream_index LIMIT 1),'') FROM playback_sessions p JOIN users u ON u.id=p.user_id JOIN client_capabilities c ON c.id=p.capability_id LEFT JOIN media_files f ON f.id=p.media_file_id WHERE p.state IN ('STARTING','PLAYING','PAUSED') AND p.last_activity_at>? ORDER BY p.started_at`, stamp(s.now().Add(-s.inactivity)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var x Session
		var res, codec string
		if err = rows.Scan(&x.ID, &x.LogicalType, &x.LogicalID, &x.State, &x.Position, &x.Duration, &x.StartedAt, &x.LastActivityAt, &x.UserDisplayName, &x.ClientName, &x.Platform, &x.Title, &res, &codec); err != nil {
			return nil, err
		}
		x.Decision.Mode = DirectPlay
		x.MediaVersion.Resolution = res
		x.MediaVersion.VideoCodec = codec
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) AdminStop(ctx context.Context, id string) error {
	now := stamp(s.now())
	r, err := s.db.ExecContext(ctx, "UPDATE playback_sessions SET state='STOPPED',completion_reason='ADMIN_TERMINATED',ended_at=?,last_activity_at=? WHERE id=? AND state IN ('STARTING','PLAYING','PAUSED')", now, now, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

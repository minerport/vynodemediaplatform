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
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrForbidden        = errors.New("forbidden")
	ErrValidation       = errors.New("validation")
	ErrUnavailable      = errors.New("media unavailable")
	ErrStale            = errors.New("stale inventory")
	ErrExpired          = errors.New("playback authorization expired")
	ErrVideoCapacity    = errors.New("video transcode capacity reached")
	ErrTranscodeStorage = errors.New("transcode storage unavailable")
)

type Service struct {
	db            *sql.DB
	now           func() time.Time
	inactivity    time.Duration
	pipeline      *FFmpegPipeline
	hls           *HLSManager
	remoteBitrate int64
}

func (s *Service) ConfigureVideo(h *HLSManager, remoteBitrate int64) {
	s.hls = h
	s.remoteBitrate = remoteBitrate
}

func New(db *sql.DB, pipeline ...*FFmpegPipeline) *Service {
	var p *FFmpegPipeline
	if len(pipeline) > 0 {
		p = pipeline[0]
	}
	s := &Service{db: db, now: time.Now, inactivity: 45 * time.Minute, pipeline: p}
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
	if in.LogicalID == "" || (in.Capabilities.SchemaVersion != 1 && in.Capabilities.SchemaVersion != 2) || in.Capabilities.ClientName == "" || len(in.Capabilities.Containers) > 20 || len(in.Capabilities.VideoCodecs) > 20 || len(in.Capabilities.AudioCodecs) > 30 || in.Capabilities.MaxAudioChannels < 0 || in.Capabilities.MaxAudioChannels > 16 || in.StartPosition < 0 || in.StartPosition > 7*24*60*60 {
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
	limit := in.BandwidthLimit
	if in.Network == NetworkRemote && limit <= 0 {
		limit = s.remoteBitrate
	}
	selected, decision := SelectPolicy(versions, in.Capabilities, in.RequestedVersionID, in.SelectedAudioTrackID, in.SelectedSubtitleTrackID, in.QualityID, limit)
	if decision.Mode != DirectPlay && decision.Mode != Unsupported && (s.pipeline == nil || !s.pipeline.Available()) {
		decision = reject(decision, "FFMPEG_UNAVAILABLE", "")
	}
	if decision.Mode == AudioTranscode && !contains(s.pipeline.Capabilities().Encoders, "aac") {
		decision = reject(decision, "AUDIO_ENCODER_UNAVAILABLE", "aac")
	}
	if decision.Mode == VideoTranscode && (s.hls == nil || !contains(s.pipeline.Capabilities().Encoders, "libx264")) {
		decision = reject(decision, "VIDEO_ENCODER_UNAVAILABLE", "libx264")
	}
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
	_, err = tx.ExecContext(ctx, `INSERT INTO client_capabilities(id,user_id,auth_session_id,schema_version,client_name,client_version,platform,platform_version,device_model,containers_json,video_codecs_json,audio_codecs_json,subtitle_formats_json,max_width,max_height,hdr_json,direct_play,max_audio_channels,fragmented_mp4,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, capID, userID, authSessionID, in.Capabilities.SchemaVersion, in.Capabilities.ClientName, in.Capabilities.ClientVersion, in.Capabilities.Platform, in.Capabilities.PlatformVersion, in.Capabilities.DeviceModel, listJSON(in.Capabilities.Containers), listJSON(in.Capabilities.VideoCodecs), listJSON(in.Capabilities.AudioCodecs), listJSON(in.Capabilities.SubtitleFormats), in.Capabilities.MaxWidth, in.Capabilities.MaxHeight, listJSON(in.Capabilities.HDR), in.Capabilities.DirectPlay, in.Capabilities.MaxAudioChannels, in.Capabilities.FragmentedMP4, stamp(now), stamp(now))
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
	if in.StartPosition > 0 {
		resume = in.StartPosition
		if duration > 0 && resume > duration {
			resume = duration
		}
	}
	state := Starting
	if decision.Mode == Unsupported {
		state = Error
	}
	planJSON, _ := json.Marshal(decision.Plan)
	_, err = tx.ExecContext(ctx, `INSERT INTO playback_sessions(id,user_id,auth_session_id,capability_id,logical_type,logical_id,media_association_id,media_file_id,mode,state,position_seconds,duration_seconds,started_at,last_activity_at,ended_at,error_code,media_token_hash,media_token_expires_at,selected_audio_track_id,selected_subtitle_track_id,pipeline_plan_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, sessionID, userID, authSessionID, capID, in.LogicalType, in.LogicalID, null(selected.ID), null(selected.FileID), decision.Mode, state, resume, duration, stamp(now), stamp(now), func() any {
		if state == Error {
			return stamp(now)
		}
		return nil
	}(), func() any {
		if state == Error && len(decision.Reasons) > 0 {
			return decision.Reasons[0].Code
		}
		return nil
	}(), hex.EncodeToString(sum[:]), stamp(now.Add(4*time.Hour)), null(decision.Plan.Audio.TrackID), null(in.SelectedSubtitleTrackID), string(planJSON))
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
	if decision.Mode != Unsupported {
		out.MediaURL = "/api/v1/playback/sessions/" + sessionID + "/media?token=" + token
	}
	if decision.Mode == VideoTranscode {
		out.HLSURL = "/api/v1/playback/sessions/" + sessionID + "/hls/master.m3u8?token=" + token
		out.MediaURL = ""
		out.Qualities = QualityProfiles(selected)
	}
	if in.SelectedSubtitleTrackID != "" && decision.Mode != Unsupported {
		out.SubtitleURL = "/api/v1/playback/sessions/" + sessionID + "/subtitles/" + in.SelectedSubtitleTrackID
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
		arows, e := s.db.QueryContext(ctx, "SELECT id,stream_index,COALESCE(codec,''),COALESCE(language,''),COALESCE(title,''),COALESCE(channels,0),is_default,commentary FROM media_streams WHERE media_file_id=? AND UPPER(stream_type)='AUDIO' ORDER BY is_default DESC,stream_index", out[i].FileID)
		if e != nil {
			return nil, e
		}
		for arows.Next() {
			var a Track
			if e = arows.Scan(&a.ID, &a.StreamIndex, &a.Codec, &a.Language, &a.Title, &a.Channels, &a.Default, &a.Commentary); e != nil {
				arows.Close()
				return nil, e
			}
			a.Kind = "AUDIO"
			a.Usable = true
			out[i].AudioTracks = append(out[i].AudioTracks, a)
			if a.Codec != "" && !contains(out[i].AudioCodecs, a.Codec) {
				out[i].AudioCodecs = append(out[i].AudioCodecs, a.Codec)
			}
		}
		arows.Close()
		srows, e := s.db.QueryContext(ctx, "SELECT id,stream_index,COALESCE(codec,''),COALESCE(language,''),COALESCE(title,''),is_default,is_forced FROM media_streams WHERE media_file_id=? AND UPPER(stream_type)='SUBTITLE' ORDER BY stream_index", out[i].FileID)
		if e != nil {
			return nil, e
		}
		for srows.Next() {
			var t Track
			if e = srows.Scan(&t.ID, &t.StreamIndex, &t.Codec, &t.Language, &t.Title, &t.Default, &t.Forced); e != nil {
				srows.Close()
				return nil, e
			}
			t.Kind = "SUBTITLE"
			t.Source = "EMBEDDED"
			t.Usable = textSubtitle(t.Codec)
			if !t.Usable {
				t.Reason = "SUBTITLE_REQUIRES_VIDEO_TRANSCODE"
			}
			out[i].SubtitleTracks = append(out[i].SubtitleTracks, t)
		}
		srows.Close()
		if e = s.discoverSidecars(ctx, &out[i]); e != nil {
			return nil, e
		}
	}
	if len(out) == 0 {
		return nil, ErrNotFound
	}
	return out, nil
}
func (s *Service) Close() {
	if s.pipeline != nil {
		s.pipeline.Close()
	}
	if s.hls != nil {
		s.hls.Close()
	}
}
func textSubtitle(codec string) bool {
	return contains([]string{"subrip", "srt", "webvtt", "ass", "ssa"}, codec)
}

func (s *Service) discoverSidecars(ctx context.Context, v *Version) error {
	var root, rel, base string
	if e := s.db.QueryRowContext(ctx, "SELECT src.normalized_path,f.relative_path,f.base_name FROM media_files f JOIN library_sources src ON src.id=f.source_id WHERE f.id=?", v.FileID).Scan(&root, &rel, &base); e != nil {
		return e
	}
	dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(rel)))
	entries, e := os.ReadDir(dir)
	if e != nil {
		return nil
	}
	prefix := strings.ToLower(base)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		if !contains([]string{"srt", "vtt", "ass", "ssa"}, ext) {
			continue
		}
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		lower := strings.ToLower(stem)
		if lower != prefix && !strings.HasPrefix(lower, prefix+".") {
			continue
		}
		st, e := entry.Info()
		if e != nil || st.Size() > maxSubtitleSize {
			continue
		}
		lang := ""
		if lower != prefix {
			lang = strings.TrimPrefix(lower, prefix+".")
		}
		sid := "sidecar-" + v.FileID + "-" + fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
		sid = sid[:len("sidecar-")+len(v.FileID)+1+16]
		relative, _ := filepath.Rel(root, filepath.Join(dir, name))
		_, _ = s.db.ExecContext(ctx, `INSERT INTO sidecar_subtitles(id,media_file_id,relative_path,format,language,availability,size_bytes,modified_at_ns) VALUES(?,?,?,?,?,'AVAILABLE',?,?) ON CONFLICT(media_file_id,relative_path) DO UPDATE SET format=excluded.format,language=excluded.language,availability='AVAILABLE',size_bytes=excluded.size_bytes,modified_at_ns=excluded.modified_at_ns`, sid, v.FileID, filepath.ToSlash(relative), ext, lang, st.Size(), st.ModTime().UnixNano())
		usable := ext == "srt" || ext == "vtt"
		reason := ""
		if !usable {
			reason = "FORMAT_NOT_BROWSER_CONVERTIBLE"
		}
		v.SubtitleTracks = append(v.SubtitleTracks, Track{ID: sid, Kind: "SUBTITLE", Codec: ext, Language: lang, Usable: usable, Reason: reason, Source: "EXTERNAL", Path: filepath.Join(dir, name)})
	}
	return nil
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
		if s.pipeline != nil {
			s.pipeline.Cancel(sessionID)
		}
		if s.hls != nil {
			s.hls.Cancel(sessionID)
		}
		_, _ = s.db.ExecContext(ctx, "UPDATE transcode_sessions SET state='STOPPED',ended_at=? WHERE playback_session_id=? AND state IN ('STARTING','RUNNING')", stamp(now), sessionID)
		_, _ = s.db.ExecContext(ctx, "UPDATE playback_pipeline_instances SET state='STOPPED',ended_at=? WHERE playback_session_id=? AND state IN ('STARTING','RUNNING','STOPPING')", stamp(now), sessionID)
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
	Mode                       Mode
	Plan                       PipelinePlan
	AudioStreamIndex           int
}

func (s *Service) AuthorizeMedia(ctx context.Context, sessionID, token string) (MediaAccess, error) {
	sum := sha256.Sum256([]byte(token))
	var a MediaAccess
	var root, rel string
	var storedSize, storedMtime int64
	var state, expiry, mode, planJSON string
	err := s.db.QueryRowContext(ctx, `SELECT src.normalized_path,f.relative_path,f.size_bytes,f.modified_at_ns,p.state,p.media_token_expires_at,f.extension,p.mode,p.pipeline_plan_json,COALESCE(ms.stream_index,-1) FROM playback_sessions p JOIN media_files f ON f.id=p.media_file_id JOIN library_sources src ON src.id=f.source_id JOIN sessions s ON s.id=p.auth_session_id JOIN users u ON u.id=p.user_id LEFT JOIN media_streams ms ON ms.id=p.selected_audio_track_id WHERE p.id=? AND p.mode IN ('DIRECT_PLAY','DIRECT_STREAM','AUDIO_TRANSCODE','VIDEO_TRANSCODE') AND p.media_token_hash=? AND f.availability='AVAILABLE' AND s.revoked_at IS NULL AND s.expires_at>? AND u.status='ACTIVE'`, sessionID, hex.EncodeToString(sum[:]), stamp(s.now())).Scan(&root, &rel, &storedSize, &storedMtime, &state, &expiry, &a.MIME, &mode, &planJSON, &a.AudioStreamIndex)
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
	a.Mode = Mode(mode)
	_ = json.Unmarshal([]byte(planJSON), &a.Plan)
	_, _ = s.db.ExecContext(ctx, "UPDATE playback_sessions SET state=CASE WHEN state='STARTING' THEN 'PLAYING' ELSE state END,last_activity_at=? WHERE id=?", stamp(s.now()), sessionID)
	return a, nil
}

func (s *Service) StreamGenerated(ctx context.Context, a MediaAccess, start float64, w io.Writer) error {
	if a.Mode != DirectStream && a.Mode != AudioTranscode {
		return ErrValidation
	}
	if start < 0 || start > 7*24*3600 {
		return ErrValidation
	}
	if s.pipeline == nil {
		return ErrPipelineUnavailable
	}
	instance := id()
	now := s.now()
	_, e := s.db.ExecContext(ctx, "INSERT INTO playback_pipeline_instances(id,playback_session_id,state,mode,start_seconds,started_at) VALUES(?,?,?,?,?,?)", instance, a.SessionID, PipelineStarting, a.Mode, start, stamp(now))
	if e != nil {
		return e
	}
	s.pipeline.Cancel(a.SessionID)
	_, _ = s.db.ExecContext(ctx, "UPDATE playback_pipeline_instances SET state='STOPPING' WHERE playback_session_id=? AND state IN ('STARTING','RUNNING') AND id<>?", a.SessionID, instance)
	_, _ = s.db.ExecContext(ctx, "UPDATE playback_pipeline_instances SET state='RUNNING',running_at=?,startup_ms=? WHERE id=?", stamp(s.now()), time.Since(now).Milliseconds(), instance)
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go s.monitorAuthorization(streamCtx, cancel, a.SessionID)
	res, e := s.pipeline.Stream(streamCtx, PipelineRequest{SessionID: a.SessionID, InstanceID: instance, SourcePath: a.Path, Mode: a.Mode, Start: start, AudioStreamIndex: a.AudioStreamIndex, TargetChannels: a.Plan.Audio.TargetChannels}, w)
	state := PipelineStopped
	if e != nil && res.Code != "PIPELINE_CANCELED" {
		state = PipelineFailed
	}
	_, _ = s.db.Exec("UPDATE playback_pipeline_instances SET state=?,ended_at=?,error_code=?,safe_diagnostic=? WHERE id=?", state, stamp(s.now()), null(res.Code), truncate(res.Stderr, 32768), instance)
	return e
}
func (s *Service) HLSFile(ctx context.Context, sessionID, token, name string) (string, error) {
	a, e := s.AuthorizeMedia(ctx, sessionID, token)
	if e != nil {
		return "", e
	}
	if a.Mode != VideoTranscode {
		return "", ErrValidation
	}
	if s.hls == nil {
		return "", ErrPipelineUnavailable
	}
	var count int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM transcode_sessions WHERE playback_session_id=?", sessionID).Scan(&count)
	if count == 0 {
		progress := func(encoded, speed float64, outputBytes int64) {
			_, _ = s.db.Exec("UPDATE transcode_sessions SET encoded_seconds=?,speed=?,output_bytes=? WHERE playback_session_id=? AND state='RUNNING'", encoded, speed, outputBytes, sessionID)
		}
		done := func(runErr error) {
			state, pipelineState := "STOPPED", PipelineStopped
			if runErr != nil {
				state, pipelineState = "FAILED", PipelineFailed
			}
			now := stamp(s.now())
			_, _ = s.db.Exec("UPDATE transcode_sessions SET state=?,ended_at=?,error_code=? WHERE playback_session_id=? AND state='RUNNING'", state, now, func() any {
				if runErr != nil {
					return "FFMPEG_EXITED"
				}
				return nil
			}(), sessionID)
			_, _ = s.db.Exec("UPDATE playback_pipeline_instances SET state=?,ended_at=?,error_code=? WHERE playback_session_id=? AND mode=? AND state='RUNNING'", pipelineState, now, func() any {
				if runErr != nil {
					return "FFMPEG_EXITED"
				}
				return nil
			}(), sessionID, VideoTranscode)
		}
		if e = s.hls.Ensure(HLSRequest{SessionID: sessionID, SourcePath: a.Path, AudioStreamIndex: a.AudioStreamIndex, Plan: a.Plan, Progress: progress, Done: done}); e != nil {
			return "", e
		}
		now := stamp(s.now())
		_, e = s.db.ExecContext(ctx, "INSERT INTO transcode_sessions(id,playback_session_id,state,backend,quality_id,target_width,target_height,target_bitrate,owned_directory,started_at) VALUES(?,?,?,?,?,?,?,?,?,?)", id(), sessionID, "RUNNING", a.Plan.Backend.Actual, a.Plan.Quality, a.Plan.Video.TargetWidth, a.Plan.Video.TargetHeight, a.Plan.Video.TargetBitrate, sessionID, now)
		if e != nil {
			s.hls.Cancel(sessionID)
			return "", e
		}
		_, _ = s.db.ExecContext(ctx, "INSERT INTO playback_pipeline_instances(id,playback_session_id,state,mode,start_seconds,started_at,running_at) VALUES(?,?,?,?,?,?,?)", id(), sessionID, PipelineRunning, VideoTranscode, 0, now, now)
	}
	return s.hls.File(sessionID, name)
}
func (s *Service) monitorAuthorization(ctx context.Context, cancel context.CancelFunc, sessionID string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var n int
			e := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM playback_sessions p JOIN sessions a ON a.id=p.auth_session_id JOIN users u ON u.id=p.user_id WHERE p.id=? AND p.state IN ('STARTING','PLAYING','PAUSED') AND a.revoked_at IS NULL AND a.expires_at>? AND u.status='ACTIVE'`, sessionID, stamp(s.now())).Scan(&n)
			if e != nil || n != 1 {
				cancel()
				return
			}
		}
	}
}
func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
func (s *Service) Capabilities() FFmpegCapabilities {
	if s.pipeline == nil {
		return FFmpegCapabilities{Muxers: []string{}, Encoders: []string{}, Decoders: []string{}}
	}
	c := s.pipeline.Capabilities()
	if s.hls != nil {
		c.ActiveVideo, c.MaximumVideo = s.hls.Active()
	}
	return c
}
func (s *Service) Subtitle(ctx context.Context, sessionID, token, trackID string) ([]byte, error) {
	a, e := s.AuthorizeMedia(ctx, sessionID, token)
	if e != nil {
		return nil, e
	}
	var selected string
	if e = s.db.QueryRowContext(ctx, "SELECT COALESCE(selected_subtitle_track_id,'') FROM playback_sessions WHERE id=?", sessionID).Scan(&selected); e != nil || selected != trackID {
		return nil, ErrForbidden
	}
	if strings.HasPrefix(trackID, "sidecar-") {
		var rel, format string
		var size, mtime int64
		if e = s.db.QueryRowContext(ctx, "SELECT relative_path,format,size_bytes,modified_at_ns FROM sidecar_subtitles WHERE id=? AND media_file_id=(SELECT media_file_id FROM playback_sessions WHERE id=?) AND availability='AVAILABLE'", trackID, sessionID).Scan(&rel, &format, &size, &mtime); e != nil {
			return nil, ErrForbidden
		}
		var root string
		_ = s.db.QueryRowContext(ctx, "SELECT normalized_path FROM library_sources WHERE id=(SELECT source_id FROM media_files WHERE id=(SELECT media_file_id FROM playback_sessions WHERE id=?))", sessionID).Scan(&root)
		path := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
		if !strings.HasPrefix(path, filepath.Clean(root)+string(os.PathSeparator)) {
			return nil, ErrForbidden
		}
		st, e := os.Stat(path)
		if e != nil || st.Size() != size || st.ModTime().UnixNano() != mtime {
			return nil, ErrStale
		}
		return readSubtitle(path, format)
	}
	var index int
	var codec string
	if e = s.db.QueryRowContext(ctx, "SELECT stream_index,COALESCE(codec,'') FROM media_streams WHERE id=? AND media_file_id=(SELECT media_file_id FROM playback_sessions WHERE id=?)", trackID, sessionID).Scan(&index, &codec); e != nil || !textSubtitle(codec) {
		return nil, ErrForbidden
	}
	if s.pipeline == nil {
		return nil, ErrPipelineUnavailable
	}
	return s.pipeline.ConvertEmbeddedSubtitle(ctx, a.Path, index)
}
func (s *Service) ContinueWatching(ctx context.Context, userID string) ([]ContinueItem, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT p.logical_type,p.logical_id,CASE p.logical_type WHEN 'MOVIE' THEN COALESCE((SELECT title FROM movies WHERE id=p.logical_id),'Unknown movie') ELSE COALESCE((SELECT title FROM episodes WHERE id=p.logical_id),'Unknown episode') END,p.position_seconds,p.duration_seconds,p.last_played_at FROM user_media_progress p WHERE p.user_id=? AND p.watched=0 AND p.position_seconds>0 AND p.duration_seconds>0 ORDER BY p.last_played_at DESC LIMIT 30`, userID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ContinueItem{}
	for rows.Next() {
		var x ContinueItem
		if e = rows.Scan(&x.LogicalType, &x.LogicalID, &x.Title, &x.Position, &x.Duration, &x.LastPlayedAt); e != nil {
			return nil, e
		}
		x.Progress = x.Position / x.Duration
		out = append(out, x)
	}
	return out, rows.Err()
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
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,p.logical_type,p.logical_id,p.state,p.position_seconds,p.duration_seconds,p.started_at,p.last_activity_at,u.display_name,c.client_name,c.platform,CASE p.logical_type WHEN 'MOVIE' THEN COALESCE((SELECT title FROM movies WHERE id=p.logical_id),'Unknown movie') ELSE COALESCE((SELECT e.title FROM episodes e WHERE e.id=p.logical_id),'Unknown episode') END,COALESCE(f.resolution_class,''),COALESCE((SELECT codec FROM media_streams WHERE media_file_id=f.id AND UPPER(stream_type)='VIDEO' ORDER BY stream_index LIMIT 1),''),p.mode,p.pipeline_plan_json FROM playback_sessions p JOIN users u ON u.id=p.user_id JOIN client_capabilities c ON c.id=p.capability_id LEFT JOIN media_files f ON f.id=p.media_file_id WHERE p.state IN ('STARTING','PLAYING','PAUSED') AND p.last_activity_at>? ORDER BY p.started_at`, stamp(s.now().Add(-s.inactivity)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var x Session
		var res, codec, mode, plan string
		if err = rows.Scan(&x.ID, &x.LogicalType, &x.LogicalID, &x.State, &x.Position, &x.Duration, &x.StartedAt, &x.LastActivityAt, &x.UserDisplayName, &x.ClientName, &x.Platform, &x.Title, &res, &codec, &mode, &plan); err != nil {
			return nil, err
		}
		x.Decision.Mode = Mode(mode)
		_ = json.Unmarshal([]byte(plan), &x.Decision.Plan)
		x.MediaVersion.Resolution = res
		x.MediaVersion.VideoCodec = codec
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) AdminStop(ctx context.Context, id string) error {
	if s.pipeline != nil {
		s.pipeline.Cancel(id)
	}
	if s.hls != nil {
		s.hls.Cancel(id)
	}
	_, _ = s.db.ExecContext(ctx, "UPDATE transcode_sessions SET state='STOPPED',ended_at=? WHERE playback_session_id=? AND state IN ('STARTING','RUNNING')", stamp(s.now()), id)
	_, _ = s.db.ExecContext(ctx, "UPDATE playback_pipeline_instances SET state='STOPPED',ended_at=? WHERE playback_session_id=? AND state IN ('STARTING','RUNNING','STOPPING')", stamp(s.now()), id)
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

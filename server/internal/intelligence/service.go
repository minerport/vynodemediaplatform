package intelligence

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrValidation = errors.New("validation failed")
var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("manual marker is authoritative")

type Service struct {
	db                *sql.DB
	ffmpeg, optimized string
	now               func() time.Time
	slots             chan struct{}
	mu                sync.Mutex
	running           map[string]context.CancelFunc
	schedulerCancel   context.CancelFunc
	collections       CollectionManager
}

func New(db *sql.DB, ffmpeg, optimized string) *Service {
	if ffmpeg == "" {
		ffmpeg, _ = exec.LookPath("ffmpeg")
	}
	s := &Service{db: db, ffmpeg: ffmpeg, optimized: optimized, now: time.Now, slots: make(chan struct{}, 1), running: map[string]context.CancelFunc{}}
	_, _ = db.Exec("UPDATE background_jobs SET state='INTERRUPTED',completed_at=CURRENT_TIMESTAMP,error_summary='server restarted' WHERE state='RUNNING'")
	_ = os.MkdirAll(optimized, 0700)
	return s
}
func (s *Service) StartScheduler() {
	s.mu.Lock()
	if s.schedulerCancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.schedulerCancel = cancel
	s.mu.Unlock()
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case now := <-ticker.C:
				_ = s.RunDue(ctx, now)
			case <-ctx.Done():
				return
			}
		}
	}()
}
func (s *Service) Close() {
	s.mu.Lock()
	if s.schedulerCancel != nil {
		s.schedulerCancel()
	}
	for _, cancel := range s.running {
		cancel()
	}
	s.mu.Unlock()
}
func ident() string            { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func (s *Service) Jobs(ctx context.Context) ([]Job, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,job_type,target_type,target_id,state,progress,COALESCE(error_summary,''),created_at FROM background_jobs ORDER BY created_at DESC LIMIT 200")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		var x Job
		if e = rows.Scan(&x.ID, &x.Type, &x.TargetType, &x.TargetID, &x.State, &x.Progress, &x.Error, &x.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) enqueue(ctx context.Context, kind, targetType, targetID string, priority int, request any, work func(context.Context, string) error) (Job, error) {
	if targetID == "" {
		return Job{}, ErrValidation
	}
	var existing Job
	e := s.db.QueryRowContext(ctx, "SELECT id,job_type,target_type,target_id,state,progress,COALESCE(error_summary,''),created_at FROM background_jobs WHERE job_type=? AND target_type=? AND target_id=? AND state IN ('QUEUED','RUNNING') LIMIT 1", kind, targetType, targetID).Scan(&existing.ID, &existing.Type, &existing.TargetType, &existing.TargetID, &existing.State, &existing.Progress, &existing.Error, &existing.CreatedAt)
	if e == nil {
		return existing, nil
	}
	b, _ := json.Marshal(request)
	j := Job{ID: ident(), Type: kind, TargetType: targetType, TargetID: targetID, State: "QUEUED", CreatedAt: stamp(s.now())}
	_, e = s.db.ExecContext(ctx, "INSERT INTO background_jobs(id,job_type,target_type,target_id,priority,state,request_json,created_at) VALUES(?,?,?,?,?,'QUEUED',?,?)", j.ID, kind, targetType, targetID, priority, string(b), j.CreatedAt)
	if e != nil {
		return Job{}, e
	}
	go s.run(j.ID, work)
	return j, nil
}
func (s *Service) run(id string, work func(context.Context, string) error) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.running[id] = cancel
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.running, id); s.mu.Unlock() }()
	s.slots <- struct{}{}
	defer func() { <-s.slots }()
	for {
		var active int
		_ = s.db.QueryRow("SELECT (SELECT COUNT(*) FROM playback_sessions WHERE mode='VIDEO_TRANSCODE' AND state IN ('STARTING','PLAYING'))+(SELECT COUNT(*) FROM download_jobs WHERE state IN ('QUEUED','PREPARING'))").Scan(&active)
		if active == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
	_, _ = s.db.Exec("UPDATE background_jobs SET state='RUNNING',started_at=?,attempts=attempts+1 WHERE id=?", stamp(s.now()), id)
	e := work(ctx, id)
	state := "COMPLETED"
	summary := ""
	if ctx.Err() != nil {
		state = "CANCELED"
	} else if e != nil {
		state = "FAILED"
		summary = e.Error()
	}
	_, _ = s.db.Exec("UPDATE background_jobs SET state=?,progress=CASE WHEN ?='COMPLETED' THEN 1 ELSE progress END,error_summary=?,completed_at=? WHERE id=?", state, state, summary, stamp(s.now()), id)
}
func (s *Service) Cancel(ctx context.Context, id string) error {
	s.mu.Lock()
	cancel := s.running[id]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	r, e := s.db.ExecContext(ctx, "UPDATE background_jobs SET state='CANCELED',completed_at=? WHERE id=? AND state IN ('QUEUED','RUNNING')", stamp(s.now()), id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type episodeSource struct {
	logicalType, logicalID, fileID, path string
	size, mtime                          int64
	duration                             float64
}

func (s *Service) sources(ctx context.Context, targetType, targetID string) ([]episodeSource, error) {
	if targetType == "MOVIE" {
		var x episodeSource
		var root, rel string
		x.logicalType = "MOVIE"
		e := s.db.QueryRowContext(ctx, `SELECT m.id,f.id,src.normalized_path,f.relative_path,f.size_bytes,f.modified_at_ns,COALESCE(f.duration_seconds,0) FROM movies m JOIN media_associations a ON a.entity_type='MOVIE' AND a.entity_id=m.id JOIN media_files f ON f.id=a.media_file_id JOIN library_sources src ON src.id=f.source_id WHERE m.id=? AND f.availability='AVAILABLE' AND a.association_type!='OPTIMIZED' LIMIT 1`, targetID).Scan(&x.logicalID, &x.fileID, &root, &rel, &x.size, &x.mtime, &x.duration)
		if e != nil {
			return nil, e
		}
		x.path = filepath.Join(root, filepath.FromSlash(rel))
		return []episodeSource{x}, nil
	}
	q := `SELECT e.id,f.id,src.normalized_path,f.relative_path,f.size_bytes,f.modified_at_ns,COALESCE(f.duration_seconds,0) FROM episodes e JOIN seasons se ON se.id=e.season_id JOIN media_associations a ON a.entity_type='EPISODE' AND a.entity_id=e.id JOIN media_files f ON f.id=a.media_file_id JOIN library_sources src ON src.id=f.source_id WHERE f.availability='AVAILABLE' AND `
	args := []any{targetID}
	switch targetType {
	case "EPISODE":
		q += "e.id=?"
	case "SEASON":
		q += "se.id=?"
	case "SHOW":
		q += "se.show_id=?"
	default:
		return nil, ErrValidation
	}
	q += " ORDER BY se.season_number,e.episode_number"
	rows, e := s.db.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []episodeSource{}
	for rows.Next() {
		var x episodeSource
		x.logicalType = "EPISODE"
		var root, rel string
		if e = rows.Scan(&x.logicalID, &x.fileID, &root, &rel, &x.size, &x.mtime, &x.duration); e != nil {
			return nil, e
		}
		x.path = filepath.Join(root, filepath.FromSlash(rel))
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) Analyze(ctx context.Context, targetType, targetID string) (Job, error) {
	return s.enqueue(ctx, "MARKER_ANALYSIS", targetType, targetID, 20, map[string]string{"scope": targetType}, func(c context.Context, j string) error { return s.analyze(c, j, targetType, targetID) })
}
func (s *Service) analyze(ctx context.Context, jobID, targetType, targetID string) error {
	sources, e := s.sources(ctx, targetType, targetID)
	if e != nil {
		return e
	}
	if len(sources) == 0 {
		return ErrNotFound
	}
	series := [][]string{}
	identParts := []string{}
	for i, x := range sources {
		pcm, e := s.audio(ctx, x.path)
		if e != nil {
			return e
		}
		sig := ChunkSignatures(pcm, 32000)
		series = append(series, sig)
		identParts = append(identParts, fmt.Sprintf("%s:%d:%d", x.fileID, x.size, x.mtime))
		b, _ := json.Marshal(sig)
		_, _ = s.db.ExecContext(ctx, `INSERT INTO media_fingerprints(media_file_id,kind,source_identity,features_json,size_bytes,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(media_file_id,kind) DO UPDATE SET source_identity=excluded.source_identity,features_json=excluded.features_json,size_bytes=excluded.size_bytes,created_at=excluded.created_at`, x.fileID, "AUDIO_EARLY", fmt.Sprintf("%d:%d", x.size, x.mtime), string(b), len(b), stamp(s.now()))
		_, _ = s.db.ExecContext(ctx, "UPDATE background_jobs SET progress=? WHERE id=?", .65*float64(i+1)/float64(len(sources)), jobID)
	}
	sum := sha256.Sum256([]byte(strings.Join(identParts, "|")))
	sourceID := hex.EncodeToString(sum[:16])
	if sources[0].logicalType == "EPISODE" {
		if start, end, score, ok := DetectRecurring(series); ok {
			pattern := series[0][int(start/2):int(end/2)]
			for i, x := range sources {
				if offset, matched := MatchStart(series[i], pattern); matched {
					s.persistCandidate(ctx, x.logicalType, x.logicalID, "INTRO", "AUTOMATIC_AUDIO", float64(offset*2), float64((offset+len(pattern))*2), score, sourceID)
				}
			}
		}
	}
	for i, x := range sources {
		means, e := s.lumaTail(ctx, x.path, x.duration)
		if e == nil {
			if start, end, score, ok := DetectCredits(means, x.duration); ok {
				s.persistCandidate(ctx, x.logicalType, x.logicalID, "CREDITS", "AUTOMATIC_VIDEO", start, end, score, fmt.Sprintf("%s:%d:%d", x.fileID, x.size, x.mtime))
			}
		}
		_, _ = s.db.ExecContext(ctx, "UPDATE background_jobs SET progress=? WHERE id=?", .65+.35*float64(i+1)/float64(len(sources)), jobID)
	}
	return nil
}
func (s *Service) audio(ctx context.Context, path string) ([]byte, error) {
	if s.ffmpeg == "" {
		return nil, errors.New("ffmpeg unavailable")
	}
	cmd := exec.CommandContext(ctx, s.ffmpeg, "-v", "error", "-t", "900", "-i", path, "-vn", "-ac", "1", "-ar", "8000", "-f", "s16le", "pipe:1")
	return cmd.Output()
}
func (s *Service) lumaTail(ctx context.Context, path string, duration float64) ([]float64, error) {
	start := duration - 180
	if start < 0 {
		start = 0
	}
	cmd := exec.CommandContext(ctx, s.ffmpeg, "-v", "error", "-ss", strconv.FormatFloat(start, 'f', 3, 64), "-i", path, "-vf", "fps=1,scale=32:18,format=gray", "-an", "-f", "rawvideo", "pipe:1")
	b, e := cmd.Output()
	if e != nil {
		return nil, e
	}
	means := []float64{}
	for len(b) >= 576 {
		var sum int
		for _, v := range b[:576] {
			sum += int(v)
		}
		means = append(means, float64(sum)/576)
		b = b[576:]
	}
	return means, nil
}
func (s *Service) persistCandidate(ctx context.Context, logicalType, logicalID, kind, source string, start, end, score float64, sourceID string) {
	var manual int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM media_markers WHERE logical_type=? AND logical_id=? AND marker_type=? AND source='MANUAL' AND active=1", logicalType, logicalID, kind).Scan(&manual)
	var suppressed int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM marker_suppressions WHERE logical_type=? AND logical_id=? AND marker_type=? AND source_identity=?", logicalType, logicalID, kind, sourceID).Scan(&suppressed)
	if suppressed > 0 {
		return
	}
	state := "PENDING"
	active := 0
	var policy string
	_ = s.db.QueryRowContext(ctx, "SELECT value FROM server_settings WHERE key='automatic_high_confidence_markers'").Scan(&policy)
	if manual == 0 && Classify(score) == ConfidenceHigh && policy == "true" {
		state = "ACCEPTED"
		active = 1
	}
	_, _ = s.db.ExecContext(ctx, "UPDATE media_markers SET active=0,review_state='SUPERSEDED' WHERE logical_type=? AND logical_id=? AND marker_type=? AND source!='MANUAL' AND source_identity!=?", logicalType, logicalID, kind, sourceID)
	_, _ = s.db.ExecContext(ctx, "INSERT INTO media_markers(id,logical_type,logical_id,marker_type,start_seconds,end_seconds,source,confidence,active,review_state,source_identity,created_at,updated_at) SELECT ?,?,?,?,?,?,?,?,?,?,?,?,? WHERE NOT EXISTS(SELECT 1 FROM media_markers WHERE logical_type=? AND logical_id=? AND marker_type=? AND source_identity=?)", ident(), logicalType, logicalID, kind, start, end, source, score, active, state, sourceID, stamp(s.now()), stamp(s.now()), logicalType, logicalID, kind, sourceID)
}

func (s *Service) ReviewQueue(ctx context.Context) ([]MarkerCandidate, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,logical_type,logical_id,marker_type,source,start_seconds,end_seconds,confidence,review_state,COALESCE(source_identity,''),created_at,updated_at FROM media_markers WHERE review_state='PENDING' ORDER BY confidence DESC,created_at")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []MarkerCandidate{}
	for rows.Next() {
		var x MarkerCandidate
		if e = rows.Scan(&x.ID, &x.LogicalType, &x.LogicalID, &x.Type, &x.Source, &x.Start, &x.End, &x.Confidence, &x.ReviewState, &x.SourceIdentity, &x.CreatedAt, &x.UpdatedAt); e != nil {
			return nil, e
		}
		x.ConfidenceClass = Classify(x.Confidence)
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) Review(ctx context.Context, id, action string, start, end *float64) error {
	var lt, lid, kind, sourceID string
	if s.db.QueryRowContext(ctx, "SELECT logical_type,logical_id,marker_type,COALESCE(source_identity,'') FROM media_markers WHERE id=? AND source!='MANUAL'", id).Scan(&lt, &lid, &kind, &sourceID) != nil {
		return ErrNotFound
	}
	if action == "REJECT" {
		_, e := s.db.ExecContext(ctx, "UPDATE media_markers SET active=0,review_state='REJECTED',updated_at=? WHERE id=?", stamp(s.now()), id)
		if e == nil {
			_, e = s.db.ExecContext(ctx, "INSERT OR IGNORE INTO marker_suppressions VALUES(?,?,?,?,?,?)", lt, lid, kind, sourceID, "ADMIN_REJECTED", stamp(s.now()))
		}
		return e
	}
	var manual int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM media_markers WHERE logical_type=? AND logical_id=? AND marker_type=? AND source='MANUAL' AND active=1", lt, lid, kind).Scan(&manual)
	if manual > 0 {
		return ErrConflict
	}
	if action == "ADJUST" {
		if start == nil || end == nil || *start < 0 || *end <= *start {
			return ErrValidation
		}
		_, e := s.db.ExecContext(ctx, "UPDATE media_markers SET start_seconds=?,end_seconds=?,source='MANUAL',confidence=NULL,active=1,review_state='ACCEPTED',updated_at=? WHERE id=?", *start, *end, stamp(s.now()), id)
		return e
	}
	if action != "ACCEPT" {
		return ErrValidation
	}
	_, e := s.db.ExecContext(ctx, "UPDATE media_markers SET active=1,review_state='ACCEPTED',updated_at=? WHERE id=?", stamp(s.now()), id)
	return e
}

func (s *Service) Policy(ctx context.Context) (bool, error) {
	var x string
	e := s.db.QueryRowContext(ctx, "SELECT value FROM server_settings WHERE key='automatic_high_confidence_markers'").Scan(&x)
	return x == "true", e
}
func (s *Service) SetPolicy(ctx context.Context, on bool) error {
	_, e := s.db.ExecContext(ctx, "UPDATE server_settings SET value=?,updated_at=? WHERE key='automatic_high_confidence_markers'", strconv.FormatBool(on), stamp(s.now()))
	return e
}

func safeName(id string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return -1
	}, id)
}
func checksum(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (s *Service) Optimize(ctx context.Context, logicalType, logicalID, sourceFileID, profileID string) (Job, error) {
	p, ok := Profiles[profileID]
	if !ok || !contains([]string{"MOVIE", "EPISODE"}, logicalType) {
		return Job{}, ErrValidation
	}
	return s.enqueue(ctx, "MEDIA_OPTIMIZATION", logicalType, logicalID, 30, map[string]string{"sourceFileId": sourceFileID, "profile": profileID}, func(c context.Context, j string) error {
		return s.optimize(c, j, logicalType, logicalID, sourceFileID, p)
	})
}
func (s *Service) optimize(ctx context.Context, jobID, logicalType, logicalID, sourceID string, p OptimizationProfile) error {
	var root, rel string
	var duration float64
	var width, height int
	e := s.db.QueryRowContext(ctx, "SELECT src.normalized_path,f.relative_path,COALESCE(f.duration_seconds,0),COALESCE(v.width,0),COALESCE(v.height,0) FROM media_files f JOIN library_sources src ON src.id=f.source_id LEFT JOIN media_streams v ON v.media_file_id=f.id AND v.stream_type='video' WHERE f.id=? AND f.availability='AVAILABLE' LIMIT 1", sourceID).Scan(&root, &rel, &duration, &width, &height)
	if e != nil {
		return ErrNotFound
	}
	dir := filepath.Join(s.optimized, safeName(logicalType), safeName(logicalID))
	if e = os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	partial := filepath.Join(dir, safeName(p.ID)+"-"+safeName(jobID)+".partial.mp4")
	final := strings.TrimSuffix(partial, ".partial.mp4") + ".mp4"
	defer os.Remove(partial)
	_, _ = s.db.ExecContext(ctx, "UPDATE background_jobs SET progress=.1 WHERE id=?", jobID)
	args := []string{"-v", "error", "-y", "-i", filepath.Join(root, filepath.FromSlash(rel)), "-map", "0:v:0", "-map", "0:a:0?", "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-b:v", strconv.FormatInt(p.VideoBitrate, 10), "-c:a", "aac", "-b:a", strconv.FormatInt(p.AudioBitrate, 10), "-movflags", "+faststart"}
	if p.Height > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease:force_divisible_by=2", p.Width, p.Height))
		width, height = p.Width, p.Height
	}
	args = append(args, partial)
	cmd := exec.CommandContext(ctx, s.ffmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if e = cmd.Run(); e != nil {
		return fmt.Errorf("optimization failed: %s", strings.TrimSpace(stderr.String()))
	}
	_, _ = s.db.ExecContext(ctx, "UPDATE background_jobs SET progress=.85 WHERE id=?", jobID)
	if e = os.Rename(partial, final); e != nil {
		return e
	}
	st, e := os.Stat(final)
	if e != nil {
		return e
	}
	sum, e := checksum(final)
	if e != nil {
		return e
	}
	now := stamp(s.now())
	suffix := map[bool]string{true: "movies", false: "tv"}[logicalType == "MOVIE"]
	libraryID := "vynode-optimized-" + suffix
	sourceKey := libraryID + "-source"
	libraryType := map[bool]string{true: "MOVIES", false: "TV"}[logicalType == "MOVIE"]
	_, _ = s.db.ExecContext(ctx, "INSERT OR IGNORE INTO libraries(id,name,type,created_at,updated_at) VALUES(?,'VyNode Optimized',?,?,?)", libraryID, libraryType, now, now)
	_, _ = s.db.ExecContext(ctx, "INSERT OR IGNORE INTO library_sources(id,library_id,configured_path,normalized_path,created_at) VALUES(?,?,?,?,?)", sourceKey, libraryID, s.optimized, s.optimized, now)
	relative, _ := filepath.Rel(s.optimized, final)
	fileID := "optimized-" + jobID
	assocID := "optimized-assoc-" + jobID
	resolution := fmt.Sprintf("%dp", p.Height)
	if p.Height == 0 {
		resolution = "optimized"
	}
	_, e = s.db.ExecContext(ctx, "INSERT INTO media_files(id,source_id,relative_path,file_name,base_name,extension,parent_path,size_bytes,modified_at_ns,availability,probe_status,container_format,duration_seconds,bitrate,resolution_class,hdr_class,created_at,updated_at) VALUES(?,?,?,?,?,'.mp4',?,?,?,?, 'OK','mp4',?,?,?,'SDR',?,?)", fileID, sourceKey, filepath.ToSlash(relative), filepath.Base(final), strings.TrimSuffix(filepath.Base(final), ".mp4"), filepath.ToSlash(filepath.Dir(relative)), st.Size(), st.ModTime().UnixNano(), "AVAILABLE", duration, p.VideoBitrate+p.AudioBitrate, resolution, now, now)
	if e != nil {
		return e
	}
	_, _ = s.db.ExecContext(ctx, "INSERT INTO media_streams(id,media_file_id,stream_index,stream_type,codec,width,height,is_default) VALUES(?,?,0,'video','h264',?,?,1)", fileID+"-v", fileID, width, height)
	_, _ = s.db.ExecContext(ctx, "INSERT INTO media_streams(id,media_file_id,stream_index,stream_type,codec,channels,is_default) VALUES(?,?,1,'audio','aac',2,1)", fileID+"-a", fileID)
	_, e = s.db.ExecContext(ctx, "INSERT INTO media_associations(id,media_file_id,entity_type,entity_id,association_type,version_label,created_at) VALUES(?,?,?,?, 'OPTIMIZED',?,?)", assocID, fileID, logicalType, logicalID, p.Label, now)
	if e != nil {
		return e
	}
	_, e = s.db.ExecContext(ctx, "INSERT INTO optimized_media(id,source_media_file_id,derived_media_file_id,logical_type,logical_id,profile,status,relative_path,size_bytes,checksum_sha256,job_id,created_at,completed_at) VALUES(?,?,?,?,?,?,'COMPLETED',?,?,?,?,?,?)", ident(), sourceID, fileID, logicalType, logicalID, p.ID, filepath.ToSlash(relative), st.Size(), sum, jobID, now, now)
	return e
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func (s *Service) Optimized(ctx context.Context) ([]OptimizedMedia, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,source_media_file_id,COALESCE(derived_media_file_id,''),logical_type,logical_id,profile,status,COALESCE(size_bytes,0),created_at FROM optimized_media ORDER BY created_at DESC")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []OptimizedMedia{}
	for rows.Next() {
		var x OptimizedMedia
		if e = rows.Scan(&x.ID, &x.SourceMediaFileID, &x.DerivedMediaFileID, &x.LogicalType, &x.LogicalID, &x.Profile, &x.Status, &x.SizeBytes, &x.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) DeleteOptimized(ctx context.Context, id string) error {
	var fileID, rel, status string
	if s.db.QueryRowContext(ctx, "SELECT COALESCE(derived_media_file_id,''),relative_path,status FROM optimized_media WHERE id=?", id).Scan(&fileID, &rel, &status) != nil {
		return ErrNotFound
	}
	if status != "COMPLETED" || fileID == "" {
		return ErrValidation
	}
	root, err := filepath.Abs(s.optimized)
	if err != nil {
		return err
	}
	path, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil || path == root || !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		return ErrValidation
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM media_associations WHERE media_file_id=? AND association_type='OPTIMIZED'", fileID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM media_files WHERE id=?", fileID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM optimized_media WHERE id=?", id); err != nil {
		return err
	}
	if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return tx.Commit()
}

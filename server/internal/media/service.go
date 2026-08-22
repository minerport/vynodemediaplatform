package media

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrValidation       = errors.New("validation failed")
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("conflict")
	ErrProbeUnavailable = errors.New("ffprobe unavailable")
)

var candidates = map[string]bool{".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true, ".ts": true, ".m2ts": true, ".mpg": true, ".mpeg": true, ".webm": true}
var skippedDirs = map[string]bool{".trash": true, ".trashes": true, "$recycle.bin": true, "system volume information": true, "@eaDir": true}

type Service struct {
	db                      *sql.DB
	probe                   MediaProbe
	configDir, transcodeDir string
	mu                      sync.Mutex
	running                 map[string]context.CancelFunc
}

func New(db *sql.DB, probe MediaProbe, configDir, transcodeDir string) *Service {
	return &Service{db: db, probe: probe, configDir: configDir, transcodeDir: transcodeDir, running: map[string]context.CancelFunc{}}
}
func id() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b)
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func now() string                  { return time.Now().UTC().Format(time.RFC3339Nano) }
func validType(t LibraryType) bool { return t == LibraryMovies || t == LibraryTV }

func (s *Service) ValidatePath(path string, libraryID string) (string, error) {
	if path == "" || strings.ContainsRune(path, 0) || !filepath.IsAbs(path) {
		return "", ErrValidation
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", ErrValidation
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", ErrValidation
	}
	st, err := os.Stat(resolved)
	if err != nil || !st.IsDir() {
		return "", ErrValidation
	}
	f, err := os.Open(resolved)
	if err != nil {
		return "", ErrValidation
	}
	_, readErr := f.Readdirnames(1)
	_ = f.Close()
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) && readErr.Error() != "EOF" {
		return "", ErrValidation
	}
	for _, forbidden := range []string{s.configDir, s.transcodeDir} {
		if forbidden == "" {
			continue
		}
		f, _ := filepath.Abs(filepath.Clean(forbidden))
		if sameOrChild(resolved, f) || sameOrChild(f, resolved) {
			return "", ErrValidation
		}
	}
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM library_sources WHERE library_id=? AND normalized_path=?", libraryID, filepath.Clean(resolved)).Scan(&count)
	if count > 0 {
		return "", ErrConflict
	}
	return filepath.Clean(resolved), nil
}
func sameOrChild(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && (rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."))
}

func (s *Service) CreateLibrary(ctx context.Context, name string, t LibraryType, enabled bool) (Library, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 || !validType(t) {
		return Library{}, ErrValidation
	}
	x := Library{ID: id(), Name: name, Type: t, Enabled: enabled, CreatedAt: now()}
	x.UpdatedAt = x.CreatedAt
	_, err := s.db.ExecContext(ctx, "INSERT INTO libraries(id,name,type,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?)", x.ID, x.Name, x.Type, x.Enabled, x.CreatedAt, x.UpdatedAt)
	return x, err
}
func (s *Service) ListLibraries(ctx context.Context) ([]Library, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT l.id,l.name,l.type,l.enabled,l.created_at,l.updated_at,COUNT(f.id),COALESCE(SUM(CASE WHEN f.availability='AVAILABLE' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN f.availability='MISSING' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN f.probe_status='ERROR' THEN 1 ELSE 0 END),0) FROM libraries l LEFT JOIN library_sources so ON so.library_id=l.id LEFT JOIN media_files f ON f.source_id=so.id GROUP BY l.id ORDER BY l.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Library
	for rows.Next() {
		var x Library
		if err := rows.Scan(&x.ID, &x.Name, &x.Type, &x.Enabled, &x.CreatedAt, &x.UpdatedAt, &x.FileCount, &x.AvailableCount, &x.MissingCount, &x.ProbeFailureCount); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) GetLibrary(ctx context.Context, libraryID string) (Library, error) {
	var x Library
	err := s.db.QueryRowContext(ctx, "SELECT id,name,type,enabled,created_at,updated_at FROM libraries WHERE id=?", libraryID).Scan(&x.ID, &x.Name, &x.Type, &x.Enabled, &x.CreatedAt, &x.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return x, ErrNotFound
	}
	if err != nil {
		return x, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,library_id,configured_path,normalized_path,enabled,created_at,last_attempted_scan_at,last_successful_scan_at,last_scan_status,last_scan_error FROM library_sources WHERE library_id=? ORDER BY configured_path", libraryID)
	if err != nil {
		return x, err
	}
	defer rows.Close()
	for rows.Next() {
		var v Source
		var a, b, st, e sql.NullString
		if err = rows.Scan(&v.ID, &v.LibraryID, &v.ConfiguredPath, &v.NormalizedPath, &v.Enabled, &v.CreatedAt, &a, &b, &st, &e); err != nil {
			return x, err
		}
		v.LastAttemptedScanAt = ptr(a)
		v.LastSuccessfulScanAt = ptr(b)
		v.LastScanStatus = ptr(st)
		v.LastScanError = ptr(e)
		x.Sources = append(x.Sources, v)
	}
	return x, rows.Err()
}
func ptr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
func (s *Service) UpdateLibrary(ctx context.Context, libraryID, name string, enabled *bool) (Library, error) {
	x, err := s.GetLibrary(ctx, libraryID)
	if err != nil {
		return x, err
	}
	if strings.TrimSpace(name) != "" {
		x.Name = strings.TrimSpace(name)
	}
	if enabled != nil {
		x.Enabled = *enabled
	}
	x.UpdatedAt = now()
	_, err = s.db.ExecContext(ctx, "UPDATE libraries SET name=?,enabled=?,updated_at=? WHERE id=?", x.Name, x.Enabled, x.UpdatedAt, x.ID)
	return x, err
}
func (s *Service) DeleteLibrary(ctx context.Context, libraryID string) error {
	r, err := s.db.ExecContext(ctx, "DELETE FROM libraries WHERE id=?", libraryID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Service) AddSource(ctx context.Context, libraryID, path string) (Source, error) {
	if _, err := s.GetLibrary(ctx, libraryID); err != nil {
		return Source{}, err
	}
	normalized, err := s.ValidatePath(path, libraryID)
	if err != nil {
		return Source{}, err
	}
	x := Source{ID: id(), LibraryID: libraryID, ConfiguredPath: path, NormalizedPath: normalized, Enabled: true, CreatedAt: now()}
	_, err = s.db.ExecContext(ctx, "INSERT INTO library_sources(id,library_id,configured_path,normalized_path,enabled,created_at) VALUES(?,?,?,?,1,?)", x.ID, x.LibraryID, x.ConfiguredPath, x.NormalizedPath, x.CreatedAt)
	return x, err
}
func (s *Service) RemoveSource(ctx context.Context, libraryID, sourceID string) error {
	r, err := s.db.ExecContext(ctx, "DELETE FROM library_sources WHERE id=? AND library_id=?", sourceID, libraryID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) StartScan(ctx context.Context, libraryID string) (Job, error) {
	if !s.probe.Available() {
		return Job{}, ErrProbeUnavailable
	}
	if _, err := s.GetLibrary(ctx, libraryID); err != nil {
		return Job{}, err
	}
	s.mu.Lock()
	if _, ok := s.running[libraryID]; ok {
		s.mu.Unlock()
		var j Job
		_ = s.db.QueryRowContext(ctx, "SELECT id,library_id,state,created_at FROM scan_jobs WHERE library_id=? AND state IN ('QUEUED','RUNNING') ORDER BY created_at DESC LIMIT 1", libraryID).Scan(&j.ID, &j.LibraryID, &j.State, &j.CreatedAt)
		return j, ErrConflict
	}
	runCtx, cancel := context.WithCancel(context.Background())
	s.running[libraryID] = cancel
	s.mu.Unlock()
	j := Job{ID: id(), LibraryID: libraryID, State: "QUEUED", CreatedAt: now()}
	_, err := s.db.ExecContext(ctx, "INSERT INTO scan_jobs(id,library_id,state,created_at) VALUES(?,?,?,?)", j.ID, j.LibraryID, j.State, j.CreatedAt)
	if err != nil {
		s.clear(libraryID)
		return Job{}, err
	}
	go s.scan(runCtx, j)
	return j, nil
}
func (s *Service) clear(libraryID string) { s.mu.Lock(); delete(s.running, libraryID); s.mu.Unlock() }
func (s *Service) Cancel(libraryID, jobID string) error {
	s.mu.Lock()
	cancel := s.running[libraryID]
	s.mu.Unlock()
	if cancel == nil {
		return ErrNotFound
	}
	cancel()
	_, _ = s.db.Exec("UPDATE scan_jobs SET state='CANCELED',completed_at=? WHERE id=? AND library_id=?", now(), jobID, libraryID)
	return nil
}
func (s *Service) scan(ctx context.Context, j Job) {
	defer s.clear(j.LibraryID)
	started := now()
	_, _ = s.db.Exec("UPDATE scan_jobs SET state='RUNNING',started_at=? WHERE id=?", started, j.ID)
	lib, err := s.GetLibrary(ctx, j.LibraryID)
	if err != nil {
		s.finish(j.ID, "FAILED", err.Error())
		return
	}
	hadErrors := false
	for _, source := range lib.Sources {
		if !source.Enabled {
			continue
		}
		if ctx.Err() != nil {
			s.finish(j.ID, "CANCELED", "")
			return
		}
		if err = s.scanSource(ctx, j.ID, source); err != nil {
			hadErrors = true
			s.scanError(j.ID, source.ID, "", "SOURCE_UNAVAILABLE", "Source is unavailable or unreadable.")
			_, _ = s.db.Exec("UPDATE library_sources SET last_attempted_scan_at=?,last_scan_status='FAILED',last_scan_error='Source is unavailable or unreadable.' WHERE id=?", now(), source.ID)
		}
	}
	if ctx.Err() != nil {
		s.finish(j.ID, "CANCELED", "")
		return
	}
	if hadErrors {
		s.finish(j.ID, "COMPLETED_WITH_ERRORS", "")
	} else {
		s.finish(j.ID, "COMPLETED", "")
	}
}
func (s *Service) finish(jobID, state, summary string) {
	_, _ = s.db.Exec("UPDATE scan_jobs SET state=?,completed_at=?,error_summary=? WHERE id=?", state, now(), summary, jobID)
}
func (s *Service) scanError(jobID, sourceID, rel, code, msg string) {
	if len(msg) > 512 {
		msg = msg[:512]
	}
	_, _ = s.db.Exec("INSERT INTO scan_errors(scan_job_id,source_id,relative_path,error_code,safe_message,created_at) VALUES(?,?,?,?,?,?)", jobID, sourceID, rel, code, msg, now())
	_, _ = s.db.Exec("UPDATE scan_jobs SET files_failed=files_failed+1 WHERE id=?", jobID)
}
func (s *Service) scanSource(ctx context.Context, jobID string, source Source) error {
	st, err := os.Stat(source.NormalizedPath)
	if err != nil || !st.IsDir() {
		return ErrValidation
	}
	attempt := now()
	_, _ = s.db.Exec("UPDATE library_sources SET last_attempted_scan_at=?,last_scan_status='RUNNING',last_scan_error=NULL WHERE id=?", attempt, source.ID)
	walkErr := filepath.WalkDir(source.NormalizedPath, func(path string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			s.scanError(jobID, source.ID, "", "FILESYSTEM_ERROR", "A filesystem entry could not be read.")
			return nil
		}
		if d.IsDir() {
			if path != source.NormalizedPath && (d.Type()&os.ModeSymlink != 0 || skippedDirs[strings.ToLower(d.Name())]) {
				return filepath.SkipDir
			}
			_, _ = s.db.Exec("UPDATE scan_jobs SET directories_visited=directories_visited+1 WHERE id=?", jobID)
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		_, _ = s.db.Exec("UPDATE scan_jobs SET files_discovered=files_discovered+1 WHERE id=?", jobID)
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !candidates[ext] {
			return nil
		}
		rel, err := filepath.Rel(source.NormalizedPath, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil
		}
		_, _ = s.db.Exec("UPDATE scan_jobs SET candidates_found=candidates_found+1,current_relative_path=? WHERE id=?", rel, jobID)
		if err = s.processFile(ctx, jobID, source, path, rel, d, ext); err != nil {
			s.scanError(jobID, source.ID, rel, "PROBE_FAILED", "Media inspection failed.")
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	result, err := s.db.Exec("UPDATE media_files SET availability='MISSING',updated_at=? WHERE source_id=? AND availability='AVAILABLE' AND (last_seen_scan_id IS NULL OR last_seen_scan_id<>?)", now(), source.ID, jobID)
	if err == nil {
		n, _ := result.RowsAffected()
		_, _ = s.db.Exec("UPDATE scan_jobs SET files_missing=files_missing+? WHERE id=?", n, jobID)
	}
	finished := now()
	_, _ = s.db.Exec("UPDATE library_sources SET last_successful_scan_at=?,last_scan_status='COMPLETED',last_scan_error=NULL WHERE id=?", finished, source.ID)
	return nil
}
func (s *Service) processFile(ctx context.Context, jobID string, source Source, path, rel string, d fs.DirEntry, ext string) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	var existingID, status string
	var size, mtime int64
	err = s.db.QueryRowContext(ctx, "SELECT id,size_bytes,modified_at_ns,availability FROM media_files WHERE source_id=? AND relative_path=?", source.ID, rel).Scan(&existingID, &size, &mtime, &status)
	if err == nil && size == info.Size() && mtime == info.ModTime().UnixNano() {
		_, err = s.db.ExecContext(ctx, "UPDATE media_files SET availability='AVAILABLE',last_seen_scan_id=?,updated_at=? WHERE id=?", jobID, now(), existingID)
		_, _ = s.db.Exec("UPDATE scan_jobs SET files_unchanged=files_unchanged+1 WHERE id=?", jobID)
		return err
	}
	isNew := errors.Is(err, sql.ErrNoRows)
	if err != nil && !isNew {
		return err
	}
	probe, err := s.probe.Probe(ctx, path)
	if err != nil {
		if !isNew {
			_, _ = s.db.Exec("UPDATE media_files SET probe_status='ERROR',probe_error=?,last_seen_scan_id=?,availability='AVAILABLE',updated_at=? WHERE id=?", "Media inspection failed.", jobID, now(), existingID)
		}
		return err
	}
	h := ParseFilename(d.Name())
	fid := existingID
	if isNew {
		fid = id()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	resolution, hdr := "OTHER", "SDR"
	for _, stream := range probe.Streams {
		if stream.Type == "VIDEO" {
			resolution = Resolution(stream.Width, stream.Height)
			if stream.ColorTransfer == "smpte2084" {
				hdr = "HDR10_OR_PQ"
			} else if stream.ColorTransfer == "arib-std-b67" {
				hdr = "HLG"
			}
			break
		}
	}
	parent := filepath.Dir(rel)
	base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
	if isNew {
		_, err = tx.ExecContext(ctx, `INSERT INTO media_files(id,source_id,relative_path,file_name,base_name,extension,parent_path,size_bytes,modified_at_ns,availability,probe_status,probe_error,container_format,duration_seconds,bitrate,resolution_class,hdr_class,candidate_title,candidate_year,season_number,episode_start,episode_end,created_at,updated_at,last_seen_scan_id) VALUES(?,?,?,?,?,?,?,?,?,'AVAILABLE','OK',NULL,?,?,?,?,?,?,?,?,?,?,?,?,?)`, fid, source.ID, rel, d.Name(), base, ext, parent, info.Size(), info.ModTime().UnixNano(), probe.ContainerFormat, probe.Duration, probe.Bitrate, resolution, hdr, h.CandidateTitle, nullInt(h.CandidateYear), nullInt(h.SeasonNumber), nullInt(h.EpisodeStart), nullInt(h.EpisodeEnd), now(), now(), jobID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE media_files SET file_name=?,base_name=?,extension=?,parent_path=?,size_bytes=?,modified_at_ns=?,availability='AVAILABLE',probe_status='OK',probe_error=NULL,container_format=?,duration_seconds=?,bitrate=?,resolution_class=?,hdr_class=?,candidate_title=?,candidate_year=?,season_number=?,episode_start=?,episode_end=?,updated_at=?,last_seen_scan_id=? WHERE id=?`, d.Name(), base, ext, parent, info.Size(), info.ModTime().UnixNano(), probe.ContainerFormat, probe.Duration, probe.Bitrate, resolution, hdr, h.CandidateTitle, nullInt(h.CandidateYear), nullInt(h.SeasonNumber), nullInt(h.EpisodeStart), nullInt(h.EpisodeEnd), now(), jobID, fid)
		if err == nil {
			_, err = tx.ExecContext(ctx, "DELETE FROM media_streams WHERE media_file_id=?", fid)
		}
	}
	if err != nil {
		return err
	}
	for _, stream := range probe.Streams {
		stream.ID = id()
		_, err = tx.ExecContext(ctx, `INSERT INTO media_streams(id,media_file_id,stream_index,stream_type,codec,profile,codec_level,width,height,pixel_format,bit_depth,frame_rate,scan_type,bitrate,language,title,is_default,is_forced,channels,channel_layout,sample_rate,color_primaries,color_transfer,color_space,color_range,hearing_impaired,commentary) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, stream.ID, fid, stream.Index, stream.Type, stream.Codec, stream.Profile, stream.Level, stream.Width, stream.Height, stream.PixelFormat, stream.BitDepth, stream.FrameRate, stream.ScanType, stream.Bitrate, stream.Language, stream.Title, stream.Default, stream.Forced, stream.Channels, stream.ChannelLayout, stream.SampleRate, stream.ColorPrimaries, stream.ColorTransfer, stream.ColorSpace, stream.ColorRange, stream.HearingImpaired, stream.Commentary)
		if err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	field := "files_updated"
	if isNew {
		field = "files_added"
	}
	_, _ = s.db.Exec("UPDATE scan_jobs SET files_probed=files_probed+1,"+field+"="+field+"+1 WHERE id=?", jobID)
	return nil
}
func nullInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func (s *Service) GetJob(ctx context.Context, libraryID, jobID string) (Job, error) {
	var j Job
	var started, done, current, summary sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id,library_id,state,created_at,started_at,completed_at,directories_visited,files_discovered,candidates_found,files_probed,files_added,files_updated,files_unchanged,files_missing,files_failed,current_relative_path,error_summary FROM scan_jobs WHERE id=? AND library_id=?`, jobID, libraryID).Scan(&j.ID, &j.LibraryID, &j.State, &j.CreatedAt, &started, &done, &j.DirectoriesVisited, &j.FilesDiscovered, &j.CandidatesFound, &j.FilesProbed, &j.FilesAdded, &j.FilesUpdated, &j.FilesUnchanged, &j.FilesMissing, &j.FilesFailed, &current, &summary)
	if errors.Is(err, sql.ErrNoRows) {
		return j, ErrNotFound
	}
	j.StartedAt = started.String
	j.CompletedAt = done.String
	j.CurrentRelativePath = current.String
	j.ErrorSummary = summary.String
	return j, err
}
func (s *Service) ListFiles(ctx context.Context, libraryID string, limit, offset int) ([]File, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.QueryContext(ctx, `SELECT f.id,f.source_id,f.relative_path,f.file_name,f.base_name,f.extension,f.parent_path,f.size_bytes,f.modified_at_ns,f.availability,f.probe_status,COALESCE(f.probe_error,''),COALESCE(f.container_format,''),COALESCE(f.duration_seconds,0),COALESCE(f.bitrate,0),COALESCE(f.resolution_class,''),COALESCE(f.hdr_class,''),COALESCE(f.candidate_title,''),COALESCE(f.candidate_year,0),COALESCE(f.season_number,0),COALESCE(f.episode_start,0),COALESCE(f.episode_end,0),f.created_at,f.updated_at FROM media_files f JOIN library_sources s ON s.id=f.source_id WHERE s.library_id=? ORDER BY f.relative_path LIMIT ? OFFSET ?`, libraryID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []File
	for rows.Next() {
		var f File
		if err = rows.Scan(&f.ID, &f.SourceID, &f.RelativePath, &f.FileName, &f.BaseName, &f.Extension, &f.ParentPath, &f.SizeBytes, &f.ModifiedAtNS, &f.Availability, &f.ProbeStatus, &f.ProbeError, &f.ContainerFormat, &f.DurationSeconds, &f.Bitrate, &f.ResolutionClass, &f.HDRClass, &f.CandidateTitle, &f.CandidateYear, &f.SeasonNumber, &f.EpisodeStart, &f.EpisodeEnd, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
func (s *Service) GetFile(ctx context.Context, fileID string) (File, error) {
	var f File
	err := s.db.QueryRowContext(ctx, `SELECT id,source_id,relative_path,file_name,base_name,extension,parent_path,size_bytes,modified_at_ns,availability,probe_status,COALESCE(probe_error,''),COALESCE(container_format,''),COALESCE(duration_seconds,0),COALESCE(bitrate,0),COALESCE(resolution_class,''),COALESCE(hdr_class,''),COALESCE(candidate_title,''),COALESCE(candidate_year,0),COALESCE(season_number,0),COALESCE(episode_start,0),COALESCE(episode_end,0),created_at,updated_at FROM media_files WHERE id=?`, fileID).Scan(&f.ID, &f.SourceID, &f.RelativePath, &f.FileName, &f.BaseName, &f.Extension, &f.ParentPath, &f.SizeBytes, &f.ModifiedAtNS, &f.Availability, &f.ProbeStatus, &f.ProbeError, &f.ContainerFormat, &f.DurationSeconds, &f.Bitrate, &f.ResolutionClass, &f.HDRClass, &f.CandidateTitle, &f.CandidateYear, &f.SeasonNumber, &f.EpisodeStart, &f.EpisodeEnd, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	if err != nil {
		return f, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,stream_index,stream_type,COALESCE(codec,''),COALESCE(profile,''),COALESCE(codec_level,0),COALESCE(width,0),COALESCE(height,0),COALESCE(pixel_format,''),COALESCE(bit_depth,0),COALESCE(frame_rate,''),COALESCE(scan_type,''),COALESCE(bitrate,0),COALESCE(language,''),COALESCE(title,''),is_default,is_forced,COALESCE(channels,0),COALESCE(channel_layout,''),COALESCE(sample_rate,0),COALESCE(color_primaries,''),COALESCE(color_transfer,''),COALESCE(color_space,''),COALESCE(color_range,''),hearing_impaired,commentary FROM media_streams WHERE media_file_id=? ORDER BY stream_index`, fileID)
	if err != nil {
		return f, err
	}
	defer rows.Close()
	for rows.Next() {
		var x Stream
		if err = rows.Scan(&x.ID, &x.Index, &x.Type, &x.Codec, &x.Profile, &x.Level, &x.Width, &x.Height, &x.PixelFormat, &x.BitDepth, &x.FrameRate, &x.ScanType, &x.Bitrate, &x.Language, &x.Title, &x.Default, &x.Forced, &x.Channels, &x.ChannelLayout, &x.SampleRate, &x.ColorPrimaries, &x.ColorTransfer, &x.ColorSpace, &x.ColorRange, &x.HearingImpaired, &x.Commentary); err != nil {
			return f, err
		}
		f.Streams = append(f.Streams, x)
	}
	return f, rows.Err()
}
func (s *Service) Capability(ctx context.Context) map[string]any {
	return map[string]any{"available": s.probe.Available(), "version": s.probe.Version(ctx)}
}
func SortedCandidateExtensions() []string {
	out := make([]string, 0, len(candidates))
	for x := range candidates {
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}
func (s *Service) Audit(ctx context.Context, actor, event, target, requestID string) {
	_, _ = s.db.ExecContext(ctx, "INSERT INTO audit_events(actor_user_id,action,target_type,target_id,request_id,metadata_json) VALUES(?,?,?,?,?,'{}')", actor, event, "library", target, requestID)
}

var _ = fmt.Sprintf

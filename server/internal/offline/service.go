package offline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vynode/media/server/internal/auth"
)

var (
	ErrInvalid  = errors.New("invalid offline request")
	ErrDenied   = errors.New("offline request denied")
	ErrNotFound = errors.New("offline resource not found")
	ErrNotReady = errors.New("offline asset not ready")
	ErrStorage  = errors.New("device or server storage limit")
)

type Service struct {
	db           *sql.DB
	root, ffmpeg string
	now          func() time.Time
	queue        chan string
	stop         chan struct{}
	done         chan struct{}
	mu           sync.Mutex
	cancels      map[string]context.CancelFunc
	emit         func(context.Context, string, map[string]any, string)
}

func New(db *sql.DB, root, ffmpeg string) (*Service, error) {
	abs, err := filepath.Abs(root)
	if err != nil || strings.TrimSpace(root) == "" {
		return nil, ErrInvalid
	}
	if err = os.MkdirAll(filepath.Join(abs, "assets"), 0700); err != nil {
		return nil, err
	}
	marker := filepath.Join(abs, ".vynode-download-cache")
	if _, err = os.Stat(marker); os.IsNotExist(err) {
		if err = os.WriteFile(marker, []byte("v1\n"), 0600); err != nil {
			return nil, err
		}
	}
	s := &Service{db: db, root: abs, ffmpeg: ffmpeg, now: time.Now, queue: make(chan string, 128), stop: make(chan struct{}), done: make(chan struct{}), cancels: map[string]context.CancelFunc{}}
	_, _ = db.Exec("UPDATE download_jobs SET state='INTERRUPTED',completed_at=CURRENT_TIMESTAMP,last_error='server restarted' WHERE state IN ('QUEUED','PREPARING'); UPDATE download_assets SET state='FAILED',last_error='server restarted' WHERE state='PREPARING'")
	s.cleanupPartials()
	go s.worker()
	rows, _ := db.Query("SELECT asset_id FROM download_jobs WHERE state='INTERRUPTED' ORDER BY created_at")
	pending := []string{}
	if rows != nil {
		for rows.Next() {
			var id string
			_ = rows.Scan(&id)
			pending = append(pending, id)
		}
		rows.Close()
	}
	for _, id := range pending {
		_, _ = db.Exec("UPDATE download_jobs SET state='QUEUED',started_at=NULL,completed_at=NULL,last_error=NULL WHERE asset_id=?; UPDATE download_assets SET state='PREPARING',last_error=NULL WHERE id=?", id, id)
		s.queue <- id
	}
	return s, nil
}
func (s *Service) ConfigureEvents(fn func(context.Context, string, map[string]any, string)) {
	s.emit = fn
}
func (s *Service) Close() {
	close(s.stop)
	s.mu.Lock()
	for _, cancel := range s.cancels {
		cancel()
	}
	s.mu.Unlock()
	<-s.done
}
func (s *Service) worker() {
	defer close(s.done)
	for {
		select {
		case id := <-s.queue:
			select {
			case <-s.stop:
				return
			default:
			}
			s.prepare(id)
		case <-s.stop:
			s.mu.Lock()
			for _, c := range s.cancels {
				c()
			}
			s.mu.Unlock()
			return
		}
	}
}
func (s *Service) cleanupPartials() {
	entries, _ := os.ReadDir(filepath.Join(s.root, "assets"))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".partial.mp4") && validID(strings.TrimSuffix(e.Name(), ".partial.mp4")) {
			_ = os.Remove(filepath.Join(s.root, "assets", e.Name()))
		}
	}
}
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func hashText(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func validID(v string) bool    { ok, _ := regexp.MatchString(`^[a-f0-9-]{36}$`, v); return ok }
func fileHash(path string) (string, int64, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", 0, e
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, e
}

func (s *Service) device(ctx context.Context, p auth.Principal) (string, error) {
	var id, kind string
	e := s.db.QueryRowContext(ctx, "SELECT d.id,d.authorization_type FROM sessions x JOIN devices d ON d.id=x.device_id WHERE x.id=? AND x.user_id=? AND x.revoked_at IS NULL AND x.expires_at>?", p.SessionID, p.UserID, stamp(s.now())).Scan(&id, &kind)
	if e != nil || kind != "PAIRED" {
		return "", ErrDenied
	}
	return id, nil
}
func (s *Service) has(ctx context.Context, p auth.Principal, kind, id string) bool {
	if p.Role == auth.RoleOwner || p.Role == auth.RoleAdmin {
		return true
	}
	var n int
	if kind == "SHOW" {
		e := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM seasons se JOIN episodes ep ON ep.season_id=se.id JOIN media_associations a ON a.entity_type='EPISODE' AND a.entity_id=ep.id JOIN media_files f ON f.id=a.media_file_id JOIN library_sources ls ON ls.id=f.source_id JOIN library_access_grants g ON g.library_id=ls.library_id AND g.user_id=? AND g.permission='DOWNLOAD' WHERE se.show_id=? AND f.availability='AVAILABLE'`, p.UserID, id).Scan(&n)
		return e == nil && n > 0
	}
	e := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_associations a JOIN media_files f ON f.id=a.media_file_id JOIN library_sources ls ON ls.id=f.source_id JOIN library_access_grants g ON g.library_id=ls.library_id AND g.user_id=? AND g.permission='DOWNLOAD' WHERE a.entity_type=? AND a.entity_id=? AND f.availability='AVAILABLE'`, p.UserID, kind, id).Scan(&n)
	return e == nil && n > 0
}
func (s *Service) Profiles(ctx context.Context) ([]QualityProfile, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,label,max_width,max_height,video_bitrate,audio_bitrate,video_codec,audio_codec,audio_channels,container,profile_version FROM download_quality_profiles ORDER BY CASE id WHEN 'ORIGINAL' THEN 0 WHEN 'HIGH' THEN 1 WHEN 'MEDIUM' THEN 2 ELSE 3 END")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []QualityProfile{}
	for rows.Next() {
		var x QualityProfile
		if e = rows.Scan(&x.ID, &x.Label, &x.MaxWidth, &x.MaxHeight, &x.VideoBitrate, &x.AudioBitrate, &x.VideoCodec, &x.AudioCodec, &x.AudioChannels, &x.Container, &x.Version); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) Settings(ctx context.Context) (Settings, error) {
	var x Settings
	err := s.db.QueryRowContext(ctx, `SELECT ds.cache_quota_bytes,COALESCE(SUM(CASE WHEN a.owned=1 THEN COALESCE(a.size_bytes,a.estimated_size_bytes) ELSE 0 END),0),COALESCE(SUM(CASE WHEN a.state='READY' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN a.state='PREPARING' THEN 1 ELSE 0 END),0) FROM download_settings ds LEFT JOIN download_assets a ON 1=1 WHERE ds.id=1 GROUP BY ds.cache_quota_bytes`).Scan(&x.CacheQuotaBytes, &x.CacheBytes, &x.ReadyAssets, &x.PreparingAssets)
	return x, err
}

func (s *Service) SetSettings(ctx context.Context, quota int64) (Settings, error) {
	if quota < 0 {
		return Settings{}, ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, "UPDATE download_settings SET cache_quota_bytes=?,updated_at=? WHERE id=1", quota, stamp(s.now()))
	if err != nil {
		return Settings{}, err
	}
	return s.Settings(ctx)
}

type source struct {
	id, path, container, vcodec, acodec string
	width, height                       int
	size, mtime                         int64
	duration                            float64
}

func (s *Service) source(ctx context.Context, kind, id string) (source, error) {
	var x source
	var root, rel string
	e := s.db.QueryRowContext(ctx, `SELECT f.id,ls.normalized_path,f.relative_path,COALESCE(f.container_format,''),COALESCE(v.codec,''),COALESCE(aud.codec,''),COALESCE(v.width,0),COALESCE(v.height,0),f.size_bytes,f.modified_at_ns,COALESCE(f.duration_seconds,0) FROM media_associations a JOIN media_files f ON f.id=a.media_file_id JOIN library_sources ls ON ls.id=f.source_id LEFT JOIN media_streams v ON v.media_file_id=f.id AND v.stream_type='video' LEFT JOIN media_streams aud ON aud.media_file_id=f.id AND aud.stream_type='audio' WHERE a.entity_type=? AND a.entity_id=? AND f.availability='AVAILABLE' AND a.association_type!='OPTIMIZED' ORDER BY f.size_bytes DESC LIMIT 1`, kind, id).Scan(&x.id, &root, &rel, &x.container, &x.vcodec, &x.acodec, &x.width, &x.height, &x.size, &x.mtime, &x.duration)
	if e == sql.ErrNoRows {
		return x, ErrNotFound
	}
	if e != nil {
		return x, e
	}
	x.path = filepath.Join(root, filepath.FromSlash(rel))
	return x, nil
}
func (s *Service) profile(ctx context.Context, id string) (QualityProfile, error) {
	var x QualityProfile
	e := s.db.QueryRowContext(ctx, "SELECT id,label,max_width,max_height,video_bitrate,audio_bitrate,video_codec,audio_codec,audio_channels,container,profile_version FROM download_quality_profiles WHERE id=?", strings.ToUpper(id)).Scan(&x.ID, &x.Label, &x.MaxWidth, &x.MaxHeight, &x.VideoBitrate, &x.AudioBitrate, &x.VideoCodec, &x.AudioCodec, &x.AudioChannels, &x.Container, &x.Version)
	if e != nil {
		return x, ErrInvalid
	}
	return x, nil
}
func optimizedProfile(id string) string {
	return map[string]string{"HIGH": "remote-1080p", "MEDIUM": "mobile-720p", "LOW": "mobile-480p"}[id]
}
func (s *Service) Plan(ctx context.Context, kind, id, profileID string) (Plan, error) {
	kind = strings.ToUpper(kind)
	p, e := s.profile(ctx, profileID)
	if e != nil || !(kind == "MOVIE" || kind == "EPISODE") {
		return Plan{}, ErrInvalid
	}
	src, e := s.source(ctx, kind, id)
	if e != nil {
		return Plan{}, e
	}
	plan := Plan{LogicalType: kind, LogicalID: id, SourceMediaFileID: src.id, ProfileID: p.ID, ProfileVersion: p.Version, SourceContainer: src.container, SourceVideoCodec: src.vcodec, SourceAudioCodec: src.acodec, SourceWidth: src.width, SourceHeight: src.height, OutputContainer: p.Container, OutputVideoCodec: p.VideoCodec, OutputAudioCodec: p.AudioCodec, OutputWidth: p.MaxWidth, OutputHeight: p.MaxHeight, OutputVideoBitrate: p.VideoBitrate, OutputAudioBitrate: p.AudioBitrate}
	if p.ID == "ORIGINAL" {
		plan.Mode = "ORIGINAL_COPY"
		plan.Reason = "ORIGINAL_REQUESTED"
		return plan, nil
	}
	var n int
	if op := optimizedProfile(p.ID); op != "" && s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM optimized_media WHERE source_media_file_id=? AND profile=? AND status='COMPLETED'", src.id, op).Scan(&n) == nil && n > 0 {
		plan.Mode = "EXISTING_OPTIMIZED_VERSION"
		plan.Reason = "COMPATIBLE_OPTIMIZED_VERSION_EXISTS"
		return plan, nil
	}
	plan.Mode = "GENERATED_OFFLINE_VERSION"
	plan.Reason = "DEVICE_PROFILE_REQUIRES_COMPATIBLE_VERSION"
	return plan, nil
}

func (s *Service) Create(ctx context.Context, p auth.Principal, in CreateRequest) (Download, error) {
	device, e := s.device(ctx, p)
	if e != nil {
		return Download{}, e
	}
	in.LogicalType = strings.ToUpper(in.LogicalType)
	in.ProfileID = strings.ToUpper(in.ProfileID)
	if !s.has(ctx, p, in.LogicalType, in.LogicalID) {
		return Download{}, ErrDenied
	}
	plan, e := s.Plan(ctx, in.LogicalType, in.LogicalID, in.ProfileID)
	if e != nil {
		return Download{}, e
	}
	src, e := s.source(ctx, in.LogicalType, in.LogicalID)
	if e != nil {
		return Download{}, e
	}
	estimate := src.size
	if plan.Mode == "GENERATED_OFFLINE_VERSION" {
		estimate = int64(src.duration*float64(plan.OutputVideoBitrate+plan.OutputAudioBitrate)/8) + 8*1024*1024
	}
	var available, minimum int64
	if s.db.QueryRowContext(ctx, "SELECT available_bytes,minimum_free_bytes FROM device_storage_reports WHERE device_id=?", device).Scan(&available, &minimum) == nil && available-minimum < estimate {
		return Download{}, ErrStorage
	}
	if plan.Mode == "GENERATED_OFFLINE_VERSION" && !s.cacheFits(ctx, estimate) {
		return Download{}, ErrStorage
	}
	identity := hashText(fmt.Sprintf("%s:%d:%d:%s:%d", src.id, src.size, src.mtime, in.ProfileID, plan.ProfileVersion))
	now := stamp(s.now())
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return Download{}, e
	}
	defer tx.Rollback()
	var asset, state string
	e = tx.QueryRowContext(ctx, "SELECT id,state FROM download_assets WHERE identity_key=?", identity).Scan(&asset, &state)
	created := false
	retry := false
	if e == sql.ErrNoRows {
		asset = auth.ID()
		state = "PREPARING"
		owned := 1
		var opt any
		if plan.Mode != "GENERATED_OFFLINE_VERSION" {
			state = "READY"
			owned = 0
		}
		if plan.Mode == "EXISTING_OPTIMIZED_VERSION" {
			_ = tx.QueryRowContext(ctx, "SELECT id FROM optimized_media WHERE source_media_file_id=? AND profile=? AND status='COMPLETED' LIMIT 1", src.id, optimizedProfile(in.ProfileID)).Scan(&opt)
		}
		contentType := "video/mp4"
		if plan.Mode == "ORIGINAL_COPY" {
			contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(src.path)))
			if contentType == "" {
				contentType = "application/octet-stream"
			}
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO download_assets(id,identity_key,source_media_file_id,source_size_bytes,source_modified_at_ns,optimized_media_id,profile_id,profile_version,mode,state,owned,estimated_size_bytes,content_type,duration_seconds,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", asset, identity, src.id, src.size, src.mtime, opt, in.ProfileID, plan.ProfileVersion, plan.Mode, state, owned, estimate, contentType, src.duration, now)
		created = e == nil
	} else if e != nil {
		return Download{}, e
	} else if plan.Mode == "GENERATED_OFFLINE_VERSION" && (state == "FAILED" || state == "CANCELED" || state == "STALE") {
		state = "PREPARING"
		retry = true
		_, e = tx.ExecContext(ctx, "UPDATE download_assets SET state='PREPARING',last_error=NULL WHERE id=?; INSERT INTO download_jobs(id,asset_id,state,priority,progress,created_at,started_at,completed_at,last_error) VALUES(?,?,'QUEUED',20,0,?,NULL,NULL,NULL) ON CONFLICT(asset_id) DO UPDATE SET state='QUEUED',progress=0,started_at=NULL,completed_at=NULL,last_error=NULL", asset, auth.ID(), asset, now)
	}
	id := auth.ID()
	status := map[bool]string{true: "PREPARING", false: "READY"}[state == "PREPARING"]
	_, e = tx.ExecContext(ctx, "INSERT INTO device_downloads(id,user_id,device_id,logical_type,logical_id,asset_id,profile_id,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(device_id,logical_type,logical_id,profile_id) DO UPDATE SET asset_id=CASE WHEN device_downloads.asset_id!=excluded.asset_id OR device_downloads.status IN ('REMOVED','REVOKED','FAILED','STALE') THEN excluded.asset_id ELSE device_downloads.asset_id END,status=CASE WHEN device_downloads.asset_id!=excluded.asset_id OR device_downloads.status IN ('REMOVED','REVOKED','FAILED','STALE') THEN excluded.status ELSE device_downloads.status END,updated_at=excluded.updated_at RETURNING id", id, p.UserID, device, in.LogicalType, in.LogicalID, asset, in.ProfileID, status, now, now)
	if e != nil {
		return Download{}, e
	}
	_ = tx.QueryRowContext(ctx, "SELECT id FROM device_downloads WHERE device_id=? AND logical_type=? AND logical_id=? AND profile_id=?", device, in.LogicalType, in.LogicalID, in.ProfileID).Scan(&id)
	if created && state == "PREPARING" {
		job := auth.ID()
		_, e = tx.ExecContext(ctx, "INSERT INTO download_jobs(id,asset_id,state,priority,created_at) VALUES(?,?,'QUEUED',20,?)", job, asset, now)
	}
	if e == nil {
		e = s.changeTx(ctx, tx, p.UserID, device, "DOWNLOAD_ASSIGNED", "DOWNLOAD", id, map[string]any{"downloadId": id, "assetId": asset})
	}
	if e = tx.Commit(); e != nil {
		return Download{}, e
	}
	if created && state == "READY" {
		if e = s.finalizeExisting(ctx, asset, plan.Mode, src.path); e != nil {
			return Download{}, e
		}
	}
	if (created || retry) && state == "PREPARING" {
		s.queue <- asset
	}
	return s.Get(ctx, p, id)
}

func (s *Service) cacheFits(ctx context.Context, estimate int64) bool {
	var quota, used int64
	if s.db.QueryRowContext(ctx, "SELECT cache_quota_bytes FROM download_settings WHERE id=1").Scan(&quota) != nil || quota == 0 {
		return true
	}
	_ = s.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(CASE WHEN size_bytes IS NOT NULL THEN size_bytes ELSE estimated_size_bytes END),0) FROM download_assets WHERE owned=1 AND state IN ('PREPARING','READY')").Scan(&used)
	fits := used+estimate <= quota
	if !fits {
		s.emitEvent(ctx, "DOWNLOAD_CACHE_LOW_SPACE", map[string]any{"quotaBytes": quota, "usedBytes": used, "requestedBytes": estimate}, fmt.Sprintf("download-cache-low:%d", quota))
	}
	return fits
}
func (s *Service) finalizeExisting(ctx context.Context, asset, mode, sourcePath string) error {
	path := sourcePath
	if mode == "EXISTING_OPTIMIZED_VERSION" {
		var root, rel string
		if e := s.db.QueryRowContext(ctx, `SELECT ls.normalized_path,f.relative_path FROM download_assets da JOIN optimized_media o ON o.id=da.optimized_media_id JOIN media_files f ON f.id=o.derived_media_file_id JOIN library_sources ls ON ls.id=f.source_id WHERE da.id=?`, asset).Scan(&root, &rel); e != nil {
			return e
		}
		path = filepath.Join(root, filepath.FromSlash(rel))
	}
	sum, n, e := fileHash(path)
	if e != nil {
		return e
	}
	_, e = s.db.ExecContext(ctx, "UPDATE download_assets SET size_bytes=?,checksum_sha256=?,ready_at=?,state='READY' WHERE id=?", n, sum, stamp(s.now()), asset)
	return e
}
func (s *Service) prepare(asset string) {
	var active int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM playback_sessions WHERE mode IN ('AUDIO_TRANSCODE','VIDEO_TRANSCODE') AND state IN ('STARTING','PLAYING','PAUSED')").Scan(&active)
	if active > 0 {
		select {
		case <-time.After(time.Second):
			select {
			case s.queue <- asset:
			case <-s.stop:
			}
		case <-s.stop:
		}
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancels[asset] = cancel
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.cancels, asset); s.mu.Unlock() }()
	var job, sourceID, profileID string
	if s.db.QueryRow("SELECT j.id,a.source_media_file_id,a.profile_id FROM download_jobs j JOIN download_assets a ON a.id=j.asset_id WHERE a.id=? AND j.state='QUEUED'", asset).Scan(&job, &sourceID, &profileID) != nil {
		return
	}
	now := stamp(s.now())
	if tx, txErr := s.db.Begin(); txErr == nil {
		if _, txErr = tx.Exec("UPDATE download_jobs SET state='PREPARING',started_at=? WHERE id=?", now, job); txErr == nil {
			_, txErr = tx.Exec("UPDATE download_assets SET state='PREPARING' WHERE id=?", asset)
		}
		if txErr == nil {
			txErr = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		if txErr != nil {
			return
		}
	} else {
		return
	}
	var root, rel string
	if s.db.QueryRow("SELECT ls.normalized_path,f.relative_path FROM media_files f JOIN library_sources ls ON ls.id=f.source_id WHERE f.id=?", sourceID).Scan(&root, &rel) != nil {
		return
	}
	p, err := s.profile(ctx, profileID)
	if err != nil {
		return
	}
	partial := filepath.Join(s.root, "assets", asset+".partial.mp4")
	final := filepath.Join(s.root, "assets", asset+".mp4")
	args := []string{"-v", "error", "-y", "-i", filepath.Join(root, filepath.FromSlash(rel)), "-map", "0:v:0", "-map", "0:a:0?", "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-vf", fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease:force_divisible_by=2", p.MaxWidth, p.MaxHeight), "-b:v", strconv.FormatInt(p.VideoBitrate, 10), "-c:a", "aac", "-ac", strconv.Itoa(p.AudioChannels), "-b:a", strconv.FormatInt(p.AudioBitrate, 10), "-movflags", "+faststart", partial}
	cmd := exec.CommandContext(ctx, s.ffmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil {
		err = os.Rename(partial, final)
	}
	if err == nil {
		sum, n, hashErr := fileHash(final)
		err = hashErr
		if err == nil {
			tx, txErr := s.db.Begin()
			if txErr == nil {
				readyAt := stamp(s.now())
				if _, txErr = tx.Exec("UPDATE download_assets SET state='READY',relative_path=?,size_bytes=?,checksum_sha256=?,ready_at=?,last_error=NULL WHERE id=?", filepath.ToSlash(filepath.Join("assets", asset+".mp4")), n, sum, readyAt, asset); txErr == nil {
					_, txErr = tx.Exec("UPDATE download_jobs SET state='READY',progress=1,output_bytes=?,completed_at=? WHERE id=?", n, readyAt, job)
				}
				if txErr == nil {
					_, txErr = tx.Exec("UPDATE device_downloads SET status='READY',updated_at=? WHERE asset_id=? AND status='PREPARING'", readyAt, asset)
				}
				if txErr == nil {
					txErr = tx.Commit()
				} else {
					_ = tx.Rollback()
				}
			}
			err = txErr
			if err == nil {
				s.emitEvent(ctx, "DOWNLOAD_READY", map[string]any{"assetId": asset}, "download-ready:"+asset)
			}
		}
	}
	if err != nil {
		_ = os.Remove(partial)
		state := "FAILED"
		if errors.Is(ctx.Err(), context.Canceled) {
			state = "CANCELED"
		}
		summary := strings.TrimSpace(stderr.String())
		if len(summary) > 300 {
			summary = summary[:300]
		}
		if tx, txErr := s.db.Begin(); txErr == nil {
			failedAt := stamp(s.now())
			if _, txErr = tx.Exec("UPDATE download_assets SET state=?,last_error=? WHERE id=?", state, summary, asset); txErr == nil {
				_, txErr = tx.Exec("UPDATE download_jobs SET state=?,last_error=?,completed_at=? WHERE id=?", state, summary, failedAt, job)
			}
			if txErr == nil {
				_, txErr = tx.Exec("UPDATE device_downloads SET status='FAILED',updated_at=? WHERE asset_id=? AND status='PREPARING'", failedAt, asset)
			}
			if txErr == nil {
				_ = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
		}
		s.emitEvent(ctx, "DOWNLOAD_FAILED", map[string]any{"assetId": asset}, "download-failed:"+asset)
	}
}
func (s *Service) emitEvent(ctx context.Context, t string, p map[string]any, d string) {
	if s.emit != nil {
		s.emit(ctx, t, p, d)
	}
}

func (s *Service) Get(ctx context.Context, p auth.Principal, id string) (Download, error) {
	device, e := s.device(ctx, p)
	if e != nil {
		return Download{}, e
	}
	var x Download
	e = s.db.QueryRowContext(ctx, `SELECT d.id,d.user_id,d.device_id,d.logical_type,d.logical_id,d.asset_id,d.profile_id,d.status,a.mode,a.state,COALESCE(a.size_bytes,0),COALESCE(a.checksum_sha256,''),a.content_type,d.transfer_bytes,d.created_at,d.updated_at,COALESCE(j.progress,CASE WHEN a.state='READY' THEN 1 ELSE 0 END) FROM device_downloads d JOIN download_assets a ON a.id=d.asset_id LEFT JOIN download_jobs j ON j.asset_id=a.id WHERE d.id=? AND d.user_id=? AND d.device_id=?`, id, p.UserID, device).Scan(&x.ID, &x.UserID, &x.DeviceID, &x.LogicalType, &x.LogicalID, &x.AssetID, &x.ProfileID, &x.Status, &x.Mode, &x.AssetState, &x.SizeBytes, &x.ChecksumSHA256, &x.ContentType, &x.TransferBytes, &x.CreatedAt, &x.UpdatedAt, &x.Progress)
	if e != nil {
		return x, ErrNotFound
	}
	x.Plan, _ = s.Plan(ctx, x.LogicalType, x.LogicalID, x.ProfileID)
	return x, nil
}
func (s *Service) List(ctx context.Context, p auth.Principal) ([]Download, error) {
	device, e := s.device(ctx, p)
	if e != nil {
		return nil, e
	}
	rows, e := s.db.QueryContext(ctx, "SELECT id FROM device_downloads WHERE user_id=? AND device_id=? AND status!='REMOVED' ORDER BY created_at DESC", p.UserID, device)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	err := rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []Download{}
	for _, id := range ids {
		if x, e := s.Get(ctx, p, id); e == nil {
			out = append(out, x)
		}
	}
	return out, nil
}
func (s *Service) Delete(ctx context.Context, p auth.Principal, id string) error {
	x, e := s.Get(ctx, p, id)
	if e != nil {
		return e
	}
	now := stamp(s.now())
	_, e = s.db.ExecContext(ctx, "UPDATE device_downloads SET status='REMOVAL_REQUESTED',updated_at=? WHERE id=?", now, id)
	if e == nil {
		_, _ = s.db.ExecContext(ctx, "INSERT INTO sync_changes(user_id,device_id,change_type,entity_type,entity_id,payload_json,created_at) VALUES(?,?, 'DOWNLOAD_REMOVED','DOWNLOAD',?,'{}',?)", p.UserID, x.DeviceID, id, now)
	}
	return e
}
func (s *Service) Cancel(ctx context.Context, p auth.Principal, id string) error {
	x, e := s.Get(ctx, p, id)
	if e != nil {
		return e
	}
	s.mu.Lock()
	c := s.cancels[x.AssetID]
	s.mu.Unlock()
	if c != nil {
		c()
	}
	_, e = s.db.ExecContext(ctx, "UPDATE download_jobs SET state='CANCELED',completed_at=? WHERE asset_id=? AND state IN ('QUEUED','PREPARING')", stamp(s.now()), x.AssetID)
	return e
}
func (s *Service) Path(ctx context.Context, p auth.Principal, id string) (Download, string, error) {
	x, e := s.Get(ctx, p, id)
	if e != nil {
		return x, "", e
	}
	if x.Status == "REVOKED" || !s.has(ctx, p, x.LogicalType, x.LogicalID) {
		return x, "", ErrDenied
	}
	if x.AssetState != "READY" {
		return x, "", ErrNotReady
	}
	var rel, source, opt string
	var sourceSize, sourceMTime, currentSize, currentMTime int64
	e = s.db.QueryRowContext(ctx, "SELECT COALESCE(a.relative_path,''),a.source_media_file_id,COALESCE(a.optimized_media_id,''),a.source_size_bytes,a.source_modified_at_ns,f.size_bytes,f.modified_at_ns FROM download_assets a JOIN media_files f ON f.id=a.source_media_file_id WHERE a.id=?", x.AssetID).Scan(&rel, &source, &opt, &sourceSize, &sourceMTime, &currentSize, &currentMTime)
	if e != nil {
		return x, "", e
	}
	if sourceSize != currentSize || sourceMTime != currentMTime {
		_, _ = s.db.ExecContext(ctx, "UPDATE download_assets SET state='STALE',last_error='source identity changed' WHERE id=?; UPDATE device_downloads SET status='STALE',updated_at=? WHERE asset_id=? AND status NOT IN ('REMOVED','REVOKED')", x.AssetID, stamp(s.now()), x.AssetID)
		return x, "", ErrNotReady
	}
	if x.Mode == "GENERATED_OFFLINE_VERSION" {
		path := filepath.Join(s.root, filepath.FromSlash(rel))
		abs, _ := filepath.Abs(path)
		if !strings.HasPrefix(abs, s.root+string(os.PathSeparator)) {
			return x, "", ErrDenied
		}
		return x, abs, nil
	}
	var root, mrel string
	if opt != "" {
		e = s.db.QueryRowContext(ctx, "SELECT ls.normalized_path,f.relative_path FROM optimized_media o JOIN media_files f ON f.id=o.derived_media_file_id JOIN library_sources ls ON ls.id=f.source_id WHERE o.id=?", opt).Scan(&root, &mrel)
	} else {
		e = s.db.QueryRowContext(ctx, "SELECT ls.normalized_path,f.relative_path FROM media_files f JOIN library_sources ls ON ls.id=f.source_id WHERE f.id=?", source).Scan(&root, &mrel)
	}
	return x, filepath.Join(root, filepath.FromSlash(mrel)), e
}

func SafeFilename(v string) string {
	v = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\\|?*`, r) {
			return '_'
		}
		return r
	}, strings.TrimSpace(v))
	v = strings.Trim(v, " .")
	base := strings.ToUpper(strings.TrimSuffix(v, filepath.Ext(v)))
	reserved := map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true, "COM1": true, "LPT1": true}
	if reserved[base] {
		v = "_" + v
	}
	if len([]rune(v)) > 120 {
		v = string([]rune(v)[:120])
	}
	if v == "" {
		v = "VyNode Download"
	}
	return v
}
func (s *Service) Manifest(ctx context.Context, p auth.Principal, id string) (Manifest, error) {
	x, e := s.Get(ctx, p, id)
	if e != nil {
		return Manifest{}, e
	}
	m := Manifest{SchemaVersion: 1, Download: x, FileURL: "/api/v1/downloads/" + id + "/file"}
	var title, overview string
	var year int
	if x.LogicalType == "MOVIE" {
		_ = s.db.QueryRowContext(ctx, "SELECT title,COALESCE(year,0),COALESCE(overview,'') FROM movies WHERE id=?", x.LogicalID).Scan(&title, &year, &overview)
	} else {
		var show string
		var season, episode int
		_ = s.db.QueryRowContext(ctx, "SELECT s.title,se.season_number,e.episode_number,e.title,COALESCE(e.overview,'') FROM episodes e JOIN seasons se ON se.id=e.season_id JOIN shows s ON s.id=se.show_id WHERE e.id=?", x.LogicalID).Scan(&show, &season, &episode, &title, &overview)
		title = fmt.Sprintf("%s - S%02dE%02d - %s", show, season, episode, title)
	}
	m.Title, m.Year, m.Overview = title, year, overview
	ext := ".mp4"
	if x.Mode == "ORIGINAL_COPY" {
		var rel string
		_ = s.db.QueryRowContext(ctx, "SELECT f.relative_path FROM download_assets a JOIN media_files f ON f.id=a.source_media_file_id WHERE a.id=?", x.AssetID).Scan(&rel)
		if candidate := filepath.Ext(rel); candidate != "" {
			ext = candidate
		}
	}
	m.SuggestedName = SafeFilename(title) + ext
	_ = s.db.QueryRowContext(ctx, "SELECT duration_seconds FROM download_assets WHERE id=?", x.AssetID).Scan(&m.Duration)
	_ = s.db.QueryRowContext(ctx, "SELECT position_seconds,watched FROM user_media_progress WHERE user_id=? AND logical_type=? AND logical_id=?", p.UserID, x.LogicalType, x.LogicalID).Scan(&m.Position, &m.Watched)
	artKind, artID := x.LogicalType, x.LogicalID
	if x.LogicalType == "EPISODE" {
		artKind = "SHOW"
		_ = s.db.QueryRowContext(ctx, "SELECT se.show_id FROM episodes ep JOIN seasons se ON se.id=ep.season_id WHERE ep.id=?", x.LogicalID).Scan(&artID)
	}
	revisionParts := []string{title, strconv.Itoa(year), overview}
	artRows, _ := s.db.QueryContext(ctx, "SELECT id,artwork_type,COALESCE(etag,''),COALESCE(mime_type,'') FROM artwork WHERE entity_type=? AND entity_id=? AND selected=1 AND cached_relative_path IS NOT NULL ORDER BY artwork_type", artKind, artID)
	if artRows != nil {
		for artRows.Next() {
			var a Artwork
			_ = artRows.Scan(&a.ID, &a.Type, &a.ETag, &a.MimeType)
			a.URL = "/api/v1/artwork/" + a.ID + "/content"
			m.Artwork = append(m.Artwork, a)
			m.ArtworkURLs = append(m.ArtworkURLs, a.URL)
			revisionParts = append(revisionParts, a.ID, a.ETag)
		}
		artRows.Close()
	}
	m.PresentationRevision = hashText(strings.Join(revisionParts, "\x00"))
	return m, nil
}
func (s *Service) changeTx(ctx context.Context, tx *sql.Tx, user, device, t, et, id string, payload any) error {
	b, _ := json.Marshal(payload)
	_, e := tx.ExecContext(ctx, "INSERT INTO sync_changes(user_id,device_id,change_type,entity_type,entity_id,payload_json,created_at) VALUES(?,?,?,?,?,?,?)", user, device, t, et, id, string(b), stamp(s.now()))
	return e
}

func (s *Service) Push(ctx context.Context, p auth.Principal, in Push) (SyncState, error) {
	device, e := s.device(ctx, p)
	if e != nil {
		return SyncState{}, e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return SyncState{}, e
	}
	defer tx.Rollback()
	now := stamp(s.now())
	if in.Storage != nil {
		r := in.Storage
		if r.TotalBytes < 0 || r.AvailableBytes < 0 || r.VyNodeBytes < 0 || r.MinimumFreeBytes < 0 || r.AvailableBytes > r.TotalBytes {
			return SyncState{}, ErrInvalid
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO device_storage_reports(device_id,user_id,total_bytes,available_bytes,vynode_bytes,minimum_free_bytes,reported_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET total_bytes=excluded.total_bytes,available_bytes=excluded.available_bytes,vynode_bytes=excluded.vynode_bytes,minimum_free_bytes=excluded.minimum_free_bytes,reported_at=excluded.reported_at`, device, p.UserID, r.TotalBytes, r.AvailableBytes, r.VyNodeBytes, r.MinimumFreeBytes, now)
	}
	for _, i := range in.Inventory {
		if e != nil {
			break
		}
		if !map[string]bool{"READY": true, "DOWNLOADING": true, "DOWNLOADED": true, "REMOVED": true, "FAILED": true, "STALE": true}[i.State] {
			return SyncState{}, ErrInvalid
		}
		_, e = tx.ExecContext(ctx, "UPDATE device_downloads SET status=?,transfer_bytes=?,downloaded_at=COALESCE(NULLIF(?,''),downloaded_at),last_verified_at=COALESCE(NULLIF(?,''),last_verified_at),updated_at=? WHERE id=? AND asset_id=? AND user_id=? AND device_id=?", i.State, i.SizeBytes, i.DownloadedAt, i.LastVerifiedAt, now, i.DownloadID, i.AssetID, p.UserID, device)
	}
	for _, ev := range in.Progress {
		if e != nil {
			break
		}
		e = s.applyProgress(ctx, tx, p.UserID, device, ev, now)
	}
	if e != nil {
		return SyncState{}, e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO device_sync_state(device_id,user_id,last_sync_at,updated_at) VALUES(?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET last_sync_at=excluded.last_sync_at,updated_at=excluded.updated_at`, device, p.UserID, now, now)
	if e = tx.Commit(); e != nil {
		return SyncState{}, e
	}
	s.cleanupAssets(ctx)
	_ = s.FillSubscriptions(ctx, p)
	return s.Pull(ctx, p, 0, 100)
}

func (s *Service) cleanupAssets(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,COALESCE(relative_path,'') FROM download_assets a WHERE owned=1 AND NOT EXISTS(SELECT 1 FROM device_downloads d WHERE d.asset_id=a.id AND d.status NOT IN ('REMOVED','REVOKED'))`)
	if err != nil {
		return
	}
	type orphan struct{ id, rel string }
	orphans := []orphan{}
	for rows.Next() {
		var x orphan
		_ = rows.Scan(&x.id, &x.rel)
		orphans = append(orphans, x)
	}
	rows.Close()
	for _, x := range orphans {
		if validID(x.id) && x.rel == filepath.ToSlash(filepath.Join("assets", x.id+".mp4")) {
			tx, err := s.db.BeginTx(ctx, nil)
			if err != nil {
				continue
			}
			_, err = tx.ExecContext(ctx, "DELETE FROM device_downloads WHERE asset_id=? AND status IN ('REMOVED','REVOKED')", x.id)
			if err == nil {
				_, err = tx.ExecContext(ctx, "DELETE FROM download_assets WHERE id=?", x.id)
			}
			if err == nil {
				err = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
			if err == nil {
				_ = os.Remove(filepath.Join(s.root, filepath.FromSlash(x.rel)))
			}
		}
	}
}
func (s *Service) applyProgress(ctx context.Context, tx *sql.Tx, user, device string, ev ProgressEvent, received string) error {
	if ev.EventID == "" || ev.SequenceEpoch == "" || ev.DeviceSequence < 1 || ev.Position < 0 || ev.Duration < 0 || !(ev.LogicalType == "MOVIE" || ev.LogicalType == "EPISODE") {
		return ErrInvalid
	}
	var authorized int
	e := tx.QueryRowContext(ctx, `SELECT CASE WHEN EXISTS(SELECT 1 FROM users WHERE id=? AND role IN ('OWNER','ADMIN')) OR EXISTS(SELECT 1 FROM media_associations a JOIN media_files f ON f.id=a.media_file_id JOIN library_sources ls ON ls.id=f.source_id JOIN library_access_grants g ON g.library_id=ls.library_id AND g.user_id=? AND g.permission='DOWNLOAD' WHERE a.entity_type=? AND a.entity_id=? AND f.availability='AVAILABLE') THEN 1 ELSE 0 END`, user, user, ev.LogicalType, ev.LogicalID).Scan(&authorized)
	if e != nil || authorized == 0 {
		return ErrDenied
	}
	r, e := tx.ExecContext(ctx, "INSERT OR IGNORE INTO offline_progress_events(device_id,user_id,event_id,sequence_epoch,device_sequence,logical_type,logical_id,position_seconds,duration_seconds,watched,explicit_action,occurred_at,received_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)", device, user, ev.EventID, ev.SequenceEpoch, ev.DeviceSequence, ev.LogicalType, ev.LogicalID, ev.Position, ev.Duration, ev.Watched, ev.ExplicitAction, ev.OccurredAt, received)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return nil
	}
	var pos, dur float64
	var watched bool
	var last string
	_ = tx.QueryRowContext(ctx, "SELECT position_seconds,duration_seconds,watched,last_played_at FROM user_media_progress WHERE user_id=? AND logical_type=? AND logical_id=?", user, ev.LogicalType, ev.LogicalID).Scan(&pos, &dur, &watched, &last)
	apply := false
	newWatched := watched
	if ev.ExplicitAction == "UNWATCHED" {
		apply = true
		newWatched = false
	} else if ev.ExplicitAction == "WATCHED" || ev.Watched {
		apply = true
		newWatched = true
	} else if !watched {
		occurred, oe := time.Parse(time.RFC3339Nano, ev.OccurredAt)
		previous, pe := time.Parse(time.RFC3339Nano, last)
		if oe == nil && pe == nil && occurred.After(previous) && occurred.Before(s.now().Add(24*time.Hour)) {
			apply = true
		} else if ev.Position > pos {
			apply = true
		}
	}
	if apply {
		if newWatched {
			ev.Position = 0
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO user_media_progress(user_id,logical_type,logical_id,position_seconds,duration_seconds,watched,last_played_at,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(user_id,logical_type,logical_id) DO UPDATE SET position_seconds=excluded.position_seconds,duration_seconds=excluded.duration_seconds,watched=excluded.watched,last_played_at=excluded.last_played_at,updated_at=excluded.updated_at`, user, ev.LogicalType, ev.LogicalID, ev.Position, ev.Duration, newWatched, received, received)
		if e == nil {
			_, _ = tx.ExecContext(ctx, "UPDATE offline_progress_events SET applied=1 WHERE device_id=? AND event_id=?", device, ev.EventID)
			e = s.changeTx(ctx, tx, user, device, "PROGRESS_UPDATED", ev.LogicalType, ev.LogicalID, map[string]any{"position": ev.Position, "watched": newWatched})
			if e == nil && newWatched && ev.LogicalType == "EPISODE" {
				rows, queryErr := tx.QueryContext(ctx, `SELECT d.id FROM device_downloads d JOIN episodes ep ON ep.id=d.logical_id JOIN seasons se ON se.id=ep.season_id JOIN download_subscriptions sub ON sub.device_id=d.device_id AND sub.user_id=d.user_id AND sub.show_id=se.show_id WHERE d.user_id=? AND d.device_id=? AND d.logical_id=? AND d.status NOT IN ('REMOVED','REVOKED','REMOVAL_REQUESTED') AND sub.enabled=1 AND sub.remove_watched=1`, user, device, ev.LogicalID)
				ids := []string{}
				if queryErr == nil {
					for rows.Next() {
						var id string
						_ = rows.Scan(&id)
						ids = append(ids, id)
					}
					rows.Close()
				}
				for _, id := range ids {
					_, e = tx.ExecContext(ctx, "UPDATE device_downloads SET status='REMOVAL_REQUESTED',updated_at=? WHERE id=?", received, id)
					if e == nil {
						e = s.changeTx(ctx, tx, user, device, "DOWNLOAD_REMOVED", "DOWNLOAD", id, map[string]any{"reason": "WATCHED_SUBSCRIPTION_EPISODE"})
					}
				}
			}
		}
	}
	return e
}
func (s *Service) Pull(ctx context.Context, p auth.Principal, cursor int64, limit int) (SyncState, error) {
	device, e := s.device(ctx, p)
	if e != nil {
		return SyncState{}, e
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	out := SyncState{Cursor: cursor}
	var min, max int64
	_ = s.db.QueryRowContext(ctx, "SELECT COALESCE(MIN(sequence),0),COALESCE(MAX(sequence),0) FROM sync_changes WHERE device_id=?", device).Scan(&min, &max)
	if cursor > 0 && min > 0 && cursor < min-1 {
		out.FullResyncRequired = true
		out.Downloads, _ = s.List(ctx, p)
		out.Subscriptions, _ = s.Subscriptions(ctx, p)
		out.Cursor = max
		_, _ = s.db.ExecContext(ctx, "UPDATE device_sync_state SET cursor=?,last_sync_at=?,updated_at=? WHERE device_id=?", out.Cursor, stamp(s.now()), stamp(s.now()), device)
		return out, nil
	}
	rows, e := s.db.QueryContext(ctx, "SELECT sequence,change_type,entity_type,entity_id,payload_json,created_at FROM sync_changes WHERE device_id=? AND sequence>? ORDER BY sequence LIMIT ?", device, cursor, limit+1)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var x Change
		var raw string
		_ = rows.Scan(&x.Sequence, &x.Type, &x.EntityType, &x.EntityID, &raw, &x.CreatedAt)
		_ = json.Unmarshal([]byte(raw), &x.Payload)
		if len(out.Changes) == limit {
			out.HasMore = true
			break
		}
		out.Changes = append(out.Changes, x)
		out.Cursor = x.Sequence
	}
	err := rows.Err()
	rows.Close()
	_ = s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(device_sequence),0) FROM offline_progress_events WHERE device_id=?", device).Scan(&out.LastDeviceSequence)
	_, _ = s.db.ExecContext(ctx, `INSERT INTO device_sync_state(device_id,user_id,cursor,last_device_sequence,last_sync_at,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET cursor=excluded.cursor,last_device_sequence=excluded.last_device_sequence,last_sync_at=excluded.last_sync_at,updated_at=excluded.updated_at`, device, p.UserID, out.Cursor, out.LastDeviceSequence, stamp(s.now()), stamp(s.now()))
	return out, err
}

func (s *Service) CreateSubscription(ctx context.Context, p auth.Principal, in SubscriptionRequest) (Subscription, error) {
	device, e := s.device(ctx, p)
	if e != nil {
		return Subscription{}, e
	}
	if in.DesiredCount < 1 || in.DesiredCount > 20 || !s.has(ctx, p, "SHOW", in.ShowID) {
		return Subscription{}, ErrDenied
	}
	if _, e = s.profile(ctx, in.ProfileID); e != nil {
		return Subscription{}, e
	}
	id := auth.ID()
	now := stamp(s.now())
	_, e = s.db.ExecContext(ctx, "INSERT INTO download_subscriptions(id,user_id,device_id,show_id,enabled,desired_count,profile_id,remove_watched,wifi_only,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,'ACTIVE',?,?) ON CONFLICT(device_id,show_id) DO UPDATE SET enabled=excluded.enabled,desired_count=excluded.desired_count,profile_id=excluded.profile_id,remove_watched=excluded.remove_watched,wifi_only=excluded.wifi_only,status='ACTIVE',updated_at=excluded.updated_at", id, p.UserID, device, in.ShowID, in.Enabled, in.DesiredCount, strings.ToUpper(in.ProfileID), in.RemoveWatched, in.WiFiOnly, now, now)
	if e != nil {
		return Subscription{}, e
	}
	_ = s.FillSubscriptions(ctx, p)
	list, _ := s.Subscriptions(ctx, p)
	for _, x := range list {
		if x.ShowID == in.ShowID {
			return x, nil
		}
	}
	return Subscription{}, ErrNotFound
}
func (s *Service) Subscriptions(ctx context.Context, p auth.Principal) ([]Subscription, error) {
	device, e := s.device(ctx, p)
	if e != nil {
		return nil, e
	}
	rows, e := s.db.QueryContext(ctx, "SELECT id,user_id,device_id,show_id,profile_id,status,enabled,remove_watched,wifi_only,desired_count,created_at,updated_at FROM download_subscriptions WHERE user_id=? AND device_id=? ORDER BY created_at", p.UserID, device)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Subscription{}
	for rows.Next() {
		var x Subscription
		if e = rows.Scan(&x.ID, &x.UserID, &x.DeviceID, &x.ShowID, &x.ProfileID, &x.Status, &x.Enabled, &x.RemoveWatched, &x.WiFiOnly, &x.DesiredCount, &x.CreatedAt, &x.UpdatedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) FillSubscriptions(ctx context.Context, p auth.Principal) error {
	subs, e := s.Subscriptions(ctx, p)
	if e != nil {
		return e
	}
	for _, sub := range subs {
		if !sub.Enabled || (sub.Status != "ACTIVE" && sub.Status != "STORAGE_LIMITED") {
			continue
		}
		var available, minimum int64
		_ = s.db.QueryRowContext(ctx, "SELECT available_bytes,minimum_free_bytes FROM device_storage_reports WHERE device_id=?", sub.DeviceID).Scan(&available, &minimum)
		budget := available - minimum
		if available > 0 && budget > 0 && sub.Status == "STORAGE_LIMITED" {
			_, _ = s.db.ExecContext(ctx, "UPDATE download_subscriptions SET status='ACTIVE',updated_at=? WHERE id=?", stamp(s.now()), sub.ID)
		}
		rows, e := s.db.QueryContext(ctx, `SELECT ep.id FROM episodes ep JOIN seasons se ON se.id=ep.season_id JOIN media_associations a ON a.entity_type='EPISODE' AND a.entity_id=ep.id JOIN media_files f ON f.id=a.media_file_id LEFT JOIN user_media_progress p ON p.user_id=? AND p.logical_type='EPISODE' AND p.logical_id=ep.id WHERE se.show_id=? AND se.season_number>0 AND f.availability='AVAILABLE' AND COALESCE(p.watched,0)=0 AND NOT EXISTS(SELECT 1 FROM device_downloads d WHERE d.device_id=? AND d.logical_type='EPISODE' AND d.logical_id=ep.id AND d.status NOT IN ('REMOVED','REVOKED')) GROUP BY ep.id ORDER BY se.season_number,ep.episode_number LIMIT ?`, p.UserID, sub.ShowID, sub.DeviceID, sub.DesiredCount)
		if e != nil {
			continue
		}
		ids := []string{}
		for rows.Next() {
			var id string
			_ = rows.Scan(&id)
			ids = append(ids, id)
		}
		rows.Close()
		var current int
		_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_downloads d JOIN episodes ep ON ep.id=d.logical_id JOIN seasons se ON se.id=ep.season_id WHERE d.device_id=? AND d.logical_type='EPISODE' AND se.show_id=? AND d.status NOT IN ('REMOVED','REVOKED','REMOVAL_REQUESTED')", sub.DeviceID, sub.ShowID).Scan(&current)
		needed := sub.DesiredCount - current
		if needed < len(ids) {
			if needed < 0 {
				needed = 0
			}
			ids = ids[:needed]
		}
		for _, id := range ids {
			estimate := int64(0)
			if src, sourceErr := s.source(ctx, "EPISODE", id); sourceErr == nil {
				estimate = src.size
				if plan, planErr := s.Plan(ctx, "EPISODE", id, sub.ProfileID); planErr == nil && plan.Mode == "GENERATED_OFFLINE_VERSION" {
					estimate = int64(src.duration*float64(plan.OutputVideoBitrate+plan.OutputAudioBitrate)/8) + 8*1024*1024
				}
			}
			if available > 0 && (budget <= 0 || estimate > budget) {
				_, _ = s.db.ExecContext(ctx, "UPDATE download_subscriptions SET status='STORAGE_LIMITED',updated_at=? WHERE id=?", stamp(s.now()), sub.ID)
				break
			}
			if _, createErr := s.Create(ctx, p, CreateRequest{LogicalType: "EPISODE", LogicalID: id, ProfileID: sub.ProfileID}); createErr == nil && available > 0 {
				budget -= estimate
			}
		}
	}
	return nil
}

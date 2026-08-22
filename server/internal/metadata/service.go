package metadata

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotFound   = errors.New("metadata not found")
	ErrValidation = errors.New("metadata validation")
	ErrConflict   = errors.New("metadata conflict")
)

type Service struct {
	db                    *sql.DB
	configDir             string
	provider              Provider
	language, region      string
	artworkBase           string
	allowInsecureProvider bool
	artworkTimeout        time.Duration
}

func New(db *sql.DB, configDir string, provider Provider, testProviderOptions ...string) *Service {
	s := &Service{db: db, configDir: configDir, provider: provider, language: "en-US", region: "US", artworkBase: "https://image.tmdb.org/t/p/original", artworkTimeout: 15 * time.Second}
	if len(testProviderOptions) > 0 && testProviderOptions[0] != "" {
		s.artworkBase = testProviderOptions[0]
	}
	if len(testProviderOptions) > 1 && testProviderOptions[1] == "true" {
		s.allowInsecureProvider = true
	}
	return s
}
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b)
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func timestamp() string         { return time.Now().UTC().Format(time.RFC3339Nano) }
func cleanSort(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func safeSummary(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < ' ' && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
	if len(s) > 512 {
		s = s[:512]
	}
	return s
}

func LoadToken(configDir string) string {
	if v := strings.TrimSpace(os.Getenv("VYNODE_TMDB_TOKEN")); v != "" {
		return v
	}
	b, _ := os.ReadFile(filepath.Join(configDir, "secrets", "tmdb.token"))
	return strings.TrimSpace(string(b))
}
func StoreToken(configDir, token string) error {
	token = strings.TrimSpace(token)
	if len(token) < 16 {
		return ErrValidation
	}
	dir := filepath.Join(configDir, "secrets")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "tmdb-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.WriteString(token)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, "tmdb.token"))
}
func (s *Service) ProviderStatus(ctx context.Context) ProviderStatus {
	enabled := true
	language, region := s.language, s.region
	var v string
	if s.db.QueryRowContext(ctx, "SELECT value FROM server_settings WHERE key='metadata_provider_enabled'").Scan(&v) == nil {
		enabled = v != "false"
	}
	_ = s.db.QueryRowContext(ctx, "SELECT value FROM server_settings WHERE key='metadata_language'").Scan(&language)
	_ = s.db.QueryRowContext(ctx, "SELECT value FROM server_settings WHERE key='metadata_region'").Scan(&region)
	configured := LoadToken(s.configDir) != ""
	status := "ready"
	if !enabled {
		status = "disabled"
	} else if !configured {
		status = "not_configured"
	}
	return ProviderStatus{Enabled: enabled, Configured: configured, Language: language, Region: region, Status: status}
}
func (s *Service) Configure(ctx context.Context, enabled bool, token, language, region string) error {
	if token != "" {
		if err := StoreToken(s.configDir, token); err != nil {
			return err
		}
		if provider, ok := s.provider.(*TMDb); ok {
			provider.token = strings.TrimSpace(token)
		}
	}
	if language == "" {
		language = "en-US"
	}
	if region == "" {
		region = "US"
	}
	if len(language) > 16 || len(region) > 8 {
		return ErrValidation
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range map[string]string{"metadata_provider_enabled": strconv.FormatBool(enabled), "metadata_language": language, "metadata_region": region} {
		if _, err = tx.ExecContext(ctx, "INSERT INTO server_settings(key,value,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at", k, v, timestamp()); err != nil {
			return err
		}
	}
	s.language, s.region = language, region
	return tx.Commit()
}
func (s *Service) TestProvider(ctx context.Context) error {
	if s.provider == nil {
		return ErrProviderUnavailable
	}
	return s.provider.Test(ctx)
}

func (s *Service) SearchProvider(ctx context.Context, kind, title string, year int) ([]Candidate, error) {
	if s.provider == nil {
		return nil, ErrProviderUnavailable
	}
	key := strings.Join([]string{s.provider.Name(), kind, strings.ToLower(strings.TrimSpace(title)), strconv.Itoa(year), s.language, s.region}, "|")
	var cached string
	if s.db.QueryRowContext(ctx, "SELECT response_json FROM metadata_provider_cache WHERE cache_key=? AND expires_at>?", key, timestamp()).Scan(&cached) == nil {
		var out []Candidate
		if json.Unmarshal([]byte(cached), &out) == nil {
			return out, nil
		}
	}
	var out []Candidate
	var err error
	if kind == "MOVIE" {
		out, err = s.provider.SearchMovies(ctx, title, year, s.language, s.region)
	} else if kind == "SHOW" {
		out, err = s.provider.SearchShows(ctx, title, year, s.language, s.region)
	} else {
		return nil, ErrValidation
	}
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(out)
	_, _ = s.db.ExecContext(ctx, `INSERT INTO metadata_provider_cache(cache_key,provider,response_json,expires_at,created_at) VALUES(?,?,?,?,?) ON CONFLICT(cache_key) DO UPDATE SET response_json=excluded.response_json,expires_at=excluded.expires_at`, key, s.provider.Name(), string(data), time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339Nano), timestamp())
	return out, nil
}

type MetadataJob struct {
	ID             string `json:"id"`
	LibraryID      string `json:"libraryId"`
	State          string `json:"state"`
	Processed      int    `json:"processed"`
	Matched        int    `json:"matched"`
	Ambiguous      int    `json:"ambiguous"`
	Unmatched      int    `json:"unmatched"`
	Failed         int    `json:"failed"`
	TotalFiles     int    `json:"totalFiles"`
	AlreadyMatched int    `json:"alreadyMatched"`
	ErrorSummary   string `json:"errorSummary,omitempty"`
}

func (s *Service) StartIdentify(ctx context.Context, libraryID string) (MetadataJob, error) {
	j := MetadataJob{ID: newID(), LibraryID: libraryID, State: "QUEUED"}
	var count int
	if e := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM libraries WHERE id=?", libraryID).Scan(&count); e != nil || count == 0 {
		return j, ErrNotFound
	}
	if e := s.db.QueryRowContext(ctx, `SELECT id,state FROM metadata_jobs WHERE library_id=? AND state IN ('QUEUED','RUNNING') ORDER BY created_at DESC LIMIT 1`, libraryID).Scan(&j.ID, &j.State); e == nil {
		return j, ErrConflict
	}
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN EXISTS(SELECT 1 FROM media_associations a WHERE a.media_file_id=f.id) THEN 1 ELSE 0 END),0) FROM media_files f JOIN library_sources src ON src.id=f.source_id WHERE src.library_id=? AND f.availability='AVAILABLE'`, libraryID).Scan(&j.TotalFiles, &j.AlreadyMatched)
	_, e := s.db.ExecContext(ctx, "INSERT INTO metadata_jobs(id,library_id,operation,state,created_at) VALUES(?,?,'IDENTIFY','QUEUED',?)", j.ID, libraryID, timestamp())
	if e == nil {
		_, e = s.db.ExecContext(ctx, "UPDATE metadata_jobs SET total_files=?,already_matched=? WHERE id=?", j.TotalFiles, j.AlreadyMatched, j.ID)
	}
	if e != nil {
		return j, e
	}
	go s.runIdentify(j.ID, libraryID)
	return j, nil
}
func (s *Service) runIdentify(jobID, libraryID string) {
	ctx := context.Background()
	_, _ = s.db.ExecContext(ctx, "UPDATE metadata_jobs SET state='RUNNING',started_at=? WHERE id=?", timestamp(), jobID)
	rows, e := s.db.QueryContext(ctx, `SELECT f.id,f.candidate_title,COALESCE(f.candidate_year,0),f.parent_path,COALESCE(f.season_number,0),COALESCE(f.episode_start,0),COALESCE(f.episode_end,0),l.type FROM media_files f JOIN library_sources src ON src.id=f.source_id JOIN libraries l ON l.id=src.library_id WHERE l.id=? AND f.availability='AVAILABLE' AND NOT EXISTS(SELECT 1 FROM media_associations a WHERE a.media_file_id=f.id)`, libraryID)
	if e != nil {
		s.finishJob(jobID, "FAILED", safeSummary(e.Error()))
		return
	}
	type item struct {
		id, title          string
		year               int
		parent             string
		season, start, end int
		kind               string
	}
	var items []item
	for rows.Next() {
		var x item
		_ = rows.Scan(&x.id, &x.title, &x.year, &x.parent, &x.season, &x.start, &x.end, &x.kind)
		items = append(items, x)
	}
	rows.Close()
	for _, x := range items {
		kind := "MOVIE"
		if x.kind == "TV" {
			kind = "SHOW"
		}
		c, e := s.SearchProvider(ctx, kind, x.title, x.year)
		if e != nil {
			_, _ = s.db.ExecContext(ctx, "UPDATE metadata_jobs SET processed=processed+1,failed=failed+1 WHERE id=?", jobID)
			_ = s.RecordAttempt(ctx, x.id, Match{State: "ERROR", Confidence: "LOW"}, safeSummary(e.Error()))
			continue
		}
		m := Score(x.title, x.year, x.parent, c)
		if kind == "SHOW" && x.start > 0 && m.Score >= 80 {
			m.Score += 15
			m.Signals = append(m.Signals, "parsed season and episode structure")
			if len(c) == 1 {
				m.State = "MATCHED"
				m.Confidence = "HIGH"
			}
		}
		_ = s.RecordAttempt(ctx, x.id, m, "")
		column := "unmatched"
		if m.State == "AMBIGUOUS" {
			column = "ambiguous"
		} else if m.State == "MATCHED" && m.Candidate != nil {
			var matchErr error
			if kind == "MOVIE" {
				_, matchErr = s.MatchMovie(ctx, x.id, m.Candidate.ProviderID, false)
			} else {
				_, matchErr = s.MatchTV(ctx, x.id, m.Candidate.ProviderID, x.season, x.start, x.end, false)
			}
			if matchErr == nil {
				column = "matched"
			} else {
				column = "failed"
			}
		}
		_, _ = s.db.ExecContext(ctx, "UPDATE metadata_jobs SET processed=processed+1,"+column+"="+column+"+1 WHERE id=?", jobID)
	}
	var failures int
	_ = s.db.QueryRowContext(ctx, "SELECT failed FROM metadata_jobs WHERE id=?", jobID).Scan(&failures)
	state := "COMPLETED"
	if failures > 0 {
		state = "COMPLETED_WITH_ERRORS"
	}
	s.finishJob(jobID, state, "")
}
func (s *Service) finishJob(id, state, summary string) {
	_, _ = s.db.Exec("UPDATE metadata_jobs SET state=?,completed_at=?,error_summary=? WHERE id=?", state, timestamp(), summary, id)
}
func (s *Service) GetMetadataJob(ctx context.Context, libraryID, id string) (MetadataJob, error) {
	var j MetadataJob
	e := s.db.QueryRowContext(ctx, "SELECT id,COALESCE(library_id,''),state,processed,matched,ambiguous,unmatched,failed,total_files,already_matched,COALESCE(error_summary,'') FROM metadata_jobs WHERE id=? AND library_id=?", id, libraryID).Scan(&j.ID, &j.LibraryID, &j.State, &j.Processed, &j.Matched, &j.Ambiguous, &j.Unmatched, &j.Failed, &j.TotalFiles, &j.AlreadyMatched, &j.ErrorSummary)
	if e != nil {
		return j, ErrNotFound
	}
	return j, nil
}
func (s *Service) Movies(ctx context.Context, q string, limit, offset int) ([]Movie, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,title,sort_title,COALESCE(year,0),COALESCE(release_date,''),COALESCE(runtime_minutes,0),COALESCE(overview,''),COALESCE(content_rating,''),COALESCE(rating_value,0),COALESCE(rating_votes,0),COALESCE(primary_provider,''),COALESCE(provider_id,''),metadata_state,created_at,updated_at FROM movies WHERE orphaned=0 AND title LIKE ? ORDER BY sort_title LIMIT ? OFFSET ?`, "%"+q+"%", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Movie
	for rows.Next() {
		var x Movie
		if err = rows.Scan(&x.ID, &x.Title, &x.SortTitle, &x.Year, &x.ReleaseDate, &x.RuntimeMinutes, &x.Overview, &x.ContentRating, &x.Rating, &x.VoteCount, &x.PrimaryProvider, &x.ProviderID, &x.MetadataState, &x.CreatedAt, &x.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) Movie(ctx context.Context, id string) (Movie, error) {
	items, err := s.Movies(ctx, "", 100, 0)
	if err != nil {
		return Movie{}, err
	}
	for _, x := range items {
		if x.ID == id {
			x.Genres, _ = s.genres(ctx, "MOVIE", id)
			x.Versions, _ = s.versions(ctx, "MOVIE", id)
			x.Credits, _ = s.credits(ctx, "MOVIE", id)
			x.Companies, _ = s.companies(ctx, "MOVIE", id)
			return x, nil
		}
	}
	return Movie{}, ErrNotFound
}
func (s *Service) Shows(ctx context.Context, q string, limit, offset int) ([]Show, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,title,sort_title,COALESCE(year,0),COALESCE(first_air_date,''),COALESCE(overview,''),COALESCE(rating_value,0),COALESCE(rating_votes,0),COALESCE(primary_provider,''),COALESCE(provider_id,''),metadata_state,created_at,updated_at FROM shows WHERE orphaned=0 AND title LIKE ? ORDER BY sort_title LIMIT ? OFFSET ?`, "%"+q+"%", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Show
	for rows.Next() {
		var x Show
		if err = rows.Scan(&x.ID, &x.Title, &x.SortTitle, &x.Year, &x.FirstAirDate, &x.Overview, &x.Rating, &x.VoteCount, &x.PrimaryProvider, &x.ProviderID, &x.MetadataState, &x.CreatedAt, &x.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) Show(ctx context.Context, id string) (Show, error) {
	items, err := s.Shows(ctx, "", 100, 0)
	if err != nil {
		return Show{}, err
	}
	for _, x := range items {
		if x.ID == id {
			x.Genres, _ = s.genres(ctx, "SHOW", id)
			x.Seasons, _ = s.seasons(ctx, id)
			x.Credits, _ = s.credits(ctx, "SHOW", id)
			x.Companies, _ = s.companies(ctx, "SHOW", id)
			return x, nil
		}
	}
	return Show{}, ErrNotFound
}
func (s *Service) genres(ctx context.Context, kind, id string) ([]string, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT g.name FROM genres g JOIN media_genres mg ON mg.genre_id=g.id WHERE mg.entity_type=? AND mg.entity_id=? ORDER BY g.name", kind, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var v []string
	for rows.Next() {
		var x string
		_ = rows.Scan(&x)
		v = append(v, x)
	}
	return v, rows.Err()
}
func (s *Service) credits(ctx context.Context, kind, id string) ([]Credit, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT p.name,c.credit_type,COALESCE(c.character_name,''),COALESCE(c.display_order,0) FROM credits c JOIN people p ON p.id=c.person_id WHERE c.entity_type=? AND c.entity_id=? ORDER BY CASE c.credit_type WHEN 'DIRECTOR' THEN 0 WHEN 'CREATOR' THEN 0 WHEN 'WRITER' THEN 1 ELSE 2 END,c.display_order,p.name`, kind, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []Credit
	for rows.Next() {
		var x Credit
		if e = rows.Scan(&x.Name, &x.Type, &x.Character, &x.Order); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) companies(ctx context.Context, kind, id string) ([]string, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT c.name FROM production_companies c JOIN media_production_companies m ON m.company_id=c.id WHERE m.entity_type=? AND m.entity_id=? ORDER BY c.name`, kind, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var x string
		if e = rows.Scan(&x); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) versions(ctx context.Context, kind, id string) ([]Version, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT a.id,f.id,COALESCE(a.version_label,''),COALESCE(f.resolution_class,''),COALESCE((SELECT codec FROM media_streams WHERE media_file_id=f.id AND stream_type='video' ORDER BY stream_index LIMIT 1),''),COALESCE(f.hdr_class,'') FROM media_associations a JOIN media_files f ON f.id=a.media_file_id WHERE a.entity_type=? AND a.entity_id=?`, kind, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var v []Version
	for rows.Next() {
		var x Version
		_ = rows.Scan(&x.ID, &x.FileID, &x.Label, &x.Resolution, &x.Codec, &x.HDR)
		v = append(v, x)
	}
	return v, rows.Err()
}
func (s *Service) seasons(ctx context.Context, showID string) ([]Season, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,season_number,COALESCE(title,''),COALESCE(overview,''),COALESCE(air_date,'') FROM seasons WHERE show_id=? ORDER BY season_number", showID)
	if e != nil {
		return nil, e
	}
	var v []Season
	for rows.Next() {
		var x Season
		_ = rows.Scan(&x.ID, &x.SeasonNumber, &x.Title, &x.Overview, &x.AirDate)
		v = append(v, x)
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return nil, e
	}
	rows.Close()
	for i := range v {
		er, _ := s.db.QueryContext(ctx, `SELECT e.id,e.episode_number,e.title,COALESCE(e.overview,''),COALESCE(e.air_date,''),COALESCE(e.runtime_minutes,0),EXISTS(SELECT 1 FROM media_associations a WHERE a.entity_type='EPISODE' AND a.entity_id=e.id) FROM episodes e WHERE e.season_id=? ORDER BY e.episode_number`, v[i].ID)
		for er.Next() {
			var z Episode
			_ = er.Scan(&z.ID, &z.EpisodeNumber, &z.Title, &z.Overview, &z.AirDate, &z.RuntimeMinutes, &z.Available)
			v[i].Episodes = append(v[i].Episodes, z)
		}
		er.Close()
	}
	return v, nil
}

func (s *Service) MatchMovie(ctx context.Context, fileID, providerID string, manual bool) (string, error) {
	if s.provider == nil {
		return "", ErrProviderUnavailable
	}
	d, e := s.provider.Movie(ctx, providerID, s.language, s.region)
	if e != nil {
		return "", e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var id string
	e = tx.QueryRowContext(ctx, "SELECT id FROM movies WHERE primary_provider='TMDB' AND provider_id=?", providerID).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		id = newID()
		n := timestamp()
		_, e = tx.ExecContext(ctx, `INSERT INTO movies(id,title,original_title,sort_title,year,release_date,runtime_minutes,overview,tagline,status,original_language,rating_value,rating_votes,primary_provider,provider_id,metadata_state,last_metadata_refresh_at,manual_override,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'TMDB',?,'IDENTIFIED',?,?,?,?)`, id, d.Title, d.OriginalTitle, cleanSort(d.Title), d.Year, d.ReleaseDate, d.RuntimeMinutes, d.Overview, d.Tagline, d.Status, d.OriginalLanguage, d.Rating, d.VoteCount, providerID, n, manual, n, n)
	}
	if e != nil {
		return "", e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM media_associations WHERE media_file_id=?", fileID); e != nil {
		return "", e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO media_associations(id,media_file_id,entity_type,entity_id,association_type,created_at) VALUES(?,?,'MOVIE',?,'VERSION',?)", newID(), fileID, id, timestamp()); e != nil {
		return "", e
	}
	if e = s.storeGenres(ctx, tx, "MOVIE", id, d.Genres); e != nil {
		return "", e
	}
	if e = s.storeEnrichment(ctx, tx, "MOVIE", id, d.ExternalIDs, d.Credits, d.Companies); e != nil {
		return "", e
	}
	if e = tx.Commit(); e != nil {
		return "", e
	}
	for _, a := range d.Artwork {
		if artworkID, err := s.AddArtwork(ctx, "MOVIE", id, a); err == nil {
			go func() { _ = s.CacheArtwork(context.Background(), artworkID) }()
		}
	}
	return id, nil
}
func (s *Service) HasAssociation(ctx context.Context, fileID string) bool {
	var n int
	return s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM media_associations WHERE media_file_id=?", fileID).Scan(&n) == nil && n > 0
}

func (s *Service) Refresh(ctx context.Context, kind, id string) error {
	n := timestamp()
	if kind == "MOVIE" {
		var providerID string
		if e := s.db.QueryRowContext(ctx, "SELECT provider_id FROM movies WHERE id=?", id).Scan(&providerID); e != nil {
			return ErrNotFound
		}
		d, e := s.provider.Movie(ctx, providerID, s.language, s.region)
		if e != nil {
			_, _ = s.db.ExecContext(ctx, "UPDATE movies SET metadata_state='REFRESH_ERROR',last_metadata_error=?,updated_at=? WHERE id=?", safeSummary(e.Error()), n, id)
			return e
		}
		_, e = s.db.ExecContext(ctx, `UPDATE movies SET title=?,original_title=?,sort_title=CASE WHEN manual_override=1 THEN sort_title ELSE ? END,year=?,release_date=?,runtime_minutes=?,overview=?,tagline=?,status=?,original_language=?,rating_value=?,rating_votes=?,metadata_state='IDENTIFIED',last_metadata_error=NULL,last_metadata_refresh_at=?,updated_at=? WHERE id=?`, d.Title, d.OriginalTitle, cleanSort(d.Title), d.Year, d.ReleaseDate, d.RuntimeMinutes, d.Overview, d.Tagline, d.Status, d.OriginalLanguage, d.Rating, d.VoteCount, n, n, id)
		return e
	}
	if kind == "SHOW" {
		var providerID string
		if e := s.db.QueryRowContext(ctx, "SELECT provider_id FROM shows WHERE id=?", id).Scan(&providerID); e != nil {
			return ErrNotFound
		}
		d, e := s.provider.Show(ctx, providerID, s.language, s.region)
		if e != nil {
			_, _ = s.db.ExecContext(ctx, "UPDATE shows SET metadata_state='REFRESH_ERROR',last_metadata_error=?,updated_at=? WHERE id=?", safeSummary(e.Error()), n, id)
			return e
		}
		_, e = s.db.ExecContext(ctx, `UPDATE shows SET title=?,original_title=?,sort_title=CASE WHEN manual_override=1 THEN sort_title ELSE ? END,year=?,first_air_date=?,status=?,overview=?,original_language=?,rating_value=?,rating_votes=?,metadata_state='IDENTIFIED',last_metadata_error=NULL,last_metadata_refresh_at=?,updated_at=? WHERE id=?`, d.Title, d.OriginalTitle, cleanSort(d.Title), d.Year, d.FirstAirDate, d.Status, d.Overview, d.OriginalLanguage, d.Rating, d.VoteCount, n, n, id)
		return e
	}
	return ErrValidation
}
func (s *Service) MatchTV(ctx context.Context, fileID, showProviderID string, season, start, end int, manual bool) (string, error) {
	if season < 0 || start < 1 || end < start {
		return "", ErrValidation
	}
	show, e := s.provider.Show(ctx, showProviderID, s.language, s.region)
	if e != nil {
		return "", e
	}
	sd, e := s.provider.Season(ctx, showProviderID, season, s.language, s.region)
	if e != nil {
		return "", e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	n := timestamp()
	var showID string
	e = tx.QueryRowContext(ctx, "SELECT id FROM shows WHERE primary_provider='TMDB' AND provider_id=?", showProviderID).Scan(&showID)
	if errors.Is(e, sql.ErrNoRows) {
		showID = newID()
		_, e = tx.ExecContext(ctx, `INSERT INTO shows(id,title,original_title,sort_title,year,first_air_date,status,overview,original_language,rating_value,rating_votes,primary_provider,provider_id,metadata_state,last_metadata_refresh_at,manual_override,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,'TMDB',?,'IDENTIFIED',?,?,?,?)`, showID, show.Title, show.OriginalTitle, cleanSort(show.Title), show.Year, show.FirstAirDate, show.Status, show.Overview, show.OriginalLanguage, show.Rating, show.VoteCount, showProviderID, n, manual, n, n)
	}
	if e != nil {
		return "", e
	}
	var seasonID string
	e = tx.QueryRowContext(ctx, "SELECT id FROM seasons WHERE show_id=? AND season_number=?", showID, season).Scan(&seasonID)
	if errors.Is(e, sql.ErrNoRows) {
		seasonID = newID()
		_, e = tx.ExecContext(ctx, "INSERT INTO seasons(id,show_id,season_number,title,overview,air_date,provider_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)", seasonID, showID, season, sd.Title, sd.Overview, sd.AirDate, sd.ProviderID, n, n)
	}
	if e != nil {
		return "", e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM media_associations WHERE media_file_id=?", fileID); e != nil {
		return "", e
	}
	for ep := start; ep <= end; ep++ {
		var details *EpisodeDetails
		for i := range sd.Episodes {
			if sd.Episodes[i].EpisodeNumber == ep {
				details = &sd.Episodes[i]
				break
			}
		}
		if details == nil {
			return "", fmt.Errorf("%w: episode not found", ErrValidation)
		}
		var episodeID string
		e = tx.QueryRowContext(ctx, "SELECT id FROM episodes WHERE season_id=? AND episode_number=?", seasonID, ep).Scan(&episodeID)
		if errors.Is(e, sql.ErrNoRows) {
			episodeID = newID()
			_, e = tx.ExecContext(ctx, "INSERT INTO episodes(id,season_id,episode_number,title,overview,air_date,runtime_minutes,provider_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)", episodeID, seasonID, ep, details.Title, details.Overview, details.AirDate, details.RuntimeMinutes, details.ProviderID, n, n)
		}
		if e != nil {
			return "", e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO media_associations(id,media_file_id,entity_type,entity_id,association_type,created_at) VALUES(?,?,'EPISODE',?,'EPISODE',?)", newID(), fileID, episodeID, n); e != nil {
			return "", e
		}
	}
	if e = s.storeGenres(ctx, tx, "SHOW", showID, show.Genres); e != nil {
		return "", e
	}
	if e = s.storeEnrichment(ctx, tx, "SHOW", showID, show.ExternalIDs, show.Credits, show.Companies); e != nil {
		return "", e
	}
	if e = tx.Commit(); e != nil {
		return "", e
	}
	for _, a := range show.Artwork {
		if artworkID, err := s.AddArtwork(ctx, "SHOW", showID, a); err == nil {
			go func() { _ = s.CacheArtwork(context.Background(), artworkID) }()
		}
	}
	return showID, nil
}
func (s *Service) storeGenres(ctx context.Context, tx *sql.Tx, kind, entity string, genres []ProviderGenre) error {
	for _, g := range genres {
		id := "TMDB:" + g.ID
		if _, e := tx.ExecContext(ctx, "INSERT INTO genres(id,provider,provider_id,name) VALUES(?,'TMDB',?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name", id, g.ID, g.Name); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, "INSERT OR IGNORE INTO media_genres(entity_type,entity_id,genre_id) VALUES(?,?,?)", kind, entity, id); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) storeEnrichment(ctx context.Context, tx *sql.Tx, kind, entity string, external map[string]string, credits []ProviderCredit, companies []ProviderCompany) error {
	n := timestamp()
	for provider, value := range external {
		if value == "" || value == "0" {
			continue
		}
		if _, e := tx.ExecContext(ctx, `INSERT INTO external_ids(id,entity_type,entity_id,provider,external_id,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(entity_type,entity_id,provider) DO UPDATE SET external_id=excluded.external_id`, newID(), kind, entity, provider, value, n); e != nil {
			return e
		}
	}
	for _, c := range credits {
		if c.Person.ID == "" || c.Person.Name == "" {
			continue
		}
		personID := "TMDB:" + c.Person.ID
		if _, e := tx.ExecContext(ctx, `INSERT INTO people(id,name,primary_provider,provider_id,created_at,updated_at) VALUES(?,?,'TMDB',?,?,?) ON CONFLICT(primary_provider,provider_id) DO UPDATE SET name=excluded.name,updated_at=excluded.updated_at`, personID, c.Person.Name, c.Person.ID, n, n); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, `INSERT OR IGNORE INTO credits(id,entity_type,entity_id,person_id,credit_type,character_name,display_order) VALUES(?,?,?,?,?,?,?)`, newID(), kind, entity, personID, c.Type, c.Character, c.Order); e != nil {
			return e
		}
	}
	for _, c := range companies {
		if c.ID == "" || c.Name == "" {
			continue
		}
		companyID := "TMDB:" + c.ID
		if _, e := tx.ExecContext(ctx, `INSERT INTO production_companies(id,name,provider,provider_id) VALUES(?,?,'TMDB',?) ON CONFLICT(provider,provider_id) DO UPDATE SET name=excluded.name`, companyID, c.Name, c.ID); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, "INSERT OR IGNORE INTO media_production_companies(entity_type,entity_id,company_id) VALUES(?,?,?)", kind, entity, companyID); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) Unmatch(ctx context.Context, fileID string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "DELETE FROM media_associations WHERE media_file_id=?", fileID); e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `UPDATE movies SET orphaned=NOT EXISTS(SELECT 1 FROM media_associations WHERE entity_type='MOVIE' AND entity_id=movies.id)`)
	if e == nil {
		_, e = tx.ExecContext(ctx, `UPDATE shows SET orphaned=NOT EXISTS(SELECT 1 FROM seasons s JOIN episodes ep ON ep.season_id=s.id JOIN media_associations a ON a.entity_type='EPISODE' AND a.entity_id=ep.id WHERE s.show_id=shows.id)`)
	}
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) RecordAttempt(ctx context.Context, fileID string, m Match, errorText string) error {
	errorText = safeSummary(errorText)
	ex, _ := json.Marshal(m.Signals)
	cs, _ := json.Marshal(m.Candidates)
	_, e := s.db.ExecContext(ctx, `INSERT INTO metadata_match_attempts(id,media_file_id,state,provider,selected_provider_id,score,confidence,explanation_json,candidates_json,error_summary,created_at,updated_at) VALUES(?,? ,?,'TMDB',?,?,?,?,?,?,?,?)`, newID(), fileID, m.State, func() string {
		if m.Candidate != nil {
			return m.Candidate.ProviderID
		}
		return ""
	}(), m.Score, m.Confidence, string(ex), string(cs), errorText, timestamp(), timestamp())
	return e
}
func (s *Service) Unmatched(ctx context.Context) ([]map[string]any, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT f.id,f.file_name,COALESCE(f.candidate_title,''),COALESCE(f.candidate_year,0),COALESCE(f.season_number,0),COALESCE(f.episode_start,0),COALESCE(f.episode_end,0),COALESCE(m.state,'UNMATCHED'),COALESCE(m.score,0),COALESCE(m.confidence,'LOW'),COALESCE(m.candidates_json,'[]') FROM media_files f LEFT JOIN metadata_match_attempts m ON m.id=(SELECT id FROM metadata_match_attempts WHERE media_file_id=f.id ORDER BY updated_at DESC LIMIT 1) WHERE NOT EXISTS(SELECT 1 FROM media_associations a WHERE a.media_file_id=f.id) ORDER BY f.created_at LIMIT 200`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, name, title, state, confidence, candidates string
		var year, season, start, end, score int
		if e = rows.Scan(&id, &name, &title, &year, &season, &start, &end, &state, &score, &confidence, &candidates); e != nil {
			return nil, e
		}
		var c any
		_ = json.Unmarshal([]byte(candidates), &c)
		out = append(out, map[string]any{"fileId": id, "fileName": name, "candidateTitle": title, "candidateYear": year, "seasonNumber": season, "episodeStart": start, "episodeEnd": end, "state": state, "score": score, "confidence": confidence, "candidates": c})
	}
	return out, rows.Err()
}
func (s *Service) Audit(ctx context.Context, user, event, target, request string) {
	_, _ = s.db.ExecContext(ctx, "INSERT INTO audit_events(actor_user_id,action,target_type,target_id,request_id,metadata_json) VALUES(?,?,'METADATA',?,?,'{}')", user, event, target, request)
}

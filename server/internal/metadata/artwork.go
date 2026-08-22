package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxArtworkBytes = 15 << 20

func validEntityType(v string) bool {
	switch v {
	case "MOVIE", "SHOW", "SEASON", "EPISODE":
		return true
	}
	return false
}
func validArtworkType(v string) bool {
	switch v {
	case "POSTER", "BACKDROP", "LOGO", "SEASON_POSTER", "EPISODE_STILL":
		return true
	}
	return false
}

func (s *Service) Artwork(ctx context.Context, kind, id string) ([]Artwork, error) {
	if !validEntityType(kind) {
		return nil, ErrValidation
	}
	rows, e := s.db.QueryContext(ctx, `SELECT id,entity_type,entity_id,artwork_type,provider,COALESCE(language,''),COALESCE(width,0),COALESCE(height,0),selected,manual_selection,cached_relative_path IS NOT NULL,COALESCE(mime_type,'') FROM artwork WHERE entity_type=? AND entity_id=? ORDER BY artwork_type,selected DESC,vote_average DESC`, kind, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []Artwork
	for rows.Next() {
		var x Artwork
		if e = rows.Scan(&x.ID, &x.EntityType, &x.EntityID, &x.Type, &x.Provider, &x.Language, &x.Width, &x.Height, &x.Selected, &x.ManualSelection, &x.Cached, &x.MimeType); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) SelectArtwork(ctx context.Context, kind, entity, id string) error {
	if !validEntityType(kind) {
		return ErrValidation
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var typ string
	if e = tx.QueryRowContext(ctx, "SELECT artwork_type FROM artwork WHERE id=? AND entity_type=? AND entity_id=?", id, kind, entity).Scan(&typ); e != nil {
		return ErrNotFound
	}
	if _, e = tx.ExecContext(ctx, "UPDATE artwork SET selected=0 WHERE entity_type=? AND entity_id=? AND artwork_type=?", kind, entity, typ); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE artwork SET selected=1,manual_selection=1 WHERE id=?", id); e != nil {
		return e
	}
	return tx.Commit()
}

func (s *Service) AddArtwork(ctx context.Context, kind, entity string, a ProviderArtwork) (string, error) {
	if !validEntityType(kind) || !validArtworkType(a.Type) || a.Path == "" || !strings.HasPrefix(a.Path, "/") {
		return "", ErrValidation
	}
	id := newID()
	_, e := s.db.ExecContext(ctx, `INSERT INTO artwork(id,entity_type,entity_id,artwork_type,provider,provider_path,language,width,height,vote_average,selected,created_at) VALUES(?,?,?,?, 'TMDB',?,?,?,?,?,NOT EXISTS(SELECT 1 FROM artwork WHERE entity_type=? AND entity_id=? AND artwork_type=? AND selected=1),?) ON CONFLICT(entity_type,entity_id,artwork_type,provider,provider_path) DO UPDATE SET language=excluded.language,width=excluded.width,height=excluded.height,vote_average=excluded.vote_average`, id, kind, entity, a.Type, a.Path, a.Language, a.Width, a.Height, a.VoteAverage, kind, entity, a.Type, timestamp())
	if e != nil {
		return "", e
	}
	return id, nil
}
func (s *Service) CacheArtwork(ctx context.Context, id, imageBase string) error {
	if imageBase == "" {
		imageBase = "https://image.tmdb.org/t/p/original"
	}
	base, e := url.Parse(imageBase)
	if e != nil || base.Scheme != "https" || !strings.EqualFold(base.Hostname(), "image.tmdb.org") {
		return ErrValidation
	}
	var providerPath string
	if e = s.db.QueryRowContext(ctx, "SELECT provider_path FROM artwork WHERE id=?", id).Scan(&providerPath); e != nil {
		return ErrNotFound
	}
	remote := strings.TrimRight(imageBase, "/") + "/" + strings.TrimLeft(providerPath, "/")
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, remote, nil)
	if e != nil {
		return ErrValidation
	}
	client := http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || !strings.EqualFold(req.URL.Hostname(), base.Hostname()) {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	resp, e := client.Do(req)
	if e != nil {
		return fmt.Errorf("%w: artwork request", ErrProviderUnavailable)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%w: artwork HTTP %d", ErrProviderUnavailable, resp.StatusCode)
	}
	mime := strings.Split(resp.Header.Get("Content-Type"), ";")[0]
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/gif" {
		return ErrProviderResponse
	}
	dir := filepath.Join(s.configDir, "cache", "artwork")
	if e = os.MkdirAll(dir, 0750); e != nil {
		return e
	}
	sum := sha256.Sum256([]byte(id + providerPath))
	name := hex.EncodeToString(sum[:]) + map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif"}[mime]
	tmp, e := os.CreateTemp(dir, "artwork-*.tmp")
	if e != nil {
		return e
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	limited := io.LimitReader(resp.Body, maxArtworkBytes+1)
	n, e := io.Copy(tmp, limited)
	if closeErr := tmp.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	if n > maxArtworkBytes {
		return ErrProviderResponse
	}
	f, e := os.Open(tmpName)
	if e != nil {
		return e
	}
	_, format, e := image.DecodeConfig(f)
	_ = f.Close()
	if e != nil || ((mime == "image/jpeg" && format != "jpeg") || (mime == "image/png" && format != "png") || (mime == "image/gif" && format != "gif")) {
		return ErrProviderResponse
	}
	final := filepath.Join(dir, name)
	if e = os.Rename(tmpName, final); e != nil {
		return e
	}
	rel := filepath.ToSlash(filepath.Join("cache", "artwork", name))
	_, e = s.db.ExecContext(ctx, "UPDATE artwork SET cached_relative_path=?,mime_type=?,etag=? WHERE id=?", rel, mime, `"`+hex.EncodeToString(sum[:])+`"`, id)
	return e
}
func (s *Service) ArtworkFile(ctx context.Context, id string) (path, mime, etag string, e error) {
	var rel string
	e = s.db.QueryRowContext(ctx, "SELECT COALESCE(cached_relative_path,''),COALESCE(mime_type,''),COALESCE(etag,'') FROM artwork WHERE id=?", id).Scan(&rel, &mime, &etag)
	if e != nil || rel == "" {
		return "", "", "", ErrNotFound
	}
	root := filepath.Join(s.configDir, "cache", "artwork")
	path = filepath.Join(s.configDir, filepath.FromSlash(rel))
	clean, er := filepath.Abs(path)
	rootAbs, _ := filepath.Abs(root)
	if er != nil || !strings.HasPrefix(clean, rootAbs+string(os.PathSeparator)) {
		return "", "", "", ErrValidation
	}
	return clean, mime, etag, nil
}

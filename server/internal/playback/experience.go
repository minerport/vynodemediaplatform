package playback

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strings"
)

var languageCode = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`)

func normalizeLanguage(value string) string {
	v := strings.TrimSpace(strings.ReplaceAll(value, "_", "-"))
	if v == "" || strings.EqualFold(v, "und") || strings.EqualFold(v, "unknown") {
		return "und"
	}
	parts := strings.Split(v, "-")
	parts[0] = strings.ToLower(parts[0])
	aliases := map[string]string{"eng": "en", "jpn": "ja", "spa": "es", "fra": "fr", "fre": "fr", "deu": "de", "ger": "de", "ita": "it", "por": "pt", "zho": "zh", "chi": "zh", "kor": "ko"}
	if alias := aliases[parts[0]]; alias != "" {
		parts[0] = alias
	}
	if len(parts) == 2 {
		parts[1] = strings.ToUpper(parts[1])
	}
	return strings.Join(parts, "-")
}

func defaultPreferences() PlaybackPreferences {
	return PlaybackPreferences{AudioLanguages: []string{"en"}, SubtitleLanguages: []string{"en"}, SubtitleMode: "WHEN_AUDIO_NOT_PREFERRED", AutoplayNext: true, LocalQualityID: "auto", RemoteQualityID: "auto", AvoidCommentary: true}
}

func validatePreferences(p PlaybackPreferences) (PlaybackPreferences, error) {
	if len(p.AudioLanguages) > 10 || len(p.SubtitleLanguages) > 10 || !contains([]string{"OFF", "ALWAYS", "WHEN_AUDIO_NOT_PREFERRED", "FORCED_ONLY"}, p.SubtitleMode) || !validQuality(p.LocalQualityID) || !validQuality(p.RemoteQualityID) {
		return p, ErrValidation
	}
	normalize := func(values []string) ([]string, error) {
		out := make([]string, 0, len(values))
		seen := map[string]bool{}
		for _, raw := range values {
			v := normalizeLanguage(raw)
			if v == "und" || !languageCode.MatchString(v) {
				return nil, ErrValidation
			}
			if !seen[v] {
				seen[v], out = true, append(out, v)
			}
		}
		return out, nil
	}
	var err error
	if p.AudioLanguages, err = normalize(p.AudioLanguages); err != nil {
		return p, err
	}
	if p.SubtitleLanguages, err = normalize(p.SubtitleLanguages); err != nil {
		return p, err
	}
	return p, nil
}

func validQuality(v string) bool {
	return contains([]string{"auto", "original", "1080p", "720p", "480p"}, v)
}

func (s *Service) Preferences(ctx context.Context, userID string) (PlaybackPreferences, error) {
	p := defaultPreferences()
	var audio, subtitles string
	err := s.db.QueryRowContext(ctx, "SELECT audio_languages_json,subtitle_languages_json,subtitle_mode,autoplay_next,local_quality_id,remote_quality_id,avoid_commentary,prefer_hearing_impaired FROM user_playback_preferences WHERE user_id=?", userID).Scan(&audio, &subtitles, &p.SubtitleMode, &p.AutoplayNext, &p.LocalQualityID, &p.RemoteQualityID, &p.AvoidCommentary, &p.PreferHearingImpaired)
	if err == sql.ErrNoRows {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	_ = json.Unmarshal([]byte(audio), &p.AudioLanguages)
	_ = json.Unmarshal([]byte(subtitles), &p.SubtitleLanguages)
	return p, nil
}

func (s *Service) SetPreferences(ctx context.Context, userID string, p PlaybackPreferences) (PlaybackPreferences, error) {
	p, err := validatePreferences(p)
	if err != nil {
		return p, err
	}
	audio, _ := json.Marshal(p.AudioLanguages)
	subtitles, _ := json.Marshal(p.SubtitleLanguages)
	_, err = s.db.ExecContext(ctx, `INSERT INTO user_playback_preferences(user_id,audio_languages_json,subtitle_languages_json,subtitle_mode,autoplay_next,local_quality_id,remote_quality_id,avoid_commentary,prefer_hearing_impaired,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET audio_languages_json=excluded.audio_languages_json,subtitle_languages_json=excluded.subtitle_languages_json,subtitle_mode=excluded.subtitle_mode,autoplay_next=excluded.autoplay_next,local_quality_id=excluded.local_quality_id,remote_quality_id=excluded.remote_quality_id,avoid_commentary=excluded.avoid_commentary,prefer_hearing_impaired=excluded.prefer_hearing_impaired,updated_at=excluded.updated_at`, userID, string(audio), string(subtitles), p.SubtitleMode, p.AutoplayNext, p.LocalQualityID, p.RemoteQualityID, p.AvoidCommentary, p.PreferHearingImpaired, stamp(s.now()))
	return p, err
}

func languageRank(language string, preferred []string) int {
	language = normalizeLanguage(language)
	for i, p := range preferred {
		if language == p || strings.Split(language, "-")[0] == strings.Split(p, "-")[0] {
			return i
		}
	}
	return len(preferred) + 1
}

func semanticAudio(tracks []Track, preferred []string, avoidCommentary bool) (Track, bool) {
	if len(tracks) == 0 {
		return Track{}, false
	}
	copyTracks := append([]Track(nil), tracks...)
	sort.SliceStable(copyTracks, func(i, j int) bool {
		a, b := copyTracks[i], copyTracks[j]
		if avoidCommentary && a.Commentary != b.Commentary {
			return !a.Commentary
		}
		ra, rb := languageRank(a.Language, preferred), languageRank(b.Language, preferred)
		if ra != rb {
			return ra < rb
		}
		if a.Default != b.Default {
			return a.Default
		}
		return a.StreamIndex < b.StreamIndex
	})
	return copyTracks[0], true
}

func semanticSubtitle(tracks []Track, audio Track, p PlaybackPreferences) (Track, bool) {
	if p.SubtitleMode == "OFF" {
		return Track{}, false
	}
	usable := make([]Track, 0, len(tracks))
	for _, t := range tracks {
		if t.Usable {
			usable = append(usable, t)
		}
	}
	if len(usable) == 0 {
		return Track{}, false
	}
	if p.SubtitleMode == "FORCED_ONLY" {
		forced := usable[:0]
		for _, t := range usable {
			if t.Forced {
				forced = append(forced, t)
			}
		}
		usable = forced
	} else if p.SubtitleMode == "WHEN_AUDIO_NOT_PREFERRED" && languageRank(audio.Language, p.AudioLanguages) < len(p.AudioLanguages) {
		return Track{}, false
	}
	if len(usable) == 0 {
		return Track{}, false
	}
	sort.SliceStable(usable, func(i, j int) bool {
		a, b := usable[i], usable[j]
		if a.Forced != b.Forced {
			return a.Forced
		}
		ra, rb := languageRank(a.Language, p.SubtitleLanguages), languageRank(b.Language, p.SubtitleLanguages)
		if ra != rb {
			return ra < rb
		}
		if p.PreferHearingImpaired && a.HearingImpaired != b.HearingImpaired {
			return a.HearingImpaired
		}
		if a.Default != b.Default {
			return a.Default
		}
		return a.StreamIndex < b.StreamIndex
	})
	return usable[0], true
}

func findSemanticTrack(versions []Version, id string) (Track, bool) {
	for _, v := range versions {
		if t, ok := findTrack(v.AudioTracks, id); ok {
			return t, true
		}
		if t, ok := findTrack(v.SubtitleTracks, id); ok {
			return t, true
		}
	}
	return Track{}, false
}

func matchTrack(tracks []Track, semantic Track) (Track, bool) {
	best := []Track{}
	for _, t := range tracks {
		if normalizeLanguage(t.Language) == normalizeLanguage(semantic.Language) && t.Commentary == semantic.Commentary && t.Forced == semantic.Forced {
			best = append(best, t)
		}
	}
	if len(best) == 0 {
		return Track{}, false
	}
	sort.SliceStable(best, func(i, j int) bool { return best[i].Default })
	return best[0], true
}

func (s *Service) Markers(ctx context.Context, logicalType, logicalID string) ([]Marker, error) {
	var available int
	if err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM media_associations a JOIN media_files f ON f.id=a.media_file_id WHERE a.entity_type=? AND a.entity_id=? AND f.availability='AVAILABLE')", logicalType, logicalID).Scan(&available); err != nil {
		return nil, err
	}
	if available == 0 {
		return nil, ErrNotFound
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,logical_type,logical_id,marker_type,start_seconds,end_seconds,source,confidence,created_at,updated_at FROM media_markers WHERE logical_type=? AND logical_id=? AND active=1 AND review_state='ACCEPTED' ORDER BY start_seconds,marker_type", logicalType, logicalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Marker{}
	for rows.Next() {
		var x Marker
		var confidence sql.NullFloat64
		if err = rows.Scan(&x.ID, &x.LogicalType, &x.LogicalID, &x.Type, &x.Start, &x.End, &x.Source, &confidence, &x.CreatedAt, &x.UpdatedAt); err != nil {
			return nil, err
		}
		if confidence.Valid {
			x.Confidence = &confidence.Float64
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) SaveMarker(ctx context.Context, marker Marker) (Marker, error) {
	if marker.ID != "" {
		var existing Marker
		if err := s.db.QueryRowContext(ctx, "SELECT logical_type,logical_id,marker_type,created_at FROM media_markers WHERE id=?", marker.ID).Scan(&existing.LogicalType, &existing.LogicalID, &existing.Type, &existing.CreatedAt); err != nil {
			return marker, ErrNotFound
		}
		marker.LogicalType, marker.LogicalID, marker.CreatedAt = existing.LogicalType, existing.LogicalID, existing.CreatedAt
		if marker.Type == "" {
			marker.Type = existing.Type
		}
	}
	if marker.LogicalType != "MOVIE" && marker.LogicalType != "EPISODE" || !contains([]string{"INTRO", "RECAP", "CREDITS", "POST_CREDITS", "CUSTOM"}, marker.Type) || math.IsNaN(marker.Start) || math.IsNaN(marker.End) || math.IsInf(marker.Start, 0) || math.IsInf(marker.End, 0) || marker.Start < 0 || marker.End <= marker.Start || marker.End > 7*24*3600 {
		return marker, ErrValidation
	}
	var duration float64
	_ = s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(f.duration_seconds),0) FROM media_associations a JOIN media_files f ON f.id=a.media_file_id WHERE a.entity_type=? AND a.entity_id=?", marker.LogicalType, marker.LogicalID).Scan(&duration)
	if duration <= 0 {
		return marker, ErrNotFound
	}
	if marker.End > duration+2 {
		return marker, ErrValidation
	}
	now := stamp(s.now())
	if marker.ID == "" {
		marker.ID = id()
		marker.CreatedAt = now
	}
	marker.UpdatedAt = now
	marker.Source = "MANUAL"
	_, err := s.db.ExecContext(ctx, `INSERT INTO media_markers(id,logical_type,logical_id,marker_type,start_seconds,end_seconds,source,confidence,active,review_state,created_at,updated_at) VALUES(?,?,?,?,?,?,?,NULL,1,'ACCEPTED',?,?) ON CONFLICT(id) DO UPDATE SET marker_type=excluded.marker_type,start_seconds=excluded.start_seconds,end_seconds=excluded.end_seconds,source='MANUAL',confidence=NULL,active=1,review_state='ACCEPTED',source_identity=NULL,updated_at=excluded.updated_at`, marker.ID, marker.LogicalType, marker.LogicalID, marker.Type, marker.Start, marker.End, marker.Source, marker.CreatedAt, marker.UpdatedAt)
	if err == nil {
		_, _ = s.db.ExecContext(ctx, "UPDATE media_markers SET active=0,review_state='SUPERSEDED',updated_at=? WHERE logical_type=? AND logical_id=? AND marker_type=? AND source!='MANUAL'", now, marker.LogicalType, marker.LogicalID, marker.Type)
	}
	return marker, err
}
func (s *Service) DeleteMarker(ctx context.Context, id string) error {
	r, e := s.db.ExecContext(ctx, "DELETE FROM media_markers WHERE id=?", id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Navigation(ctx context.Context, userID, logicalType, logicalID string) (Navigation, error) {
	p, _ := s.Preferences(ctx, userID)
	n := Navigation{Autoplay: p.AutoplayNext, CountdownSeconds: 10}
	if logicalType != "EPISODE" {
		return n, nil
	}
	type row struct {
		item    NavigationItem
		season  int
		episode int
	}
	rows, err := s.db.QueryContext(ctx, `SELECT e.id,sh.id,sh.title,e.title,se.season_number,e.episode_number,EXISTS(SELECT 1 FROM media_associations a JOIN media_files f ON f.id=a.media_file_id WHERE a.entity_type='EPISODE' AND a.entity_id=e.id AND f.availability='AVAILABLE') FROM episodes e JOIN seasons se ON se.id=e.season_id JOIN shows sh ON sh.id=se.show_id WHERE sh.id=(SELECT sh2.id FROM episodes e2 JOIN seasons se2 ON se2.id=e2.season_id JOIN shows sh2 ON sh2.id=se2.show_id WHERE e2.id=?) AND se.season_number>0 ORDER BY se.season_number,e.episode_number`, logicalID)
	if err != nil {
		return n, err
	}
	defer rows.Close()
	all := []row{}
	for rows.Next() {
		var x row
		if err = rows.Scan(&x.item.LogicalID, &x.item.ShowID, &x.item.ShowTitle, &x.item.Title, &x.item.SeasonNumber, &x.item.EpisodeNumber, &x.item.Available); err != nil {
			return n, err
		}
		x.season = x.item.SeasonNumber
		x.episode = x.item.EpisodeNumber
		if x.item.Available {
			all = append(all, x)
		}
	}
	for i, x := range all {
		if x.item.LogicalID == logicalID {
			if i > 0 {
				v := all[i-1].item
				n.Previous = &v
			}
			if i+1 < len(all) {
				v := all[i+1].item
				n.Next = &v
			}
			break
		}
	}
	return n, rows.Err()
}

func (s *Service) DismissContinue(ctx context.Context, userID, kind, logicalID string) error {
	if kind != "MOVIE" && kind != "EPISODE" {
		return ErrValidation
	}
	_, e := s.db.ExecContext(ctx, `INSERT INTO continue_watching_dismissals(user_id,logical_type,logical_id,dismissed_at) VALUES(?,?,?,?) ON CONFLICT(user_id,logical_type,logical_id) DO UPDATE SET dismissed_at=excluded.dismissed_at`, userID, kind, logicalID, stamp(s.now()))
	return e
}
func (s *Service) ResetProgress(ctx context.Context, userID, kind, logicalID string) error {
	if kind != "MOVIE" && kind != "EPISODE" {
		return ErrValidation
	}
	_, e := s.db.ExecContext(ctx, "UPDATE user_media_progress SET position_seconds=0,watched=0,updated_at=? WHERE user_id=? AND logical_type=? AND logical_id=?", stamp(s.now()), userID, kind, logicalID)
	return e
}

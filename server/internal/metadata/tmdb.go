package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrProviderUnavailable = errors.New("metadata provider unavailable")
	ErrUnauthorized        = errors.New("metadata provider credential rejected")
	ErrRateLimited         = errors.New("metadata provider rate limited")
	ErrProviderResponse    = errors.New("invalid metadata provider response")
)

type TMDb struct {
	base, token, userAgent string
	client                 *http.Client
}

func NewTMDb(base, token, version string) *TMDb {
	if base == "" {
		base = "https://api.themoviedb.org/3"
	}
	root, _ := url.Parse(base)
	client := &http.Client{Timeout: 10 * time.Second}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || req.URL.Scheme != "https" || root == nil || !strings.EqualFold(req.URL.Hostname(), root.Hostname()) {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return &TMDb{base: strings.TrimRight(base, "/"), token: token, userAgent: "VyNode-Media/" + version, client: client}
}

func (t *TMDb) Name() string { return "TMDB" }
func (t *TMDb) request(ctx context.Context, path string, q url.Values, out any) error {
	if t.token == "" {
		return ErrProviderUnavailable
	}
	u, err := url.Parse(t.base + path)
	if err != nil {
		return ErrProviderUnavailable
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ErrProviderUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+t.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", t.userAgent)
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: request failed", ErrProviderUnavailable)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 401, 403:
		return ErrUnauthorized
	case 429:
		return ErrRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: HTTP %d", ErrProviderUnavailable, resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, 2<<20+1)
	data, err := io.ReadAll(limited)
	if err != nil || len(data) > 2<<20 {
		return ErrProviderResponse
	}
	if err = json.Unmarshal(data, out); err != nil {
		return ErrProviderResponse
	}
	return nil
}
func common(language, region string) url.Values {
	q := url.Values{}
	if language != "" {
		q.Set("language", language)
	}
	if region != "" {
		q.Set("region", region)
	}
	return q
}
func yearOf(s string) int {
	if len(s) < 4 {
		return 0
	}
	n, _ := strconv.Atoi(s[:4])
	return n
}

type tmdbSearch struct {
	Results []struct {
		ID           int     `json:"id"`
		Title        string  `json:"title"`
		Name         string  `json:"name"`
		ReleaseDate  string  `json:"release_date"`
		FirstAirDate string  `json:"first_air_date"`
		Overview     string  `json:"overview"`
		PosterPath   string  `json:"poster_path"`
		Popularity   float64 `json:"popularity"`
	} `json:"results"`
}

func (t *TMDb) SearchMovies(ctx context.Context, title string, year int, language, region string) ([]Candidate, error) {
	q := common(language, region)
	q.Set("query", title)
	if year > 0 {
		q.Set("year", strconv.Itoa(year))
	}
	var x tmdbSearch
	if err := t.request(ctx, "/search/movie", q, &x); err != nil {
		return nil, err
	}
	out := make([]Candidate, len(x.Results))
	for i, v := range x.Results {
		out[i] = Candidate{ProviderID: strconv.Itoa(v.ID), Title: v.Title, Year: yearOf(v.ReleaseDate), Overview: v.Overview, PosterPath: v.PosterPath, Popularity: v.Popularity}
	}
	return out, nil
}
func (t *TMDb) SearchShows(ctx context.Context, title string, year int, language, region string) ([]Candidate, error) {
	q := common(language, region)
	q.Set("query", title)
	if year > 0 {
		q.Set("first_air_date_year", strconv.Itoa(year))
	}
	var x tmdbSearch
	if err := t.request(ctx, "/search/tv", q, &x); err != nil {
		return nil, err
	}
	out := make([]Candidate, len(x.Results))
	for i, v := range x.Results {
		out[i] = Candidate{ProviderID: strconv.Itoa(v.ID), Title: v.Name, Year: yearOf(v.FirstAirDate), Overview: v.Overview, PosterPath: v.PosterPath, Popularity: v.Popularity}
	}
	return out, nil
}

type tmdbDetails struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	Name             string  `json:"name"`
	OriginalTitle    string  `json:"original_title"`
	OriginalName     string  `json:"original_name"`
	ReleaseDate      string  `json:"release_date"`
	FirstAirDate     string  `json:"first_air_date"`
	Runtime          int     `json:"runtime"`
	Overview         string  `json:"overview"`
	Tagline          string  `json:"tagline"`
	Status           string  `json:"status"`
	OriginalLanguage string  `json:"original_language"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	Genres           []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	ExternalIDs struct {
		IMDB string `json:"imdb_id"`
		TVDB int    `json:"tvdb_id"`
	} `json:"external_ids"`
}

func genres(v []struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}) []ProviderGenre {
	r := make([]ProviderGenre, len(v))
	for i, g := range v {
		r[i] = ProviderGenre{strconv.Itoa(g.ID), g.Name}
	}
	return r
}
func (t *TMDb) Movie(ctx context.Context, id, language, region string) (MovieDetails, error) {
	q := common(language, region)
	q.Set("append_to_response", "external_ids")
	var x tmdbDetails
	if err := t.request(ctx, "/movie/"+url.PathEscape(id), q, &x); err != nil {
		return MovieDetails{}, err
	}
	d := MovieDetails{Candidate: Candidate{ProviderID: strconv.Itoa(x.ID), Title: x.Title, Year: yearOf(x.ReleaseDate), Overview: x.Overview}, OriginalTitle: x.OriginalTitle, ReleaseDate: x.ReleaseDate, RuntimeMinutes: x.Runtime, Overview: x.Overview, Tagline: x.Tagline, Status: x.Status, OriginalLanguage: x.OriginalLanguage, Rating: x.VoteAverage, VoteCount: x.VoteCount, Genres: genres(x.Genres), ExternalIDs: map[string]string{"IMDB": x.ExternalIDs.IMDB}}
	if x.PosterPath != "" {
		d.Artwork = append(d.Artwork, ProviderArtwork{Type: "POSTER", Path: x.PosterPath})
	}
	if x.BackdropPath != "" {
		d.Artwork = append(d.Artwork, ProviderArtwork{Type: "BACKDROP", Path: x.BackdropPath})
	}
	return d, nil
}
func (t *TMDb) Show(ctx context.Context, id, language, region string) (ShowDetails, error) {
	q := common(language, region)
	q.Set("append_to_response", "external_ids")
	var x tmdbDetails
	if err := t.request(ctx, "/tv/"+url.PathEscape(id), q, &x); err != nil {
		return ShowDetails{}, err
	}
	d := ShowDetails{Candidate: Candidate{ProviderID: strconv.Itoa(x.ID), Title: x.Name, Year: yearOf(x.FirstAirDate), Overview: x.Overview}, OriginalTitle: x.OriginalName, FirstAirDate: x.FirstAirDate, Status: x.Status, Overview: x.Overview, OriginalLanguage: x.OriginalLanguage, Rating: x.VoteAverage, VoteCount: x.VoteCount, Genres: genres(x.Genres), ExternalIDs: map[string]string{"TVDB": strconv.Itoa(x.ExternalIDs.TVDB)}}
	if x.PosterPath != "" {
		d.Artwork = append(d.Artwork, ProviderArtwork{Type: "POSTER", Path: x.PosterPath})
	}
	if x.BackdropPath != "" {
		d.Artwork = append(d.Artwork, ProviderArtwork{Type: "BACKDROP", Path: x.BackdropPath})
	}
	return d, nil
}

type tmdbSeason struct {
	ID           int    `json:"id"`
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	AirDate      string `json:"air_date"`
	Episodes     []struct {
		ID            int    `json:"id"`
		EpisodeNumber int    `json:"episode_number"`
		Runtime       int    `json:"runtime"`
		Name          string `json:"name"`
		Overview      string `json:"overview"`
		AirDate       string `json:"air_date"`
		StillPath     string `json:"still_path"`
	} `json:"episodes"`
}

func (t *TMDb) Season(ctx context.Context, showID string, season int, language, region string) (SeasonDetails, error) {
	var x tmdbSeason
	if err := t.request(ctx, "/tv/"+url.PathEscape(showID)+"/season/"+strconv.Itoa(season), common(language, region), &x); err != nil {
		return SeasonDetails{}, err
	}
	r := SeasonDetails{SeasonNumber: x.SeasonNumber, Title: x.Name, Overview: x.Overview, AirDate: x.AirDate, ProviderID: strconv.Itoa(x.ID)}
	for _, e := range x.Episodes {
		r.Episodes = append(r.Episodes, EpisodeDetails{EpisodeNumber: e.EpisodeNumber, RuntimeMinutes: e.Runtime, Title: e.Name, Overview: e.Overview, AirDate: e.AirDate, ProviderID: strconv.Itoa(e.ID)})
	}
	return r, nil
}
func (t *TMDb) Test(ctx context.Context) error {
	var x struct {
		ID int `json:"id"`
	}
	return t.request(ctx, "/configuration", url.Values{}, &x)
}

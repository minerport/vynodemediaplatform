package metadata_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/vynode/media/server/internal/database"
	"github.com/vynode/media/server/internal/metadata"
)

type fakeProvider struct{}

func (fakeProvider) Name() string               { return "TMDB" }
func (fakeProvider) Test(context.Context) error { return nil }
func (fakeProvider) SearchMovies(context.Context, string, int, string, string) ([]metadata.Candidate, error) {
	return []metadata.Candidate{{ProviderID: "603", Title: "The Matrix", Year: 1999}}, nil
}
func (fakeProvider) Movie(context.Context, string, string, string) (metadata.MovieDetails, error) {
	return metadata.MovieDetails{Candidate: metadata.Candidate{ProviderID: "603", Title: "The Matrix", Year: 1999}, ReleaseDate: "1999-03-31", RuntimeMinutes: 136, Genres: []metadata.ProviderGenre{{ID: "28", Name: "Action"}}, ExternalIDs: map[string]string{"IMDB": "tt0133093"}, Credits: []metadata.ProviderCredit{{Person: metadata.ProviderPerson{ID: "1", Name: "Director"}, Type: "DIRECTOR"}, {Person: metadata.ProviderPerson{ID: "2", Name: "Actor"}, Type: "ACTOR", Character: "Lead"}}, Companies: []metadata.ProviderCompany{{ID: "3", Name: "Studio"}}}, nil
}
func (fakeProvider) SearchShows(context.Context, string, int, string, string) ([]metadata.Candidate, error) {
	return []metadata.Candidate{{ProviderID: "10", Title: "Example Show", Year: 2020}}, nil
}
func (fakeProvider) Show(context.Context, string, string, string) (metadata.ShowDetails, error) {
	return metadata.ShowDetails{Candidate: metadata.Candidate{ProviderID: "10", Title: "Example Show", Year: 2020}}, nil
}
func (fakeProvider) Season(context.Context, string, int, string, string) (metadata.SeasonDetails, error) {
	return metadata.SeasonDetails{SeasonNumber: 1, ProviderID: "100", Episodes: []metadata.EpisodeDetails{{EpisodeNumber: 2, ProviderID: "102", Title: "Two"}, {EpisodeNumber: 3, ProviderID: "103", Title: "Three"}}}, nil
}

func fixture(t *testing.T) (*database.Store, *metadata.Service) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	db, e := database.Open(ctx, filepath.Join(dir, "config"))
	if e != nil {
		t.Fatal(e)
	}
	if e = db.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	n := "2026-01-01T00:00:00Z"
	_, e = db.DB.Exec(`INSERT INTO libraries(id,name,type,enabled,created_at,updated_at) VALUES('lib','Library','MOVIES',1,?,?);INSERT INTO library_sources(id,library_id,configured_path,normalized_path,created_at) VALUES('src','lib','/media','/media',?);`, n, n, n)
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"f1", "f2", "tv"} {
		_, e = db.DB.Exec(`INSERT INTO media_files(id,source_id,relative_path,file_name,base_name,extension,parent_path,size_bytes,modified_at_ns,availability,probe_status,created_at,updated_at) VALUES(?,'src',?,?,?,'.mkv','',100,1,'AVAILABLE','COMPLETE',?,?)`, id, id+".mkv", id+".mkv", id, n, n)
		if e != nil {
			t.Fatal(e)
		}
	}
	return db, metadata.New(db.DB, filepath.Join(dir, "config"), fakeProvider{})
}
func TestDuplicateMovieVersionsAndUnmatch(t *testing.T) {
	db, s := fixture(t)
	defer db.Close()
	ctx := context.Background()
	a, e := s.MatchMovie(ctx, "f1", "603", false)
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.MatchMovie(ctx, "f2", "603", false)
	if e != nil {
		t.Fatal(e)
	}
	if a != b {
		t.Fatal("duplicate logical movie")
	}
	m, e := s.Movie(ctx, a)
	if e != nil || len(m.Versions) != 2 {
		t.Fatalf("versions=%d err=%v", len(m.Versions), e)
	}
	if len(m.Credits) != 2 || len(m.Companies) != 1 {
		t.Fatalf("enrichment missing: %+v", m)
	}
	var people, external int
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM people").Scan(&people)
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM external_ids").Scan(&external)
	if people != 2 || external != 1 {
		t.Fatalf("people=%d external=%d", people, external)
	}
	if e = s.Unmatch(ctx, "f1"); e != nil {
		t.Fatal(e)
	}
	m, e = s.Movie(ctx, a)
	if e != nil || len(m.Versions) != 1 {
		t.Fatalf("movie should remain: %+v %v", m, e)
	}
	if e = s.Unmatch(ctx, "f2"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Movie(ctx, a); e != metadata.ErrNotFound {
		t.Fatalf("orphan should leave active browse, got %v", e)
	}
	var files int
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM media_files").Scan(&files)
	if files != 3 {
		t.Fatal("physical inventory changed")
	}
}
func TestMultiEpisodeAssociation(t *testing.T) {
	db, s := fixture(t)
	defer db.Close()
	id, e := s.MatchTV(context.Background(), "tv", "10", 1, 2, 3, true)
	if e != nil {
		t.Fatal(e)
	}
	show, e := s.Show(context.Background(), id)
	if e != nil {
		t.Fatal(e)
	}
	if len(show.Seasons) != 1 || len(show.Seasons[0].Episodes) != 2 {
		t.Fatalf("unexpected TV tree: %+v", show)
	}
	var n int
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM media_associations WHERE media_file_id='tv'").Scan(&n)
	if n != 2 {
		t.Fatalf("associations=%d", n)
	}
}

type countingProvider struct {
	fakeProvider
	searches int
}

func (c *countingProvider) SearchMovies(context.Context, string, int, string, string) ([]metadata.Candidate, error) {
	c.searches++
	return []metadata.Candidate{{ProviderID: "603", Title: "The Matrix", Year: 1999}}, nil
}
func TestProviderSearchCacheTTL(t *testing.T) {
	db, _ := fixture(t)
	defer db.Close()
	p := &countingProvider{}
	s := metadata.New(db.DB, t.TempDir(), p)
	for i := 0; i < 2; i++ {
		if _, e := s.SearchProvider(context.Background(), "MOVIE", "The Matrix", 1999); e != nil {
			t.Fatal(e)
		}
	}
	if p.searches != 1 {
		t.Fatalf("within TTL requests=%d", p.searches)
	}
	_, _ = db.DB.Exec("UPDATE metadata_provider_cache SET expires_at='2000-01-01T00:00:00Z'")
	if _, e := s.SearchProvider(context.Background(), "MOVIE", "The Matrix", 1999); e != nil {
		t.Fatal(e)
	}
	if p.searches != 2 {
		t.Fatalf("expired cache requests=%d", p.searches)
	}
}

type blockingProvider struct {
	fakeProvider
	release chan struct{}
}

func (b *blockingProvider) SearchMovies(context.Context, string, int, string, string) ([]metadata.Candidate, error) {
	<-b.release
	return nil, nil
}
func TestDuplicateMetadataJobReturnsConflict(t *testing.T) {
	db, _ := fixture(t)
	defer db.Close()
	p := &blockingProvider{release: make(chan struct{})}
	s := metadata.New(db.DB, t.TempDir(), p)
	first, e := s.StartIdentify(context.Background(), "lib")
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.StartIdentify(context.Background(), "lib")
	if e != metadata.ErrConflict || second.ID != first.ID {
		t.Fatalf("job=%+v err=%v", second, e)
	}
	close(p.release)
}

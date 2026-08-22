package metadata

import "context"

type Provider interface {
	Name() string
	SearchMovies(context.Context, string, int, string, string) ([]Candidate, error)
	Movie(context.Context, string, string, string) (MovieDetails, error)
	SearchShows(context.Context, string, int, string, string) ([]Candidate, error)
	Show(context.Context, string, string, string) (ShowDetails, error)
	Season(context.Context, string, int, string, string) (SeasonDetails, error)
	Test(context.Context) error
}

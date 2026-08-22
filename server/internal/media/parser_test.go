package media

import "testing"

func TestFilenameParser(t *testing.T) {
	tests := []struct {
		name, title              string
		year, season, start, end int
	}{
		{"The.Thing.1982.1080p.BluRay.mkv", "The Thing", 1982, 0, 0, 0},
		{"Example.Show.S01E02.1080p.mkv", "Example Show", 0, 1, 2, 2},
		{"Example.Show.S01E02E03.mkv", "Example Show", 0, 1, 2, 3},
		{"Example Show 1x01.mkv", "Example Show", 0, 1, 1, 1},
	}
	for _, tt := range tests {
		got := ParseFilename(tt.name)
		if got.CandidateTitle != tt.title || got.CandidateYear != tt.year || got.SeasonNumber != tt.season || got.EpisodeStart != tt.start || got.EpisodeEnd != tt.end {
			t.Errorf("%s: %#v", tt.name, got)
		}
	}
}

func TestResolutionClassification(t *testing.T) {
	for _, x := range []struct {
		w, h int
		want string
	}{{3840, 1600, "2160P"}, {1920, 800, "1080P"}, {1280, 720, "720P"}, {640, 480, "SD"}, {0, 0, "OTHER"}} {
		if got := Resolution(x.w, x.h); got != x.want {
			t.Errorf("%dx%d: %s", x.w, x.h, got)
		}
	}
}

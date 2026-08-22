package playback

import (
	"context"
	"testing"
)

func TestSemanticAudioPreferenceAndCommentaryAvoidance(t *testing.T) {
	tracks := []Track{
		{ID: "commentary", Language: "eng", Commentary: true, Default: true, StreamIndex: 1},
		{ID: "japanese", Language: "ja", StreamIndex: 4},
		{ID: "main", Language: "en", StreamIndex: 7},
	}
	got, ok := semanticAudio(tracks, []string{"ja", "en"}, true)
	if !ok || got.ID != "japanese" {
		t.Fatalf("selected %+v", got)
	}
	got, _ = semanticAudio(tracks, []string{"en"}, true)
	if got.ID != "main" {
		t.Fatalf("commentary selected: %+v", got)
	}
}

func TestSubtitleModes(t *testing.T) {
	tracks := []Track{{ID: "normal", Language: "en", Usable: true}, {ID: "forced", Language: "en", Forced: true, Usable: true}, {ID: "hi", Language: "en", HearingImpaired: true, Usable: true}}
	p := defaultPreferences()
	p.SubtitleMode = "FORCED_ONLY"
	got, ok := semanticSubtitle(tracks, Track{Language: "ja"}, p)
	if !ok || got.ID != "forced" {
		t.Fatal(got)
	}
	p.SubtitleMode = "WHEN_AUDIO_NOT_PREFERRED"
	if _, ok = semanticSubtitle(tracks, Track{Language: "en"}, p); ok {
		t.Fatal("normal subtitles enabled for preferred audio")
	}
	got, ok = semanticSubtitle(tracks, Track{Language: "ja"}, p)
	if !ok || got.Language != "en" {
		t.Fatal(got)
	}
	p.SubtitleMode = "OFF"
	if _, ok = semanticSubtitle(tracks, Track{Language: "ja"}, p); ok {
		t.Fatal("subtitle selected while off")
	}
}

func TestVersionSwitchUsesSemanticTrackNotIndex(t *testing.T) {
	versions := []Version{{ID: "a", AudioTracks: []Track{{ID: "a-ja", Language: "ja", StreamIndex: 2}}}, {ID: "b", AudioTracks: []Track{{ID: "b-commentary", Language: "en", Commentary: true, StreamIndex: 2}, {ID: "b-ja", Language: "ja", StreamIndex: 8}}}}
	semantic, ok := findSemanticTrack(versions, "a-ja")
	if !ok {
		t.Fatal("missing semantic source")
	}
	got, ok := matchTrack(versions[1].AudioTracks, semantic)
	if !ok || got.ID != "b-ja" {
		t.Fatalf("mapped %+v", got)
	}
}

func TestPreferencesArePerUserAndDismissalReturnsAfterPlayback(t *testing.T) {
	s, done, _, _ := fixture(t)
	defer done()
	ctx := context.Background()
	a := defaultPreferences()
	a.AudioLanguages = []string{"ja", "en"}
	a.AutoplayNext = true
	b := defaultPreferences()
	b.AudioLanguages = []string{"en"}
	b.AutoplayNext = false
	if _, e := s.SetPreferences(ctx, "u1", a); e != nil {
		t.Fatal(e)
	}
	if _, e := s.SetPreferences(ctx, "u2", b); e != nil {
		t.Fatal(e)
	}
	pa, _ := s.Preferences(ctx, "u1")
	pb, _ := s.Preferences(ctx, "u2")
	if pa.AudioLanguages[0] != "ja" || pb.AudioLanguages[0] != "en" || !pa.AutoplayNext || pb.AutoplayNext {
		t.Fatalf("a=%+v b=%+v", pa, pb)
	}
	if e := s.DismissContinue(ctx, "u1", "MOVIE", "m"); e != nil {
		t.Fatal(e)
	}
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM continue_watching_dismissals WHERE user_id='u1' AND logical_id='m'").Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
	if _, e := s.Start(ctx, "u1", "s1", StartRequest{LogicalType: "MOVIE", LogicalID: "m", Capabilities: browser()}); e != nil {
		t.Fatal(e)
	}
	_ = s.db.QueryRow("SELECT COUNT(*) FROM continue_watching_dismissals WHERE user_id='u1' AND logical_id='m'").Scan(&count)
	if count != 0 {
		t.Fatal("dismissal did not clear")
	}
}

func TestMarkerValidationAndCreditsCompletion(t *testing.T) {
	s, done, _, _ := fixture(t)
	defer done()
	ctx := context.Background()
	if _, e := s.SaveMarker(ctx, Marker{LogicalType: "MOVIE", LogicalID: "m", Type: "INTRO", Start: 12, End: 5}); e != ErrValidation {
		t.Fatalf("invalid=%v", e)
	}
	credits, e := s.SaveMarker(ctx, Marker{LogicalType: "MOVIE", LogicalID: "m", Type: "CREDITS", Start: 80, End: 98})
	if e != nil || credits.Source != "MANUAL" {
		t.Fatal(credits, e)
	}
	x, e := s.Start(ctx, "u1", "s1", StartRequest{LogicalType: "MOVIE", LogicalID: "m", Capabilities: browser()})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Update(ctx, "u1", x.ID, Progress{State: Playing, Position: 80, Duration: 100}); e != nil {
		t.Fatal(e)
	}
	p, _ := s.Progress(ctx, "u1", "MOVIE", "m")
	if !p.Watched {
		t.Fatal("credits did not complete")
	}
}

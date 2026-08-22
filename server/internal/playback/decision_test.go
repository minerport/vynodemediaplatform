package playback

import "testing"

func browser() CapabilityProfile {
	return CapabilityProfile{SchemaVersion: 1, ClientName: "test", DirectPlay: true, Containers: []string{"mp4"}, VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}, MaxWidth: 1920, MaxHeight: 1080}
}
func TestSelectCompatibleLowerVersion(t *testing.T) {
	v, d := Select([]Version{{ID: "4k", Container: "mkv", VideoCodec: "hevc", AudioCodecs: []string{"truehd"}, Width: 3840, Height: 2160, Available: true}, {ID: "1080", Container: "mp4", VideoCodec: "h264", AudioCodecs: []string{"aac"}, Width: 1920, Height: 1080, Available: true}}, browser(), "")
	if d.Mode != DirectPlay || v.ID != "1080" {
		t.Fatalf("selected=%s decision=%+v", v.ID, d)
	}
}
func TestDecisionReasonCodes(t *testing.T) {
	cases := []struct {
		name string
		v    Version
		code string
	}{{"container", Version{Container: "mkv", VideoCodec: "h264", AudioCodecs: []string{"aac"}, Available: true}, "CONTAINER_UNSUPPORTED"}, {"video", Version{Container: "mp4", VideoCodec: "hevc", AudioCodecs: []string{"aac"}, Available: true}, "VIDEO_CODEC_UNSUPPORTED"}, {"audio", Version{Container: "mp4", VideoCodec: "h264", AudioCodecs: []string{"dts"}, Available: true}, "AUDIO_CODEC_UNSUPPORTED"}, {"resolution", Version{Container: "mp4", VideoCodec: "h264", AudioCodecs: []string{"aac"}, Width: 3840, Height: 2160, Available: true}, "RESOLUTION_UNSUPPORTED"}, {"hdr", Version{Container: "mp4", VideoCodec: "h264", AudioCodecs: []string{"aac"}, Width: 1920, Height: 1080, HDR: "HDR10", Available: true}, "HDR_UNSUPPORTED"}, {"missing", Version{Container: "mp4", VideoCodec: "h264", AudioCodecs: []string{"aac"}}, "MEDIA_UNAVAILABLE"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := Decide(c.v, browser())
			found := false
			for _, r := range d.Reasons {
				if r.Code == c.code {
					found = true
				}
			}
			if d.Mode != Unsupported || !found {
				t.Fatalf("decision=%+v", d)
			}
		})
	}
}
func TestManualUnsupportedAndNoCompatible(t *testing.T) {
	versions := []Version{{ID: "a", Container: "mkv", VideoCodec: "hevc", Available: true}}
	_, d := Select(versions, browser(), "a")
	if d.Mode != Unsupported || d.Reasons[0].Code != "CONTAINER_UNSUPPORTED" {
		t.Fatal(d)
	}
	_, d = Select(versions, browser(), "")
	if d.Reasons[0].Code != "NO_COMPATIBLE_VERSION" {
		t.Fatal(d)
	}
}

func TestPipelineDecisionHierarchy(t *testing.T) {
	p := browser()
	p.SchemaVersion = 2
	p.FragmentedMP4 = true
	p.MaxAudioChannels = 6
	remux := Version{ID: "mkv", Container: "mkv", VideoCodec: "h264", AudioTracks: []Track{{ID: "a1", Codec: "aac", Channels: 6, Default: true}}, Available: true}
	d := Decide(remux, p)
	if d.Mode != DirectStream || d.Plan.Video.Action != "COPY" || d.Plan.Audio.Action != "COPY" || d.Plan.Container.Target != "mp4" {
		t.Fatalf("remux=%+v", d)
	}
	audio := remux
	audio.AudioTracks[0].Codec = "ac3"
	d = Decide(audio, p)
	if d.Mode != AudioTranscode || d.Plan.Video.Action != "COPY" || d.Plan.Audio.TargetCodec != "aac" || d.Plan.Audio.TargetChannels != 6 {
		t.Fatalf("audio=%+v", d)
	}
	direct := Version{ID: "direct", Container: "mp4", VideoCodec: "h264", AudioTracks: []Track{{ID: "a", Codec: "aac", Default: true}}, Available: true}
	v, d := Select([]Version{remux, direct}, p, "")
	if v.ID != "direct" || d.Mode != DirectPlay {
		t.Fatalf("priority=%s %+v", v.ID, d)
	}
}

func TestVideoTranscodePolicyAndHierarchy(t *testing.T) {
	p := browser()
	p.SchemaVersion = 2
	p.FragmentedMP4 = true
	hevc := Version{ID: "hevc", Container: "mkv", VideoCodec: "hevc", AudioTracks: []Track{{Codec: "aac", Default: true}}, Width: 1920, Height: 1080, Bitrate: 12_000_000, HDR: "SDR", Available: true}
	d := DecideWithPolicy(hevc, p, "", "", "720p", 5_000_000)
	if d.Mode != VideoTranscode || d.Plan.Video.TargetCodec != "h264" || d.Plan.Video.TargetHeight != 720 || d.Plan.Video.PixelFormat != "yuv420p" {
		t.Fatalf("video=%+v", d)
	}
	if d.Plan.Video.TargetWidth != 1280 || d.Plan.Video.TargetBitrate != 4_000_000 {
		t.Fatalf("quality=%+v", d.Plan.Video)
	}
	compatible := Version{ID: "720", Container: "mp4", VideoCodec: "h264", AudioTracks: []Track{{Codec: "aac", Default: true}}, Width: 1280, Height: 720, Bitrate: 3_000_000, HDR: "SDR", Available: true}
	v, chosen := SelectPolicy([]Version{hevc, compatible}, p, "", "", "", "720p", 5_000_000)
	if v.ID != "720" || chosen.Mode != DirectPlay {
		t.Fatalf("unnecessary transcode: %s %+v", v.ID, chosen)
	}
}

func TestBitrateResolutionAndEvenScaling(t *testing.T) {
	p := browser()
	p.SchemaVersion = 2
	p.FragmentedMP4 = true
	v := Version{ID: "high", Container: "mp4", VideoCodec: "h264", AudioTracks: []Track{{Codec: "aac"}}, Width: 1919, Height: 1079, Bitrate: 50_000_000, HDR: "SDR", Available: true}
	d := DecideWithPolicy(v, p, "", "", "720p", 8_000_000)
	if d.Mode != VideoTranscode || d.Plan.Video.TargetWidth%2 != 0 || d.Plan.Video.TargetHeight%2 != 0 {
		t.Fatalf("decision=%+v", d)
	}
	want := map[string]bool{"BITRATE_LIMIT_EXCEEDED": false, "RESOLUTION_LIMIT_EXCEEDED": false}
	for _, r := range d.Reasons {
		if _, ok := want[r.Code]; ok {
			want[r.Code] = true
		}
	}
	for k, ok := range want {
		if !ok {
			t.Fatalf("missing %s", k)
		}
	}
}

func TestHDRTranscodeFailsTruthfully(t *testing.T) {
	p := browser()
	p.SchemaVersion = 2
	p.FragmentedMP4 = true
	d := Decide(Version{ID: "hdr", Container: "mkv", VideoCodec: "hevc", AudioCodecs: []string{"aac"}, HDR: "HDR10_OR_PQ", Available: true}, p)
	if d.Mode != Unsupported || d.Reasons[len(d.Reasons)-1].Code != "HDR_TONE_MAPPING_UNAVAILABLE" {
		t.Fatal(d)
	}
}

func TestTrackSelectionAndImageSubtitleTruth(t *testing.T) {
	p := browser()
	p.SchemaVersion = 2
	p.FragmentedMP4 = true
	v := Version{ID: "v", Container: "mkv", VideoCodec: "h264", Available: true, AudioTracks: []Track{{ID: "commentary", Codec: "aac", Commentary: true, Default: true}, {ID: "main", Codec: "aac"}}, SubtitleTracks: []Track{{ID: "pgs", Codec: "hdmv_pgs_subtitle", Usable: false}}}
	d := DecideTracks(v, p, "", "")
	if d.Plan.Audio.TrackID != "main" {
		t.Fatal(d.Plan.Audio)
	}
	d = DecideTracks(v, p, "main", "pgs")
	if d.Mode != Unsupported || d.Reasons[len(d.Reasons)-1].Code != "SUBTITLE_REQUIRES_VIDEO_TRANSCODE" {
		t.Fatal(d)
	}
}

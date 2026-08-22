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

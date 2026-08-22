package playback

import "strings"

func Decide(v Version, p CapabilityProfile) Decision {
	d := Decision{Mode: DirectPlay, MediaVersionID: v.ID, Container: v.Container, VideoCodec: v.VideoCodec, AudioCodecs: v.AudioCodecs, Reasons: []Reason{}}
	if !v.Available {
		d.Reasons = append(d.Reasons, Reason{"MEDIA_UNAVAILABLE", ""})
	}
	if !p.DirectPlay {
		d.Reasons = append(d.Reasons, Reason{"DIRECT_PLAY_DISABLED", ""})
	}
	if !contains(p.Containers, v.Container) {
		d.Reasons = append(d.Reasons, Reason{"CONTAINER_UNSUPPORTED", v.Container})
	}
	if !contains(p.VideoCodecs, v.VideoCodec) {
		d.Reasons = append(d.Reasons, Reason{"VIDEO_CODEC_UNSUPPORTED", v.VideoCodec})
	}
	for _, a := range v.AudioCodecs {
		if !contains(p.AudioCodecs, a) {
			d.Reasons = append(d.Reasons, Reason{"AUDIO_CODEC_UNSUPPORTED", a})
		}
	}
	if (p.MaxWidth > 0 && v.Width > p.MaxWidth) || (p.MaxHeight > 0 && v.Height > p.MaxHeight) {
		d.Reasons = append(d.Reasons, Reason{"RESOLUTION_UNSUPPORTED", v.Resolution})
	}
	if v.HDR != "" && strings.ToUpper(v.HDR) != "SDR" && !contains(p.HDR, v.HDR) {
		d.Reasons = append(d.Reasons, Reason{"HDR_UNSUPPORTED", v.HDR})
	}
	if len(d.Reasons) > 0 {
		d.Mode = Unsupported
	}
	return d
}
func contains(values []string, value string) bool {
	for _, x := range values {
		if strings.EqualFold(strings.TrimSpace(x), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

func Select(versions []Version, p CapabilityProfile, requested string) (Version, Decision) {
	if requested != "" {
		for _, v := range versions {
			if v.ID == requested {
				return v, Decide(v, p)
			}
		}
		return Version{}, Decision{Mode: Unsupported, Reasons: []Reason{{"VERSION_NOT_FOUND", ""}}}
	}
	var best Version
	for _, v := range versions {
		if Decide(v, p).Mode != DirectPlay {
			continue
		}
		if best.ID == "" || v.Height > best.Height || (v.Height == best.Height && v.Bitrate > best.Bitrate) {
			best = v
		}
	}
	if best.ID != "" {
		return best, Decide(best, p)
	}
	return Version{}, Decision{Mode: Unsupported, Reasons: []Reason{{"NO_COMPATIBLE_VERSION", ""}}}
}

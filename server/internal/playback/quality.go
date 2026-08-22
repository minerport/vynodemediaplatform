package playback

import "strings"

var qualityProfiles = []QualityProfile{
	{ID: "1080p", Label: "1080p", MaxWidth: 1920, MaxHeight: 1080, TargetVideoBitrate: 8_000_000, MaxVideoBitrate: 10_000_000, AudioBitrate: 192_000},
	{ID: "720p", Label: "720p", MaxWidth: 1280, MaxHeight: 720, TargetVideoBitrate: 4_000_000, MaxVideoBitrate: 5_000_000, AudioBitrate: 192_000},
	{ID: "480p", Label: "480p", MaxWidth: 854, MaxHeight: 480, TargetVideoBitrate: 1_500_000, MaxVideoBitrate: 2_000_000, AudioBitrate: 128_000},
}

func QualityProfiles(v Version) []QualityProfile {
	out := []QualityProfile{{ID: "original", Label: "Original", MaxWidth: v.Width, MaxHeight: v.Height, TargetVideoBitrate: v.Bitrate, MaxVideoBitrate: v.Bitrate}}
	for i, q := range qualityProfiles {
		if v.Height == 0 || q.MaxHeight <= v.Height || i == len(qualityProfiles)-1 {
			out = append(out, q)
		}
	}
	return out
}

func quality(id string, v Version, p CapabilityProfile, bandwidth int64) QualityProfile {
	if strings.EqualFold(id, "original") {
		return QualityProfile{ID: "original", Label: "Original", MaxWidth: v.Width, MaxHeight: v.Height, TargetVideoBitrate: v.Bitrate, MaxVideoBitrate: v.Bitrate}
	}
	for _, q := range qualityProfiles {
		if strings.EqualFold(q.ID, id) {
			return q
		}
	}
	maxH := p.MaxHeight
	if maxH <= 0 {
		maxH = v.Height
	}
	for _, q := range qualityProfiles {
		if q.MaxHeight <= maxH && (v.Height == 0 || q.MaxHeight <= v.Height) && (bandwidth <= 0 || q.MaxVideoBitrate+q.AudioBitrate <= bandwidth) {
			return q
		}
	}
	return qualityProfiles[len(qualityProfiles)-1]
}

func evenScale(v Version, q QualityProfile) (int, int) {
	w, h := v.Width, v.Height
	if w <= 0 || h <= 0 {
		return q.MaxWidth, q.MaxHeight
	}
	if w <= q.MaxWidth && h <= q.MaxHeight {
		return w - w%2, h - h%2
	}
	r := float64(q.MaxWidth) / float64(w)
	if x := float64(q.MaxHeight) / float64(h); x < r {
		r = x
	}
	w = int(float64(w) * r)
	h = int(float64(h) * r)
	return w - w%2, h - h%2
}

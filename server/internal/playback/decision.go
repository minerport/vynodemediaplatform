package playback

import "strings"

func Decide(v Version, p CapabilityProfile) Decision { return DecideTracks(v, p, "", "") }
func DecideTracks(v Version, p CapabilityProfile, audioID, subtitleID string) Decision {
	d := Decision{MediaVersionID: v.ID, Container: v.Container, VideoCodec: v.VideoCodec, AudioCodecs: v.AudioCodecs, Reasons: []Reason{}, Plan: PipelinePlan{Video: StreamPlan{Action: "COPY", SourceCodec: v.VideoCodec}, Container: ContainerPlan{Source: v.Container, Target: v.Container}, Subtitles: SubtitlePlan{Action: "NONE"}}}
	if !v.Available {
		return reject(d, "MEDIA_UNAVAILABLE", "")
	}
	if !contains(p.Containers, v.Container) && (!p.FragmentedMP4 || !contains(p.Containers, "mp4")) {
		return reject(d, "CONTAINER_UNSUPPORTED", v.Container)
	}
	if !contains(p.VideoCodecs, v.VideoCodec) {
		return reject(d, "VIDEO_CODEC_UNSUPPORTED", v.VideoCodec)
	}
	if (p.MaxWidth > 0 && v.Width > p.MaxWidth) || (p.MaxHeight > 0 && v.Height > p.MaxHeight) {
		return reject(d, "RESOLUTION_UNSUPPORTED", v.Resolution)
	}
	if v.HDR != "" && strings.ToUpper(v.HDR) != "SDR" && !contains(p.HDR, v.HDR) {
		return reject(d, "HDR_UNSUPPORTED", v.HDR)
	}
	a, ok := chooseAudio(v.AudioTracks, audioID)
	if !ok && len(v.AudioCodecs) > 0 {
		a = Track{Codec: v.AudioCodecs[0], Channels: 2, Usable: true}
		ok = true
	}
	if ok {
		d.Plan.Audio = StreamPlan{Action: "COPY", SourceCodec: a.Codec, SourceChannels: a.Channels, TargetChannels: a.Channels, TrackID: a.ID}
	}
	if subtitleID != "" {
		s, found := findTrack(v.SubtitleTracks, subtitleID)
		if !found {
			return reject(d, "SUBTITLE_NOT_FOUND", subtitleID)
		}
		if !s.Usable {
			return reject(d, "SUBTITLE_REQUIRES_VIDEO_TRANSCODE", s.Codec)
		}
		d.Plan.Subtitles = SubtitlePlan{Action: "WEBVTT", TrackID: s.ID}
	}
	containerOK := contains(p.Containers, v.Container)
	audioOK := !ok || contains(p.AudioCodecs, a.Codec)
	if p.DirectPlay && containerOK && audioOK {
		d.Mode = DirectPlay
		return d
	}
	if !p.FragmentedMP4 || !contains(p.Containers, "mp4") {
		if !containerOK {
			return reject(d, "CONTAINER_UNSUPPORTED", v.Container)
		}
		return reject(d, "AUDIO_CODEC_UNSUPPORTED", a.Codec)
	}
	d.Plan.Container.Target = "mp4"
	if audioOK {
		d.Mode = DirectStream
		d.Reasons = append(d.Reasons, Reason{"SOURCE_CONTAINER_UNSUPPORTED", v.Container}, Reason{"VIDEO_STREAM_COPY_SUPPORTED", v.VideoCodec}, Reason{"AUDIO_STREAM_COPY_SUPPORTED", a.Codec}, Reason{"TARGET_CONTAINER_SUPPORTED", "mp4"})
		return d
	}
	if !contains(p.AudioCodecs, "aac") {
		return reject(d, "NO_COMPATIBLE_AUDIO_TARGET", a.Codec)
	}
	target := a.Channels
	if target <= 0 {
		target = 2
	}
	if p.MaxAudioChannels > 0 && target > p.MaxAudioChannels {
		target = p.MaxAudioChannels
	}
	if target > 6 {
		target = 6
	}
	d.Mode = AudioTranscode
	d.Plan.Audio.Action = "TRANSCODE"
	d.Plan.Audio.TargetCodec = "aac"
	d.Plan.Audio.TargetChannels = target
	d.Reasons = append(d.Reasons, Reason{"AUDIO_CODEC_UNSUPPORTED", a.Codec}, Reason{"VIDEO_STREAM_COPY_SUPPORTED", v.VideoCodec}, Reason{"AUDIO_TRANSCODE_AVAILABLE", "aac"}, Reason{"TARGET_CONTAINER_SUPPORTED", "mp4"})
	return d
}
func reject(d Decision, code, value string) Decision {
	d.Mode = Unsupported
	d.Reasons = append(d.Reasons, Reason{code, value})
	return d
}
func chooseAudio(ts []Track, id string) (Track, bool) {
	if id != "" {
		return findTrack(ts, id)
	}
	for _, t := range ts {
		if t.Default && !t.Commentary {
			return t, true
		}
	}
	for _, t := range ts {
		if !t.Commentary {
			return t, true
		}
	}
	if len(ts) > 0 {
		return ts[0], true
	}
	return Track{}, false
}
func findTrack(ts []Track, id string) (Track, bool) {
	for _, t := range ts {
		if t.ID == id {
			return t, true
		}
	}
	return Track{}, false
}
func contains(v []string, x string) bool {
	for _, s := range v {
		if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(x)) {
			return true
		}
	}
	return false
}
func Select(vs []Version, p CapabilityProfile, requested string) (Version, Decision) {
	return SelectTracks(vs, p, requested, "", "")
}
func SelectTracks(vs []Version, p CapabilityProfile, requested, audio, subtitle string) (Version, Decision) {
	if requested != "" {
		for _, v := range vs {
			if v.ID == requested {
				return v, DecideTracks(v, p, audio, subtitle)
			}
		}
		return Version{}, Decision{Mode: Unsupported, Reasons: []Reason{{"VERSION_NOT_FOUND", ""}}}
	}
	for _, mode := range []Mode{DirectPlay, DirectStream, AudioTranscode} {
		var best Version
		var bd Decision
		for _, v := range vs {
			d := DecideTracks(v, p, audio, subtitle)
			if d.Mode == mode && (best.ID == "" || v.Height > best.Height || (v.Height == best.Height && v.Bitrate > best.Bitrate)) {
				best, bd = v, d
			}
		}
		if best.ID != "" {
			return best, bd
		}
	}
	return Version{}, Decision{Mode: Unsupported, Reasons: []Reason{{"NO_COMPATIBLE_VERSION", ""}}}
}

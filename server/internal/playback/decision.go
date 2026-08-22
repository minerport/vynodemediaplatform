package playback

import (
	"fmt"
	"strings"
)

func Decide(v Version, p CapabilityProfile) Decision { return DecideWithPolicy(v, p, "", "", "", 0) }
func DecideTracks(v Version, p CapabilityProfile, audioID, subtitleID string) Decision {
	return DecideWithPolicy(v, p, audioID, subtitleID, "", 0)
}
func DecideWithPolicy(v Version, p CapabilityProfile, audioID, subtitleID, qualityID string, bandwidth int64) Decision {
	d := Decision{MediaVersionID: v.ID, Container: v.Container, VideoCodec: v.VideoCodec, AudioCodecs: v.AudioCodecs, Reasons: []Reason{}, Plan: PipelinePlan{Video: StreamPlan{Action: "COPY", SourceCodec: v.VideoCodec}, Container: ContainerPlan{Source: v.Container, Target: v.Container}, Subtitles: SubtitlePlan{Action: "NONE"}}}
	if !v.Available {
		return reject(d, "MEDIA_UNAVAILABLE", "")
	}
	q := quality(qualityID, v, p, bandwidth)
	videoOK := contains(p.VideoCodecs, v.VideoCodec)
	resolutionOK := (p.MaxWidth <= 0 || v.Width <= p.MaxWidth) && (p.MaxHeight <= 0 || v.Height <= p.MaxHeight) && (q.MaxWidth <= 0 || v.Width <= q.MaxWidth) && (q.MaxHeight <= 0 || v.Height <= q.MaxHeight)
	bitrateOK := bandwidth <= 0 || v.Bitrate <= 0 || v.Bitrate <= bandwidth
	hdrOK := v.HDR == "" || strings.EqualFold(v.HDR, "SDR") || contains(p.HDR, v.HDR)
	if !p.FragmentedMP4 {
		if !contains(p.Containers, v.Container) {
			return reject(d, "CONTAINER_UNSUPPORTED", v.Container)
		}
		if !videoOK {
			return reject(d, "VIDEO_CODEC_UNSUPPORTED", v.VideoCodec)
		}
		if !resolutionOK {
			return reject(d, "RESOLUTION_UNSUPPORTED", v.Resolution)
		}
		if !hdrOK {
			return reject(d, "HDR_UNSUPPORTED", v.HDR)
		}
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
	if p.DirectPlay && containerOK && audioOK && videoOK && resolutionOK && bitrateOK && hdrOK {
		d.Mode = DirectPlay
		return d
	}
	if videoOK && resolutionOK && bitrateOK && hdrOK && (!p.FragmentedMP4 || !contains(p.Containers, "mp4")) {
		if !containerOK {
			return reject(d, "CONTAINER_UNSUPPORTED", v.Container)
		}
		return reject(d, "AUDIO_CODEC_UNSUPPORTED", a.Codec)
	}
	d.Plan.Container.Target = "mp4"
	if videoOK && resolutionOK && bitrateOK && hdrOK && audioOK {
		d.Mode = DirectStream
		d.Reasons = append(d.Reasons, Reason{"SOURCE_CONTAINER_UNSUPPORTED", v.Container}, Reason{"VIDEO_STREAM_COPY_SUPPORTED", v.VideoCodec}, Reason{"AUDIO_STREAM_COPY_SUPPORTED", a.Codec}, Reason{"TARGET_CONTAINER_SUPPORTED", "mp4"})
		return d
	}
	if videoOK && resolutionOK && bitrateOK && hdrOK && !contains(p.AudioCodecs, "aac") {
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
	if videoOK && resolutionOK && bitrateOK && hdrOK {
		d.Mode = AudioTranscode
		d.Plan.Audio.Action = "TRANSCODE"
		d.Plan.Audio.TargetCodec = "aac"
		d.Plan.Audio.TargetChannels = target
		d.Reasons = append(d.Reasons, Reason{"AUDIO_CODEC_UNSUPPORTED", a.Codec}, Reason{"VIDEO_STREAM_COPY_SUPPORTED", v.VideoCodec}, Reason{"AUDIO_TRANSCODE_AVAILABLE", "aac"}, Reason{"TARGET_CONTAINER_SUPPORTED", "mp4"})
		return d
	}
	if !hdrOK {
		return reject(d, "HDR_TONE_MAPPING_UNAVAILABLE", v.HDR)
	}
	if !contains(p.VideoCodecs, "h264") || !p.FragmentedMP4 {
		return reject(d, "NO_COMPATIBLE_VIDEO_TARGET", v.VideoCodec)
	}
	w, h := evenScale(v, q)
	d.Mode = VideoTranscode
	d.Plan.Quality = q.ID
	d.Plan.Container.Target = "HLS_FMP4"
	d.Plan.Backend = BackendPlan{Requested: "AUTO", Actual: "SOFTWARE"}
	d.Plan.Video = StreamPlan{Action: "TRANSCODE", SourceCodec: v.VideoCodec, TargetCodec: "h264", SourceWidth: v.Width, SourceHeight: v.Height, TargetWidth: w, TargetHeight: h, SourceBitrate: v.Bitrate, TargetBitrate: q.TargetVideoBitrate, PixelFormat: "yuv420p", Encoder: "libx264", HDRHandling: "PRESERVE_SDR"}
	if !audioOK {
		d.Plan.Audio.Action = "TRANSCODE"
		d.Plan.Audio.TargetCodec = "aac"
		d.Plan.Audio.TargetChannels = target
	}
	if !videoOK {
		d.Reasons = append(d.Reasons, Reason{"VIDEO_CODEC_UNSUPPORTED", v.VideoCodec})
	}
	if !resolutionOK {
		d.Reasons = append(d.Reasons, Reason{"RESOLUTION_LIMIT_EXCEEDED", v.Resolution})
	}
	if !bitrateOK {
		d.Reasons = append(d.Reasons, Reason{"BITRATE_LIMIT_EXCEEDED", fmt.Sprint(bandwidth)})
	}
	d.Reasons = append(d.Reasons, Reason{"VIDEO_TRANSCODE_AVAILABLE", "h264"})
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
	return SelectPolicy(vs, p, requested, audio, subtitle, "", 0)
}
func SelectPolicy(vs []Version, p CapabilityProfile, requested, audio, subtitle, qualityID string, bandwidth int64) (Version, Decision) {
	if requested != "" {
		for _, v := range vs {
			if v.ID == requested {
				return v, DecideWithPolicy(v, p, audio, subtitle, qualityID, bandwidth)
			}
		}
		return Version{}, Decision{Mode: Unsupported, Reasons: []Reason{{"VERSION_NOT_FOUND", ""}}}
	}
	for _, mode := range []Mode{DirectPlay, DirectStream, AudioTranscode, VideoTranscode} {
		var best Version
		var bd Decision
		for _, v := range vs {
			d := DecideWithPolicy(v, p, audio, subtitle, qualityID, bandwidth)
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

package playback

type Mode string
type State string
type PipelineState string

const (
	DirectPlay       Mode          = "DIRECT_PLAY"
	DirectStream     Mode          = "DIRECT_STREAM"
	AudioTranscode   Mode          = "AUDIO_TRANSCODE"
	Unsupported      Mode          = "UNSUPPORTED"
	VideoTranscode   Mode          = "VIDEO_TRANSCODE" // reserved for Phase 6
	Starting         State         = "STARTING"
	Playing          State         = "PLAYING"
	Paused           State         = "PAUSED"
	Stopped          State         = "STOPPED"
	Completed        State         = "COMPLETED"
	Error            State         = "ERROR"
	PipelineStarting PipelineState = "STARTING"
	PipelineRunning  PipelineState = "RUNNING"
	PipelineStopping PipelineState = "STOPPING"
	PipelineStopped  PipelineState = "STOPPED"
	PipelineFailed   PipelineState = "FAILED"
)

type CapabilityProfile struct {
	SchemaVersion    int      `json:"schemaVersion"`
	ClientName       string   `json:"clientName"`
	ClientVersion    string   `json:"clientVersion,omitempty"`
	Platform         string   `json:"platform"`
	PlatformVersion  string   `json:"platformVersion,omitempty"`
	DeviceModel      string   `json:"deviceModel,omitempty"`
	Containers       []string `json:"supportedContainers"`
	VideoCodecs      []string `json:"supportedVideoCodecs"`
	AudioCodecs      []string `json:"supportedAudioCodecs"`
	SubtitleFormats  []string `json:"subtitleFormats,omitempty"`
	MaxWidth         int      `json:"maximumVideoWidth,omitempty"`
	MaxHeight        int      `json:"maximumVideoHeight,omitempty"`
	MaxAudioChannels int      `json:"maximumAudioChannels,omitempty"`
	HDR              []string `json:"hdrCapabilities,omitempty"`
	DirectPlay       bool     `json:"directPlaySupport"`
	FragmentedMP4    bool     `json:"fragmentedMp4Support,omitempty"`
}
type Reason struct {
	Code  string `json:"code"`
	Value string `json:"value,omitempty"`
}
type Track struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Codec           string `json:"codec"`
	Language        string `json:"language,omitempty"`
	Title           string `json:"title,omitempty"`
	Channels        int    `json:"channels,omitempty"`
	Default         bool   `json:"default"`
	Forced          bool   `json:"forced,omitempty"`
	Commentary      bool   `json:"commentary,omitempty"`
	HearingImpaired bool   `json:"hearingImpaired,omitempty"`
	Usable          bool   `json:"usable"`
	Reason          string `json:"reason,omitempty"`
	Source          string `json:"source,omitempty"`
	StreamIndex     int    `json:"-"`
	Path            string `json:"-"`
}
type Version struct {
	ID             string   `json:"id"`
	FileID         string   `json:"-"`
	Container      string   `json:"container"`
	VideoCodec     string   `json:"videoCodec"`
	AudioCodecs    []string `json:"audioCodecs"`
	AudioTracks    []Track  `json:"audioTracks,omitempty"`
	SubtitleTracks []Track  `json:"subtitleTracks,omitempty"`
	Width          int      `json:"width,omitempty"`
	Height         int      `json:"height,omitempty"`
	Bitrate        int64    `json:"bitrate,omitempty"`
	HDR            string   `json:"hdr,omitempty"`
	Resolution     string   `json:"resolution,omitempty"`
	Label          string   `json:"label,omitempty"`
	Available      bool     `json:"available"`
}
type StreamPlan struct {
	Action         string `json:"action"`
	SourceCodec    string `json:"sourceCodec,omitempty"`
	TargetCodec    string `json:"targetCodec,omitempty"`
	SourceChannels int    `json:"sourceChannels,omitempty"`
	TargetChannels int    `json:"targetChannels,omitempty"`
	TrackID        string `json:"trackId,omitempty"`
	SourceWidth    int    `json:"sourceWidth,omitempty"`
	SourceHeight   int    `json:"sourceHeight,omitempty"`
	TargetWidth    int    `json:"targetWidth,omitempty"`
	TargetHeight   int    `json:"targetHeight,omitempty"`
	SourceBitrate  int64  `json:"sourceBitrate,omitempty"`
	TargetBitrate  int64  `json:"targetBitrate,omitempty"`
	PixelFormat    string `json:"pixelFormat,omitempty"`
	Encoder        string `json:"encoder,omitempty"`
	HDRHandling    string `json:"hdrHandling,omitempty"`
}
type ContainerPlan struct {
	Source string `json:"source"`
	Target string `json:"target"`
}
type SubtitlePlan struct {
	Action  string `json:"action"`
	TrackID string `json:"trackId,omitempty"`
}
type PipelinePlan struct {
	Video     StreamPlan    `json:"video"`
	Audio     StreamPlan    `json:"audio"`
	Container ContainerPlan `json:"container"`
	Subtitles SubtitlePlan  `json:"subtitles"`
	Quality   string        `json:"quality,omitempty"`
	Backend   BackendPlan   `json:"backend"`
}
type BackendPlan struct {
	Requested      string `json:"requested"`
	Actual         string `json:"actual"`
	FallbackReason string `json:"fallbackReason,omitempty"`
}
type QualityProfile struct {
	ID                 string `json:"id"`
	Label              string `json:"label"`
	MaxWidth           int    `json:"maxWidth"`
	MaxHeight          int    `json:"maxHeight"`
	TargetVideoBitrate int64  `json:"targetVideoBitrate"`
	MaxVideoBitrate    int64  `json:"maxVideoBitrate"`
	AudioBitrate       int64  `json:"audioBitrate"`
}
type NetworkContext string

const (
	NetworkLocal  NetworkContext = "LOCAL"
	NetworkRemote NetworkContext = "REMOTE"
)

type Decision struct {
	Mode           Mode         `json:"mode"`
	MediaVersionID string       `json:"mediaVersionId,omitempty"`
	Container      string       `json:"container,omitempty"`
	VideoCodec     string       `json:"videoCodec,omitempty"`
	AudioCodecs    []string     `json:"audioCodecs,omitempty"`
	Plan           PipelinePlan `json:"plan"`
	Reasons        []Reason     `json:"reasons"`
}
type Session struct {
	ID               string           `json:"id"`
	UserID           string           `json:"-"`
	LogicalType      string           `json:"logicalType"`
	LogicalID        string           `json:"logicalId"`
	MediaVersion     Version          `json:"selectedVersion"`
	Decision         Decision         `json:"decision"`
	State            State            `json:"state"`
	Position         float64          `json:"position"`
	Duration         float64          `json:"duration"`
	ResumePosition   float64          `json:"resumePosition"`
	MediaURL         string           `json:"mediaUrl,omitempty"`
	HLSURL           string           `json:"hlsUrl,omitempty"`
	Qualities        []QualityProfile `json:"availableQualities,omitempty"`
	SubtitleURL      string           `json:"subtitleUrl,omitempty"`
	StartedAt        string           `json:"startedAt"`
	LastActivityAt   string           `json:"lastActivityAt"`
	Title            string           `json:"title,omitempty"`
	UserDisplayName  string           `json:"userDisplayName,omitempty"`
	ClientName       string           `json:"clientName,omitempty"`
	Platform         string           `json:"platform,omitempty"`
	SelectedAudio    Track            `json:"selectedAudioTrack,omitempty"`
	SelectedSubtitle *Track           `json:"selectedSubtitleTrack,omitempty"`
	Markers          []Marker         `json:"markers,omitempty"`
	Navigation       Navigation       `json:"navigation,omitempty"`
	ContextID        string           `json:"playbackContextId,omitempty"`
	Network          NetworkContext   `json:"networkContext,omitempty"`
	EffectiveLimit   int64            `json:"effectiveBandwidthLimit,omitempty"`
}
type StartRequest struct {
	LogicalType             string            `json:"logicalType"`
	LogicalID               string            `json:"logicalId"`
	RequestedVersionID      string            `json:"requestedVersionId,omitempty"`
	SelectedAudioTrackID    string            `json:"selectedAudioTrackId,omitempty"`
	SelectedSubtitleTrackID string            `json:"selectedSubtitleTrackId,omitempty"`
	Resume                  bool              `json:"resume"`
	Capabilities            CapabilityProfile `json:"capabilities"`
	QualityID               string            `json:"qualityId,omitempty"`
	StartPosition           float64           `json:"startPosition,omitempty"`
	Network                 NetworkContext    `json:"networkContext,omitempty"`
	BandwidthLimit          int64             `json:"bandwidthLimit,omitempty"`
	ContextID               string            `json:"playbackContextId,omitempty"`
	ContextType             string            `json:"contextType,omitempty"`
}

type PlaybackPreferences struct {
	AudioLanguages        []string `json:"preferredAudioLanguages"`
	SubtitleLanguages     []string `json:"preferredSubtitleLanguages"`
	SubtitleMode          string   `json:"subtitleMode"`
	AutoplayNext          bool     `json:"autoplayNextEpisode"`
	LocalQualityID        string   `json:"localQualityId"`
	RemoteQualityID       string   `json:"remoteQualityId"`
	AvoidCommentary       bool     `json:"avoidCommentary"`
	PreferHearingImpaired bool     `json:"preferHearingImpaired"`
}
type Marker struct {
	ID          string   `json:"id"`
	LogicalType string   `json:"logicalType"`
	LogicalID   string   `json:"logicalId"`
	Type        string   `json:"type"`
	Source      string   `json:"source"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
	Start       float64  `json:"start"`
	End         float64  `json:"end"`
	Confidence  *float64 `json:"confidence,omitempty"`
}
type NavigationItem struct {
	LogicalID     string `json:"logicalId"`
	ShowID        string `json:"showId"`
	ShowTitle     string `json:"showTitle"`
	Title         string `json:"title"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	Available     bool   `json:"available"`
}
type Navigation struct {
	Previous         *NavigationItem `json:"previous,omitempty"`
	Next             *NavigationItem `json:"next,omitempty"`
	Autoplay         bool            `json:"autoplay"`
	CountdownSeconds int             `json:"countdownSeconds"`
}
type Progress struct {
	State    State   `json:"state"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
}
type WatchProgress struct {
	LogicalType  string  `json:"logicalType"`
	LogicalID    string  `json:"logicalId"`
	Position     float64 `json:"position"`
	Duration     float64 `json:"duration"`
	Watched      bool    `json:"watched"`
	LastPlayedAt string  `json:"lastPlayedAt"`
}
type ContinueItem struct {
	LogicalType  string  `json:"logicalType"`
	LogicalID    string  `json:"logicalId"`
	Title        string  `json:"title"`
	Position     float64 `json:"position"`
	Duration     float64 `json:"duration"`
	Progress     float64 `json:"progress"`
	LastPlayedAt string  `json:"lastPlayedAt"`
}

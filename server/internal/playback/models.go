package playback

type Mode string
type State string

const (
	DirectPlay  Mode  = "DIRECT_PLAY"
	Unsupported Mode  = "UNSUPPORTED"
	Starting    State = "STARTING"
	Playing     State = "PLAYING"
	Paused      State = "PAUSED"
	Stopped     State = "STOPPED"
	Completed   State = "COMPLETED"
	Error       State = "ERROR"
)

type CapabilityProfile struct {
	SchemaVersion   int      `json:"schemaVersion"`
	ClientName      string   `json:"clientName"`
	ClientVersion   string   `json:"clientVersion,omitempty"`
	Platform        string   `json:"platform"`
	PlatformVersion string   `json:"platformVersion,omitempty"`
	DeviceModel     string   `json:"deviceModel,omitempty"`
	Containers      []string `json:"supportedContainers"`
	VideoCodecs     []string `json:"supportedVideoCodecs"`
	AudioCodecs     []string `json:"supportedAudioCodecs"`
	SubtitleFormats []string `json:"subtitleFormats,omitempty"`
	MaxWidth        int      `json:"maximumVideoWidth,omitempty"`
	MaxHeight       int      `json:"maximumVideoHeight,omitempty"`
	HDR             []string `json:"hdrCapabilities,omitempty"`
	DirectPlay      bool     `json:"directPlaySupport"`
}

type Reason struct {
	Code  string `json:"code"`
	Value string `json:"value,omitempty"`
}
type Version struct {
	ID          string   `json:"id"`
	FileID      string   `json:"-"`
	Container   string   `json:"container"`
	VideoCodec  string   `json:"videoCodec"`
	AudioCodecs []string `json:"audioCodecs"`
	Width       int      `json:"width,omitempty"`
	Height      int      `json:"height,omitempty"`
	Bitrate     int64    `json:"bitrate,omitempty"`
	HDR         string   `json:"hdr,omitempty"`
	Resolution  string   `json:"resolution,omitempty"`
	Label       string   `json:"label,omitempty"`
	Available   bool     `json:"available"`
}
type Decision struct {
	Mode           Mode     `json:"mode"`
	MediaVersionID string   `json:"mediaVersionId,omitempty"`
	Container      string   `json:"container,omitempty"`
	VideoCodec     string   `json:"videoCodec,omitempty"`
	AudioCodecs    []string `json:"audioCodecs,omitempty"`
	Reasons        []Reason `json:"reasons"`
}
type Session struct {
	ID              string   `json:"id"`
	UserID          string   `json:"-"`
	LogicalType     string   `json:"logicalType"`
	LogicalID       string   `json:"logicalId"`
	MediaVersion    Version  `json:"selectedVersion"`
	Decision        Decision `json:"decision"`
	State           State    `json:"state"`
	Position        float64  `json:"position"`
	Duration        float64  `json:"duration"`
	ResumePosition  float64  `json:"resumePosition"`
	MediaURL        string   `json:"mediaUrl,omitempty"`
	StartedAt       string   `json:"startedAt"`
	LastActivityAt  string   `json:"lastActivityAt"`
	Title           string   `json:"title,omitempty"`
	UserDisplayName string   `json:"userDisplayName,omitempty"`
	ClientName      string   `json:"clientName,omitempty"`
	Platform        string   `json:"platform,omitempty"`
}
type StartRequest struct {
	LogicalType        string            `json:"logicalType"`
	LogicalID          string            `json:"logicalId"`
	RequestedVersionID string            `json:"requestedVersionId,omitempty"`
	Resume             bool              `json:"resume"`
	Capabilities       CapabilityProfile `json:"capabilities"`
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

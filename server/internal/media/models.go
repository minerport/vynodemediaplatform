package media

type LibraryType string

const (
	LibraryMovies LibraryType = "MOVIES"
	LibraryTV     LibraryType = "TV"
)

type Library struct {
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	Type              LibraryType `json:"type"`
	Enabled           bool        `json:"enabled"`
	CreatedAt         string      `json:"createdAt"`
	UpdatedAt         string      `json:"updatedAt"`
	Sources           []Source    `json:"sources,omitempty"`
	FileCount         int64       `json:"fileCount"`
	AvailableCount    int64       `json:"availableCount"`
	MissingCount      int64       `json:"missingCount"`
	ProbeFailureCount int64       `json:"probeFailureCount"`
}
type Source struct {
	ID                   string  `json:"id"`
	LibraryID            string  `json:"libraryId"`
	ConfiguredPath       string  `json:"configuredPath"`
	NormalizedPath       string  `json:"normalizedPath"`
	Enabled              bool    `json:"enabled"`
	CreatedAt            string  `json:"createdAt"`
	LastAttemptedScanAt  *string `json:"lastAttemptedScanAt,omitempty"`
	LastSuccessfulScanAt *string `json:"lastSuccessfulScanAt,omitempty"`
	LastScanStatus       *string `json:"lastScanStatus,omitempty"`
	LastScanError        *string `json:"lastScanError,omitempty"`
}
type Stream struct {
	ID              string `json:"id"`
	Index           int    `json:"index"`
	Type            string `json:"type"`
	Codec           string `json:"codec"`
	Profile         string `json:"profile"`
	Level           int    `json:"level,omitempty"`
	Width           int    `json:"width,omitempty"`
	Height          int    `json:"height,omitempty"`
	BitDepth        int    `json:"bitDepth,omitempty"`
	PixelFormat     string `json:"pixelFormat,omitempty"`
	FrameRate       string `json:"frameRate,omitempty"`
	ScanType        string `json:"scanType,omitempty"`
	Bitrate         int64  `json:"bitrate,omitempty"`
	Language        string `json:"language,omitempty"`
	Title           string `json:"title,omitempty"`
	Default         bool   `json:"default"`
	Forced          bool   `json:"forced"`
	Channels        int    `json:"channels,omitempty"`
	ChannelLayout   string `json:"channelLayout,omitempty"`
	SampleRate      int    `json:"sampleRate,omitempty"`
	ColorPrimaries  string `json:"colorPrimaries,omitempty"`
	ColorTransfer   string `json:"colorTransfer,omitempty"`
	ColorSpace      string `json:"colorSpace,omitempty"`
	ColorRange      string `json:"colorRange,omitempty"`
	HearingImpaired bool   `json:"hearingImpaired"`
	Commentary      bool   `json:"commentary"`
}
type File struct {
	ID              string   `json:"id"`
	SourceID        string   `json:"sourceId"`
	RelativePath    string   `json:"relativePath"`
	FileName        string   `json:"fileName"`
	BaseName        string   `json:"baseName"`
	Extension       string   `json:"extension"`
	ParentPath      string   `json:"parentPath"`
	SizeBytes       int64    `json:"sizeBytes"`
	ModifiedAtNS    int64    `json:"modifiedAtNs"`
	Availability    string   `json:"availability"`
	ProbeStatus     string   `json:"probeStatus"`
	ProbeError      string   `json:"probeError,omitempty"`
	ContainerFormat string   `json:"containerFormat,omitempty"`
	DurationSeconds float64  `json:"durationSeconds,omitempty"`
	Bitrate         int64    `json:"bitrate,omitempty"`
	ResolutionClass string   `json:"resolutionClass,omitempty"`
	HDRClass        string   `json:"hdrClass,omitempty"`
	CandidateTitle  string   `json:"candidateTitle,omitempty"`
	CandidateYear   int      `json:"candidateYear,omitempty"`
	SeasonNumber    int      `json:"seasonNumber,omitempty"`
	EpisodeStart    int      `json:"episodeStart,omitempty"`
	EpisodeEnd      int      `json:"episodeEnd,omitempty"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
	Streams         []Stream `json:"streams,omitempty"`
}
type Job struct {
	ID                  string `json:"id"`
	LibraryID           string `json:"libraryId"`
	State               string `json:"state"`
	CreatedAt           string `json:"createdAt"`
	StartedAt           string `json:"startedAt,omitempty"`
	CompletedAt         string `json:"completedAt,omitempty"`
	DirectoriesVisited  int64  `json:"directoriesVisited"`
	FilesDiscovered     int64  `json:"filesDiscovered"`
	CandidatesFound     int64  `json:"candidatesFound"`
	FilesProbed         int64  `json:"filesProbed"`
	FilesAdded          int64  `json:"filesAdded"`
	FilesUpdated        int64  `json:"filesUpdated"`
	FilesUnchanged      int64  `json:"filesUnchanged"`
	FilesMissing        int64  `json:"filesMissing"`
	FilesFailed         int64  `json:"filesFailed"`
	CurrentRelativePath string `json:"currentRelativePath,omitempty"`
	ErrorSummary        string `json:"errorSummary,omitempty"`
}
type ProbeResult struct {
	ContainerFormat string
	Duration        float64
	Bitrate         int64
	Streams         []Stream
}
type FilenameHints struct {
	CandidateTitle                                        string
	CandidateYear, SeasonNumber, EpisodeStart, EpisodeEnd int
}

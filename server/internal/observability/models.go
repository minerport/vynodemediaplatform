package observability

import "time"

var EventCatalog = map[string]EventDefinition{
	"TEST": {"TEST", "SERVER", "INFO"}, "SERVER_STARTED": {"SERVER_STARTED", "SERVER", "INFO"},
	"SOURCE_UNAVAILABLE": {"SOURCE_UNAVAILABLE", "LIBRARY", "ERROR"}, "SOURCE_RECOVERED": {"SOURCE_RECOVERED", "LIBRARY", "INFO"},
	"HEALTH_ERROR_OPENED": {"HEALTH_ERROR_OPENED", "HEALTH", "ERROR"}, "HEALTH_ERROR_RESOLVED": {"HEALTH_ERROR_RESOLVED", "HEALTH", "INFO"},
	"INVITATION_ACCEPTED": {"INVITATION_ACCEPTED", "SHARING", "INFO"}, "NEW_DEVICE_PAIRED": {"NEW_DEVICE_PAIRED", "SECURITY", "INFO"},
	"REMOTE_ENDPOINT_CHANGED":  {"REMOTE_ENDPOINT_CHANGED", "NETWORK", "INFO"},
	"LAN_DISCOVERY_STARTED":    {"LAN_DISCOVERY_STARTED", "NETWORK", "INFO"},
	"LAN_DISCOVERY_STOPPED":    {"LAN_DISCOVERY_STOPPED", "NETWORK", "INFO"},
	"LAN_DISCOVERY_FAILED":     {"LAN_DISCOVERY_FAILED", "NETWORK", "WARNING"},
	"PORT_MAPPING_ESTABLISHED": {"PORT_MAPPING_ESTABLISHED", "NETWORK", "INFO"},
	"PORT_MAPPING_FAILED":      {"PORT_MAPPING_FAILED", "NETWORK", "WARNING"},
	"DOWNLOAD_READY":           {"DOWNLOAD_READY", "DOWNLOAD", "INFO"},
	"DOWNLOAD_FAILED":          {"DOWNLOAD_FAILED", "DOWNLOAD", "WARNING"},
	"DOWNLOAD_CACHE_LOW_SPACE": {"DOWNLOAD_CACHE_LOW_SPACE", "DOWNLOAD", "WARNING"},
}

type EventDefinition struct{ Type, Category, Severity string }
type Event struct {
	ID, Type, Category, Severity, CreatedAt string
	Payload                                 map[string]any
}
type HealthIssue struct {
	ID, Category, Severity, ReferenceType, ReferenceID, Description, Status string
	FirstDetectedAt, LastDetectedAt, ResolvedAt, IgnoredAt                  string
}
type Destination struct {
	ID, Name, URL                                   string
	Enabled, AllowPrivateNetwork, AllowInsecureHTTP bool
	HasSecret                                       bool
	MaxAttempts                                     int
	EventTypes                                      []string
	CreatedAt, UpdatedAt                            string
	Secret                                          string `json:"-"`
}
type Delivery struct {
	ID, EventID, EventType, DestinationID, DestinationName, Status string
	AttemptCount, LastHTTPStatus                                   int
	LastError, NextAttemptAt, DeliveredAt, CreatedAt               string
}
type DiskMetric struct {
	Label, Path                           string
	TotalBytes, UsedBytes, AvailableBytes uint64
}
type Metrics struct {
	UptimeSeconds                                      int64
	OperatingSystem, Architecture                      string
	GoRoutines                                         int
	GoHeapBytes, GoSystemBytes                         uint64
	ProcessRSSBytes                                    uint64
	SystemMemoryTotalBytes, SystemMemoryAvailableBytes uint64
	Disks                                              []DiskMetric
	ActivePlaybackSessions, ActiveFFmpegProcesses      int
}
type Analytics struct {
	From, To                                                                               string
	TotalPlays, UniqueUsers, MoviesPlayed, EpisodesPlayed, PlaybackErrors, CompletionCount int
	PlaybackSeconds                                                                        float64
	Modes                                                                                  map[string]int
	TopMedia, TopUsers, Devices                                                            []Breakdown
}
type Breakdown struct {
	Key, Label string
	Count      int
}
type LibraryStats struct {
	Movies, Shows, Episodes, PhysicalFiles, AvailableFiles, MissingFiles, UnmatchedFiles, OptimizedVersions int
	Resolution, VideoCodecs, HDR                                                                            map[string]int
}
type Job struct {
	ID, Type, Target, State, CreatedAt, StartedAt, CompletedAt, Error string
	Progress                                                          float64
	Priority                                                          int
}
type Dashboard struct {
	Version, Commit, ServerName, InstanceID, DatabaseType, FFmpegVersion, FFprobeVersion string
	FFmpegPath, FFmpegSource, FFprobePath, FFprobeSource                                 string
	Metrics                                                                              Metrics
	Health                                                                               map[string]int
	Libraries                                                                            LibraryStats
	FailedJobs                                                                           int
	RecentEvents                                                                         []Event
}
type Paths struct{ Config, Transcode, Optimized, Downloads string }
type SystemInfo struct {
	Version, Commit, ServerName, InstanceID, DatabaseType, OS, Architecture, FFmpeg, FFprobe string
	FFmpegSource, FFprobeSource                                                              string
	StartedAt                                                                                time.Time
}

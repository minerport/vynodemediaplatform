package offline

type QualityProfile struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	VideoCodec    string `json:"videoCodec"`
	AudioCodec    string `json:"audioCodec"`
	Container     string `json:"container"`
	MaxWidth      int    `json:"maxWidth"`
	MaxHeight     int    `json:"maxHeight"`
	AudioChannels int    `json:"audioChannels"`
	VideoBitrate  int64  `json:"videoBitrate"`
	AudioBitrate  int64  `json:"audioBitrate"`
	Version       int    `json:"version"`
}

type Plan struct {
	Mode               string `json:"mode"`
	Reason             string `json:"reason"`
	LogicalType        string `json:"logicalType"`
	LogicalID          string `json:"logicalId"`
	SourceMediaFileID  string `json:"sourceMediaFileId"`
	ProfileID          string `json:"profileId"`
	ProfileVersion     int    `json:"profileVersion"`
	SourceContainer    string `json:"sourceContainer"`
	SourceVideoCodec   string `json:"sourceVideoCodec"`
	SourceAudioCodec   string `json:"sourceAudioCodec"`
	SourceWidth        int    `json:"sourceWidth"`
	SourceHeight       int    `json:"sourceHeight"`
	OutputContainer    string `json:"outputContainer"`
	OutputVideoCodec   string `json:"outputVideoCodec"`
	OutputAudioCodec   string `json:"outputAudioCodec"`
	OutputWidth        int    `json:"outputWidth"`
	OutputHeight       int    `json:"outputHeight"`
	OutputVideoBitrate int64  `json:"outputVideoBitrate"`
	OutputAudioBitrate int64  `json:"outputAudioBitrate"`
}

type Download struct {
	ID             string  `json:"id"`
	UserID         string  `json:"userId"`
	DeviceID       string  `json:"deviceId"`
	LogicalType    string  `json:"logicalType"`
	LogicalID      string  `json:"logicalId"`
	AssetID        string  `json:"assetId"`
	ProfileID      string  `json:"profileId"`
	Status         string  `json:"status"`
	Mode           string  `json:"mode"`
	AssetState     string  `json:"assetState"`
	ChecksumSHA256 string  `json:"checksumSha256,omitempty"`
	ContentType    string  `json:"contentType"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
	SizeBytes      int64   `json:"sizeBytes"`
	TransferBytes  int64   `json:"transferBytes"`
	Progress       float64 `json:"progress"`
	Plan           Plan    `json:"plan"`
}

type Manifest struct {
	SchemaVersion        int        `json:"schemaVersion"`
	Download             Download   `json:"download"`
	Title                string     `json:"title"`
	Year                 int        `json:"year,omitempty"`
	Overview             string     `json:"overview,omitempty"`
	Duration             float64    `json:"durationSeconds"`
	SuggestedName        string     `json:"suggestedFilename"`
	FileURL              string     `json:"fileUrl"`
	ArtworkURLs          []string   `json:"artworkUrls"`
	Artwork              []Artwork  `json:"artwork"`
	PresentationRevision string     `json:"presentationRevision"`
	Subtitles            []Subtitle `json:"subtitles"`
	Watched              bool       `json:"watched"`
	Position             float64    `json:"positionSeconds"`
}
type Artwork struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	ETag     string `json:"etag,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}
type Subtitle struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Format   string `json:"format"`
	URL      string `json:"url"`
}
type StorageReport struct {
	TotalBytes       int64 `json:"totalBytes"`
	AvailableBytes   int64 `json:"availableBytes"`
	VyNodeBytes      int64 `json:"vyNodeBytes"`
	MinimumFreeBytes int64 `json:"minimumFreeBytes"`
}
type InventoryItem struct {
	DownloadID     string `json:"downloadId"`
	AssetID        string `json:"assetId"`
	State          string `json:"state"`
	ChecksumSHA256 string `json:"checksumSha256,omitempty"`
	DownloadedAt   string `json:"downloadedAt,omitempty"`
	LastVerifiedAt string `json:"lastVerifiedAt,omitempty"`
	SizeBytes      int64  `json:"sizeBytes"`
}
type ProgressEvent struct {
	EventID        string  `json:"eventId"`
	SequenceEpoch  string  `json:"sequenceEpoch"`
	LogicalType    string  `json:"logicalType"`
	LogicalID      string  `json:"logicalId"`
	OccurredAt     string  `json:"occurredAt"`
	ExplicitAction string  `json:"explicitAction,omitempty"`
	DeviceSequence int64   `json:"deviceSequence"`
	Position       float64 `json:"positionSeconds"`
	Duration       float64 `json:"durationSeconds"`
	Watched        bool    `json:"watched"`
}
type Push struct {
	Progress  []ProgressEvent `json:"progressEvents"`
	Inventory []InventoryItem `json:"inventory"`
	Storage   *StorageReport  `json:"storage,omitempty"`
}
type Change struct {
	Sequence   int64          `json:"sequence"`
	Type       string         `json:"type"`
	EntityType string         `json:"entityType"`
	EntityID   string         `json:"entityId"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  string         `json:"createdAt"`
}
type SyncState struct {
	Cursor             int64          `json:"cursor"`
	LastDeviceSequence int64          `json:"lastDeviceSequence"`
	FullResyncRequired bool           `json:"fullResyncRequired"`
	Downloads          []Download     `json:"downloads,omitempty"`
	Subscriptions      []Subscription `json:"subscriptions,omitempty"`
	Changes            []Change       `json:"changes"`
	HasMore            bool           `json:"hasMore"`
}
type Subscription struct {
	ID            string `json:"id"`
	UserID        string `json:"userId"`
	DeviceID      string `json:"deviceId"`
	ShowID        string `json:"showId"`
	ProfileID     string `json:"profileId"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
	Enabled       bool   `json:"enabled"`
	RemoveWatched bool   `json:"removeWatched"`
	WiFiOnly      bool   `json:"wifiOnly"`
	DesiredCount  int    `json:"desiredCount"`
}
type CreateRequest struct {
	LogicalType string `json:"logicalType"`
	LogicalID   string `json:"logicalId"`
	ProfileID   string `json:"profileId"`
}
type SubscriptionRequest struct {
	ShowID        string `json:"showId"`
	ProfileID     string `json:"profileId"`
	DesiredCount  int    `json:"desiredCount"`
	Enabled       bool   `json:"enabled"`
	RemoveWatched bool   `json:"removeWatched"`
	WiFiOnly      bool   `json:"wifiOnly"`
}
type Settings struct {
	CacheQuotaBytes int64 `json:"cacheQuotaBytes"`
	CacheBytes      int64 `json:"cacheBytes"`
	ReadyAssets     int   `json:"readyAssets"`
	PreparingAssets int   `json:"preparingAssets"`
}

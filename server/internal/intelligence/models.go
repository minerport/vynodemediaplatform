package intelligence

type Confidence string

const (
	ConfidenceHigh   Confidence = "HIGH"
	ConfidenceMedium Confidence = "MEDIUM"
	ConfidenceLow    Confidence = "LOW"
)

func Classify(score float64) Confidence {
	if score >= .82 {
		return ConfidenceHigh
	}
	if score >= .60 {
		return ConfidenceMedium
	}
	return ConfidenceLow
}

type MarkerCandidate struct {
	ID, LogicalType, LogicalID, Type, Source, ReviewState, SourceIdentity, CreatedAt, UpdatedAt string
	Start, End, Confidence                                                                      float64
	ConfidenceClass                                                                             Confidence
}

type Job struct {
	ID, Type, TargetType, TargetID, State, Error, CreatedAt string
	Progress                                                float64
}

type OptimizationProfile struct {
	ID, Label                  string
	Width, Height              int
	VideoBitrate, AudioBitrate int64
}
type OptimizedMedia struct {
	ID, SourceMediaFileID, DerivedMediaFileID, LogicalType, LogicalID, Profile, Status, CreatedAt string
	SizeBytes                                                                                     int64
}

var Profiles = map[string]OptimizationProfile{
	"mobile-480p":     {"mobile-480p", "Mobile 480p", 854, 480, 1_200_000, 128_000},
	"mobile-720p":     {"mobile-720p", "Mobile 720p", 1280, 720, 2_500_000, 128_000},
	"remote-1080p":    {"remote-1080p", "Remote 1080p", 1920, 1080, 6_000_000, 192_000},
	"compatible-h264": {"compatible-h264", "Compatible H.264", 0, 0, 8_000_000, 192_000},
}

type Condition struct {
	Field, Operator string
	Value           any
}
type Action struct{ Type, Profile string }
type Schedule struct {
	Hour   int `json:"hour"`
	Minute int `json:"minute"`
}
type Rule struct {
	ID, Name, Trigger, Timezone, LastExecutionAt string
	Enabled                                      bool
	Conditions                                   []Condition
	Actions                                      []Action
	Schedule                                     *Schedule
}
type DryRun struct {
	Matches []string `json:"matches"`
	Actions int      `json:"actionsExecuted"`
}

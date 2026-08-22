package metadata

type Candidate struct {
	ProviderID string  `json:"providerId"`
	Title      string  `json:"title"`
	Year       int     `json:"year,omitempty"`
	Overview   string  `json:"overview,omitempty"`
	PosterPath string  `json:"posterPath,omitempty"`
	Popularity float64 `json:"popularity,omitempty"`
}

type Match struct {
	State      string      `json:"state"`
	Confidence string      `json:"confidence"`
	Score      int         `json:"score"`
	Candidate  *Candidate  `json:"candidate,omitempty"`
	Candidates []Candidate `json:"candidates"`
	Signals    []string    `json:"signals"`
}

type Movie struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	SortTitle        string    `json:"sortTitle"`
	Overview         string    `json:"overview"`
	PrimaryProvider  string    `json:"primaryProvider"`
	ProviderID       string    `json:"providerId"`
	MetadataState    string    `json:"metadataState"`
	CreatedAt        string    `json:"createdAt"`
	UpdatedAt        string    `json:"updatedAt"`
	OriginalTitle    string    `json:"originalTitle,omitempty"`
	ReleaseDate      string    `json:"releaseDate,omitempty"`
	Tagline          string    `json:"tagline,omitempty"`
	ContentRating    string    `json:"contentRating,omitempty"`
	Status           string    `json:"status,omitempty"`
	OriginalLanguage string    `json:"originalLanguage,omitempty"`
	Year             int       `json:"year,omitempty"`
	RuntimeMinutes   int       `json:"runtimeMinutes,omitempty"`
	Rating           float64   `json:"rating,omitempty"`
	VoteCount        int       `json:"voteCount,omitempty"`
	Genres           []string  `json:"genres"`
	Versions         []Version `json:"versions,omitempty"`
	Credits          []Credit  `json:"credits,omitempty"`
	Companies        []string  `json:"companies,omitempty"`
}

type Show struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	SortTitle        string   `json:"sortTitle"`
	Overview         string   `json:"overview"`
	PrimaryProvider  string   `json:"primaryProvider"`
	ProviderID       string   `json:"providerId"`
	MetadataState    string   `json:"metadataState"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
	OriginalTitle    string   `json:"originalTitle,omitempty"`
	FirstAirDate     string   `json:"firstAirDate,omitempty"`
	Status           string   `json:"status,omitempty"`
	OriginalLanguage string   `json:"originalLanguage,omitempty"`
	Year             int      `json:"year,omitempty"`
	Rating           float64  `json:"rating,omitempty"`
	VoteCount        int      `json:"voteCount,omitempty"`
	Genres           []string `json:"genres"`
	Seasons          []Season `json:"seasons,omitempty"`
	Credits          []Credit `json:"credits,omitempty"`
	Companies        []string `json:"companies,omitempty"`
}

type Season struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Overview     string    `json:"overview"`
	AirDate      string    `json:"airDate"`
	SeasonNumber int       `json:"seasonNumber"`
	Episodes     []Episode `json:"episodes"`
}
type Episode struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Overview       string `json:"overview"`
	AirDate        string `json:"airDate"`
	EpisodeNumber  int    `json:"episodeNumber"`
	RuntimeMinutes int    `json:"runtimeMinutes"`
	Available      bool   `json:"available"`
}
type Version struct {
	ID         string `json:"id"`
	FileID     string `json:"fileId"`
	Label      string `json:"label"`
	Resolution string `json:"resolution"`
	Codec      string `json:"codec"`
	HDR        string `json:"hdr"`
}

type MovieDetails struct {
	Candidate
	OriginalTitle, ReleaseDate, Runtime, Overview, Tagline, Status, OriginalLanguage, ContentRating string
	RuntimeMinutes, VoteCount                                                                       int
	Rating                                                                                          float64
	Genres                                                                                          []ProviderGenre
	ExternalIDs                                                                                     map[string]string
	Artwork                                                                                         []ProviderArtwork
	Credits                                                                                         []ProviderCredit
	Companies                                                                                       []ProviderCompany
}

type ShowDetails struct {
	Candidate
	OriginalTitle, FirstAirDate, Status, Overview, OriginalLanguage string
	VoteCount                                                       int
	Rating                                                          float64
	Genres                                                          []ProviderGenre
	ExternalIDs                                                     map[string]string
	Artwork                                                         []ProviderArtwork
	Credits                                                         []ProviderCredit
	Companies                                                       []ProviderCompany
}

type SeasonDetails struct {
	SeasonNumber                         int
	Title, Overview, AirDate, ProviderID string
	Episodes                             []EpisodeDetails
	Artwork                              []ProviderArtwork
}
type EpisodeDetails struct {
	EpisodeNumber, RuntimeMinutes        int
	Title, Overview, AirDate, ProviderID string
	Artwork                              []ProviderArtwork
}
type ProviderGenre struct{ ID, Name string }
type ProviderPerson struct{ ID, Name string }
type ProviderCredit struct {
	Person          ProviderPerson
	Type, Character string
	Order           int
}
type ProviderCompany struct{ ID, Name string }
type Credit struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Character string `json:"character,omitempty"`
	Order     int    `json:"order,omitempty"`
}
type ProviderArtwork struct {
	Type, Path, Language string
	Width, Height        int
	VoteAverage          float64
}

type Artwork struct {
	ID              string `json:"id"`
	EntityType      string `json:"entityType"`
	EntityID        string `json:"entityId"`
	Type            string `json:"type"`
	Provider        string `json:"provider"`
	Language        string `json:"language,omitempty"`
	MimeType        string `json:"mimeType,omitempty"`
	Width           int    `json:"width,omitempty"`
	Height          int    `json:"height,omitempty"`
	Selected        bool   `json:"selected"`
	ManualSelection bool   `json:"manualSelection"`
	Cached          bool   `json:"cached"`
}

type ProviderStatus struct {
	Enabled    bool   `json:"enabled"`
	Configured bool   `json:"configured"`
	Language   string `json:"language"`
	Region     string `json:"region"`
	Status     string `json:"status"`
}

package curation

import "errors"

var ErrValidation = errors.New("validation failed")
var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")

type Item struct {
	Type, ID, Title, Subtitle, ArtworkID, Availability string
	Year                                               int
	Rating                                             float64
	Position                                           int
}

type Collection struct {
	ID, Name, Description, SortTitle, Scope, OwnerUserID, Ordering, CreatedAt, UpdatedAt string
	ArtworkItemType, ArtworkItemID                                                       string
	Items                                                                                []Item
}

type RuleNode struct {
	Logic    string     `json:"logic,omitempty"`
	Children []RuleNode `json:"children,omitempty"`
	Field    string     `json:"field,omitempty"`
	Operator string     `json:"operator,omitempty"`
	Value    any        `json:"value,omitempty"`
}

type SmartCollection struct {
	ID, Name, Description, Scope, OwnerUserID, SortField, SortDirection, CreatedAt, UpdatedAt string
	RuleSchemaVersion                                                                         int
	Rule                                                                                      RuleNode
	Limit                                                                                     int
	Items                                                                                     []Item
	ArtworkItemType, ArtworkItemID                                                            string
}

type Playlist struct {
	ID, OwnerUserID, Name, Description, CreatedAt, UpdatedAt string
	Items                                                    []Item
}

type HomeRow struct {
	ID, Type, Title, SourceID string
	Enabled                   bool
	Position, Limit           int
	Items                     []Item
	SeeAll                    string
}

type Home struct {
	Rows []HomeRow `json:"rows"`
}

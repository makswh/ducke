package ddl

import (
	"context"
)

// Known hoster names
const (
	HosterPixelDrain = "PixelDrain"
	HosterGoFile     = "GoFile"
	HosterFileditch  = "Fileditch"
	Hoster1fichier   = "1fichier"
	HosterDirect     = "Direct"
)

// ResolvedItem represents a resolved direct download link
type ResolvedItem struct {
	DirectURL   string            `json:"directUrl"`
	Filename    string            `json:"filename"`
	SizeBytes   int64             `json:"sizeBytes"`
	Headers     map[string]string `json:"headers,omitempty"`
	Hoster      string            `json:"hoster"`
	OriginalURL string            `json:"originalUrl"`
}

// HosterResolver defines the interface for link extractors
type HosterResolver interface {
	Name() string
	CanHandle(rawURL string) bool
	Resolve(ctx context.Context, rawURL string) (*ResolvedItem, error)
}

// PartPlan contains mirrors for a specific archive part/chunk
type PartPlan struct {
	PartIndex   int             `json:"partIndex"` // 1-indexed: 1, 2, ...
	TotalParts  int             `json:"totalParts"`
	Mirrors     []*ResolvedItem `json:"mirrors"` // all hosters offering this part
	SelectedURL *ResolvedItem   `json:"selectedUrl,omitempty"`
}

// GameDownloadPlan encapsulates all resolved parts and mirrors for a game
type GameDownloadPlan struct {
	GameTitle       string     `json:"gameTitle"`
	Parts           []PartPlan `json:"parts"`
	TotalBytes      int64      `json:"totalBytes"`
	AvailableHosts  []string   `json:"availableHosts"`
	SelectedHoster  string     `json:"selectedHoster"`
}

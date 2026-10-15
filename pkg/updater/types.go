package updater

import "time"

// UpdateInfo contains metadata about available update
type UpdateInfo struct {
	CurrentVersion string    `json:"currentVersion"`
	LatestVersion  string    `json:"latestVersion"`
	HasUpdate      bool      `json:"hasUpdate"`
	ReleaseNotes   string    `json:"releaseNotes"`
	ReleaseName    string    `json:"releaseName"`
	ReleaseDate    time.Time `json:"releaseDate"`
	ReleaseURL     string    `json:"releaseUrl"`
	DownloadURL    string    `json:"downloadUrl"`
	AssetName      string    `json:"assetName"`
	AssetSize      int64     `json:"assetSize"`
}

// UpdateProgress models streaming progress during asset download
type UpdateProgress struct {
	DownloadedBytes  int64   `json:"downloadedBytes"`
	TotalBytes       int64   `json:"totalBytes"`
	Percent          float64 `json:"percent"`
	SpeedBytesPerSec int64   `json:"speedBytesPerSec"`
	Done             bool    `json:"done"`
	Error            string  `json:"error,omitempty"`
}

// UpdateStatus represents the state machine of the updater
type UpdateStatus struct {
	State        string      `json:"state"` // "idle", "checking", "available", "up_to_date", "downloading", "ready_to_install", "error"
	ErrorMessage string      `json:"errorMessage,omitempty"`
	UpdateInfo   *UpdateInfo `json:"updateInfo,omitempty"`
}

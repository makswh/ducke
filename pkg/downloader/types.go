package downloader

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// DownloadStatus represents current task state
type DownloadStatus string

const (
	StatusQueued      DownloadStatus = "queued"
	StatusScanning    DownloadStatus = "scanning"
	StatusDownloading DownloadStatus = "downloading"
	StatusPaused      DownloadStatus = "paused"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
	StatusCancelled   DownloadStatus = "cancelled"
)

// DownloadProgressEvent is payload emitted to frontend
type DownloadProgressEvent struct {
	DownloadID       string         `json:"downloadId"`
	GameID           int64          `json:"gameId"`
	GameTitle        string         `json:"gameTitle"`
	CoverImage       string         `json:"coverImage,omitempty"`
	DownloadedBytes  int64          `json:"downloadedBytes"`
	TotalBytes       int64          `json:"totalBytes"`
	ProgressPercent  float64        `json:"progressPercent"`
	SpeedBytesPerSec int64          `json:"speedBytesPerSec"`
	SpeedDisplay     string         `json:"speedDisplay"`
	ETASSeconds      int64          `json:"etaSeconds"`
	ETADisplay       string         `json:"etaDisplay"`
	Status           DownloadStatus `json:"status"`
	CurrentFile      string         `json:"currentFile"`
	FileIndex        int            `json:"fileIndex"`
	TotalFiles       int            `json:"totalFiles"`
	LocalPath        string         `json:"localPath"`
	ErrorMessage     string         `json:"errorMessage,omitempty"`
}

type DownloadTask struct {
	ID              string
	GameID          int64
	GameTitle       string
	CoverImage      string
	RemotePath      string
	LocalPath       string
	IsDirectory     bool
	TotalBytes      int64
	DownloadedBytes atomic.Int64
	Status          DownloadStatus
	ErrorMessage    string

	// Runtime metrics
	speedBytesPerSec atomic.Int64
	smoothedSpeed    float64
	lastBytes        int64
	lastSampleTime   time.Time
	currentFile      string
	fileIndex        atomic.Int32
	totalFiles       int

	cancelCtx  context.Context
	cancelFunc context.CancelFunc
	limiter    *RateLimiter
	mu         sync.RWMutex
}

// SetSpeedLimit dynamically updates speed limit for this task
func (t *DownloadTask) SetSpeedLimit(kbps int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.limiter != nil {
		t.limiter.SetLimit(kbps)
	} else if kbps > 0 {
		t.limiter = NewRateLimiter(kbps)
	}
}

// ToEvent generates a point-in-time snapshot event without holding the lock afterwards.
func (t *DownloadTask) ToEvent() DownloadProgressEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	currentDownloaded := t.DownloadedBytes.Load()
	var pct float64 = 0
	if t.TotalBytes > 0 {
		pct = (float64(currentDownloaded) / float64(t.TotalBytes)) * 100
		if pct > 100 {
			pct = 100
		}
	}
	if t.Status == StatusCompleted {
		pct = 100.0
		if t.TotalBytes > 0 {
			currentDownloaded = t.TotalBytes
		}
	}

	speed := t.speedBytesPerSec.Load()
	var etaSec int64 = 0
	if speed > 0 && t.TotalBytes > currentDownloaded {
		etaSec = (t.TotalBytes - currentDownloaded) / speed
	}

	return DownloadProgressEvent{
		DownloadID:       t.ID,
		GameID:           t.GameID,
		GameTitle:        t.GameTitle,
		CoverImage:       t.CoverImage,
		DownloadedBytes:  currentDownloaded,
		TotalBytes:       t.TotalBytes,
		ProgressPercent:  pct,
		SpeedBytesPerSec: speed,
		SpeedDisplay:     FormatSpeed(speed),
		ETASSeconds:      etaSec,
		ETADisplay:       FormatETA(etaSec),
		Status:           t.Status,
		CurrentFile:      t.currentFile,
		FileIndex:        int(t.fileIndex.Load()),
		TotalFiles:       t.totalFiles,
		LocalPath:        t.LocalPath,
		ErrorMessage:     t.ErrorMessage,
	}
}

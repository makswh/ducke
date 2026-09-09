package downloader

import (
	"fmt"
	"time"
)

// UpdateTaskMetrics updates the throughput and smoothed speed for an active task
func UpdateTaskMetrics(task *DownloadTask, now time.Time) {
	task.mu.Lock()
	defer task.mu.Unlock()

	if task.Status != StatusDownloading && task.Status != StatusScanning {
		return
	}

	currentDownloaded := task.DownloadedBytes.Load()
	delta := currentDownloaded - task.lastBytes
	if delta < 0 {
		delta = 0
	}

	// Exponential Moving Average (EMA) for butter-smooth speed & stable ETA
	instantSpeed := float64(delta)
	if task.smoothedSpeed <= 0 {
		task.smoothedSpeed = instantSpeed
	} else {
		task.smoothedSpeed = (0.25 * instantSpeed) + (0.75 * task.smoothedSpeed)
	}
	currentSpeed := int64(task.smoothedSpeed)
	task.speedBytesPerSec.Store(currentSpeed)

	task.lastBytes = currentDownloaded
	task.lastSampleTime = now
}

// FormatSpeed formats bytes per second into human readable string
func FormatSpeed(bytesPerSec int64) string {
	if bytesPerSec <= 0 {
		return "0 KB/s"
	}
	mb := float64(bytesPerSec) / (1024 * 1024)
	if mb >= 1.0 {
		return fmt.Sprintf("%.2f MB/s", mb)
	}
	kb := float64(bytesPerSec) / 1024
	return fmt.Sprintf("%.1f KB/s", kb)
}

// FormatETA formats seconds into human readable duration
func FormatETA(seconds int64) string {
	if seconds <= 0 {
		return "--"
	}
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm %ds", seconds/60, seconds%60)
	}
	hours := seconds / 3600
	mins := (seconds % 3600) / 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}

// FormatBytes formats byte count into a clean human readable string
func FormatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
		tb = 1024 * gb
	)
	switch {
	case bytes >= tb:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(tb))
	case bytes >= gb:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}


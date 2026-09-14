package downloader

import (
	"fmt"
	"time"
)

// UpdateTaskMetrics updates the throughput and smoothed speed for an active task.
// If overrideDelta >= 0 is provided, it is used directly as the byte delta for this sample interval.
func UpdateTaskMetrics(task *DownloadTask, now time.Time, overrideDelta ...int64) {
	task.mu.Lock()
	defer task.mu.Unlock()

	if task.Status != StatusDownloading && task.Status != StatusScanning {
		return
	}

	var delta int64
	if len(overrideDelta) > 0 && overrideDelta[0] >= 0 {
		delta = overrideDelta[0]
	} else {
		currentDownloaded := task.DownloadedBytes.Load()
		delta = currentDownloaded - task.lastBytes
		if delta < 0 {
			delta = 0
		}
		task.lastBytes = currentDownloaded
	}

	// Exponential Moving Average (EMA) for responsive and butter-smooth speed & stable ETA
	instantSpeed := float64(delta)
	if task.smoothedSpeed <= 0 {
		task.smoothedSpeed = instantSpeed
	} else {
		task.smoothedSpeed = (0.35 * instantSpeed) + (0.65 * task.smoothedSpeed)
	}
	currentSpeed := int64(task.smoothedSpeed)
	task.speedBytesPerSec.Store(currentSpeed)

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


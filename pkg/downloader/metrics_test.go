package downloader

import (
	"testing"
	"time"
)

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		bytesPerSec int64
		expected    string
	}{
		{0, "0 KB/s"},
		{-50, "0 KB/s"},
		{512, "0.5 KB/s"},
		{1024, "1.0 KB/s"},
		{1024 * 1024, "1.00 MB/s"},
		{15 * 1024 * 1024, "15.00 MB/s"},
		{100 * 1024 * 1024, "100.00 MB/s"},
	}

	for _, tc := range tests {
		got := FormatSpeed(tc.bytesPerSec)
		if got != tc.expected {
			t.Errorf("FormatSpeed(%d) = %q, want %q", tc.bytesPerSec, got, tc.expected)
		}
	}
}

func TestFormatETA(t *testing.T) {
	tests := []struct {
		seconds  int64
		expected string
	}{
		{0, "--"},
		{-10, "--"},
		{45, "45s"},
		{60, "1m 0s"},
		{125, "2m 5s"},
		{3600, "1h 0m"},
		{3665, "1h 1m"},
		{7320, "2h 2m"},
	}

	for _, tc := range tests {
		got := FormatETA(tc.seconds)
		if got != tc.expected {
			t.Errorf("FormatETA(%d) = %q, want %q", tc.seconds, got, tc.expected)
		}
	}
}

func TestUpdateTaskMetrics(t *testing.T) {
	task := &DownloadTask{
		Status:         StatusDownloading,
		TotalBytes:     10000000,
		lastBytes:      0,
		lastSampleTime: time.Now().Add(-1 * time.Second),
	}

	// 1 MB downloaded in sample interval
	task.DownloadedBytes.Store(1024 * 1024)
	UpdateTaskMetrics(task, time.Now())

	speed := task.speedBytesPerSec.Load()
	if speed <= 0 {
		t.Fatalf("expected speed > 0, got %d", speed)
	}

	event := task.ToEvent()
	if event.ProgressPercent <= 0 || event.ProgressPercent > 100 {
		t.Fatalf("expected progress between 0 and 100, got %f", event.ProgressPercent)
	}
}

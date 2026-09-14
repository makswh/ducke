package downloader

import (
	"testing"
	"time"
)

func TestTorrentEngine_LifecycleAndTrackers(t *testing.T) {
	tempDir := t.TempDir()

	engine, err := NewTorrentEngine(tempDir, 0)
	if err != nil {
		t.Fatalf("failed to create torrent engine: %v", err)
	}
	defer engine.Close()

	// Verify public tier 1 trackers are populated
	if len(PublicTier1Trackers) == 0 {
		t.Fatal("expected PublicTier1Trackers to have entries")
	}

	// Test adding a magnet URI (Ubuntu mini ISO magnet for valid hash parsing)
	testMagnet := "magnet:?xt=urn:btih:3b245504cf5f11bbdbe1201cea6a6bf45a0b7758&dn=Ubuntu"
	torrentHandle, err := engine.AddMagnet("test-dl-1", testMagnet, tempDir)
	if err != nil {
		t.Fatalf("failed to add magnet: %v", err)
	}
	if torrentHandle == nil {
		t.Fatal("expected non-nil torrent handle")
	}

	// Verify handle retrieval
	gotHandle, ok := engine.GetTorrent("test-dl-1")
	if !ok || gotHandle != torrentHandle {
		t.Fatal("expected to retrieve the same torrent handle by ID")
	}

	// Test speed limit adjustments and burst scaling
	engine.SetSpeedLimit(5000) // 5 MB/s
	if engine.rateLimiter.Limit() != 5000*1024 {
		t.Errorf("expected limit to be 5000 KB/s, got %v", engine.rateLimiter.Limit())
	}
	// Burst should be at least 4MB or 5000*1024*2 (approx 10MB)
	expectedBurst := 5000 * 1024 * 2
	if engine.rateLimiter.Burst() != expectedBurst {
		t.Errorf("expected burst %d, got %d", expectedBurst, engine.rateLimiter.Burst())
	}

	engine.SetSpeedLimit(0) // Unlimited
	if engine.rateLimiter.Burst() != 16*1024*1024 {
		t.Errorf("expected unlimited burst 16MB, got %d", engine.rateLimiter.Burst())
	}

	// Cleanup
	engine.DropTorrent("test-dl-1")
	if _, ok := engine.GetTorrent("test-dl-1"); ok {
		t.Fatal("expected torrent to be dropped from engine map")
	}
}

func TestUpdateTaskMetrics_WithWireDelta(t *testing.T) {
	task := &DownloadTask{
		ID:        "torrent-test-metrics",
		IsTorrent: true,
		Status:    StatusDownloading,
	}

	now := time.Now()

	// 1. Initial tick with 0 delta (first sample)
	UpdateTaskMetrics(task, now, 0)
	if task.speedBytesPerSec.Load() != 0 {
		t.Fatalf("expected 0 initial speed, got %d", task.speedBytesPerSec.Load())
	}

	// 2. Second tick with 5 MB wire delta
	const fiveMB = 5 * 1024 * 1024
	now = now.Add(1 * time.Second)
	UpdateTaskMetrics(task, now, fiveMB)

	// Since previous speed was 0, first non-zero instant speed initializes smoothedSpeed
	if task.speedBytesPerSec.Load() != fiveMB {
		t.Fatalf("expected initial smoothed speed %d, got %d", fiveMB, task.speedBytesPerSec.Load())
	}

	// 3. Third tick with another 5 MB wire delta
	now = now.Add(1 * time.Second)
	UpdateTaskMetrics(task, now, fiveMB)
	if task.speedBytesPerSec.Load() != fiveMB {
		t.Fatalf("expected consistent smoothed speed %d, got %d", fiveMB, task.speedBytesPerSec.Load())
	}

	// 4. Fourth tick with a piece verification gap (0 bytes completed, but continuous wire transfer of 4 MB)
	const fourMB = 4 * 1024 * 1024
	now = now.Add(1 * time.Second)
	UpdateTaskMetrics(task, now, fourMB)

	// EMA: (0.35 * 4MB) + (0.65 * 5MB) = 1.4MB + 3.25MB = 4.65MB
	calculatedFloat := 0.35*float64(fourMB) + 0.65*float64(fiveMB)
	expectedSpeed := int64(calculatedFloat)
	if task.speedBytesPerSec.Load() != expectedSpeed {
		t.Fatalf("expected EMA smoothed speed %d, got %d", expectedSpeed, task.speedBytesPerSec.Load())
	}
}

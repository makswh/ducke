package downloader

import (
	"sync/atomic"
	"testing"

	"gamevault/pkg/config"
	"gamevault/pkg/database"
)

func TestDownloadTask_ToEvent(t *testing.T) {
	task := &DownloadTask{
		ID:         "test-id-123",
		GameID:     42,
		GameTitle:  "Half-Life 2",
		TotalBytes: 5000000,
		Status:     StatusDownloading,
	}
	task.DownloadedBytes.Store(2500000)
	task.speedBytesPerSec.Store(1024 * 1024)

	event := task.ToEvent()
	if event.DownloadID != "test-id-123" {
		t.Fatalf("expected ID test-id-123, got %s", event.DownloadID)
	}
	if event.GameTitle != "Half-Life 2" {
		t.Fatalf("expected title Half-Life 2, got %s", event.GameTitle)
	}
	if event.ProgressPercent < 49.9 || event.ProgressPercent > 50.1 {
		t.Fatalf("expected 50%% progress, got %f", event.ProgressPercent)
	}
	if event.SpeedDisplay != "1.00 MB/s" {
		t.Fatalf("expected 1.00 MB/s, got %s", event.SpeedDisplay)
	}
	if event.ETASSeconds <= 0 {
		t.Fatalf("expected positive ETA seconds, got %d", event.ETASSeconds)
	}
}

func TestDownloadTask_CompletedToEvent(t *testing.T) {
	task := &DownloadTask{
		ID:         "test-comp",
		GameID:     99,
		GameTitle:  "Portal",
		TotalBytes: 2000000,
		Status:     StatusCompleted,
	}

	event := task.ToEvent()
	if event.ProgressPercent != 100.0 {
		t.Fatalf("expected 100%% progress for completed task, got %f", event.ProgressPercent)
	}
	if event.DownloadedBytes != 2000000 {
		t.Fatalf("expected downloadedBytes to match totalBytes, got %d", event.DownloadedBytes)
	}
}

func TestDownloadManager_ProcessQueue(t *testing.T) {
	// Verify queue logic: 1 active, 1 queued
	queue := NewQueueController(1)
	var activeCount int32 = 1

	if queue.CanStartNext(int(atomic.LoadInt32(&activeCount))) {
		t.Fatal("expected queue to deny starting another task when 1 is already active")
	}

	atomic.StoreInt32(&activeCount, 0)
	if !queue.CanStartNext(int(atomic.LoadInt32(&activeCount))) {
		t.Fatal("expected queue to allow starting task when 0 active")
	}
}

func TestDownloadManager_SwitchActiveDownload(t *testing.T) {
	tempDir := t.TempDir()
	db, err := database.InitDB(tempDir)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	cfgMgr, err := config.NewConfigManager(tempDir)
	if err != nil {
		t.Fatalf("failed to init cfg: %v", err)
	}

	dm := NewDownloadManager(db, cfgMgr, func(event DownloadProgressEvent) {})
	defer func() {
		close(dm.stopTicker)
	}()

	task1Cancelled := false
	task1 := &DownloadTask{
		ID:         "task-1",
		GameID:     1,
		GameTitle:  "Game 1",
		TotalBytes: 1000,
		Status:     StatusDownloading,
		cancelFunc: func() {
			task1Cancelled = true
		},
	}
	task2 := &DownloadTask{
		ID:         "task-2",
		GameID:     2,
		GameTitle:  "Game 2",
		TotalBytes: 2000,
		Status:     StatusQueued,
	}

	dm.tasksMu.Lock()
	dm.tasks["task-1"] = task1
	dm.tasks["task-2"] = task2
	dm.tasksMu.Unlock()

	// When user clicks Start/Resume on task-2 while task-1 is downloading:
	_ = dm.ResumeDownload("task-2")

	// Task 1 should be paused and cancelled
	task1.mu.RLock()
	if task1.Status != StatusPaused {
		t.Fatalf("expected task-1 to be paused, got %s", task1.Status)
	}
	task1.mu.RUnlock()

	if !task1Cancelled {
		t.Fatal("expected task-1 cancelFunc to have been invoked")
	}

	// Task 2 should have been triggered (Queued or Failed due to no server configured in test)
	task2.mu.RLock()
	if task2.Status == StatusPaused {
		t.Fatalf("expected task-2 not to be paused, got %s", task2.Status)
	}
	task2.mu.RUnlock()
}

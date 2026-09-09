package downloader

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gamevault/pkg/config"
	"gamevault/pkg/database"

	"github.com/google/uuid"
)

// DownloadManager coordinates tasks, concurrency, rate limiting, and progress telemetry
type DownloadManager struct {
	db         *database.Database
	cfgManager *config.ConfigManager
	tasks      map[string]*DownloadTask
	tasksMu    sync.RWMutex
	queue      *QueueController
	onEvent    func(event DownloadProgressEvent)
	ticker     *time.Ticker
	stopTicker chan struct{}
}

// NewDownloadManager creates a manager, restores incomplete tasks from SQLite, and starts telemetry
func NewDownloadManager(
	db *database.Database,
	cfgManager *config.ConfigManager,
	onEvent func(event DownloadProgressEvent),
) *DownloadManager {
	dm := &DownloadManager{
		db:         db,
		cfgManager: cfgManager,
		tasks:      make(map[string]*DownloadTask),
		queue:      NewQueueController(1), // Default: 1 active game downloading at a time (Steam-style)
		onEvent:    onEvent,
		stopTicker: make(chan struct{}),
	}

	// Restore incomplete tasks from database on launch
	restoredCount := 0
	if history, err := db.GetAllDownloads(); err == nil {
		for _, rec := range history {
			if rec.Status != string(StatusCompleted) && rec.Status != string(StatusCancelled) {
				bg := ""
				isDir := false
				if g, err := db.GetGameByID(rec.GameID); err == nil && g != nil {
					bg = g.BackgroundImage
					if bg == "" {
						bg = g.HeaderImage
					}
					if bg == "" {
						bg = g.CapsuleImage
					}
					isDir = g.IsDirectory
				}
				task := &DownloadTask{
					ID:             rec.ID,
					GameID:         rec.GameID,
					GameTitle:      rec.GameTitle,
					CoverImage:     bg,
					RemotePath:     rec.RemotePath,
					LocalPath:      rec.LocalPath,
					IsDirectory:    isDir,
					TotalBytes:     rec.TotalBytes,
					Status:         StatusPaused,
					lastSampleTime: time.Now(),
				}
				task.DownloadedBytes.Store(rec.DownloadedBytes)
				dm.tasks[rec.ID] = task
				restoredCount++
			}
		}
	}

	if restoredCount > 0 {
		log.Printf("[Downloader] Initialized. Restored %d pending task(s) from database", restoredCount)
	} else {
		log.Printf("[Downloader] Initialized. Queue is empty")
	}

	dm.startMetricsLoop()
	return dm
}

// startMetricsLoop ticks every second to compute smooth speed and stream progress
func (dm *DownloadManager) startMetricsLoop() {
	dm.ticker = time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-dm.stopTicker:
				return
			case <-dm.ticker.C:
				dm.calculateSpeedAndEmit()
			}
		}
	}()
}

// calculateSpeedAndEmit snapshots active tasks and invokes external callbacks outside locks
func (dm *DownloadManager) calculateSpeedAndEmit() {
	dm.tasksMu.RLock()
	taskList := make([]*DownloadTask, 0, len(dm.tasks))
	for _, task := range dm.tasks {
		taskList = append(taskList, task)
	}
	dm.tasksMu.RUnlock()

	now := time.Now()
	var eventsToEmit []DownloadProgressEvent

	for _, task := range taskList {
		task.mu.RLock()
		isActive := task.Status == StatusDownloading || task.Status == StatusScanning
		task.mu.RUnlock()

		if isActive {
			UpdateTaskMetrics(task, now)
			eventsToEmit = append(eventsToEmit, task.ToEvent())
		}
	}

	if dm.onEvent != nil {
		for _, ev := range eventsToEmit {
			dm.onEvent(ev)
		}
	}
}

// emitTaskEvent snapshots a single task and dispatches event to UI
func (dm *DownloadManager) emitTaskEvent(task *DownloadTask) {
	if dm.onEvent != nil {
		dm.onEvent(task.ToEvent())
	}
}

// StartDownload initiates a download task or enqueues it if another download is already active
func (dm *DownloadManager) StartDownload(gameID int64, destinationPath string) (string, error) {
	game, err := dm.db.GetGameByID(gameID)
	if err != nil {
		log.Printf("[Downloader] ERROR: Game ID %d not found in database: %v", gameID, err)
		return "", fmt.Errorf("game not found: %w", err)
	}

	settings := dm.cfgManager.GetSettings()
	if settings.ActiveServer == nil {
		log.Printf("[Downloader] ERROR: Cannot start download for \"%s\": no active server configured", game.CleanTitle)
		return "", fmt.Errorf("no active FTP/SFTP server configured")
	}

	if destinationPath == "" {
		destinationPath = settings.DownloadPath
	}

	// Guard against destinationPath pointing to an existing file (e.g. ~/Downloads/Ducke executable)
	if fi, err := os.Stat(destinationPath); err == nil && !fi.IsDir() {
		safeDir := config.GetDefaultDownloadPath()
		log.Printf("[Downloader] WARN: Destination path \"%s\" is a regular file, not a directory! Redirecting to \"%s\"", destinationPath, safeDir)
		destinationPath = safeDir
	}

	downloadID := uuid.New().String()
	targetLocalPath := filepath.Join(destinationPath, game.CleanTitle)

	log.Printf("[Downloader] Download requested: \"%s\" (Size: %s, Directory: %v) -> Target: %s",
		game.CleanTitle, FormatBytes(game.SizeBytes), game.IsDirectory, targetLocalPath)

	bg := game.BackgroundImage
	if bg == "" {
		bg = game.HeaderImage
	}
	if bg == "" {
		bg = game.CapsuleImage
	}

	task := &DownloadTask{
		ID:             downloadID,
		GameID:         game.ID,
		GameTitle:      game.CleanTitle,
		CoverImage:     bg,
		RemotePath:     game.RemotePath,
		LocalPath:      targetLocalPath,
		IsDirectory:    game.IsDirectory,
		TotalBytes:     game.SizeBytes,
		Status:         StatusQueued,
		lastSampleTime: time.Now(),
	}

	dm.tasksMu.Lock()
	dm.tasks[downloadID] = task
	dm.tasksMu.Unlock()

	_ = dm.db.SaveDownloadRecord(database.DownloadRecord{
		ID:              downloadID,
		GameID:          game.ID,
		GameTitle:       game.CleanTitle,
		RemotePath:      game.RemotePath,
		LocalPath:       targetLocalPath,
		TotalBytes:      game.SizeBytes,
		DownloadedBytes: 0,
		Status:          string(StatusQueued),
	})

	log.Printf("[Downloader] Task enqueued: \"%s\" [ID: %s]", game.CleanTitle, downloadID)
	dm.emitTaskEvent(task)
	dm.processQueue()

	return downloadID, nil
}

// processQueue checks for available download slots and dispatches the next queued game
func (dm *DownloadManager) processQueue() {
	dm.tasksMu.Lock()
	defer dm.tasksMu.Unlock()

	activeCount := 0
	var queuedTask *DownloadTask

	for _, task := range dm.tasks {
		task.mu.RLock()
		status := task.Status
		task.mu.RUnlock()

		if status == StatusDownloading || status == StatusScanning {
			activeCount++
		} else if status == StatusQueued && queuedTask == nil {
			queuedTask = task
		}
	}

	if queuedTask != nil && dm.queue.CanStartNext(activeCount) {
		settings := dm.cfgManager.GetSettings()
		if settings.ActiveServer != nil {
			log.Printf("[Downloader] Queue manager: starting next game \"%s\"", queuedTask.GameTitle)
			go dm.executeDownload(queuedTask, *settings.ActiveServer)
		}
	}
}

// executeDownload coordinates single-file or directory download for a task
func (dm *DownloadManager) executeDownload(task *DownloadTask, srvCfg config.ServerConfig) {
	log.Printf("[Downloader] [%s] Initializing transfer via %s (%s:%d)...",
		task.GameTitle, srvCfg.Protocol, srvCfg.Host, srvCfg.Port)

	task.mu.Lock()
	task.Status = StatusScanning
	task.ErrorMessage = ""
	ctx, cancel := context.WithCancel(context.Background())
	task.cancelCtx = ctx
	task.cancelFunc = cancel
	task.mu.Unlock()

	dm.emitTaskEvent(task)

	defer func() {
		task.mu.RLock()
		status := task.Status
		errMsg := task.ErrorMessage
		task.mu.RUnlock()

		_ = dm.db.SaveDownloadRecord(database.DownloadRecord{
			ID:              task.ID,
			GameID:          task.GameID,
			GameTitle:       task.GameTitle,
			RemotePath:      task.RemotePath,
			LocalPath:       task.LocalPath,
			TotalBytes:      task.TotalBytes,
			DownloadedBytes: task.DownloadedBytes.Load(),
			Status:          string(status),
			ErrorMessage:    errMsg,
		})

		dm.emitTaskEvent(task)
		dm.processQueue() // Auto-start next queued game
	}()

	settings := dm.cfgManager.GetSettings()
	limiter := NewRateLimiter(settings.MaxSpeedKBps)

	task.mu.Lock()
	task.limiter = limiter
	task.mu.Unlock()

	var err error
	if task.IsDirectory {
		err = downloadDirectory(ctx, task, srvCfg, settings.MaxConcurrentFiles, limiter)
	} else {
		err = downloadSingleFile(ctx, task, srvCfg, limiter)
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	if err != nil {
		if task.Status != StatusPaused && task.Status != StatusCancelled {
			task.Status = StatusFailed
			task.ErrorMessage = err.Error()
			log.Printf("[Downloader] ERROR: [%s] Download failed: %v", task.GameTitle, err)
		} else {
			log.Printf("[Downloader] [%s] Download stopped with status: %s", task.GameTitle, task.Status)
		}
		return
	}

	if task.Status == StatusDownloading || task.Status == StatusScanning {
		task.Status = StatusCompleted
		task.DownloadedBytes.Store(task.TotalBytes)
		task.speedBytesPerSec.Store(0)
		task.smoothedSpeed = 0
		task.currentFile = ""
		log.Printf("[Downloader] [%s] Download completed successfully! Total downloaded: %s", task.GameTitle, FormatBytes(task.TotalBytes))
	}
}

// PauseDownload pauses an active download
func (dm *DownloadManager) PauseDownload(downloadID string) error {
	dm.tasksMu.RLock()
	task, exists := dm.tasks[downloadID]
	dm.tasksMu.RUnlock()

	if !exists {
		return fmt.Errorf("download task not found")
	}

	task.mu.Lock()
	if task.Status == StatusDownloading || task.Status == StatusScanning || task.Status == StatusQueued {
		task.Status = StatusPaused
		if task.cancelFunc != nil {
			task.cancelFunc()
		}
	}
	task.mu.Unlock()

	log.Printf("[Downloader] [%s] Download paused (Progress: %s / %s)",
		task.GameTitle, FormatBytes(task.DownloadedBytes.Load()), FormatBytes(task.TotalBytes))

	_ = dm.db.SaveDownloadRecord(database.DownloadRecord{
		ID:              task.ID,
		GameID:          task.GameID,
		GameTitle:       task.GameTitle,
		RemotePath:      task.RemotePath,
		LocalPath:       task.LocalPath,
		TotalBytes:      task.TotalBytes,
		DownloadedBytes: task.DownloadedBytes.Load(),
		Status:          string(StatusPaused),
	})

	dm.emitTaskEvent(task)
	dm.processQueue()
	return nil
}

// ResumeDownload resumes a paused, failed, or enqueued download, making it the active download (Steam-style switch)
func (dm *DownloadManager) ResumeDownload(downloadID string) error {
	dm.tasksMu.Lock()
	task, exists := dm.tasks[downloadID]

	if !exists {
		// Restore from database history
		history, _ := dm.db.GetAllDownloads()
		for _, rec := range history {
			if rec.ID == downloadID {
				bg := ""
				isDir := false
				if g, err := dm.db.GetGameByID(rec.GameID); err == nil && g != nil {
					bg = g.BackgroundImage
					if bg == "" {
						bg = g.HeaderImage
					}
					if bg == "" {
						bg = g.CapsuleImage
					}
					isDir = g.IsDirectory
				}
				task = &DownloadTask{
					ID:             rec.ID,
					GameID:         rec.GameID,
					GameTitle:      rec.GameTitle,
					CoverImage:     bg,
					RemotePath:     rec.RemotePath,
					LocalPath:      rec.LocalPath,
					IsDirectory:    isDir,
					TotalBytes:     rec.TotalBytes,
					Status:         StatusQueued,
					lastSampleTime: time.Now(),
				}
				task.DownloadedBytes.Store(rec.DownloadedBytes)
				dm.tasks[downloadID] = task
				exists = true
				break
			}
		}
	} else {
		// Refresh IsDirectory
		if g, err := dm.db.GetGameByID(task.GameID); err == nil && g != nil {
			task.IsDirectory = g.IsDirectory
		}
	}

	if !exists || task == nil {
		dm.tasksMu.Unlock()
		return fmt.Errorf("download task not found")
	}

	// 1. If this task is already active (downloading or scanning), nothing to do.
	task.mu.RLock()
	alreadyActive := (task.Status == StatusDownloading || task.Status == StatusScanning)
	task.mu.RUnlock()
	if alreadyActive {
		dm.tasksMu.Unlock()
		return nil
	}

	log.Printf("[Downloader] [%s] Download resume requested (Current: %s / %s)",
		task.GameTitle, FormatBytes(task.DownloadedBytes.Load()), FormatBytes(task.TotalBytes))

	// 2. Pause any other currently downloading/scanning tasks so this one takes over the active slot
	otherTasksToPause := make([]*DownloadTask, 0)
	for id, other := range dm.tasks {
		if id != downloadID {
			other.mu.RLock()
			if other.Status == StatusDownloading || other.Status == StatusScanning {
				otherTasksToPause = append(otherTasksToPause, other)
			}
			other.mu.RUnlock()
		}
	}
	dm.tasksMu.Unlock()

	for _, other := range otherTasksToPause {
		other.mu.Lock()
		if other.Status == StatusDownloading || other.Status == StatusScanning {
			other.Status = StatusPaused
			other.speedBytesPerSec.Store(0)
			other.smoothedSpeed = 0
			if other.cancelFunc != nil {
				other.cancelFunc()
			}
		}
		other.mu.Unlock()

		log.Printf("[Downloader] [%s] Paused to prioritize active download", other.GameTitle)

		_ = dm.db.SaveDownloadRecord(database.DownloadRecord{
			ID:              other.ID,
			GameID:          other.GameID,
			GameTitle:       other.GameTitle,
			RemotePath:      other.RemotePath,
			LocalPath:       other.LocalPath,
			TotalBytes:      other.TotalBytes,
			DownloadedBytes: other.DownloadedBytes.Load(),
			Status:          string(StatusPaused),
		})
		dm.emitTaskEvent(other)
	}

	// 3. Mark the requested task as queued and immediately execute it
	task.mu.Lock()
	task.Status = StatusQueued
	task.ErrorMessage = ""
	task.mu.Unlock()

	dm.emitTaskEvent(task)

	settings := dm.cfgManager.GetSettings()
	if settings.ActiveServer != nil {
		go dm.executeDownload(task, *settings.ActiveServer)
	} else {
		task.mu.Lock()
		task.Status = StatusFailed
		task.ErrorMessage = "No active FTP/SFTP server configured"
		task.mu.Unlock()
		log.Printf("[Downloader] ERROR: [%s] Cannot resume: no active FTP/SFTP server configured", task.GameTitle)
		dm.emitTaskEvent(task)
	}

	return nil
}

// CancelDownload cancels an active or queued download
func (dm *DownloadManager) CancelDownload(downloadID string) error {
	dm.tasksMu.RLock()
	task, exists := dm.tasks[downloadID]
	dm.tasksMu.RUnlock()

	if !exists {
		return fmt.Errorf("download task not found")
	}

	task.mu.Lock()
	task.Status = StatusCancelled
	task.speedBytesPerSec.Store(0)
	task.smoothedSpeed = 0
	if task.cancelFunc != nil {
		task.cancelFunc()
	}
	task.mu.Unlock()

	log.Printf("[Downloader] [%s] Download cancelled by user", task.GameTitle)

	_ = dm.db.SaveDownloadRecord(database.DownloadRecord{
		ID:              task.ID,
		GameID:          task.GameID,
		GameTitle:       task.GameTitle,
		RemotePath:      task.RemotePath,
		LocalPath:       task.LocalPath,
		TotalBytes:      task.TotalBytes,
		DownloadedBytes: task.DownloadedBytes.Load(),
		Status:          string(StatusCancelled),
	})

	dm.emitTaskEvent(task)
	dm.processQueue()
	return nil
}

// PauseAll pauses all active and queued downloads
func (dm *DownloadManager) PauseAll() {
	dm.tasksMu.RLock()
	var ids []string
	for id, task := range dm.tasks {
		task.mu.RLock()
		if task.Status == StatusDownloading || task.Status == StatusScanning || task.Status == StatusQueued {
			ids = append(ids, id)
		}
		task.mu.RUnlock()
	}
	dm.tasksMu.RUnlock()

	if len(ids) > 0 {
		log.Printf("[Downloader] Pausing all active downloads (%d tasks)", len(ids))
	}
	for _, id := range ids {
		_ = dm.PauseDownload(id)
	}
}

// ResumeAll marks all paused or failed downloads as queued and triggers queue processing
func (dm *DownloadManager) ResumeAll() {
	dm.tasksMu.Lock()
	count := 0
	for _, task := range dm.tasks {
		task.mu.Lock()
		if task.Status == StatusPaused || task.Status == StatusFailed {
			task.Status = StatusQueued
			task.ErrorMessage = ""
			dm.emitTaskEvent(task)
			count++
		}
		task.mu.Unlock()
	}
	dm.tasksMu.Unlock()

	if count > 0 {
		log.Printf("[Downloader] Resuming %d downloads", count)
	}
	dm.processQueue()
}

// CancelAll cancels all queued and active downloads
func (dm *DownloadManager) CancelAll() {
	dm.tasksMu.RLock()
	var ids []string
	for id, task := range dm.tasks {
		task.mu.RLock()
		if task.Status == StatusDownloading || task.Status == StatusScanning || task.Status == StatusQueued || task.Status == StatusPaused {
			ids = append(ids, id)
		}
		task.mu.RUnlock()
	}
	dm.tasksMu.RUnlock()

	if len(ids) > 0 {
		log.Printf("[Downloader] Cancelling all downloads (%d tasks)", len(ids))
	}
	for _, id := range ids {
		_ = dm.CancelDownload(id)
	}
}

// ClearCompleted removes completed and cancelled tasks from memory and database
func (dm *DownloadManager) ClearCompleted() error {
	dm.tasksMu.Lock()
	count := 0
	for id, task := range dm.tasks {
		task.mu.RLock()
		if task.Status == StatusCompleted || task.Status == StatusCancelled {
			delete(dm.tasks, id)
			count++
		}
		task.mu.RUnlock()
	}
	dm.tasksMu.Unlock()

	log.Printf("[Downloader] Cleared %d completed/cancelled downloads from view", count)
	return dm.db.ClearCompletedDownloads()
}

// DeleteTask removes task record and optionally deletes local files from disk
func (dm *DownloadManager) DeleteTask(downloadID string, removeFiles bool) error {
	dm.tasksMu.Lock()
	task, exists := dm.tasks[downloadID]
	if exists {
		if task.cancelFunc != nil {
			task.cancelFunc()
		}
		delete(dm.tasks, downloadID)
	}
	dm.tasksMu.Unlock()

	if removeFiles && exists && task.LocalPath != "" {
		log.Printf("[Downloader] Removing files from disk for deleted task: %s", task.LocalPath)
		_ = os.RemoveAll(task.LocalPath)
	}

	log.Printf("[Downloader] Deleted task record: %s (Delete files: %v)", downloadID, removeFiles)
	return dm.db.DeleteDownloadRecord(downloadID)
}

// GetTasks returns point-in-time snapshots of all tasks for UI queries
func (dm *DownloadManager) GetTasks() []DownloadProgressEvent {
	dm.tasksMu.RLock()
	defer dm.tasksMu.RUnlock()

	events := make([]DownloadProgressEvent, 0, len(dm.tasks))
	for _, task := range dm.tasks {
		events = append(events, task.ToEvent())
	}
	return events
}

// UpdateSpeedLimit updates rate limiter dynamically across all active tasks
func (dm *DownloadManager) UpdateSpeedLimit(kbps int) {
	dm.tasksMu.RLock()
	defer dm.tasksMu.RUnlock()

	if kbps <= 0 {
		log.Printf("[Downloader] Speed limit updated: unlimited")
	} else {
		log.Printf("[Downloader] Speed limit updated: %s", FormatSpeed(int64(kbps)*1024))
	}

	for _, task := range dm.tasks {
		task.SetSpeedLimit(kbps)
	}
}

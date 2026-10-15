package downloader

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gamevault/pkg/config"
	"gamevault/pkg/database"

	"github.com/google/uuid"
)

// DownloadManager coordinates tasks, concurrency, rate limiting, and progress telemetry
type DownloadManager struct {
	db            *database.Database
	cfgManager    *config.ConfigManager
	tasks         map[string]*DownloadTask
	tasksMu       sync.RWMutex
	queue         *QueueController
	onEvent       func(event DownloadProgressEvent)
	onListChanged func()
	torrentEngine *TorrentEngine
	scraper       *TrackerScraper
	ticker        *time.Ticker
	stopTicker    chan struct{}
}

// NewDownloadManager creates a manager, restores incomplete tasks from SQLite, and starts telemetry
func NewDownloadManager(
	db *database.Database,
	cfgManager *config.ConfigManager,
	onEvent func(event DownloadProgressEvent),
	onListChanged ...func(),
) *DownloadManager {
	settings := cfgManager.GetSettings()
	te, err := NewTorrentEngine(settings.DownloadPath, settings.MaxSpeedKBps)
	if err != nil {
		log.Printf("[Downloader] Warning: failed to init torrent engine: %v", err)
	}

	var listChangedCb func()
	if len(onListChanged) > 0 {
		listChangedCb = onListChanged[0]
	}

	dm := &DownloadManager{
		db:            db,
		cfgManager:    cfgManager,
		tasks:         make(map[string]*DownloadTask),
		queue:         NewQueueController(1), // Default: 1 active game downloading at a time (Steam-style)
		onEvent:       onEvent,
		onListChanged: listChangedCb,
		torrentEngine: te,
		scraper:       NewTrackerScraper(),
		stopTicker:    make(chan struct{}),
	}

	// Restore incomplete tasks from database on launch
	restoredCount := 0
	if history, err := db.GetAllDownloads(); err == nil {
		for _, rec := range history {
			if rec.Status != string(StatusCompleted) && rec.Status != string(StatusCancelled) {
				bg := ""
				isDir := false
				var gameEntity *database.GameEntity
				if g, err := db.GetGameByID(rec.GameID); err == nil && g != nil {
					gameEntity = g
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
					IsTorrent:      rec.IsTorrent,
					MagnetURI:      rec.MagnetURI,
					IsDDL:          rec.IsDDL,
					DDLHost:        rec.DDLHost,
					TotalParts:     rec.TotalParts,
					lastSampleTime: time.Now(),
				}
				task.CurrentPart.Store(int32(rec.CurrentPart))
				if gameEntity != nil && len(gameEntity.URIs) > 0 {
					task.URIs = gameEntity.URIs
				}
				task.DownloadedBytes.Store(rec.DownloadedBytes)
				dm.tasks[rec.ID] = task
				restoredCount++

				// Ensure database record is updated to paused so DB state matches in-memory state
				if rec.Status != string(StatusPaused) {
					rec.Status = string(StatusPaused)
					_ = db.SaveDownloadRecord(rec)
				}
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
			var overrideDelta int64 = -1
			if task.IsTorrent && dm.torrentEngine != nil {
				if t, ok := dm.torrentEngine.GetTorrent(task.ID); ok && t != nil {
					stats := t.Stats()
					task.mu.Lock()
					task.TorrentSeeds = stats.ConnectedSeeders
					task.TorrentPeers = stats.ActivePeers
					if task.TorrentPeers == 0 && stats.TotalPeers > 0 {
						task.TorrentPeers = stats.TotalPeers
					}
					if task.Status == StatusDownloading {
						bytes := t.BytesCompleted()
						task.DownloadedBytes.Store(bytes)
						if t.Length() > 0 && task.TotalBytes != t.Length() {
							task.TotalBytes = t.Length()
						}
					}

					// Use continuous wire read metrics for silky smooth, stable speed measurement
					wireBytes := stats.BytesReadUsefulData.Int64()
					if wireBytes == 0 {
						wireBytes = stats.BytesReadData.Int64()
					}
					if task.lastWireBytes > 0 && wireBytes >= task.lastWireBytes {
						overrideDelta = wireBytes - task.lastWireBytes
					} else if task.lastWireBytes == 0 {
						overrideDelta = 0
					}
					task.lastWireBytes = wireBytes
					task.mu.Unlock()
				}
			}
			if overrideDelta >= 0 {
				UpdateTaskMetrics(task, now, overrideDelta)
			} else {
				UpdateTaskMetrics(task, now)
			}
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

// notifyListChanged notifies frontend that the queue or tasks set has structurally changed
func (dm *DownloadManager) notifyListChanged() {
	if dm.onListChanged != nil {
		dm.onListChanged()
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
	sanitizedTitle := SanitizeFilename(game.CleanTitle)
	targetLocalPath := filepath.Join(destinationPath, sanitizedTitle)

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
	dm.notifyListChanged()
	dm.processQueue()

	return downloadID, nil
}

// StartTorrentDownload enqueues a torrent download task
func (dm *DownloadManager) StartTorrentDownload(game database.GameEntity, destinationPath string) (string, error) {
	magnetURI := game.MagnetURI
	if magnetURI == "" && strings.HasPrefix(game.RemotePath, "magnet:") {
		magnetURI = game.RemotePath
	}
	if magnetURI == "" {
		return "", fmt.Errorf("torrent game has no magnet URI")
	}

	settings := dm.cfgManager.GetSettings()
	if destinationPath == "" {
		destinationPath = settings.DownloadPath
	}

	if fi, err := os.Stat(destinationPath); err == nil && !fi.IsDir() {
		destinationPath = config.GetDefaultDownloadPath()
	}

	// Disk space check
	free, _, err := GetDiskSpace(destinationPath)
	if err == nil && game.SizeBytes > 0 && free > 0 && free < game.SizeBytes {
		return "", fmt.Errorf("недостаточно места на диске: требуется %s, доступно %s", FormatBytes(game.SizeBytes), FormatBytes(free))
	}

	downloadID := uuid.New().String()
	sanitizedTitle := SanitizeFilename(game.CleanTitle)
	targetLocalPath := filepath.Join(destinationPath, sanitizedTitle)

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
		RemotePath:     magnetURI,
		LocalPath:      targetLocalPath,
		TotalBytes:     game.SizeBytes,
		Status:         StatusQueued,
		IsTorrent:      true,
		MagnetURI:      magnetURI,
		lastSampleTime: time.Now(),
	}

	dm.tasksMu.Lock()
	dm.tasks[downloadID] = task
	dm.tasksMu.Unlock()

	_ = dm.db.SaveDownloadRecord(database.DownloadRecord{
		ID:              downloadID,
		GameID:          game.ID,
		GameTitle:       game.CleanTitle,
		RemotePath:      magnetURI,
		LocalPath:       targetLocalPath,
		TotalBytes:      game.SizeBytes,
		DownloadedBytes: 0,
		Status:          string(StatusQueued),
		IsTorrent:       true,
		MagnetURI:       magnetURI,
	})

	log.Printf("[Downloader] Torrent task enqueued: \"%s\" [ID: %s]", game.CleanTitle, downloadID)
	dm.emitTaskEvent(task)
	dm.notifyListChanged()
	dm.processQueue()

	return downloadID, nil
}

// StartDDLDownload enqueues a direct file hoster download task
func (dm *DownloadManager) StartDDLDownload(game database.GameEntity, preferredHoster string, destinationPath string) (string, error) {
	uris := game.URIs
	if len(uris) == 0 && game.RemotePath != "" {
		uris = []string{game.RemotePath}
	}
	if len(uris) == 0 {
		return "", fmt.Errorf("игра не содержит ссылок для загрузки")
	}

	// Filter out magnet links from DDL URIs, but preserve magnet link for failover
	var cleanDDLURIs []string
	foundMagnet := game.MagnetURI
	for _, u := range uris {
		trimmed := strings.TrimSpace(u)
		if strings.HasPrefix(strings.ToLower(trimmed), "magnet:") {
			if foundMagnet == "" {
				foundMagnet = trimmed
			}
			continue
		}
		cleanDDLURIs = append(cleanDDLURIs, trimmed)
	}

	// If no DDL links found but game has a torrent magnet, start torrent download directly
	if len(cleanDDLURIs) == 0 {
		if foundMagnet != "" {
			log.Printf("[Downloader] [%s] Game has torrent magnet only, redirecting to Torrent download...", game.CleanTitle)
			return dm.StartTorrentDownload(game, destinationPath)
		}
		return "", fmt.Errorf("игра не содержит ссылок для прямой загрузки")
	}

	settings := dm.cfgManager.GetSettings()
	if destinationPath == "" {
		destinationPath = settings.DownloadPath
	}

	if fi, err := os.Stat(destinationPath); err == nil && !fi.IsDir() {
		destinationPath = config.GetDefaultDownloadPath()
	}

	// Disk space check
	free, _, err := GetDiskSpace(destinationPath)
	if err == nil && game.SizeBytes > 0 && free > 0 && free < game.SizeBytes {
		return "", fmt.Errorf("недостаточно места на диске: требуется %s, доступно %s", FormatBytes(game.SizeBytes), FormatBytes(free))
	}

	downloadID := uuid.New().String()
	sanitizedTitle := SanitizeFilename(game.CleanTitle)
	targetLocalPath := filepath.Join(destinationPath, sanitizedTitle)

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
		RemotePath:     cleanDDLURIs[0],
		LocalPath:      targetLocalPath,
		TotalBytes:     game.SizeBytes,
		Status:         StatusQueued,
		IsDDL:          true,
		DDLHost:        preferredHoster,
		URIs:           cleanDDLURIs,
		MagnetURI:      foundMagnet,
		lastSampleTime: time.Now(),
	}

	dm.tasksMu.Lock()
	dm.tasks[downloadID] = task
	dm.tasksMu.Unlock()

	_ = dm.db.SaveDownloadRecord(database.DownloadRecord{
		ID:              downloadID,
		GameID:          game.ID,
		GameTitle:       game.CleanTitle,
		RemotePath:      uris[0],
		LocalPath:       targetLocalPath,
		TotalBytes:      game.SizeBytes,
		DownloadedBytes: 0,
		Status:          string(StatusQueued),
		IsDDL:           true,
		DDLHost:         preferredHoster,
	})

	log.Printf("[Downloader] DDL task enqueued: \"%s\" (Host: %s, %d URIs) [ID: %s]",
		game.CleanTitle, preferredHoster, len(uris), downloadID)
	dm.emitTaskEvent(task)
	dm.notifyListChanged()
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
		if queuedTask.IsTorrent {
			log.Printf("[Downloader] Queue manager: starting next torrent \"%s\"", queuedTask.GameTitle)
			go dm.executeTorrentDownload(queuedTask)
		} else if queuedTask.IsDDL {
			log.Printf("[Downloader] Queue manager: starting next DDL hoster transfer \"%s\"", queuedTask.GameTitle)
			go dm.executeDDLDownload(queuedTask)
		} else {
			settings := dm.cfgManager.GetSettings()
			if settings.ActiveServer != nil {
				log.Printf("[Downloader] Queue manager: starting next game \"%s\"", queuedTask.GameTitle)
				go dm.executeDownload(queuedTask, *settings.ActiveServer)
			}
		}
	}
}

// executeTorrentDownload handles torrent lifecycle and piece downloads
func (dm *DownloadManager) executeTorrentDownload(task *DownloadTask) {
	log.Printf("[Downloader] [%s] Initializing BitTorrent download (Magnet: %s)...", task.GameTitle, task.MagnetURI)

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
			IsTorrent:       true,
			MagnetURI:       task.MagnetURI,
		})

		dm.emitTaskEvent(task)
		dm.notifyListChanged()
		dm.processQueue() // Auto-start next queued game
	}()

	if dm.torrentEngine == nil {
		task.mu.Lock()
		task.Status = StatusFailed
		task.ErrorMessage = "BitTorrent engine not initialized"
		task.mu.Unlock()
		return
	}

	// If task had an error, drop previous in-memory handle so storage is reconfigured cleanly
	task.mu.RLock()
	hadPrevError := task.ErrorMessage != ""
	task.mu.RUnlock()
	if hadPrevError {
		dm.torrentEngine.DropTorrent(task.ID)
	}

	settings := dm.cfgManager.GetSettings()
	destDir := settings.DownloadPath
	if destDir == "" {
		destDir = config.GetDefaultDownloadPath()
	}

	// Resolve destination directory cleanly (avoiding trailing slashes or subfolder splits)
	if task.LocalPath != "" {
		cleanTaskPath := filepath.Clean(task.LocalPath)
		cleanSettingsPath := filepath.Clean(destDir)
		rel, err := filepath.Rel(cleanSettingsPath, cleanTaskPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			// task.LocalPath is under settings.DownloadPath
			destDir = cleanSettingsPath
		} else {
			// Download was placed in a custom folder or drive; find existing parent
			candidate := filepath.Dir(cleanTaskPath)
			for candidate != "." && candidate != "/" && filepath.Dir(candidate) != candidate {
				if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
					destDir = candidate
					break
				}
				candidate = filepath.Dir(candidate)
			}
		}
	}
	_ = os.MkdirAll(destDir, 0755)

	t, err := dm.torrentEngine.AddMagnet(task.ID, task.MagnetURI, destDir)
	if err != nil {
		task.mu.Lock()
		task.Status = StatusFailed
		task.ErrorMessage = fmt.Sprintf("failed to add torrent: %v", err)
		task.mu.Unlock()
		return
	}

	// 1. Resolve metadata (Scanning phase)
	scanTicker := time.NewTicker(2 * time.Second)
	defer scanTicker.Stop()

scanLoop:
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.GotInfo():
			log.Printf("[Downloader] [%s] Torrent metadata resolved: %s (Length: %s)",
				task.GameTitle, t.Name(), FormatBytes(t.Length()))
			break scanLoop
		case <-scanTicker.C:
			stats := t.Stats()
			task.mu.Lock()
			task.TorrentSeeds = stats.ConnectedSeeders
			task.TorrentPeers = stats.ActivePeers
			if task.TorrentPeers == 0 && stats.TotalPeers > 0 {
				task.TorrentPeers = stats.TotalPeers
			}
			task.mu.Unlock()
			dm.emitTaskEvent(task)
			log.Printf("[Downloader] [%s] Resolving torrent metadata... (Peers: %d, Seeders: %d)",
				task.GameTitle, task.TorrentPeers, task.TorrentSeeds)
		}
	}

	cleanTorrentName := SanitizeFilename(t.Name())
	actualTorrentPath := filepath.Join(destDir, cleanTorrentName)
	task.mu.Lock()
	if task.Status != StatusScanning && task.Status != StatusQueued {
		task.mu.Unlock()
		return
	}
	task.Status = StatusDownloading
	task.LocalPath = actualTorrentPath
	task.ErrorMessage = ""
	if t.Length() > 0 {
		task.TotalBytes = t.Length()
	}
	task.mu.Unlock()

	_ = dm.db.SaveDownloadRecord(database.DownloadRecord{
		ID:              task.ID,
		GameID:          task.GameID,
		GameTitle:       task.GameTitle,
		RemotePath:      task.RemotePath,
		LocalPath:       actualTorrentPath,
		TotalBytes:      task.TotalBytes,
		DownloadedBytes: task.DownloadedBytes.Load(),
		Status:          string(StatusDownloading),
		IsTorrent:       true,
		MagnetURI:       task.MagnetURI,
	})

	dm.emitTaskEvent(task)
	dm.notifyListChanged()

	if len(t.Files()) > 1 {
		_ = os.MkdirAll(actualTorrentPath, 0755)
	} else if len(t.Files()) == 1 {
		_ = os.MkdirAll(filepath.Dir(actualTorrentPath), 0755)
	}

	// 2. Register chunk write error callback to detect and report filesystem/permission issues
	t.SetOnWriteChunkError(func(err error) {
		log.Printf("[Downloader] ERROR: [%s] Torrent disk chunk write error: %v", task.GameTitle, err)
		task.mu.Lock()
		task.ErrorMessage = fmt.Sprintf("Ошибка записи на диск: %v", err)
		task.mu.Unlock()
		dm.emitTaskEvent(task)
	})

	// 3. Ensure data download is allowed and start piece downloads immediately
	t.AllowDataDownload()
	t.DownloadAll()

	// 4. Update UI with bytes already confirmed complete in the persistent PieceCompletion store
	if verified := t.BytesCompleted(); verified > 0 {
		task.DownloadedBytes.Store(verified)
		dm.emitTaskEvent(task)
		log.Printf("[Downloader] [%s] Resume state: %s already completed on disk", task.GameTitle, FormatBytes(verified))
	} else {
		log.Printf("[Downloader] [%s] Starting piece downloads (Total: %s)...", task.GameTitle, FormatBytes(task.TotalBytes))
	}

	// 4. Monitor progress & completion
	completeFlag := t.Complete()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Downloader] [%s] Torrent download context stopped with status %s", task.GameTitle, task.Status)
			return
		case <-completeFlag.On():
			task.mu.Lock()
			task.Status = StatusCompleted
			task.LocalPath = actualTorrentPath
			task.DownloadedBytes.Store(task.TotalBytes)
			task.speedBytesPerSec.Store(0)
			task.smoothedSpeed = 0
			task.mu.Unlock()
			log.Printf("[Downloader] [%s] Torrent download completed successfully! Total: %s",
				task.GameTitle, FormatBytes(task.TotalBytes))
			return
		case <-ticker.C:
			if completeFlag.Bool() || (t.BytesCompleted() >= t.Length() && t.Length() > 0) {
				task.mu.Lock()
				task.Status = StatusCompleted
				task.LocalPath = actualTorrentPath
				task.DownloadedBytes.Store(task.TotalBytes)
				task.speedBytesPerSec.Store(0)
				task.smoothedSpeed = 0
				task.mu.Unlock()
				log.Printf("[Downloader] [%s] Torrent download completed successfully! Total: %s",
					task.GameTitle, FormatBytes(task.TotalBytes))
				return
			}
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
		dm.notifyListChanged()
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
		err = downloadSingleFile(ctx, task, srvCfg, settings.MaxConcurrentFiles, limiter)
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
		if task.IsTorrent && dm.torrentEngine != nil {
			dm.torrentEngine.PauseTorrent(downloadID)
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
		IsTorrent:       task.IsTorrent,
		MagnetURI:       task.MagnetURI,
		IsDDL:           task.IsDDL,
		DDLHost:         task.DDLHost,
		CurrentPart:     int(task.CurrentPart.Load()),
		TotalParts:      task.TotalParts,
	})

	dm.emitTaskEvent(task)
	dm.notifyListChanged()
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
					IsTorrent:      rec.IsTorrent,
					MagnetURI:      rec.MagnetURI,
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
			if other.IsTorrent && dm.torrentEngine != nil {
				dm.torrentEngine.PauseTorrent(other.ID)
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
			IsTorrent:       other.IsTorrent,
			MagnetURI:       other.MagnetURI,
			IsDDL:           other.IsDDL,
			DDLHost:         other.DDLHost,
			CurrentPart:     int(other.CurrentPart.Load()),
			TotalParts:      other.TotalParts,
		})
		dm.emitTaskEvent(other)
	}

	// 3. Mark the requested task as queued and immediately execute it
	task.mu.Lock()
	hadError := task.ErrorMessage != "" || task.Status == StatusFailed
	task.Status = StatusQueued
	task.ErrorMessage = ""
	task.mu.Unlock()

	dm.emitTaskEvent(task)
	dm.notifyListChanged()

	if task.IsTorrent {
		if dm.torrentEngine != nil {
			if hadError {
				dm.torrentEngine.DropTorrent(task.ID)
			} else {
				dm.torrentEngine.ResumeTorrent(task.ID)
			}
		}
		go dm.executeTorrentDownload(task)
	} else if task.IsDDL {
		go dm.executeDDLDownload(task)
	} else {
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
			dm.notifyListChanged()
		}
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
	if task.IsTorrent && dm.torrentEngine != nil {
		dm.torrentEngine.PurgeState(downloadID)
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
		IsTorrent:       task.IsTorrent,
		MagnetURI:       task.MagnetURI,
	})

	dm.emitTaskEvent(task)
	dm.notifyListChanged()
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
	dm.notifyListChanged()
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
	dm.notifyListChanged()
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
	dm.notifyListChanged()
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
	dm.notifyListChanged()
	return dm.db.ClearCompletedDownloads()
}

// DeleteTask removes task record and optionally deletes local files from disk
func (dm *DownloadManager) DeleteTask(downloadID string, removeFiles bool) error {
	dm.tasksMu.Lock()
	task, exists := dm.tasks[downloadID]
	var localPath string
	if exists {
		if task.cancelFunc != nil {
			task.cancelFunc()
		}
		if task.IsTorrent && dm.torrentEngine != nil {
			dm.torrentEngine.PurgeState(downloadID)
		}
		localPath = task.LocalPath
		delete(dm.tasks, downloadID)
	}
	dm.tasksMu.Unlock()

	if removeFiles {
		if localPath == "" && dm.db != nil {
			if rec, err := dm.db.GetDownloadRecord(downloadID); err == nil && rec != nil {
				localPath = rec.LocalPath
			}
		}
		if localPath != "" {
			log.Printf("[Downloader] Removing files from disk for deleted task: %s", localPath)
			_ = os.RemoveAll(localPath)
			_ = os.Remove(localPath + ".duckestate")
		}
	}

	log.Printf("[Downloader] Deleted task record: %s (Delete files: %v)", downloadID, removeFiles)
	dm.notifyListChanged()
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

	if dm.torrentEngine != nil {
		dm.torrentEngine.SetSpeedLimit(kbps)
	}

	for _, task := range dm.tasks {
		task.SetSpeedLimit(kbps)
	}
}

// Shutdown stops telemetry and cleanly closes the torrent engine
func (dm *DownloadManager) Shutdown() {
	close(dm.stopTicker)
	if dm.torrentEngine != nil {
		_ = dm.torrentEngine.Close()
	}
}

// GetTorrentSeedsBatch queries live or cached seed counts for a batch of torrent variants
func (dm *DownloadManager) GetTorrentSeedsBatch(ctx context.Context, queries []TorrentSeedQuery) map[int64]TorrentSeedResult {
	if dm.scraper == nil {
		dm.scraper = NewTrackerScraper()
	}
	return dm.scraper.ScrapeBatch(ctx, queries)
}

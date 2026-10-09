package downloader

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gamevault/pkg/config"
	"gamevault/pkg/remote"
)

type fileWorkItem struct {
	remote remote.RemoteFileEntry
	offset int64
}

// ensureTargetDirectory guarantees that task.LocalPath can be created as a folder or parent folder
func ensureTargetDirectory(task *DownloadTask) error {
	targetDir := task.LocalPath
	if !task.IsDirectory {
		targetDir = filepath.Dir(task.LocalPath)
	}

	if err := os.MkdirAll(targetDir, 0755); err == nil {
		return nil
	}

	// If MkdirAll failed (e.g. ENOTDIR because /home/deck/Downloads/Ducke is a regular file)
	log.Printf("[Downloader] WARN: [%s] Cannot create destination \"%s\". Redirecting to safe games path...", task.GameTitle, targetDir)
	safeBase := config.GetDefaultDownloadPath()
	newLocalPath := filepath.Join(safeBase, task.GameTitle)
	newDir := newLocalPath
	if !task.IsDirectory {
		newDir = filepath.Dir(newLocalPath)
	}

	if err := os.MkdirAll(newDir, 0755); err != nil {
		log.Printf("[Downloader] ERROR: [%s] Failed to create fallback directory \"%s\": %v", task.GameTitle, newDir, err)
		return fmt.Errorf("failed to create directory: %w", err)
	}

	log.Printf("[Downloader] [%s] Successfully redirected download path from \"%s\" to \"%s\"", task.GameTitle, task.LocalPath, newLocalPath)
	task.mu.Lock()
	task.LocalPath = newLocalPath
	task.mu.Unlock()
	return nil
}

// downloadDirectory manages parallel multi-threaded directory download
func downloadDirectory(
	ctx context.Context,
	task *DownloadTask,
	srvCfg config.ServerConfig,
	maxConcurrentFiles int,
	limiter *RateLimiter,
) error {
	// 0. Ensure target directory exists and is not blocked by a regular file
	if err := ensureTargetDirectory(task); err != nil {
		return err
	}

	log.Printf("[Downloader] [%s] Connecting to %s server (%s:%d)...",
		task.GameTitle, strings.ToUpper(srvCfg.Protocol), srvCfg.Host, srvCfg.Port)

	client := remote.NewRemoteClient(srvCfg.Protocol)
	if err := client.Connect(srvCfg); err != nil {
		log.Printf("[Downloader] ERROR: [%s] Connection failed: %v", task.GameTitle, err)
		return fmt.Errorf("connection error: %w", err)
	}
	defer client.Close()

	// 1. Recursive file listing from remote
	log.Printf("[Downloader] [%s] Scanning remote directory tree: \"%s\"...", task.GameTitle, task.RemotePath)
	files, err := client.ListFilesRecursive(task.RemotePath)
	if err != nil {
		log.Printf("[Downloader] ERROR: [%s] Failed to scan remote files: %v", task.GameTitle, err)
		return fmt.Errorf("failed to scan remote directory: %w", err)
	}

	// 2. Pre-scan local files for smart integrity verification and resume
	var totalBytes int64 = 0
	var initialDownloaded int64 = 0
	var queue []fileWorkItem

	for _, f := range files {
		totalBytes += f.SizeBytes
		localFilePath := filepath.Join(task.LocalPath, f.RelativePath)
		if fi, err := os.Stat(localFilePath); err == nil {
			localSize := fi.Size()
			if f.SizeBytes > 0 && localSize == f.SizeBytes {
				// Fully downloaded already
				initialDownloaded += f.SizeBytes
				continue
			} else if localSize > 0 && localSize < f.SizeBytes {
				// Partial file, resume from offset
				initialDownloaded += localSize
				queue = append(queue, fileWorkItem{remote: f, offset: localSize})
				continue
			} else if localSize > f.SizeBytes {
				// File is larger than remote entry (corrupted / outdated) -> truncate to restart cleanly
				_ = os.Truncate(localFilePath, 0)
			}
		}
		queue = append(queue, fileWorkItem{remote: f, offset: 0})
	}

	// LPT (Longest Processing Time First) scheduling: sort queue largest files first.
	// Keeps all worker threads uniformly utilized, eliminating tail-latency bottlenecks.
	sort.Slice(queue, func(i, j int) bool {
		return queue[i].remote.SizeBytes > queue[j].remote.SizeBytes
	})

	log.Printf("[Downloader] [%s] Directory analysis: %d files (%s total). %d files to download (%s), %s already on disk",
		task.GameTitle, len(files), FormatBytes(totalBytes), len(queue), FormatBytes(totalBytes-initialDownloaded), FormatBytes(initialDownloaded))

	task.mu.Lock()
	if totalBytes > 0 {
		task.TotalBytes = totalBytes
	}
	task.DownloadedBytes.Store(initialDownloaded)
	task.lastBytes = initialDownloaded
	task.totalFiles = len(files)
	task.fileIndex.Store(int32(len(files) - len(queue)))
	task.Status = StatusDownloading
	task.mu.Unlock()

	if len(queue) == 0 {
		log.Printf("[Downloader] [%s] All %d files are complete and verified on disk! Skipping download.", task.GameTitle, len(files))
		task.mu.Lock()
		task.Status = StatusCompleted
		task.DownloadedBytes.Store(task.TotalBytes)
		task.speedBytesPerSec.Store(0)
		task.smoothedSpeed = 0
		task.currentFile = ""
		task.mu.Unlock()
		return nil
	}

	// 3. Worker Pool: Parallel file downloads over multiple worker connections
	concurrency := maxConcurrentFiles
	if concurrency < 2 {
		concurrency = 4
	} else if concurrency > 8 {
		concurrency = 8
	}
	if len(queue) < concurrency {
		concurrency = len(queue)
	}

	log.Printf("[Downloader] [%s] Starting transfer with %d concurrent worker connections for %d files",
		task.GameTitle, concurrency, len(queue))

	workChan := make(chan fileWorkItem, len(queue))
	for _, item := range queue {
		workChan <- item
	}
	close(workChan)

	var wg sync.WaitGroup
	var workerErr error
	var errOnce sync.Once

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerClient := remote.NewRemoteClient(srvCfg.Protocol)
			if err := workerClient.Connect(srvCfg); err != nil {
				log.Printf("[Downloader] ERROR: [%s] Worker #%d connection failed: %v", task.GameTitle, workerID+1, err)
				errOnce.Do(func() {
					workerErr = fmt.Errorf("worker connection failed: %w", err)
				})
				return
			}
			defer workerClient.Close()

			for item := range workChan {
				select {
				case <-ctx.Done():
					return
				default:
				}

				localFilePath := filepath.Join(task.LocalPath, item.remote.RelativePath)
				task.mu.Lock()
				task.currentFile = item.remote.RelativePath
				task.mu.Unlock()

				err := downloadFileWithRetry(ctx, task, workerClient, srvCfg, item.remote.FullPath, localFilePath, item.remote.SizeBytes, item.offset, limiter)
				if err != nil {
					log.Printf("[Downloader] ERROR: [%s] File transfer failed: %s: %v", task.GameTitle, item.remote.RelativePath, err)
					errOnce.Do(func() {
						workerErr = err
					})
					return
				}

				// Preserve remote modification timestamp
				if item.remote.ModTime > 0 {
					mtime := time.Unix(item.remote.ModTime, 0)
					_ = os.Chtimes(localFilePath, mtime, mtime)
				}

				idx := task.fileIndex.Add(1)
				// Log file completion selectively to avoid flooding if thousands of files
				if task.totalFiles <= 30 || item.remote.SizeBytes >= 5*1024*1024 || idx%25 == 0 || int(idx) == task.totalFiles {
					log.Printf("[Downloader] [%s] Progress [%d/%d]: %s (%s)",
						task.GameTitle, idx, task.totalFiles, filepath.Base(item.remote.RelativePath), FormatBytes(item.remote.SizeBytes))
				}
			}
		}(w)
	}

	wg.Wait()
	return workerErr
}

// downloadSingleFile manages download of a single monolithic file (ISO / Archive / Installer)
func downloadSingleFile(
	ctx context.Context,
	task *DownloadTask,
	srvCfg config.ServerConfig,
	maxConcurrentStreams int,
	limiter *RateLimiter,
) error {
	// 0. Ensure target directory exists and is not blocked by a regular file
	if err := ensureTargetDirectory(task); err != nil {
		return err
	}

	localFilePath := task.LocalPath

	// Pre-check if monolithic file is already fully downloaded on disk
	if fi, err := os.Stat(localFilePath); err == nil {
		localSize := fi.Size()
		if task.TotalBytes > 0 && localSize == task.TotalBytes {
			log.Printf("[Downloader] [%s] Single file already complete on disk (%s). Done.", task.GameTitle, FormatBytes(localSize))
			task.DownloadedBytes.Store(task.TotalBytes)
			task.mu.Lock()
			task.Status = StatusCompleted
			task.speedBytesPerSec.Store(0)
			task.smoothedSpeed = 0
			task.currentFile = ""
			task.mu.Unlock()
			removeSegmentState(localFilePath)
			return nil
		}
	}

	// 1. If monolithic file is large (>= 32MB), execute accelerated multi-segment transfer
	if task.TotalBytes >= MinSegmentedFileSize {
		log.Printf("[Downloader] [%s] Large monolithic file detected (%s >= 32MB). Attempting accelerated multi-segment transfer...",
			task.GameTitle, FormatBytes(task.TotalBytes))

		segErr := downloadSegmentedFile(ctx, task, srvCfg, maxConcurrentStreams, limiter)
		if segErr == nil {
			log.Printf("[Downloader] [%s] Multi-segment transfer completed successfully (%s)",
				task.GameTitle, FormatBytes(task.TotalBytes))
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		log.Printf("[Downloader] WARN: [%s] Multi-segment transfer unavailable (%v). Falling back to resilient single-stream transfer...",
			task.GameTitle, segErr)
	}

	// 2. Single-stream transfer (for files < 32MB or servers without REST support)
	log.Printf("[Downloader] [%s] Connecting to %s server for single-stream transfer...",
		task.GameTitle, strings.ToUpper(srvCfg.Protocol))

	client := remote.NewRemoteClient(srvCfg.Protocol)
	if err := client.Connect(srvCfg); err != nil {
		log.Printf("[Downloader] ERROR: [%s] Connection error: %v", task.GameTitle, err)
		return fmt.Errorf("connection error: %w", err)
	}
	defer client.Close()

	task.mu.Lock()
	task.totalFiles = 1
	task.fileIndex.Store(0)
	task.Status = StatusDownloading
	task.mu.Unlock()

	var offset int64 = 0
	if fi, err := os.Stat(localFilePath); err == nil {
		localSize := fi.Size()
		if task.TotalBytes > 0 && localSize > 0 && localSize < task.TotalBytes {
			offset = localSize
			log.Printf("[Downloader] [%s] Found partial file on disk (%s / %s). Resuming from offset...",
				task.GameTitle, FormatBytes(offset), FormatBytes(task.TotalBytes))
			task.DownloadedBytes.Store(offset)
			task.lastBytes = offset
		} else if localSize > task.TotalBytes {
			_ = os.Truncate(localFilePath, 0)
			offset = 0
		}
	} else {
		log.Printf("[Downloader] [%s] Starting single-stream download: %s (%s)",
			task.GameTitle, filepath.Base(task.RemotePath), FormatBytes(task.TotalBytes))
	}

	err := downloadFileWithRetry(ctx, task, client, srvCfg, task.RemotePath, localFilePath, task.TotalBytes, offset, limiter)
	if err == nil {
		removeSegmentState(localFilePath)
	}
	return err
}

// downloadFileWithRetry performs file transfer with automatic reconnect, exponential backoff, and dead-connection recovery
func downloadFileWithRetry(
	ctx context.Context,
	task *DownloadTask,
	client remote.RemoteClient,
	srvCfg config.ServerConfig,
	remotePath string,
	localPath string,
	expectedSize int64,
	startOffset int64,
	limiter *RateLimiter,
) error {
	const maxRetries = 5
	var currentOffset = startOffset

	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := streamFileChunk(ctx, task, client, remotePath, localPath, expectedSize, currentOffset, limiter)
		if err == nil {
			return nil
		}

		if ctx.Err() != nil {
			log.Printf("[Downloader] [%s] Transfer aborted: context cancelled", task.GameTitle)
			return ctx.Err()
		}

		log.Printf("[Downloader] WARN: [%s] Transfer interrupted for \"%s\" (attempt %d/%d): %v",
			task.GameTitle, filepath.Base(localPath), attempt, maxRetries, err)

		// Calculate updated offset from partial file on disk
		if fi, sErr := os.Stat(localPath); sErr == nil {
			localSize := fi.Size()
			if expectedSize > 0 && localSize == expectedSize {
				return nil
			}
			if expectedSize > 0 && localSize > expectedSize {
				_ = os.Truncate(localPath, 0)
				currentOffset = 0
			} else {
				currentOffset = localSize
			}
			log.Printf("[Downloader] [%s] Saved chunk up to %s. Reconnecting socket in %v...",
				task.GameTitle, FormatBytes(currentOffset), time.Duration(attempt)*time.Second)
		}

		// Reconnect client for fresh socket on next retry attempt with exponential backoff
		_ = client.Close()
		time.Sleep(time.Duration(attempt) * time.Second)
		if cErr := client.Connect(srvCfg); cErr != nil {
			log.Printf("[Downloader] WARN: [%s] Reconnect failed (attempt %d/%d): %v", task.GameTitle, attempt, maxRetries, cErr)
			time.Sleep(1 * time.Second)
			_ = client.Connect(srvCfg)
		}
	}

	log.Printf("[Downloader] ERROR: [%s] Transfer failed after %d retries for \"%s\"",
		task.GameTitle, maxRetries, filepath.Base(localPath))
	return fmt.Errorf("transfer failed after %d retries for %s", maxRetries, filepath.Base(localPath))
}

// streamFileChunk reads from remote client and writes to local disk with zero-allocation buffer pooling,
// responsive context abort, and network deadlock watchdog
func streamFileChunk(
	ctx context.Context,
	task *DownloadTask,
	client remote.RemoteClient,
	remotePath string,
	localPath string,
	expectedSize int64,
	offset int64,
	limiter *RateLimiter,
) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	reader, _, err := client.OpenRead(remotePath, offset)
	if err != nil {
		return fmt.Errorf("failed to read remote file: %w", err)
	}
	defer reader.Close()

	// Instant context cancellation watch: tear down network socket immediately when paused/cancelled
	stopWatch := make(chan struct{})
	defer close(stopWatch)
	go func() {
		select {
		case <-ctx.Done():
			_ = reader.Close()
		case <-stopWatch:
		}
	}()

	deadliner, hasDeadline := reader.(interface{ SetDeadline(time.Time) error })

	var flags int
	if offset > 0 {
		flags = os.O_CREATE | os.O_WRONLY
	} else {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}

	outFile, err := os.OpenFile(localPath, flags, 0644)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer outFile.Close()

	if offset > 0 {
		if _, err := outFile.Seek(offset, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek local file: %w", err)
		}
	}

	// 512KB buffered disk writer to batch filesystem writes and optimize I/O throughput
	bufWriter := bufio.NewWriterSize(outFile, 512*1024)
	defer bufWriter.Flush()

	var src io.Reader = reader
	if expectedSize > 0 && expectedSize > offset {
		src = io.LimitReader(reader, expectedSize-offset)
	}

	bufPtr := GetChunkBuffer()
	defer PutChunkBuffer(bufPtr)
	buf := *bufPtr

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if hasDeadline {
			_ = deadliner.SetDeadline(time.Now().Add(25 * time.Second))
		}

		n, rErr := src.Read(buf)
		if n > 0 {
			if limiter != nil {
				_ = limiter.Wait(ctx, int64(n))
			}

			if _, wErr := bufWriter.Write(buf[:n]); wErr != nil {
				return fmt.Errorf("failed to write local chunk: %w", wErr)
			}
			task.DownloadedBytes.Add(int64(n))
		}

		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			return fmt.Errorf("error reading remote stream: %w", rErr)
		}
	}

	return nil
}

package downloader

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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
			if f.SizeBytes > 0 && localSize >= f.SizeBytes {
				// Fully downloaded already
				initialDownloaded += f.SizeBytes
				continue
			} else if localSize > 0 && localSize < f.SizeBytes {
				// Partial file, resume from offset
				initialDownloaded += localSize
				queue = append(queue, fileWorkItem{remote: f, offset: localSize})
				continue
			}
		}
		queue = append(queue, fileWorkItem{remote: f, offset: 0})
	}

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
	limiter *RateLimiter,
) error {
	// 0. Ensure target directory exists and is not blocked by a regular file
	if err := ensureTargetDirectory(task); err != nil {
		return err
	}

	log.Printf("[Downloader] [%s] Connecting to %s server for monolithic file transfer...",
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

	localFilePath := task.LocalPath
	var offset int64 = 0
	if fi, err := os.Stat(localFilePath); err == nil {
		localSize := fi.Size()
		if task.TotalBytes > 0 && localSize >= task.TotalBytes {
			log.Printf("[Downloader] [%s] Single file already complete on disk (%s). Done.", task.GameTitle, FormatBytes(localSize))
			task.DownloadedBytes.Store(task.TotalBytes)
			task.mu.Lock()
			task.Status = StatusCompleted
			task.speedBytesPerSec.Store(0)
			task.smoothedSpeed = 0
			task.currentFile = ""
			task.mu.Unlock()
			return nil
		}
		offset = localSize
		log.Printf("[Downloader] [%s] Found partial file on disk (%s / %s). Resuming from offset...",
			task.GameTitle, FormatBytes(offset), FormatBytes(task.TotalBytes))
		task.DownloadedBytes.Store(offset)
		task.lastBytes = offset
	} else {
		log.Printf("[Downloader] [%s] Starting download: %s (%s)",
			task.GameTitle, filepath.Base(task.RemotePath), FormatBytes(task.TotalBytes))
	}

	return downloadFileWithRetry(ctx, task, client, srvCfg, task.RemotePath, localFilePath, task.TotalBytes, offset, limiter)
}

// downloadFileWithRetry performs file transfer with automatic reconnect and retries upon transient network drops
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
	const maxRetries = 4
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
			currentOffset = fi.Size()
			if expectedSize > 0 && currentOffset >= expectedSize {
				return nil
			}
			log.Printf("[Downloader] [%s] Saved chunk up to %s. Reconnecting socket in %v...",
				task.GameTitle, FormatBytes(currentOffset), time.Duration(attempt*500)*time.Millisecond)
		}

		// Reconnect client for fresh socket on next retry attempt
		_ = client.Close()
		time.Sleep(time.Duration(attempt*500) * time.Millisecond)
		_ = client.Connect(srvCfg)
	}

	log.Printf("[Downloader] ERROR: [%s] Transfer failed after %d retries for \"%s\"",
		task.GameTitle, maxRetries, filepath.Base(localPath))
	return fmt.Errorf("transfer failed after %d retries for %s", maxRetries, filepath.Base(localPath))
}

// streamFileChunk reads from remote client and writes to local disk with zero-allocation buffer pooling
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

		n, rErr := src.Read(buf)
		if n > 0 {
			if limiter != nil {
				_ = limiter.Wait(ctx, int64(n))
			}

			if _, wErr := outFile.Write(buf[:n]); wErr != nil {
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

package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gamevault/pkg/config"
	"gamevault/pkg/remote"
)

const (
	// MinSegmentedFileSize is the threshold to trigger multi-stream segmented download (32 MB)
	MinSegmentedFileSize = 32 * 1024 * 1024
	// SegmentStateExt is the extension for companion state files during segmented downloads
	SegmentStateExt = ".duckestate"
)

type segmentInfo struct {
	Index        int   `json:"index"`
	StartOffset  int64 `json:"startOffset"`
	EndOffset    int64 `json:"endOffset"`    // exclusive end
	CurrentBytes int64 `json:"currentBytes"` // downloaded bytes within this segment
}

type segmentedDownloadState struct {
	TotalBytes int64         `json:"totalBytes"`
	RemotePath string        `json:"remotePath"`
	Segments   []segmentInfo `json:"segments"`
}

func stateFilePath(localPath string) string {
	return localPath + SegmentStateExt
}

func loadSegmentState(localPath string, totalBytes int64, remotePath string, expectedSegments int) (*segmentedDownloadState, error) {
	spath := stateFilePath(localPath)
	data, err := os.ReadFile(spath)
	if err != nil {
		return nil, err
	}

	var state segmentedDownloadState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	if state.TotalBytes != totalBytes || state.RemotePath != remotePath || len(state.Segments) != expectedSegments {
		return nil, fmt.Errorf("state file parameter mismatch")
	}

	return &state, nil
}

func saveSegmentState(localPath string, state *segmentedDownloadState) error {
	spath := stateFilePath(localPath)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	tmpPath := spath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, spath)
}

func removeSegmentState(localPath string) {
	_ = os.Remove(stateFilePath(localPath))
}

// downloadSegmentedFile executes parallel chunked segmented download for large monolithic files
func downloadSegmentedFile(
	ctx context.Context,
	task *DownloadTask,
	srvCfg config.ServerConfig,
	maxConcurrentStreams int,
	limiter *RateLimiter,
) error {
	totalBytes := task.TotalBytes
	if totalBytes < MinSegmentedFileSize {
		return fmt.Errorf("file size (%d) below segmented threshold", totalBytes)
	}

	concurrency := maxConcurrentStreams
	if concurrency < 2 {
		concurrency = 4
	} else if concurrency > 8 {
		concurrency = 8
	}

	localPath := task.LocalPath
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 1. Check or initialize segment state
	state, err := loadSegmentState(localPath, totalBytes, task.RemotePath, concurrency)
	var initialDownloaded int64 = 0

	if err == nil && state != nil {
		for _, seg := range state.Segments {
			initialDownloaded += seg.CurrentBytes
		}
		log.Printf("[Downloader] [%s] Resuming segmented transfer (%d streams): %s / %s already downloaded",
			task.GameTitle, concurrency, FormatBytes(initialDownloaded), FormatBytes(totalBytes))
	} else {
		// New transfer or invalid state: pre-allocate destination file
		outFile, cErr := os.OpenFile(localPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
		if cErr != nil {
			return fmt.Errorf("failed to create target file: %w", cErr)
		}
		// Pre-allocate full size to avoid filesystem fragmentation and ensure disk space
		if tErr := outFile.Truncate(totalBytes); tErr != nil {
			log.Printf("[Downloader] WARN: [%s] File pre-allocation failed: %v", task.GameTitle, tErr)
		}
		_ = outFile.Close()

		segSize := totalBytes / int64(concurrency)
		segments := make([]segmentInfo, concurrency)
		for i := 0; i < concurrency; i++ {
			sStart := int64(i) * segSize
			sEnd := int64(i+1) * segSize
			if i == concurrency-1 {
				sEnd = totalBytes
			}
			segments[i] = segmentInfo{
				Index:        i,
				StartOffset:  sStart,
				EndOffset:    sEnd,
				CurrentBytes: 0,
			}
		}

		state = &segmentedDownloadState{
			TotalBytes: totalBytes,
			RemotePath: task.RemotePath,
			Segments:   segments,
		}
		_ = saveSegmentState(localPath, state)

		log.Printf("[Downloader] [%s] Starting multi-segment transfer: %d parallel streams (Chunk ~%s)",
			task.GameTitle, concurrency, FormatBytes(segSize))
	}

	task.mu.Lock()
	task.DownloadedBytes.Store(initialDownloaded)
	task.lastBytes = initialDownloaded
	task.totalFiles = 1
	task.fileIndex.Store(0)
	task.Status = StatusDownloading
	task.mu.Unlock()

	// 2. Open target file in shared random-access write mode
	destFile, err := os.OpenFile(localPath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for segmented writing: %w", err)
	}
	defer destFile.Close()

	// 3. Worker pool executing segments
	var wg sync.WaitGroup
	var workerErr error
	var errOnce sync.Once
	var fallbackNeeded atomic.Bool
	var stateMu sync.Mutex

	// Periodic state persistence loop
	stopStateTicker := make(chan struct{})
	defer close(stopStateTicker)
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopStateTicker:
				return
			case <-ticker.C:
				stateMu.Lock()
				_ = saveSegmentState(localPath, state)
				stateMu.Unlock()
			}
		}
	}()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(segIndex int) {
			defer wg.Done()

			seg := &state.Segments[segIndex]
			segRemaining := (seg.EndOffset - seg.StartOffset) - seg.CurrentBytes
			if segRemaining <= 0 {
				return
			}

			client := remote.NewRemoteClient(srvCfg.Protocol)
			if cErr := client.Connect(srvCfg); cErr != nil {
				log.Printf("[Downloader] ERROR: [%s] Segment stream #%d connection failed: %v",
					task.GameTitle, segIndex+1, cErr)
				errOnce.Do(func() { workerErr = cErr })
				return
			}
			defer client.Close()

			segOffset := seg.StartOffset + seg.CurrentBytes
			err := downloadSegmentRange(
				ctx,
				task,
				client,
				srvCfg,
				destFile,
				task.RemotePath,
				segOffset,
				seg.EndOffset,
				seg,
				limiter,
				&fallbackNeeded,
				&stateMu,
			)

			if err != nil && !fallbackNeeded.Load() {
				errOnce.Do(func() { workerErr = err })
			}
		}(i)
	}

	wg.Wait()

	if fallbackNeeded.Load() {
		return errServerNoRest
	}

	if workerErr != nil {
		return workerErr
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Completed successfully: remove state companion file
	removeSegmentState(localPath)
	return nil
}

var errServerNoRest = errors.New("server does not support REST command for range transfer")

func downloadSegmentRange(
	ctx context.Context,
	task *DownloadTask,
	client remote.RemoteClient,
	srvCfg config.ServerConfig,
	destFile *os.File,
	remotePath string,
	startOffset int64,
	endOffset int64,
	seg *segmentInfo,
	limiter *RateLimiter,
	fallbackFlag *atomic.Bool,
	stateMu *sync.Mutex,
) error {
	const maxSegmentRetries = 5
	currentOffset := startOffset

	for attempt := 1; attempt <= maxSegmentRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if fallbackFlag.Load() {
			return errServerNoRest
		}

		remaining := endOffset - currentOffset
		if remaining <= 0 {
			return nil
		}

		err := streamSegmentChunk(ctx, task, client, destFile, remotePath, currentOffset, remaining, seg, limiter, stateMu)
		if err == nil {
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Detect if server rejected REST/offset command (e.g., 500/504)
		errStr := strings.ToLower(err.Error())
		if currentOffset > 0 && (strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || strings.Contains(errStr, "not supported")) {
			log.Printf("[Downloader] WARN: [%s] Server rejected offset transfer at byte %d. Flagging fallback...",
				task.GameTitle, currentOffset)
			fallbackFlag.Store(true)
			return errServerNoRest
		}

		log.Printf("[Downloader] WARN: [%s] Segment #%d stream dropped at offset %s (attempt %d/%d): %v",
			task.GameTitle, seg.Index+1, FormatBytes(currentOffset), attempt, maxSegmentRetries, err)

		_ = client.Close()
		time.Sleep(time.Duration(attempt) * time.Second)
		if cErr := client.Connect(srvCfg); cErr != nil {
			time.Sleep(1 * time.Second)
			_ = client.Connect(srvCfg)
		}

		// Recalculate offset from segment tracker
		stateMu.Lock()
		currentOffset = seg.StartOffset + seg.CurrentBytes
		stateMu.Unlock()
	}

	return fmt.Errorf("segment #%d failed after %d retries", seg.Index+1, maxSegmentRetries)
}

func streamSegmentChunk(
	ctx context.Context,
	task *DownloadTask,
	client remote.RemoteClient,
	destFile *os.File,
	remotePath string,
	offset int64,
	length int64,
	seg *segmentInfo,
	limiter *RateLimiter,
	stateMu *sync.Mutex,
) error {
	reader, _, err := client.OpenRead(remotePath, offset)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Instant context cancellation watch
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
	limitReader := io.LimitReader(reader, length)

	bufPtr := GetChunkBuffer()
	defer PutChunkBuffer(bufPtr)
	buf := *bufPtr

	currentWriteOffset := offset

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if hasDeadline {
			_ = deadliner.SetDeadline(time.Now().Add(25 * time.Second))
		}

		n, rErr := limitReader.Read(buf)
		if n > 0 {
			if limiter != nil {
				_ = limiter.Wait(ctx, int64(n))
			}

			// Thread-safe random access write into pre-allocated file
			if _, wErr := destFile.WriteAt(buf[:n], currentWriteOffset); wErr != nil {
				return fmt.Errorf("disk write failed at offset %d: %w", currentWriteOffset, wErr)
			}

			task.DownloadedBytes.Add(int64(n))
			currentWriteOffset += int64(n)

			stateMu.Lock()
			seg.CurrentBytes += int64(n)
			stateMu.Unlock()
		}

		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			return fmt.Errorf("stream read error: %w", rErr)
		}
	}

	return nil
}

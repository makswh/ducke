package downloader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gamevault/pkg/database"
	"gamevault/pkg/downloader/ddl"
)

// executeDDLDownload manages the full download lifecycle for direct file hoster downloads
func (dm *DownloadManager) executeDDLDownload(task *DownloadTask) {
	log.Printf("[Downloader] [%s] Starting DDL file hoster transfer (Host: %s, URIs: %d)...",
		task.GameTitle, task.DDLHost, len(task.URIs))

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
			IsTorrent:       false,
			IsDDL:           true,
			DDLHost:         task.DDLHost,
			CurrentPart:     int(task.CurrentPart.Load()),
			TotalParts:      task.TotalParts,
		})

		dm.emitTaskEvent(task)
		dm.notifyListChanged()
		dm.processQueue() // Auto-start next queued game
	}()

	// 1. Build multi-part and mirror download plan
	grouper := ddl.NewGrouper("")
	plan, err := grouper.BuildPlan(task.URIs, task.DDLHost)
	if err != nil {
		task.mu.Lock()
		task.Status = StatusFailed
		if task.MagnetURI != "" {
			task.ErrorMessage = fmt.Sprintf("Не удалось запустить DDL (%v). Доступна альтернативная загрузка через торрент.", err)
		} else {
			task.ErrorMessage = fmt.Sprintf("failed to build download plan: %v", err)
		}
		task.mu.Unlock()
		return
	}

	task.mu.Lock()
	task.TotalParts = len(plan.Parts)
	task.totalFiles = len(plan.Parts)
	task.DDLHost = plan.SelectedHoster
	task.Status = StatusDownloading
	task.mu.Unlock()

	if err := os.MkdirAll(task.LocalPath, 0755); err != nil {
		task.mu.Lock()
		task.Status = StatusFailed
		task.ErrorMessage = fmt.Sprintf("failed to create destination folder: %v", err)
		task.mu.Unlock()
		return
	}

	var downloadedParts []string

	// 2. Download all parts in sequence
	for _, part := range plan.Parts {
		select {
		case <-ctx.Done():
			return
		default:
		}

		task.CurrentPart.Store(int32(part.PartIndex))
		task.fileIndex.Store(int32(part.PartIndex))

		// Try mirrors for this part (selected mirror first, then remaining mirrors as failover)
		mirrors := orderPartMirrors(part, task.DDLHost)
		var partErr error
		var partSavedPath string

		for _, mirror := range mirrors {
			select {
			case <-ctx.Done():
				return
			default:
			}

			log.Printf("[Downloader] [%s] Resolving Part %d/%d from %s (%s)...",
				task.GameTitle, part.PartIndex, task.TotalParts, mirror.Hoster, mirror.OriginalURL)

			resolved, rErr := grouper.ResolvePartDirectURL(ctx, mirror)
			if rErr != nil {
				log.Printf("[Downloader] WARN: [%s] Failed to resolve %s: %v. Trying next mirror...",
					task.GameTitle, mirror.Hoster, rErr)
				partErr = rErr
				continue
			}

			targetFilePath := filepath.Join(task.LocalPath, resolved.Filename)
			task.mu.Lock()
			task.currentFile = resolved.Filename
			task.mu.Unlock()
			dm.emitTaskEvent(task)

			dErr := dm.downloadSingleDDLFile(ctx, task, resolved, targetFilePath)
			if dErr != nil {
				if ctx.Err() != nil {
					// User paused or cancelled
					return
				}
				log.Printf("[Downloader] WARN: [%s] Download failed from %s: %v. Trying next mirror...",
					task.GameTitle, mirror.Hoster, dErr)
				partErr = dErr
				continue
			}

			// Successfully downloaded part!
			partSavedPath = targetFilePath
			partErr = nil
			break
		}

		if partErr != nil {
			task.mu.Lock()
			task.Status = StatusFailed
			if task.MagnetURI != "" {
				task.ErrorMessage = fmt.Sprintf("Часть %d не загружена (зеркала недоступны). Доступна альтернативная загрузка через торрент.", part.PartIndex)
			} else {
				task.ErrorMessage = fmt.Sprintf("part %d failed: %v", part.PartIndex, partErr)
			}
			task.mu.Unlock()
			return
		}

		downloadedParts = append(downloadedParts, partSavedPath)
	}

	// 3. All parts completed successfully!
	task.mu.Lock()
	task.Status = StatusCompleted
	task.mu.Unlock()
	log.Printf("[Downloader] [%s] All %d parts successfully downloaded to %s",
		task.GameTitle, task.TotalParts, task.LocalPath)

	// 4. Automatic extraction if configured
	settings := dm.cfgManager.GetSettings()
	if settings.AutoExtractArchives && len(downloadedParts) > 0 {
		firstPart := downloadedParts[0]
		// Search for .part1.rar or the first archive file
		for _, p := range downloadedParts {
			lower := strings.ToLower(p)
			if strings.Contains(lower, ".part1.rar") || strings.Contains(lower, ".part01.rar") {
				firstPart = p
				break
			}
		}
		go func(archivePath, destDir string) {
			// Small grace period for Windows filesystem handles and AV scanners to settle
			time.Sleep(500 * time.Millisecond)
			if eErr := ExtractArchive(archivePath, destDir); eErr != nil {
				log.Printf("[Downloader] [%s] Auto-extract notice: %v", task.GameTitle, eErr)
			}
		}(firstPart, task.LocalPath)
	}
}

// orderPartMirrors places preferred hoster mirror first, followed by others ordered by rank
func orderPartMirrors(part ddl.PartPlan, preferredHost string) []*ddl.ResolvedItem {
	var ordered []*ddl.ResolvedItem
	var primary *ddl.ResolvedItem

	for _, m := range part.Mirrors {
		if strings.EqualFold(m.Hoster, preferredHost) {
			primary = m
			break
		}
	}

	if primary != nil {
		ordered = append(ordered, primary)
	}

	for _, m := range part.Mirrors {
		if m != primary {
			ordered = append(ordered, m)
		}
	}

	return ordered
}

// downloadSingleDDLFile downloads a single resolved direct URL, supporting chunked multi-stream Range or single-stream
func (dm *DownloadManager) downloadSingleDDLFile(
	ctx context.Context,
	task *DownloadTask,
	resolved *ddl.ResolvedItem,
	localPath string,
) error {
	client := &http.Client{
		Timeout: 0, // No hard overall timeout; chunks are streaming
	}

	// Check existing file
	if fi, err := os.Stat(localPath); err == nil && resolved.SizeBytes > 0 && fi.Size() == resolved.SizeBytes {
		// File already exists and matches size, check if state file is absent (finished)
		if _, sErr := os.Stat(stateFilePath(localPath)); os.IsNotExist(sErr) {
			if isValidDownloadedFile(localPath) {
				log.Printf("[Downloader] [%s] File \"%s\" already completely downloaded, skipping",
					task.GameTitle, resolved.Filename)
				return nil
			}
			log.Printf("[Downloader] [%s] File \"%s\" size matches but header is invalid/corrupt, re-downloading...",
				task.GameTitle, resolved.Filename)
			_ = os.Remove(localPath)
		}
	}

	// Check Range support via HEAD / probe request
	supportsRange := false
	probeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved.DirectURL, nil)
	if err == nil {
		for k, v := range resolved.Headers {
			probeReq.Header.Set(k, v)
		}
		probeReq.Header.Set("Range", "bytes=0-0")
		if probeResp, pErr := client.Do(probeReq); pErr == nil {
			_ = probeResp.Body.Close()
			if probeResp.StatusCode == http.StatusPartialContent {
				supportsRange = true
			}
		}
	}

	// File hosters that limit concurrent connections per IP (PixelDrain, 1fichier, GoFile)
	// should strictly use single-stream with Range resume to avoid triggering 429 / max_concurrent_downloads.
	if resolved.Hoster == ddl.HosterPixelDrain ||
		resolved.Hoster == ddl.Hoster1fichier ||
		resolved.Hoster == ddl.HosterGoFile {
		return dm.downloadDDLStream(ctx, task, resolved, localPath, supportsRange)
	}

	// If large file (> 16MB) and supports Range: execute multi-stream segmented download
	if supportsRange && resolved.SizeBytes > 16*1024*1024 {
		concurrency := 4
		return dm.downloadDDLSegmented(ctx, task, resolved, localPath, concurrency)
	}

	// Otherwise execute streaming single download (with resume offset if possible)
	return dm.downloadDDLStream(ctx, task, resolved, localPath, supportsRange)
}

// downloadDDLSegmented executes multi-connection parallel chunk download using WriteAt
func (dm *DownloadManager) downloadDDLSegmented(
	ctx context.Context,
	task *DownloadTask,
	resolved *ddl.ResolvedItem,
	localPath string,
	concurrency int,
) error {
	totalBytes := resolved.SizeBytes

	// 1. Load or initialize segment state
	state, err := loadSegmentState(localPath, totalBytes, resolved.DirectURL, concurrency)
	var initialDownloaded int64 = 0

	if err == nil && state != nil {
		for _, seg := range state.Segments {
			initialDownloaded += seg.CurrentBytes
		}
		log.Printf("[Downloader] [%s] Resuming DDL segmented transfer (%d streams): %s / %s",
			task.GameTitle, concurrency, FormatBytes(initialDownloaded), FormatBytes(totalBytes))
	} else {
		// Pre-allocate target file
		outFile, cErr := os.OpenFile(localPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
		if cErr != nil {
			return fmt.Errorf("failed to create target file: %w", cErr)
		}
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
			RemotePath: resolved.DirectURL,
			Segments:   segments,
		}
		_ = saveSegmentState(localPath, state)
	}

	destFile, err := os.OpenFile(localPath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open target file for writing: %w", err)
	}
	defer destFile.Close()

	var wg sync.WaitGroup
	var workerErr error
	var errOnce sync.Once
	var stateMu sync.Mutex

	// Periodic state saver
	stopStateTicker := make(chan struct{})
	defer close(stopStateTicker)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
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

	client := &http.Client{Timeout: 0}

	for i := range state.Segments {
		seg := &state.Segments[i]
		if seg.CurrentBytes >= (seg.EndOffset - seg.StartOffset) {
			continue // Segment already finished
		}

		wg.Add(1)
		go func(s *segmentInfo) {
			defer wg.Done()

			currentOffset := s.StartOffset + s.CurrentBytes
			endOffset := s.EndOffset - 1

			req, rErr := http.NewRequestWithContext(ctx, http.MethodGet, resolved.DirectURL, nil)
			if rErr != nil {
				errOnce.Do(func() { workerErr = rErr })
				return
			}
			for k, v := range resolved.Headers {
				req.Header.Set(k, v)
			}
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", currentOffset, endOffset))

			resp, dErr := client.Do(req)
			if dErr != nil {
				if ctx.Err() == nil {
					errOnce.Do(func() { workerErr = dErr })
				}
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
				errOnce.Do(func() {
					workerErr = fmt.Errorf("server returned status %d for range %d-%d", resp.StatusCode, currentOffset, endOffset)
				})
				return
			}

			bufPtr := GetChunkBuffer()
			defer PutChunkBuffer(bufPtr)
			buf := *bufPtr

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				n, readErr := resp.Body.Read(buf)
				if n > 0 {
					// Guard against error payload returned inside chunk
					if s.Index == 0 && s.CurrentBytes == 0 {
						trimmed := bytes.TrimSpace(buf[:n])
						if bytes.HasPrefix(trimmed, []byte(`{"success":false`)) || bytes.HasPrefix(trimmed, []byte(`{"error":`)) {
							errOnce.Do(func() {
								workerErr = fmt.Errorf("server error response: %s", string(trimmed))
							})
							return
						}
					}

					if task.limiter != nil {
						task.limiter.Wait(ctx, int64(n))
					}

					// Thread-safe WriteAt to specific chunk offset
					if _, wErr := destFile.WriteAt(buf[:n], currentOffset); wErr != nil {
						errOnce.Do(func() { workerErr = wErr })
						return
					}

					currentOffset += int64(n)
					s.CurrentBytes += int64(n)
					task.DownloadedBytes.Add(int64(n))
				}

				if readErr != nil {
					if readErr == io.EOF {
						expectedSegBytes := s.EndOffset - s.StartOffset
						if s.CurrentBytes < expectedSegBytes {
							errOnce.Do(func() {
								workerErr = fmt.Errorf("segment %d stream prematurely ended: received %d of %d bytes: %w",
									s.Index, s.CurrentBytes, expectedSegBytes, io.ErrUnexpectedEOF)
							})
						}
						break
					}
					if ctx.Err() == nil {
						errOnce.Do(func() { workerErr = readErr })
					}
					return
				}
			}
		}(seg)
	}

	wg.Wait()

	stateMu.Lock()
	_ = saveSegmentState(localPath, state)
	stateMu.Unlock()

	var totalDownloaded int64
	for _, seg := range state.Segments {
		totalDownloaded += seg.CurrentBytes
	}
	if totalDownloaded < totalBytes && workerErr == nil {
		workerErr = fmt.Errorf("segmented transfer incomplete: downloaded %d of %d bytes", totalDownloaded, totalBytes)
	}

	_ = destFile.Sync()

	if workerErr != nil {
		return workerErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Clean up state file on successful completion
	removeSegmentState(localPath)
	return nil
}

// downloadDDLStream handles single-stream direct download with resume support
func (dm *DownloadManager) downloadDDLStream(
	ctx context.Context,
	task *DownloadTask,
	resolved *ddl.ResolvedItem,
	localPath string,
	supportsRange bool,
) error {
	var startOffset int64 = 0
	if supportsRange {
		if fi, err := os.Stat(localPath); err == nil && fi.Size() > 0 {
			if resolved.SizeBytes > 0 && fi.Size() < resolved.SizeBytes {
				// Verify existing portion is not corrupted/JSON error before resuming
				if isValidPartialDownload(localPath) {
					startOffset = fi.Size()
				} else {
					_ = os.Remove(localPath)
				}
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved.DirectURL, nil)
	if err != nil {
		return err
	}
	for k, v := range resolved.Headers {
		req.Header.Set(k, v)
	}
	if startOffset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startOffset))
	}

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		bodySnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodySnippet)))
	}

	var flags int
	if startOffset > 0 && resp.StatusCode == http.StatusPartialContent {
		flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	} else {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
		startOffset = 0
	}

	outFile, err := os.OpenFile(localPath, flags, 0644)
	if err != nil {
		return err
	}
	defer outFile.Close()

	bufPtr := GetChunkBuffer()
	defer PutChunkBuffer(bufPtr)
	buf := *bufPtr

	var currentBytesWritten int64 = startOffset
	checkedHeader := false

	for {
		select {
		case <-ctx.Done():
			_ = outFile.Sync()
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if !checkedHeader && startOffset == 0 {
				checkedHeader = true
				trimmed := bytes.TrimSpace(buf[:n])
				if bytes.HasPrefix(trimmed, []byte(`{"success":false`)) || bytes.HasPrefix(trimmed, []byte(`{"error":`)) {
					_ = outFile.Close()
					_ = os.Remove(localPath)
					return fmt.Errorf("server error response: %s", string(trimmed))
				}
			}

			if task.limiter != nil {
				task.limiter.Wait(ctx, int64(n))
			}

			if _, wErr := outFile.Write(buf[:n]); wErr != nil {
				return wErr
			}

			currentBytesWritten += int64(n)
			task.DownloadedBytes.Add(int64(n))
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}

	_ = outFile.Sync()

	// Verify total size if expected size was provided
	if resolved.SizeBytes > 0 && currentBytesWritten < resolved.SizeBytes {
		return fmt.Errorf("download prematurely ended: received %d of %d bytes: %w",
			currentBytesWritten, resolved.SizeBytes, io.ErrUnexpectedEOF)
	}

	return nil
}

func isValidDownloadedFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 64)
	n, err := f.Read(buf)
	if n < 4 {
		return false
	}

	trimmed := bytes.TrimSpace(buf[:n])
	// Reject JSON / HTML error responses
	if bytes.HasPrefix(trimmed, []byte(`{"`)) || bytes.HasPrefix(trimmed, []byte(`<!DOCTYPE`)) || bytes.HasPrefix(trimmed, []byte(`<html`)) {
		return false
	}

	// Format-specific signature checks
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".rar":
		// RAR 4.x or 5.x header: "Rar!\x1a\x07\x00" or "Rar!\x1a\x07\x01\x00"
		return bytes.HasPrefix(buf[:n], []byte("Rar!"))
	case ".zip":
		// ZIP header: "PK\x03\x04"
		return bytes.HasPrefix(buf[:n], []byte("PK\x03\x04"))
	case ".7z":
		// 7z header: 0x37 0x7A 0xBC 0xAF 0x27 0x1C
		return bytes.HasPrefix(buf[:n], []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C})
	}

	return true
}

func isValidPartialDownload(path string) bool {
	return isValidDownloadedFile(path)
}


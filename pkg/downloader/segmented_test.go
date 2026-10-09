package downloader

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"gamevault/pkg/config"
	"gamevault/pkg/remote"
)

// mockRemoteClient simulates an FTP/SFTP server in memory for testing
type mockRemoteClient struct {
	data          []byte
	supportResume bool
	connected     bool
}

func (m *mockRemoteClient) Connect(cfg config.ServerConfig) error {
	m.connected = true
	return nil
}

func (m *mockRemoteClient) Close() error {
	m.connected = false
	return nil
}

func (m *mockRemoteClient) ScanRepository(rootPath string) ([]remote.RemoteItem, error) {
	return nil, nil
}

func (m *mockRemoteClient) ScanRepositoryProgress(rootPath string, cb remote.ScanProgressCallback) ([]remote.RemoteItem, error) {
	return nil, nil
}

func (m *mockRemoteClient) ListDirectory(dirPath string) ([]remote.RemoteItem, error) {
	return nil, nil
}

func (m *mockRemoteClient) OpenRead(remoteFilePath string, offset int64) (io.ReadCloser, int64, error) {
	if offset > 0 && !m.supportResume {
		return nil, 0, fmt.Errorf("500 REST command not supported")
	}
	if offset < 0 || offset > int64(len(m.data)) {
		return nil, 0, fmt.Errorf("invalid offset %d", offset)
	}
	slice := m.data[offset:]
	return io.NopCloser(bytes.NewReader(slice)), int64(len(m.data)), nil
}

func (m *mockRemoteClient) GetFileSize(remoteFilePath string) (int64, error) {
	return int64(len(m.data)), nil
}

func (m *mockRemoteClient) ListFilesRecursive(dirPath string) ([]remote.RemoteFileEntry, error) {
	return []remote.RemoteFileEntry{
		{
			RelativePath: "testfile.bin",
			FullPath:     remoteFilePath(dirPath, "testfile.bin"),
			SizeBytes:    int64(len(m.data)),
		},
	}, nil
}

func remoteFilePath(dir, file string) string {
	return filepath.ToSlash(filepath.Join(dir, file))
}

func TestSegmentDivisionAndMath(t *testing.T) {
	totalBytes := int64(35 * 1024 * 1024) // 35 MB
	concurrency := 4
	segSize := totalBytes / int64(concurrency)

	var lastEnd int64 = 0
	for i := 0; i < concurrency; i++ {
		sStart := int64(i) * segSize
		sEnd := int64(i+1) * segSize
		if i == concurrency-1 {
			sEnd = totalBytes
		}

		if sStart != lastEnd {
			t.Fatalf("segment %d start %d does not match previous end %d", i, sStart, lastEnd)
		}
		if sEnd <= sStart {
			t.Fatalf("segment %d end %d <= start %d", i, sEnd, sStart)
		}
		lastEnd = sEnd
	}

	if lastEnd != totalBytes {
		t.Fatalf("expected last end to be %d, got %d", totalBytes, lastEnd)
	}
}

func TestSegmentStateLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "game.iso")
	totalBytes := int64(100 * 1024 * 1024)
	remotePath := "/games/game.iso"
	concurrency := 4

	state := &segmentedDownloadState{
		TotalBytes: totalBytes,
		RemotePath: remotePath,
		Segments: []segmentInfo{
			{Index: 0, StartOffset: 0, EndOffset: 25000000, CurrentBytes: 10000000},
			{Index: 1, StartOffset: 25000000, EndOffset: 50000000, CurrentBytes: 5000000},
			{Index: 2, StartOffset: 50000000, EndOffset: 75000000, CurrentBytes: 0},
			{Index: 3, StartOffset: 75000000, EndOffset: 100000000, CurrentBytes: 0},
		},
	}

	// 1. Save state
	if err := saveSegmentState(targetPath, state); err != nil {
		t.Fatalf("saveSegmentState failed: %v", err)
	}

	// 2. Load state
	loaded, err := loadSegmentState(targetPath, totalBytes, remotePath, concurrency)
	if err != nil {
		t.Fatalf("loadSegmentState failed: %v", err)
	}
	if loaded.TotalBytes != totalBytes {
		t.Fatalf("expected %d totalBytes, got %d", totalBytes, loaded.TotalBytes)
	}
	if loaded.Segments[0].CurrentBytes != 10000000 {
		t.Fatalf("expected 10000000 downloaded on seg 0, got %d", loaded.Segments[0].CurrentBytes)
	}

	// 3. Mismatch checks
	_, errMismatchSize := loadSegmentState(targetPath, totalBytes+100, remotePath, concurrency)
	if errMismatchSize == nil {
		t.Fatal("expected error on size mismatch, got nil")
	}

	// 4. Remove state
	removeSegmentState(targetPath)
	if _, errStat := os.Stat(stateFilePath(targetPath)); !os.IsNotExist(errStat) {
		t.Fatal("expected state file to be removed")
	}
}

func TestDownloadSegmentedFile_ByteAssembly(t *testing.T) {
	// Generate 34 MB of pseudo-random data (above MinSegmentedFileSize)
	dataSize := 34 * 1024 * 1024
	testData := make([]byte, dataSize)
	_, err := rand.Read(testData)
	if err != nil {
		t.Fatalf("failed to generate random data: %v", err)
	}

	mock := &mockRemoteClient{
		data:          testData,
		supportResume: true,
	}

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "assembly_test.bin")

	task := &DownloadTask{
		ID:         "test-seg-1",
		GameTitle:  "Assembly Test Game",
		RemotePath: "/test/file.bin",
		LocalPath:  localFile,
		TotalBytes: int64(dataSize),
		Status:     StatusQueued,
	}

	// Override RemoteClient factory for this test via custom wrapper or direct segment download
	// In downloadSegmentedFile, remote.NewRemoteClient is called. We test segment assembly directly
	// using downloadSegmentRange with the mock client across 4 workers:
	concurrency := 4
	segSize := int64(dataSize) / int64(concurrency)

	outFile, err := os.OpenFile(localFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer outFile.Close()
	_ = outFile.Truncate(int64(dataSize))

	state := &segmentedDownloadState{
		TotalBytes: int64(dataSize),
		RemotePath: task.RemotePath,
		Segments:   make([]segmentInfo, concurrency),
	}
	for i := 0; i < concurrency; i++ {
		sStart := int64(i) * segSize
		sEnd := int64(i+1) * segSize
		if i == concurrency-1 {
			sEnd = int64(dataSize)
		}
		state.Segments[i] = segmentInfo{
			Index:        i,
			StartOffset:  sStart,
			EndOffset:    sEnd,
			CurrentBytes: 0,
		}
	}

	ctx := context.Background()

	var stateMu sync.Mutex

	// Run segment streams concurrently
	errChan := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func(segIdx int) {
			seg := &state.Segments[segIdx]
			err := streamSegmentChunk(
				ctx,
				task,
				mock,
				outFile,
				task.RemotePath,
				seg.StartOffset,
				seg.EndOffset-seg.StartOffset,
				seg,
				nil,
				&stateMu,
			)
			errChan <- err
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		if sErr := <-errChan; sErr != nil {
			t.Fatalf("worker %d failed: %v", i, sErr)
		}
	}

	// Verify byte-level equality
	savedData, err := os.ReadFile(localFile)
	if err != nil {
		t.Fatalf("failed to read assembled file: %v", err)
	}
	if len(savedData) != len(testData) {
		t.Fatalf("assembled file size %d != test data size %d", len(savedData), len(testData))
	}
	if !bytes.Equal(savedData, testData) {
		t.Fatal("assembled file content does not match source byte for byte!")
	}
}

func TestDownloadSegmentRange_FallbackOnNoREST(t *testing.T) {
	mockNoResume := &mockRemoteClient{
		data:          make([]byte, 1024*1024),
		supportResume: false,
	}

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "fallback_test.bin")
	outFile, err := os.OpenFile(localFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer outFile.Close()

	task := &DownloadTask{
		ID:         "test-fallback-1",
		GameTitle:  "Fallback Game",
		RemotePath: "/test/file.bin",
		LocalPath:  localFile,
		TotalBytes: 1024 * 1024,
		Status:     StatusDownloading,
	}

	seg := &segmentInfo{
		Index:        1,
		StartOffset:  512 * 1024,
		EndOffset:    1024 * 1024,
		CurrentBytes: 0,
	}

	var fallbackFlag atomic.Bool
	var stateMu sync.Mutex

	err = downloadSegmentRange(
		context.Background(),
		task,
		mockNoResume,
		config.ServerConfig{Protocol: "ftp"},
		outFile,
		task.RemotePath,
		seg.StartOffset,
		seg.EndOffset,
		seg,
		nil,
		&fallbackFlag,
		&stateMu,
	)

	if err != errServerNoRest {
		t.Fatalf("expected errServerNoRest on unsupported REST, got %v", err)
	}
	if !fallbackFlag.Load() {
		t.Fatal("expected fallbackFlag to be true")
	}
}

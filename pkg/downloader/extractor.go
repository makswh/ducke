package downloader

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ExtractArchive attempts to extract a downloaded archive (.zip, .rar, .7z)
func ExtractArchive(firstFilePath string, destDir string) error {
	ext := strings.ToLower(filepath.Ext(firstFilePath))
	base := strings.ToLower(filepath.Base(firstFilePath))

	log.Printf("[Extractor] Starting extraction for %s into %s", firstFilePath, destDir)

	if ext == ".zip" {
		return extractZip(firstFilePath, destDir)
	}

	// For .rar, .7z, or multi-part archives (.part1.rar, .001)
	if ext == ".rar" || ext == ".7z" || strings.Contains(base, ".part1.rar") || strings.Contains(base, ".part01.rar") || ext == ".001" {
		return extractExternalArchive(firstFilePath, destDir)
	}

	return fmt.Errorf("unsupported archive format for %s", firstFilePath)
}

func extractZip(zipPath string, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		targetPath := filepath.Join(destDir, f.Name)
		// Guard against ZipSlip vulnerability
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(destDir)) {
			continue
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(targetPath, f.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// findExtractorBinary searches for 7z or unrar executable
func findExtractorBinary() (string, []string) {
	// 1. Look in PATH
	candidates := []string{"7z", "7za", "unrar", "rar"}
	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil {
			if strings.Contains(strings.ToLower(c), "7z") {
				return path, []string{"x", "-y"}
			}
			return path, []string{"x", "-y"}
		}
	}

	// 2. Windows specific standard locations
	if runtime.GOOS == "windows" {
		winCandidates := []struct {
			path string
			args []string
		}{
			{`C:\Program Files\7-Zip\7z.exe`, []string{"x", "-y"}},
			{`C:\Program Files (x86)\7-Zip\7z.exe`, []string{"x", "-y"}},
			{`C:\Program Files\WinRAR\UnRAR.exe`, []string{"x", "-y"}},
			{`C:\Program Files\WinRAR\WinRAR.exe`, []string{"x", "-ibck", "-y"}},
		}
		for _, wc := range winCandidates {
			if _, err := os.Stat(wc.path); err == nil {
				return wc.path, wc.args
			}
		}
	}

	return "", nil
}

func extractExternalArchive(archivePath string, destDir string) error {
	bin, defaultArgs := findExtractorBinary()
	if bin == "" {
		log.Printf("[Extractor] No 7-Zip or UnRAR binary found on system. Archive left as is: %s", archivePath)
		return nil
	}

	// 1. Wait for file handle release (prevent Windows sharing violation with AV scanners / disk flushes)
	for attempt := 0; attempt < 10; attempt++ {
		f, err := os.OpenFile(archivePath, os.O_RDONLY, 0)
		if err == nil {
			_ = f.Close()
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	args := append([]string{}, defaultArgs...)
	args = append(args, fmt.Sprintf("-o%s", destDir), archivePath)

	var output []byte
	var err error
	for retry := 1; retry <= 3; retry++ {
		cmd := exec.Command(bin, args...)
		output, err = cmd.CombinedOutput()
		if err == nil {
			break
		}
		outStr := string(output)
		if strings.Contains(outStr, "занят другим процессом") ||
			strings.Contains(strings.ToLower(outStr), "used by another process") ||
			strings.Contains(strings.ToLower(outStr), "sharing violation") {
			log.Printf("[Extractor] Archive %s temporarily locked by another process (attempt %d/3), retrying...", archivePath, retry)
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}

	if err != nil {
		log.Printf("[Extractor] Extraction command (%s) returned error: %v, output: %s", bin, err, string(output))
		return fmt.Errorf("archive extraction failed: %w", err)
	}

	log.Printf("[Extractor] Successfully extracted %s", archivePath)
	return nil
}

package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	DefaultGitHubRepo = "makswh/ducke"
	DefaultTimeout    = 30 * time.Second
)

// GitHubRelease represents relevant fields from GitHub /releases/latest endpoint
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt time.Time     `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Assets      []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
}

// ProgressCallback is invoked as chunks of update file arrive
type ProgressCallback func(p UpdateProgress)

// Updater manages checking, downloading, and applying updates
type Updater struct {
	Repo           string
	CurrentVersion string
	AppDataDir     string
	HTTPClient     *http.Client

	mu               sync.Mutex
	status           UpdateStatus
	latestDownloaded string // Path to downloaded asset
	cancelDownload   context.CancelFunc
}

// NewUpdater initializes updater instance
func NewUpdater(repo, currentVersion, appDataDir string) *Updater {
	if repo == "" {
		repo = DefaultGitHubRepo
	}
	return &Updater{
		Repo:           repo,
		CurrentVersion: currentVersion,
		AppDataDir:     appDataDir,
		HTTPClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		status: UpdateStatus{
			State: "idle",
		},
	}
}

// GetStatus returns the current status
func (u *Updater) GetStatus() UpdateStatus {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.status
}

// CheckForUpdates queries GitHub API for the latest release
func (u *Updater) CheckForUpdates(ctx context.Context) (*UpdateInfo, error) {
	u.mu.Lock()
	u.status = UpdateStatus{State: "checking"}
	u.mu.Unlock()

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", u.Repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		u.setError(err.Error())
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", fmt.Sprintf("Ducke-App/%s (%s; %s)", u.CurrentVersion, runtime.GOOS, runtime.GOARCH))

	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		u.setError(fmt.Sprintf("Ошибка связи с GitHub: %v", err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		u.mu.Lock()
		u.status = UpdateStatus{State: "up_to_date"}
		u.mu.Unlock()
		return &UpdateInfo{
			CurrentVersion: u.CurrentVersion,
			HasUpdate:      false,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("GitHub API вернул статус %d", resp.StatusCode)
		u.setError(err.Error())
		return nil, err
	}

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		u.setError(fmt.Sprintf("Не удалось разобрать ответ GitHub: %v", err))
		return nil, err
	}

	hasUpdate := IsNewerVersion(u.CurrentVersion, rel.TagName)

	// Locate appropriate asset for current OS/Arch
	matchedAsset := u.findMatchingAsset(rel.Assets)

	info := &UpdateInfo{
		CurrentVersion: u.CurrentVersion,
		LatestVersion:  rel.TagName,
		HasUpdate:      hasUpdate,
		ReleaseName:    rel.Name,
		ReleaseNotes:   rel.Body,
		ReleaseDate:    rel.PublishedAt,
		ReleaseURL:     rel.HTMLURL,
	}

	if matchedAsset != nil {
		info.DownloadURL = matchedAsset.BrowserDownloadURL
		info.AssetName = matchedAsset.Name
		info.AssetSize = matchedAsset.Size
	}

	u.mu.Lock()
	if hasUpdate {
		u.status = UpdateStatus{
			State:      "available",
			UpdateInfo: info,
		}
	} else {
		u.status = UpdateStatus{
			State:      "up_to_date",
			UpdateInfo: info,
		}
	}
	u.mu.Unlock()

	return info, nil
}

// findMatchingAsset selects binary/archive suited for this operating system
func (u *Updater) findMatchingAsset(assets []GitHubAsset) *GitHubAsset {
	osName := runtime.GOOS       // "windows", "linux"
	archName := runtime.GOARCH   // "amd64", "arm64"

	var preferredExtension string
	if osName == "windows" {
		preferredExtension = ".exe"
	}

	// 1. Try matching OS + Arch + Extension
	for _, a := range assets {
		lower := strings.ToLower(a.Name)
		if strings.Contains(lower, osName) && (strings.Contains(lower, archName) || strings.Contains(lower, "x64")) {
			return &a
		}
	}

	// 2. Try matching OS
	for _, a := range assets {
		lower := strings.ToLower(a.Name)
		if strings.Contains(lower, osName) {
			return &a
		}
	}

	// 3. Fallback for Windows: any .exe or .zip
	if osName == "windows" {
		for _, a := range assets {
			lower := strings.ToLower(a.Name)
			if strings.HasSuffix(lower, ".exe") || strings.HasSuffix(lower, ".zip") {
				return &a
			}
		}
	} else {
		// Fallback for Linux
		for _, a := range assets {
			lower := strings.ToLower(a.Name)
			if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".AppImage") || !strings.Contains(lower, ".") {
				return &a
			}
		}
	}

	if len(assets) > 0 && preferredExtension != "" {
		for _, a := range assets {
			if strings.HasSuffix(strings.ToLower(a.Name), preferredExtension) {
				return &a
			}
		}
	}

	if len(assets) > 0 {
		return &assets[0]
	}

	return nil
}

// DownloadUpdate downloads the release asset file to temporary updates directory
func (u *Updater) DownloadUpdate(ctx context.Context, onProgress ProgressCallback) (string, error) {
	u.mu.Lock()
	info := u.status.UpdateInfo
	if info == nil || info.DownloadURL == "" {
		u.mu.Unlock()
		return "", fmt.Errorf("нет ссылки для загрузки обновления")
	}

	ctx, cancel := context.WithCancel(ctx)
	u.cancelDownload = cancel
	u.status = UpdateStatus{
		State:      "downloading",
		UpdateInfo: info,
	}
	u.mu.Unlock()

	updatesDir := filepath.Join(u.AppDataDir, "updates")
	if err := os.MkdirAll(updatesDir, 0755); err != nil {
		u.setError(err.Error())
		return "", err
	}

	fileName := info.AssetName
	if fileName == "" {
		fileName = filepath.Base(info.DownloadURL)
		if fileName == "" || fileName == "/" || fileName == "." {
			if runtime.GOOS == "windows" {
				fileName = "Ducke_update.exe"
			} else {
				fileName = "Ducke_update"
			}
		}
	}

	destPath := filepath.Join(updatesDir, fileName)
	tempPath := destPath + ".tmp"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.DownloadURL, nil)
	if err != nil {
		u.setError(err.Error())
		return "", err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("Ducke-App/%s", u.CurrentVersion))

	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		u.setError(fmt.Sprintf("Ошибка скачивания: %v", err))
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("сервер вернул статус %d при загрузке", resp.StatusCode)
		u.setError(err.Error())
		return "", err
	}

	totalBytes := resp.ContentLength
	if totalBytes <= 0 {
		totalBytes = info.AssetSize
	}

	out, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		u.setError(err.Error())
		return "", err
	}
	defer out.Close()

	var downloaded int64
	buf := make([]byte, 64*1024) // 64KB chunk
	startTime := time.Now()
	lastReportTime := time.Now()
	var lastDownloaded int64

	for {
		select {
		case <-ctx.Done():
			_ = out.Close()
			_ = os.Remove(tempPath)
			u.setError("Загрузка отменена")
			return "", ctx.Err()
		default:
		}

		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := out.Write(buf[:n]); wErr != nil {
				u.setError(wErr.Error())
				return "", wErr
			}
			downloaded += int64(n)

			now := time.Now()
			if now.Sub(lastReportTime) >= 150*time.Millisecond || rErr != nil {
				deltaSec := now.Sub(lastReportTime).Seconds()
				var speed int64
				if deltaSec > 0 {
					speed = int64(float64(downloaded-lastDownloaded) / deltaSec)
				}
				lastReportTime = now
				lastDownloaded = downloaded

				var pct float64
				if totalBytes > 0 {
					pct = (float64(downloaded) / float64(totalBytes)) * 100
					if pct > 100 {
						pct = 100
					}
				}

				if onProgress != nil {
					onProgress(UpdateProgress{
						DownloadedBytes:  downloaded,
						TotalBytes:       totalBytes,
						Percent:          pct,
						SpeedBytesPerSec: speed,
						Done:             false,
					})
				}
			}
		}

		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			u.setError(rErr.Error())
			return "", rErr
		}
	}

	_ = out.Close()

	// Atomically move .tmp to destPath
	_ = os.Remove(destPath)
	if err := os.Rename(tempPath, destPath); err != nil {
		u.setError(err.Error())
		return "", err
	}

	u.mu.Lock()
	u.latestDownloaded = destPath
	u.status = UpdateStatus{
		State:      "ready_to_install",
		UpdateInfo: info,
	}
	u.mu.Unlock()

	if onProgress != nil {
		totalDuration := time.Since(startTime).Seconds()
		var avgSpeed int64
		if totalDuration > 0 {
			avgSpeed = int64(float64(downloaded) / totalDuration)
		}
		onProgress(UpdateProgress{
			DownloadedBytes:  downloaded,
			TotalBytes:       downloaded,
			Percent:          100.0,
			SpeedBytesPerSec: avgSpeed,
			Done:             true,
		})
	}

	return destPath, nil
}

func (u *Updater) setError(msg string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.status = UpdateStatus{
		State:        "error",
		ErrorMessage: msg,
		UpdateInfo:   u.status.UpdateInfo,
	}
}

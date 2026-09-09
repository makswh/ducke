package main

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gamevault/pkg/config"
	"gamevault/pkg/database"
	"gamevault/pkg/downloader"
	"gamevault/pkg/logger"
	"gamevault/pkg/metadata"
	"gamevault/pkg/remote"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx          context.Context
	db           *database.Database
	cfgManager   *config.ConfigManager
	steamService *metadata.SteamService
	downloader   *downloader.DownloadManager
	catalogMu    sync.Mutex
	appDataDir   string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// User data directory: AppData on Windows, ~/.config on Linux
	appDir, err := os.UserConfigDir()
	if err != nil {
		appDir = "."
	}
	appDataDir := filepath.Join(appDir, "Ducke")
	oldDir := filepath.Join(appDir, "GameVault")

	// Seamless migration from legacy GameVault folder if Ducke does not exist yet
	if _, err := os.Stat(appDataDir); os.IsNotExist(err) {
		if _, errOld := os.Stat(oldDir); errOld == nil {
			if renameErr := os.Rename(oldDir, appDataDir); renameErr != nil {
				appDataDir = oldDir
			}
		}
	}
	a.appDataDir = appDataDir

	appLogger := logger.InitLogger(2000)
	appLogger.SetHook(func(entry logger.LogEntry) {
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "log:entry", entry)
		}
	})

	cfgMgr, err := config.NewConfigManager(appDataDir)
	if err != nil {
		log.Fatalf("failed to init config manager: %v", err)
	}
	a.cfgManager = cfgMgr

	appLogger.SetEnabled(cfgMgr.GetSettings().EnableLogs)
	log.Printf("[System] Ducke started (AppData: %s, LogsEnabled: %v)", appDataDir, cfgMgr.GetSettings().EnableLogs)

	db, err := database.InitDB(appDataDir)
	if err != nil {
		log.Fatalf("failed to init database: %v", err)
	}
	a.db = db

	a.steamService = metadata.NewSteamService(db)

	// Initialize downloader and stream progress via Wails Events
	a.downloader = downloader.NewDownloadManager(db, cfgMgr, func(event downloader.DownloadProgressEvent) {
		wailsRuntime.EventsEmit(a.ctx, "download:progress", event)
	})

	// Auto-purge any historical corrupt matches where title similarity < 70%
	if purgedCount, err := a.db.PurgeMismatchedMetadata(metadata.CalculateTitleSimilarity, 0.70); err == nil && purgedCount > 0 {
		log.Printf("[Steam] Purged %d mismatched metadata records for fresh lookup", purgedCount)
	}

	// Reset any historical unmatched games (e.g. Abathor) so modern Steam Suggest engine processes them
	if resetCount, err := a.db.ResetUnmatchedGames(); err == nil && resetCount > 0 {
		log.Printf("[Steam] Re-queued %d unmatched games for enrichment with modern Suggest API", resetCount)
	}

	// Start progressive background enrichment worker
	a.steamService.StartBackgroundEnrichment(func(gameID int64, appID int) {
		if game, err := a.db.GetGameByID(gameID); err == nil {
			wailsRuntime.EventsEmit(a.ctx, "game:enriched", game)
		}
	})
}

func (a *App) shutdown(ctx context.Context) {
	if a.steamService != nil {
		a.steamService.StopBackgroundEnrichment()
	}
	if a.db != nil {
		_ = a.db.Close()
	}
}

// ==========================================
// Catalog & Games Methods
// ==========================================

func (a *App) GetCatalog(forceRefresh bool) ([]database.GameEntity, error) {
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()

	if forceRefresh {
		_, _ = a.db.PurgeMismatchedMetadata(metadata.CalculateTitleSimilarity, 0.70)
		_, _ = a.db.ResetUnmatchedGames()
	}

	games, err := a.db.GetAllGames()
	if err != nil {
		return nil, err
	}

	if len(games) > 0 && !forceRefresh {
		return games, nil
	}

	// Scan remote repository
	settings := a.cfgManager.GetSettings()
	if settings.ActiveServer == nil {
		return games, nil // Return cached games if no server configured yet
	}

	remoteDir := strings.TrimSpace(settings.ActiveServer.RemoteDir)
	if remoteDir == "/0" || remoteDir == "0" || remoteDir == "" {
		remoteDir = "/"
		settings.ActiveServer.RemoteDir = "/"
		_ = a.cfgManager.SaveSettings(settings)
	}

	log.Printf("[Remote] Connecting to %s server (%s:%d) to scan directory: \"%s\"...",
		strings.ToUpper(settings.ActiveServer.Protocol), settings.ActiveServer.Host, settings.ActiveServer.Port, remoteDir)

	client := remote.NewRemoteClient(settings.ActiveServer.Protocol)
	if err := client.Connect(*settings.ActiveServer); err != nil {
		log.Printf("[Remote] ERROR: Connection failed: %v", err)
		return games, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer client.Close()

	remoteItems, err := client.ScanRepository(remoteDir)
	if err != nil {
		log.Printf("[Remote] ERROR: Repository scan failed: %v", err)
		return games, fmt.Errorf("failed to scan repository %s: %w", remoteDir, err)
	}

	log.Printf("[Remote] Repository scan complete: found %d items", len(remoteItems))

	if err := a.db.UpsertGames(remoteItems); err != nil {
		log.Printf("[Remote] ERROR: Failed to cache games in database: %v", err)
		return nil, fmt.Errorf("failed to cache games: %w", err)
	}

	return a.db.GetAllGames()
}

type GamePageDetails struct {
	Game             database.GameEntity               `json:"game"`
	DownloadStatus   string                            `json:"downloadStatus"` // "none", "queued", "downloading", "paused", "completed", "failed"
	DownloadProgress *downloader.DownloadProgressEvent `json:"downloadProgress,omitempty"`
	LocalPath        string                            `json:"localPath,omitempty"`
	IsInstalled      bool                              `json:"isInstalled"`
	LogoURL          string                            `json:"logoUrl,omitempty"`
	BannerURL        string                            `json:"bannerUrl,omitempty"`
	CoverURL         string                            `json:"coverUrl,omitempty"`
	BackgroundURL    string                            `json:"backgroundUrl,omitempty"`
}

func (a *App) GetGameDetails(gameID int64) (*database.GameEntity, error) {
	return a.db.GetGameByID(gameID)
}

// GetGamePageDetails returns complete unified game page details including live download/installation state and resolved artwork
func (a *App) GetGamePageDetails(gameID int64) (*GamePageDetails, error) {
	game, err := a.db.GetGameByID(gameID)
	if err != nil {
		return nil, err
	}
	if game == nil {
		return nil, fmt.Errorf("game not found")
	}

	details := &GamePageDetails{
		Game:           *game,
		DownloadStatus: "none",
	}

	// 1. Check if game is in active download tasks
	if a.downloader != nil {
		tasks := a.downloader.GetTasks()
		for _, t := range tasks {
			if t.GameID == gameID {
				taskCopy := t
				details.DownloadProgress = &taskCopy
				details.DownloadStatus = string(t.Status)
				details.LocalPath = t.LocalPath
				break
			}
		}
	}

	// 2. If not active, check download history in database
	if details.DownloadProgress == nil {
		if rec, err := a.db.GetDownloadRecordByGameID(gameID); err == nil && rec != nil {
			details.DownloadStatus = rec.Status
			details.LocalPath = rec.LocalPath
		}
	}

	// 3. Check installation state on disk
	if details.LocalPath != "" {
		if fi, err := os.Stat(details.LocalPath); err == nil {
			details.IsInstalled = true
			if fi.IsDir() {
				if entries, err := os.ReadDir(details.LocalPath); err == nil && len(entries) > 0 {
					details.IsInstalled = true
				}
			}
		}
	} else {
		settings := a.cfgManager.GetSettings()
		candidates := []string{
			filepath.Join(settings.DownloadPath, game.CleanTitle),
			filepath.Join(settings.DownloadPath, game.RawName),
		}
		for _, c := range candidates {
			if fi, err := os.Stat(c); err == nil {
				if fi.IsDir() {
					if entries, err := os.ReadDir(c); err == nil && len(entries) > 0 {
						details.LocalPath = c
						details.IsInstalled = true
						break
					}
				} else {
					details.LocalPath = c
					details.IsInstalled = true
					break
				}
			}
		}
	}

	// 4. Fast local artwork resolution (zero network delay)
	if game.SteamAppID > 0 {
		details.LogoURL = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/logo.png", game.SteamAppID)
	}
	if game.HeaderImage != "" {
		details.BannerURL = game.HeaderImage
	}
	if game.CapsuleImage != "" {
		details.CoverURL = game.CapsuleImage
	}
	details.BackgroundURL = game.BackgroundImage

	// 5. Asynchronously fetch missing Steam reviews in the background if needed (non-blocking)
	if details.Game.SteamAppID > 0 && (details.Game.TotalReviews == 0 || details.Game.ReviewScoreDesc == "") {
		go func(gameID int64, appID int) {
			if a.steamService != nil {
				desc, pct, tot, pos := a.steamService.FetchSteamReviewSummary(appID)
				if tot > 0 {
					_ = a.db.UpdateSteamReviewSummary(appID, desc, pct, tot, pos)
					if a.ctx != nil {
						wailsRuntime.EventsEmit(a.ctx, "game:reviews-updated", map[string]interface{}{
							"gameId":          gameID,
							"steamAppId":      appID,
							"reviewScoreDesc": desc,
							"reviewPercent":   pct,
							"totalReviews":    tot,
							"positiveReviews": pos,
						})
					}
				}
			}
		}(game.ID, details.Game.SteamAppID)
	}

	return details, nil
}

// OpenGameFolder opens the directory where the game is downloaded
func (a *App) OpenGameFolder(gameID int64) error {
	details, err := a.GetGamePageDetails(gameID)
	if err != nil {
		return err
	}
	if details.LocalPath != "" {
		if fi, err := os.Stat(details.LocalPath); err == nil {
			if fi.IsDir() {
				return a.OpenLocalFolder(details.LocalPath)
			}
			return a.OpenLocalFolder(filepath.Dir(details.LocalPath))
		}
	}
	settings := a.cfgManager.GetSettings()
	return a.OpenLocalFolder(settings.DownloadPath)
}

// LaunchGameByGameID finds the downloaded game and launches its executable
func (a *App) LaunchGameByGameID(gameID int64) error {
	details, err := a.GetGamePageDetails(gameID)
	if err != nil {
		return err
	}
	if details.LocalPath != "" {
		return a.LaunchGame(details.LocalPath)
	}
	return fmt.Errorf("игра еще не установлена")
}

func (a *App) UpdateSteamAppID(gameID int64, appID int) error {
	if appID > 0 {
		_, _ = a.steamService.FetchAppDetails(appID)
		if err := a.db.SetGameAppID(gameID, appID); err != nil {
			return err
		}
	} else {
		if err := a.db.ResetGameMetadata(gameID); err != nil {
			return err
		}
	}

	if game, err := a.db.GetGameByID(gameID); err == nil {
		wailsRuntime.EventsEmit(a.ctx, "game:enriched", game)
	}
	return nil
}

func (a *App) ResetGameMetadata(gameID int64) error {
	if err := a.db.ResetGameMetadata(gameID); err != nil {
		return err
	}
	if game, err := a.db.GetGameByID(gameID); err == nil {
		wailsRuntime.EventsEmit(a.ctx, "game:enriched", game)
	}
	return nil
}

func (a *App) EnrichGameNow(gameID int64) (*database.GameEntity, error) {
	game, err := a.steamService.EnrichGame(gameID)
	if err == nil && game != nil {
		wailsRuntime.EventsEmit(a.ctx, "game:enriched", game)
	}
	return game, err
}

func (a *App) SearchSteamCandidates(query string) ([]metadata.SteamCandidate, error) {
	return a.steamService.SearchSteamCandidates(query)
}

// GetGameMovies retrieves fresh trailers (including modern HLS streams) for a given Steam AppID
func (a *App) GetGameMovies(appID int) ([]database.SteamMovie, error) {
	if appID <= 0 {
		return []database.SteamMovie{}, nil
	}
	meta, err := a.steamService.FetchAppDetails(appID)
	if err != nil {
		return []database.SteamMovie{}, err
	}
	if meta == nil {
		return []database.SteamMovie{}, nil
	}
	return meta.Movies, nil
}

type SteamSearchResult struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (a *App) SearchSteam(query string) ([]SteamSearchResult, error) {
	appID, name, err := a.steamService.SearchGame(query)
	if err != nil {
		return nil, err
	}
	if appID == 0 {
		return []SteamSearchResult{}, nil
	}
	return []SteamSearchResult{
		{ID: appID, Name: name},
	}, nil
}

// ResolveGameCover looks up a vertical 600x900 cover on SteamGridDB for games where Steam portrait cover is missing
func (a *App) ResolveGameCover(gameID int64, title string, steamAppID int) (string, error) {
	if a.steamService == nil {
		return "", fmt.Errorf("steam service not initialized")
	}

	coverURL, err := a.steamService.GetSteamGridCover(title)
	if err != nil || coverURL == "" {
		return "", err
	}

	// Update in database cache if steamAppID is valid
	if steamAppID != 0 && a.db != nil {
		_ = a.db.UpdateSteamMetadataCover(steamAppID, coverURL)
	}

	return coverURL, nil
}

// ResolveGameBanner returns a landscape or portrait banner/cover for download cards
func (a *App) ResolveGameBanner(gameID int64, title string, steamAppID int) (string, error) {
	if a.steamService == nil {
		return "", fmt.Errorf("steam service not initialized")
	}

	bannerURL, err := a.steamService.GetSteamGridBanner(title)
	if err == nil && bannerURL != "" {
		return bannerURL, nil
	}

	// Fallback to portrait cover
	return a.ResolveGameCover(gameID, title, steamAppID)
}

// ResolveGameLogo returns a transparent logo from SteamGridDB or Steam for the given game
func (a *App) ResolveGameLogo(gameID int64, title string, steamAppID int) (string, error) {
	if a.steamService == nil {
		return "", fmt.Errorf("steam service not initialized")
	}

	// 1. Try SteamGridDB logo first (SteamGridDB logos are high quality, transparent PNGs)
	logoURL, err := a.steamService.GetSteamGridLogo(title)
	if err == nil && logoURL != "" {
		return logoURL, nil
	}

	// 2. If steamAppID is present, fallback to Steam store logo
	if steamAppID > 0 {
		steamLogo := fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/logo.png", steamAppID)
		return steamLogo, nil
	}

	return "", fmt.Errorf("no logo found for %q", title)
}

// ExtractDominantColor extracts the most saturated, vibrant theme color from a game's cover image
func (a *App) ExtractDominantColor(imageURL string) string {
	if imageURL == "" {
		return "#66c0f4"
	}

	httpClient := &http.Client{Timeout: 4 * time.Second}
	resp, err := httpClient.Get(imageURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "#66c0f4"
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return "#66c0f4"
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return "#66c0f4"
	}

	var bestColor string
	var maxScore float64 = -1
	var rSum, gSum, bSum, count uint64

	stepX := w / 32
	if stepX < 1 {
		stepX = 1
	}
	stepY := h / 32
	if stepY < 1 {
		stepY = 1
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			r32, g32, b32, a32 := img.At(x, y).RGBA()
			if a32 < 0x8000 {
				continue
			}
			r := float64(r32 >> 8)
			g := float64(g32 >> 8)
			b := float64(b32 >> 8)

			brightness := (r*299 + g*587 + b*114) / 1000.0
			if brightness < 30 || brightness > 235 {
				continue
			}

			maxC := math.Max(r, math.Max(g, b))
			minC := math.Min(r, math.Min(g, b))
			delta := maxC - minC
			var saturation float64
			if maxC > 0 {
				saturation = delta / maxC
			}

			// Score favoring saturated, vivid colors for dark mode UI
			score := saturation*3.0 + (1.0 - math.Abs(brightness-130)/130.0)
			if score > maxScore && saturation > 0.20 {
				maxScore = score
				bestColor = fmt.Sprintf("rgb(%d, %d, %d)", int(r), int(g), int(b))
			}
			rSum += uint64(r)
			gSum += uint64(g)
			bSum += uint64(b)
			count++
		}
	}

	if maxScore > 0.5 && bestColor != "" {
		return bestColor
	}
	if count > 0 {
		return fmt.Sprintf("rgb(%d, %d, %d)", rSum/count, gSum/count, bSum/count)
	}
	return "#66c0f4"
}

// ==========================================
// Downloader Methods
// ==========================================

func (a *App) StartDownload(gameID int64, destinationPath string) (string, error) {
	return a.downloader.StartDownload(gameID, destinationPath)
}

func (a *App) PauseDownload(downloadID string) error {
	return a.downloader.PauseDownload(downloadID)
}

func (a *App) ResumeDownload(downloadID string) error {
	return a.downloader.ResumeDownload(downloadID)
}

func (a *App) CancelDownload(downloadID string) error {
	return a.downloader.CancelDownload(downloadID)
}

func (a *App) PauseAllDownloads() {
	a.downloader.PauseAll()
}

func (a *App) ResumeAllDownloads() {
	a.downloader.ResumeAll()
}

func (a *App) CancelAllDownloads() {
	a.downloader.CancelAll()
}

func (a *App) ClearCompletedDownloads() error {
	return a.downloader.ClearCompleted()
}

func (a *App) DeleteDownloadRecord(downloadID string, removeFiles bool) error {
	return a.downloader.DeleteTask(downloadID, removeFiles)
}

type DiskSpaceInfo struct {
	FreeBytes  int64  `json:"freeBytes"`
	TotalBytes int64  `json:"totalBytes"`
	FreeGB     string `json:"freeGB"`
	TotalGB    string `json:"totalGB"`
}

func (a *App) GetDiskSpaceInfo(path string) DiskSpaceInfo {
	if path == "" {
		path = a.cfgManager.GetSettings().DownloadPath
	}
	free, total, err := downloader.GetDiskSpace(path)
	if err != nil {
		return DiskSpaceInfo{FreeBytes: 0, TotalBytes: 0, FreeGB: "—", TotalGB: "—"}
	}
	return DiskSpaceInfo{
		FreeBytes:  free,
		TotalBytes: total,
		FreeGB:     fmt.Sprintf("%.1f GB", float64(free)/(1024*1024*1024)),
		TotalGB:    fmt.Sprintf("%.1f GB", float64(total)/(1024*1024*1024)),
	}
}

// GetStorageDrives discovers all available drives (internal, SD cards, USB) with capacity and usage
func (a *App) GetStorageDrives() []downloader.StorageDriveInfo {
	downloadPath := a.cfgManager.GetSettings().DownloadPath
	return downloader.GetSystemDrives(downloadPath)
}

func (a *App) GetDownloads() []downloader.DownloadProgressEvent {
	return a.downloader.GetTasks()
}

func (a *App) GetDownloadHistory() ([]database.DownloadRecord, error) {
	return a.db.GetAllDownloads()
}

// ==========================================
// Configuration & Settings Methods
// ==========================================

func (a *App) GetSettings() config.AppSettings {
	return a.cfgManager.GetSettings()
}

func (a *App) SaveSettings(settings config.AppSettings) error {
	oldSettings := a.cfgManager.GetSettings()
	if err := a.cfgManager.SaveSettings(settings); err != nil {
		return err
	}
	newSettings := a.cfgManager.GetSettings()

	// If speed limit changed, dynamically update active download tasks
	if oldSettings.MaxSpeedKBps != newSettings.MaxSpeedKBps && a.downloader != nil {
		a.downloader.UpdateSpeedLimit(newSettings.MaxSpeedKBps)
	}

	// Update logger state if logging setting changed
	if oldSettings.EnableLogs != newSettings.EnableLogs {
		logger.GetLogger().SetEnabled(newSettings.EnableLogs)
		if newSettings.EnableLogs {
			log.Println("[System] Diagnostics logging enabled by user")
		} else {
			log.Println("[System] Diagnostics logging disabled by user")
		}
	}

	wailsRuntime.EventsEmit(a.ctx, "settings:updated", newSettings)
	return nil
}

// UpdateSpeedLimit adjusts speed limit dynamically in config and active downloader
func (a *App) UpdateSpeedLimit(kbps int) error {
	settings := a.cfgManager.GetSettings()
	settings.MaxSpeedKBps = kbps
	return a.SaveSettings(settings)
}

// SetActiveServer switches active server by ID
func (a *App) SetActiveServer(serverID string) error {
	if err := a.cfgManager.SetActiveServer(serverID); err != nil {
		return err
	}
	settings := a.cfgManager.GetSettings()
	wailsRuntime.EventsEmit(a.ctx, "settings:updated", settings)
	return nil
}

// UpsertServer creates or updates a server entry
func (a *App) UpsertServer(srv config.ServerConfig) error {
	if err := a.cfgManager.UpsertServer(srv); err != nil {
		return err
	}
	settings := a.cfgManager.GetSettings()
	wailsRuntime.EventsEmit(a.ctx, "settings:updated", settings)
	return nil
}

// DeleteServer removes a server from saved list
func (a *App) DeleteServer(serverID string) error {
	if err := a.cfgManager.DeleteServer(serverID); err != nil {
		return err
	}
	settings := a.cfgManager.GetSettings()
	wailsRuntime.EventsEmit(a.ctx, "settings:updated", settings)
	return nil
}

// OpenConfigFolder opens the Ducke configuration/data directory in native explorer
func (a *App) OpenConfigFolder() error {
	path := a.appDataDir
	if path == "" {
		appDir, _ := os.UserConfigDir()
		path = filepath.Join(appDir, "Ducke")
	}
	return a.OpenLocalFolder(path)
}

// ClearMetadataCache resets unmatched and purged metadata cache
func (a *App) ClearMetadataCache() (int, error) {
	if a.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	purged, err := a.db.PurgeMismatchedMetadata(metadata.CalculateTitleSimilarity, 0.70)
	resetCount, _ := a.db.ResetUnmatchedGames()
	return purged + int(resetCount), err
}

func (a *App) ImportFileZillaXML(xmlContent string) ([]config.ServerConfig, error) {
	servers, err := config.ParseFileZillaXML([]byte(xmlContent))
	if err != nil {
		return nil, err
	}

	settings := a.cfgManager.GetSettings()
	settings.SavedServers = servers
	if len(servers) > 0 {
		settings.ActiveServer = &servers[0]
	}
	_ = a.SaveSettings(settings)

	return servers, nil
}

// ImportFileZillaFile opens native file dialog to select FileZilla XML and imports it
func (a *App) ImportFileZillaFile() ([]config.ServerConfig, error) {
	filePath, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Выберите XML файл FileZilla",
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "XML Files (*.xml)",
				Pattern:     "*.xml",
			},
			{
				DisplayName: "All Files (*.*)",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil || filePath == "" {
		return nil, err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return a.ImportFileZillaXML(string(content))
}

type ConnectionTestResult struct {
	Success       bool   `json:"success"`
	ProtocolUsed  string `json:"protocolUsed"`
	ErrorMessage  string `json:"errorMessage"`
}

func (a *App) TestConnection(cfg config.ServerConfig) ConnectionTestResult {
	log.Printf("[Remote] Testing %s connection to %s:%d (user: \"%s\")...",
		strings.ToUpper(cfg.Protocol), cfg.Host, cfg.Port, cfg.User)
	protocol, err := remote.TestServerConnection(cfg)
	if err != nil {
		log.Printf("[Remote] ERROR: Connection test failed: %v", err)
		return ConnectionTestResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}
	}
	log.Printf("[Remote] Connection test successful via %s", strings.ToUpper(protocol))
	return ConnectionTestResult{
		Success:      true,
		ProtocolUsed: protocol,
	}
}

// SelectDirectory opens native file explorer dialog to select a folder
func (a *App) SelectDirectory() (string, error) {
	selected, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Game Download Folder",
	})
	return selected, err
}

// OpenLocalFolder opens native file manager at folder path
func (a *App) OpenLocalFolder(folderPath string) error {
	if folderPath == "" {
		return fmt.Errorf("path is empty")
	}

	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", folderPath).Start()
	case "darwin":
		return exec.Command("open", folderPath).Start()
	default: // linux, steamOS
		return exec.Command("xdg-open", folderPath).Start()
	}
}

// LaunchGame attempts to launch the downloaded game executable or opens its folder
func (a *App) LaunchGame(folderOrFilePath string) error {
	if folderOrFilePath == "" {
		return fmt.Errorf("path is empty")
	}

	fi, err := os.Stat(folderOrFilePath)
	if err != nil {
		return err
	}

	// If it's directly an executable or script
	if !fi.IsDir() {
		ext := strings.ToLower(filepath.Ext(folderOrFilePath))
		if ext == ".exe" || ext == ".bat" || ext == ".cmd" {
			dir := filepath.Dir(folderOrFilePath)
			cmd := exec.Command(folderOrFilePath)
			cmd.Dir = dir
			return cmd.Start()
		}
		return a.OpenLocalFolder(filepath.Dir(folderOrFilePath))
	}

	// If it's a directory, search for executables
	entries, err := os.ReadDir(folderOrFilePath)
	if err == nil {
		var candidates []string
		for _, e := range entries {
			if !e.IsDir() {
				ext := strings.ToLower(filepath.Ext(e.Name()))
				if ext == ".exe" {
					nameLower := strings.ToLower(e.Name())
					// Exclude uninstaller, updater, crash handler tools
					if !strings.Contains(nameLower, "unins") &&
						!strings.Contains(nameLower, "crash") &&
						!strings.Contains(nameLower, "update") &&
						!strings.Contains(nameLower, "setup") {
						candidates = append(candidates, filepath.Join(folderOrFilePath, e.Name()))
					}
				}
			}
		}

		if len(candidates) == 1 {
			cmd := exec.Command(candidates[0])
			cmd.Dir = folderOrFilePath
			return cmd.Start()
		}
	}

	// Fallback to opening folder in explorer
	return a.OpenLocalFolder(folderOrFilePath)
}

// TriggerSteamOSKeyboard calls steam://open/keyboard on SteamOS
func (a *App) TriggerSteamOSKeyboard() {
	if runtime.GOOS == "linux" {
		_ = exec.Command("xdg-open", "steam://open/keyboard").Start()
	}
}

// IsLinuxSystem returns true if running on Linux or SteamOS
func (a *App) IsLinuxSystem() bool {
	return runtime.GOOS == "linux"
}

// FindLinuxInstaller searches for a .run file in the downloaded game directory or file
func (a *App) FindLinuxInstaller(folderOrFilePath string) string {
	if runtime.GOOS != "linux" || folderOrFilePath == "" {
		return ""
	}

	fi, err := os.Stat(folderOrFilePath)
	if err != nil {
		return ""
	}

	// 1. Directly a .run file
	if !fi.IsDir() {
		if strings.EqualFold(filepath.Ext(folderOrFilePath), ".run") {
			return folderOrFilePath
		}
		return ""
	}

	// 2. Directory: search root entries first
	entries, err := os.ReadDir(folderOrFilePath)
	if err != nil {
		return ""
	}

	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".run") {
			return filepath.Join(folderOrFilePath, e.Name())
		}
	}

	// 3. Search 1 subdirectory level deep (common when games are inside a subfolder)
	for _, e := range entries {
		if e.IsDir() {
			subPath := filepath.Join(folderOrFilePath, e.Name())
			if subEntries, sErr := os.ReadDir(subPath); sErr == nil {
				for _, subE := range subEntries {
					if !subE.IsDir() && strings.EqualFold(filepath.Ext(subE.Name()), ".run") {
						return filepath.Join(subPath, subE.Name())
					}
				}
			}
		}
	}

	return ""
}

// LaunchLinuxInstaller makes the .run file executable and launches it inside Konsole/terminal or directly
func (a *App) LaunchLinuxInstaller(installerPath string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("linux installer execution is only supported on Linux/SteamOS")
	}

	if installerPath == "" {
		return fmt.Errorf("installer path is empty")
	}

	fi, err := os.Stat(installerPath)
	if err != nil {
		return fmt.Errorf("installer file not found: %w", err)
	}
	if fi.IsDir() {
		return fmt.Errorf("installer path is a directory, not a file")
	}

	// Ensure executable permissions (chmod +x)
	_ = os.Chmod(installerPath, 0755)

	installerDir := filepath.Dir(installerPath)
	log.Printf("[Installer] Launching Linux installer: %s (dir: %s)", installerPath, installerDir)

	// Check if running inside Flatpak sandbox
	isFlatpak := os.Getenv("FLATPAK_ID") != ""
	if !isFlatpak {
		if _, err := os.Stat("/.flatpak-info"); err == nil {
			isFlatpak = true
		}
	}

	// Bash script: ensures executable, checks for available terminal on SteamOS/Linux (Konsole, xterm, gnome-terminal)
	script := `chmod +x "$0" && if command -v konsole >/dev/null 2>&1; then konsole --workdir "$(dirname "$0")" -e "$0"; elif command -v gnome-terminal >/dev/null 2>&1; then gnome-terminal --working-directory="$(dirname "$0")" -- "$0"; elif command -v xterm >/dev/null 2>&1; then xterm -hold -e "$0"; else "$0"; fi`

	var cmd *exec.Cmd
	if isFlatpak {
		// Inside Flatpak: spawn process on the host OS
		cmd = exec.Command("flatpak-spawn", "--host", "bash", "-c", script, installerPath)
	} else {
		// Running natively on Linux
		cmd = exec.Command("bash", "-c", script, installerPath)
		cmd.Dir = installerDir
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[Installer] Failed to spawn terminal: %v, falling back to direct execution", err)
		fallbackCmd := exec.Command(installerPath)
		fallbackCmd.Dir = installerDir
		return fallbackCmd.Start()
	}

	log.Printf("[Installer] Installer successfully launched for %s", filepath.Base(installerPath))
	return nil
}

type AppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// GetAppInfo returns application name and version from project configuration
func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:    "Ducke",
		Version: "1.0.0",
	}
}

// ==========================================
// Diagnostics & Logging Methods
// ==========================================

// GetLogs returns all captured in-memory log entries
func (a *App) GetLogs() []logger.LogEntry {
	return logger.GetLogger().GetEntries()
}

// ClearLogs clears the current in-memory log buffer
func (a *App) ClearLogs() {
	logger.GetLogger().Clear()
	log.Println("[System] Log buffer cleared by user")
}

// LogMessage allows frontend to record logs
func (a *App) LogMessage(level, source, message string) {
	logger.GetLogger().AddEntry(level, source, message)
}

// GetLogsText returns formatted plain text of all logs
func (a *App) GetLogsText() string {
	return logger.GetLogger().GetFormattedText()
}

// ExportLogs saves the logs to a file via save dialog or specified path
func (a *App) ExportLogs(targetPath string) (string, error) {
	if targetPath == "" {
		defaultFilename := fmt.Sprintf("ducke_logs_%s.txt", time.Now().Format("20060102_150405"))
		chosenPath, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
			DefaultFilename: defaultFilename,
			Title:           "Экспорт журнала логов",
			Filters: []wailsRuntime.FileFilter{
				{DisplayName: "Текстовые файлы (*.txt)", Pattern: "*.txt"},
				{DisplayName: "Логи (*.log)", Pattern: "*.log"},
				{DisplayName: "Все файлы (*.*)", Pattern: "*.*"},
			},
		})
		if err != nil || chosenPath == "" {
			return "", err
		}
		targetPath = chosenPath
	}

	err := logger.GetLogger().ExportToFile(targetPath)
	if err != nil {
		return "", err
	}
	log.Printf("[System] Successfully exported logs to %s", targetPath)
	return targetPath, nil
}



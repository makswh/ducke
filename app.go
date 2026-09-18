package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"gamevault/pkg/collections"
	"gamevault/pkg/config"
	"gamevault/pkg/database"
	"gamevault/pkg/downloader"
	"gamevault/pkg/launcher"
	"gamevault/pkg/logger"
	"gamevault/pkg/metadata"
	"gamevault/pkg/remote"
	"gamevault/pkg/steam"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx                context.Context
	db                 *database.Database
	cfgManager         *config.ConfigManager
	steamService       *metadata.SteamService
	steamManager       *steam.Manager
	downloader         *downloader.DownloadManager
	collectionsService *collections.StopGameService
	catalogMu           sync.Mutex
	torrentMu           sync.RWMutex
	torrentCache        []database.GameEntity
	torrentCacheByID    map[int64]int
	torrentCacheByAppID map[int][]int
	torrentCacheValid   bool
	enrichmentInFlight  sync.Map
	appDataDir          string
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

	appLogger := logger.InitLogger(3000)
	appLogger.SetHook(func(entry logger.LogEntry) {
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "log:entry", entry)
		}
	})

	// Setup persistent file logging in %APPDATA%/Ducke/logs/ducke.log immediately
	logDir := filepath.Join(appDataDir, "logs")
	_ = os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "ducke.log")
	if err := appLogger.SetLogFile(logFile); err != nil {
		log.Printf("[System] Failed to initialize file logger at %s: %v", logFile, err)
	}

	cfgMgr, err := config.NewConfigManager(appDataDir)
	if err != nil {
		log.Fatalf("failed to init config manager: %v", err)
	}
	a.cfgManager = cfgMgr

	appLogger.SetEnabled(cfgMgr.GetSettings().EnableLogs)
	log.Printf("[System] ===========================================")
	log.Printf("[System] Ducke v1.1.5 starting up")
	log.Printf("[System] AppData: %s", appDataDir)
	log.Printf("[System] LogFile: %s", logFile)
	log.Printf("[System] Runtime: %s %s (%d CPUs)", runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	log.Printf("[System] Settings: LogsEnabled=%v, TorrentSources=%d", cfgMgr.GetSettings().EnableLogs, len(cfgMgr.GetSettings().TorrentSources))
	for _, src := range cfgMgr.GetSettings().TorrentSources {
		log.Printf("[System] -> Source: \"%s\" (enabled=%v, items=%d, url=%s)", src.Name, src.Enabled, src.ItemCount, src.URL)
	}
	log.Printf("[System] ===========================================")

	dbStart := time.Now()
	db, err := database.InitDB(appDataDir)
	if err != nil {
		log.Fatalf("failed to init database: %v", err)
	}
	a.db = db
	log.Printf("[System] SQLite database ready (took %v)", time.Since(dbStart))

	a.steamService = metadata.NewSteamService(db)
	a.collectionsService = collections.NewStopGameService(db)
	a.steamManager = steam.NewManager()

	// Initialize downloader and stream progress via Wails Events
	a.downloader = downloader.NewDownloadManager(db, cfgMgr, func(event downloader.DownloadProgressEvent) {
		wailsRuntime.EventsEmit(a.ctx, "download:progress", event)
	})

	// Pre-warm torrent catalog cache into memory in background immediately
	go func() {
		log.Printf("[Torrent] Starting background catalog pre-warming...")
		pwStart := time.Now()
		games, err := a.GetTorrentCatalog(false)
		log.Printf("[Torrent] Background catalog pre-warming finished (games=%d, err=%v, took %v)", len(games), err, time.Since(pwStart))
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "torrents:updated", nil)
		}
	}()

	// Start background enrichment without blocking SQLite startup queries
	go func() {
		time.Sleep(6000 * time.Millisecond) // Give frontend quiet time to fetch initial catalog
		log.Printf("[System] Starting background enrichment worker...")
		a.triggerBackgroundEnrichment()
	}()
}

func (a *App) triggerBackgroundEnrichment() {
	if a.steamService == nil {
		return
	}
	a.steamService.StartBackgroundEnrichment(
		func(gameID int64, appID int) {
			if game, err := a.db.GetGameByID(gameID); err == nil && game != nil {
				a.updateTorrentCacheItem(game)
				if a.ctx != nil {
					wailsRuntime.EventsEmit(a.ctx, "game:enriched", game)
					if game.IconURL != "" {
						wailsRuntime.EventsEmit(a.ctx, "game:icon-updated", map[string]interface{}{
							"gameId":  gameID,
							"appId":   appID,
							"iconUrl": game.IconURL,
						})
					}
				}
			}
		},
		func(progress metadata.MetadataProgress) {
			if a.ctx != nil {
				wailsRuntime.EventsEmit(a.ctx, "metadata:progress", progress)
			}
		},
	)
}

func (a *App) shutdown(ctx context.Context) {
	if a.steamService != nil {
		a.steamService.StopBackgroundEnrichment()
	}
	if a.downloader != nil {
		a.downloader.Shutdown()
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
		_, _ = a.db.SyncDuplicateGamesMetadata()
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

	wailsRuntime.EventsEmit(a.ctx, "catalog:status", map[string]string{"status": "connecting", "message": "Подключение к серверу..."})
	client := remote.NewRemoteClient(settings.ActiveServer.Protocol)
	if err := client.Connect(*settings.ActiveServer); err != nil {
		log.Printf("[Remote] ERROR: Connection failed: %v", err)
		wailsRuntime.EventsEmit(a.ctx, "catalog:status", map[string]string{"status": "error", "message": "Ошибка подключения к серверу"})
		return games, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer client.Close()

	wailsRuntime.EventsEmit(a.ctx, "catalog:status", map[string]string{"status": "scanning", "message": "Сканирование каталога..."})
	remoteItems, err := client.ScanRepository(remoteDir)
	if err != nil {
		log.Printf("[Remote] ERROR: Repository scan failed: %v", err)
		wailsRuntime.EventsEmit(a.ctx, "catalog:status", map[string]string{"status": "error", "message": "Ошибка сканирования репозитория"})
		return games, fmt.Errorf("failed to scan repository %s: %w", remoteDir, err)
	}

	log.Printf("[Remote] Repository scan complete: found %d items", len(remoteItems))

	if err := a.db.UpsertGames(remoteItems); err != nil {
		log.Printf("[Remote] ERROR: Failed to cache games in database: %v", err)
		wailsRuntime.EventsEmit(a.ctx, "catalog:status", map[string]string{"status": "error", "message": "Ошибка сохранения каталога"})
		return nil, fmt.Errorf("failed to cache games: %w", err)
	}

	wailsRuntime.EventsEmit(a.ctx, "catalog:status", map[string]string{"status": "ready", "message": ""})
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
		details.LogoURL = fmt.Sprintf("https://shared.steamstatic.com/store_item_assets/steam/apps/%d/logo.png", game.SteamAppID)
	}
	if game.HeaderImage != "" {
		details.BannerURL = game.HeaderImage
	}
	if game.CapsuleImage != "" {
		details.CoverURL = game.CapsuleImage
	}
	details.BackgroundURL = game.BackgroundImage

	// 5. Asynchronously fetch missing Steam store details, tags, and reviews in background if needed (non-blocking)
	if details.Game.SteamAppID > 0 {
		needReviews := details.Game.TotalReviews == 0 || details.Game.ReviewScoreDesc == ""
		needDetails := (details.Game.ShortDescription == "" && details.Game.DetailedDescription == "") || (len(details.Game.Screenshots) == 0 && len(details.Game.Genres) == 0)
		needTags := len(details.Game.Tags) == 0
		if needReviews || needDetails || needTags {
			appID := details.Game.SteamAppID
			if _, loaded := a.enrichmentInFlight.LoadOrStore(appID, true); !loaded {
				go func(gameID int64, appID int, fetchReviews, fetchDetails, fetchTags bool) {
					defer a.enrichmentInFlight.Delete(appID)
					if a.steamService != nil {
						if fetchDetails {
							meta, err := a.steamService.FetchAppDetailsPriority(appID)
							if err == nil && meta != nil {
								if updatedGame, err := a.db.GetGameByID(gameID); err == nil && updatedGame != nil {
									if a.ctx != nil {
										wailsRuntime.EventsEmit(a.ctx, "game:enriched", updatedGame)
									}
								}
								// If reviews were populated by FetchAppDetails, notify UI immediately without a second request
								if meta.TotalReviews > 0 && a.ctx != nil {
									wailsRuntime.EventsEmit(a.ctx, "game:reviews-updated", map[string]interface{}{
										"gameId":          gameID,
										"steamAppId":      appID,
										"reviewScoreDesc": meta.ReviewScoreDesc,
										"reviewPercent":   meta.ReviewPercent,
										"totalReviews":    meta.TotalReviews,
										"positiveReviews": meta.TotalPositive,
									})
								}
							}
						} else {
							if fetchTags {
								tags := a.steamService.FetchAppTags(appID)
								if len(tags) > 0 {
									_ = a.db.UpdateSteamMetadataTags(appID, tags)
									if updatedGame, err := a.db.GetGameByID(gameID); err == nil && updatedGame != nil {
										if a.ctx != nil {
											wailsRuntime.EventsEmit(a.ctx, "game:enriched", updatedGame)
										}
									}
								}
							}
							if fetchReviews {
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
						}
					}
				}(details.Game.ID, appID, needReviews, needDetails, needTags)
			}
		}
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

// SetFavoriteLaunchConfig saves a custom executable path and launch arguments for a favorite game
func (a *App) SetFavoriteLaunchConfig(gameID int64, exePath, launchArgs string) error {
	if err := a.db.SetFavoriteLaunchConfig(gameID, exePath, launchArgs); err != nil {
		return err
	}
	wailsRuntime.EventsEmit(a.ctx, "favorites:updated", nil)
	return nil
}

type FavoriteLaunchConfig struct {
	ExePath    string `json:"exePath"`
	LaunchArgs string `json:"launchArgs"`
}

// GetFavoriteLaunchConfig returns saved custom exe path and launch arguments for a game
func (a *App) GetFavoriteLaunchConfig(gameID int64) (FavoriteLaunchConfig, error) {
	if a.db == nil {
		return FavoriteLaunchConfig{}, fmt.Errorf("database not initialized")
	}
	exePath, launchArgs, err := a.db.GetFavoriteLaunchConfig(gameID)
	if err != nil {
		return FavoriteLaunchConfig{}, err
	}
	return FavoriteLaunchConfig{
		ExePath:    exePath,
		LaunchArgs: launchArgs,
	}, nil
}

// FindGameExecutables scans the game's downloaded directory for possible game executables
func (a *App) FindGameExecutables(gameID int64) ([]string, error) {
	details, err := a.GetGamePageDetails(gameID)
	if err != nil {
		return nil, err
	}
	folderPath := details.LocalPath
	if folderPath == "" {
		return nil, fmt.Errorf("локальная папка игры не найдена")
	}

	fi, err := os.Stat(folderPath)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		ext := strings.ToLower(filepath.Ext(folderPath))
		if ext == ".exe" || ext == ".bat" || ext == ".cmd" {
			return []string{folderPath}, nil
		}
		return nil, nil
	}

	isExcludedExe := func(name string) bool {
		lower := strings.ToLower(name)
		excludedSubstrings := []string{
			"unins", "setup", "crash", "reporter", "update", "vcredist",
			"dxwebsetup", "directx", "dotnet", "redist", "eula",
			"install", "config", "benchmark", "loader", "patcher",
		}
		for _, s := range excludedSubstrings {
			if strings.Contains(lower, s) {
				return true
			}
		}
		return false
	}

	var candidates []string
	var secondaryCandidates []string

	baseDepth := strings.Count(filepath.Clean(folderPath), string(os.PathSeparator))
	_ = filepath.WalkDir(folderPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		currentDepth := strings.Count(filepath.Clean(path), string(os.PathSeparator))
		if d.IsDir() {
			if currentDepth-baseDepth > 3 {
				return filepath.SkipDir
			}
			lowerDir := strings.ToLower(d.Name())
			if lowerDir == "$recycle.bin" || lowerDir == ".git" || lowerDir == "_redist" || lowerDir == "redist" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext == ".exe" || ext == ".bat" || ext == ".cmd" {
			if isExcludedExe(d.Name()) {
				secondaryCandidates = append(secondaryCandidates, path)
			} else {
				candidates = append(candidates, path)
			}
		}
		return nil
	})

	if len(candidates) > 0 {
		return candidates, nil
	}
	return secondaryCandidates, nil
}

// SelectGameExeFile opens a file dialog filtered to executable files and returns the chosen path
func (a *App) SelectGameExeFile(defaultDir string) string {
	opts := wailsRuntime.OpenDialogOptions{
		Title: "Выберите исполняемый файл игры",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Исполняемые файлы (*.exe, *.bat, *.cmd)", Pattern: "*.exe;*.bat;*.cmd"},
			{DisplayName: "Все файлы (*.*)", Pattern: "*.*"},
		},
	}
	defaultDir = strings.TrimSpace(defaultDir)
	if defaultDir != "" {
		if fi, err := os.Stat(defaultDir); err == nil {
			if fi.IsDir() {
				opts.DefaultDirectory = defaultDir
			} else {
				opts.DefaultDirectory = filepath.Dir(defaultDir)
			}
		}
	}
	path, err := wailsRuntime.OpenFileDialog(a.ctx, opts)
	if err != nil {
		return ""
	}
	return path
}

// LaunchGameWithCustomConfig launches a game using a custom exe path from favorites,
// falling back to the standard install-path detection if none is set.
func (a *App) LaunchGameWithCustomConfig(gameID int64) error {
	exePath, launchArgs, err := a.db.GetFavoriteLaunchConfig(gameID)
	if err == nil && exePath != "" {
		return launcher.LaunchExecutable(exePath, launchArgs)
	}
	return a.LaunchGameByGameID(gameID)
}

// AddGameToSteam adds or updates a favorite game shortcut in Steam library with covers, banner, hero, logo
func (a *App) AddGameToSteam(gameID int64) (*steam.SteamExportResult, error) {
	if a.steamManager == nil {
		a.steamManager = steam.NewManager()
	}

	game, err := a.db.GetGameByID(gameID)
	if err != nil || game == nil {
		return nil, fmt.Errorf("игра не найдена: %w", err)
	}

	exePath, launchArgs, _ := a.db.GetFavoriteLaunchConfig(gameID)

	// If no custom exe is set, check if we can find one automatically from local path
	if exePath == "" {
		candidates, err := a.FindGameExecutables(gameID)
		if err == nil && len(candidates) > 0 {
			exePath = candidates[0]
		}
	}

	if exePath == "" {
		return nil, fmt.Errorf("для добавления в Steam необходимо указать исполняемый файл (.exe) в настройках игры")
	}

	details, _ := a.GetGamePageDetails(gameID)

	// Clean title for Steam display: prioritize official Steam metadata title
	cleanTitle := a.resolveGameSteamTitle(game)
	if details != nil {
		dSteamTitle := strings.TrimSpace(details.Game.SteamTitle)
		if dSteamTitle != "" && !strings.HasPrefix(strings.ToLower(dSteamTitle), "steam app ") {
			cleanTitle = cleanSteamTitle(dSteamTitle)
		}
	}
	if cleanTitle == "" {
		cleanTitle = cleanSteamTitle(game.RawName)
	}

	// Artwork URLs
	coverURL := game.CapsuleImage
	heroURL := game.BackgroundImage
	bannerURL := game.HeaderImage
	var logoURL string
	if details != nil {
		logoURL = details.LogoURL
		if details.BannerURL != "" {
			bannerURL = details.BannerURL
		}
		if details.CoverURL != "" {
			coverURL = details.CoverURL
		}
		if details.BackgroundURL != "" {
			heroURL = details.BackgroundURL
		}
	}

	// Fallback to official Steam CDN if steamAppId > 0
	if game.SteamAppID > 0 {
		if coverURL == "" {
			coverURL = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/library_600x900_2x.jpg", game.SteamAppID)
		}
		if heroURL == "" {
			heroURL = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/library_hero.jpg", game.SteamAppID)
		}
		if logoURL == "" {
			logoURL = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/logo.png", game.SteamAppID)
		}
		if bannerURL == "" {
			bannerURL = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/header.jpg", game.SteamAppID)
		}
	}

	req := steam.AddShortcutRequest{
		AppName:       cleanTitle,
		ExePath:       exePath,
		LaunchOptions: launchArgs,
		CoverURL:      coverURL,
		HeroURL:       heroURL,
		LogoURL:       logoURL,
		BannerURL:     bannerURL,
		Tags:          []string{"Ducke"},
	}

	if len(game.Genres) > 0 {
		req.Tags = append(req.Tags, game.Genres...)
	}

	return a.steamManager.AddOrUpdateGame(req)
}

// CheckGameInSteam checks if the game is already in user's Steam library
func (a *App) CheckGameInSteam(gameID int64) (bool, error) {
	if a.steamManager == nil {
		a.steamManager = steam.NewManager()
	}

	game, err := a.db.GetGameByID(gameID)
	if err != nil || game == nil {
		return false, err
	}

	exePath, _, _ := a.db.GetFavoriteLaunchConfig(gameID)
	cleanTitle := a.resolveGameSteamTitle(game)

	inSteam, _, err := a.steamManager.IsGameInSteam(cleanTitle, exePath)
	return inSteam, err
}

// RemoveGameFromSteam removes game shortcut and artwork from user's Steam library
func (a *App) RemoveGameFromSteam(gameID int64) error {
	if a.steamManager == nil {
		a.steamManager = steam.NewManager()
	}

	game, err := a.db.GetGameByID(gameID)
	if err != nil || game == nil {
		return err
	}

	exePath, _, _ := a.db.GetFavoriteLaunchConfig(gameID)
	cleanTitle := a.resolveGameSteamTitle(game)

	inSteam, appID, err := a.steamManager.IsGameInSteam(cleanTitle, exePath)
	if err != nil || !inSteam {
		return err
	}

	return a.steamManager.RemoveGameShortcut(appID)
}

var (
	rxSpacedAbbr7  = regexp.MustCompile(`\b([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\b`)
	rxSpacedAbbr6  = regexp.MustCompile(`\b([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\b`)
	rxSpacedAbbr5  = regexp.MustCompile(`\b([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\b`)
	rxSpacedAbbr4  = regexp.MustCompile(`\b([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\b`)
	rxSpacedAbbr3  = regexp.MustCompile(`\b([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\s+([A-Za-zА-Яа-я])\b`)

	rxAllBrackets  = regexp.MustCompile(`\[[^\]]*\]`)
	rxParenYears   = regexp.MustCompile(`\(\s*\d{4}(?:\s*[-–—/]\s*\d{4})?\s*\)`)
	rxParenTags    = regexp.MustCompile(`(?i)\((?:repack|rip|версия|от|by|portable|gog|pc|[\d.,\s/\\+-]+)[^\)]*\)`)
	rxVersionTag   = regexp.MustCompile(`(?i)\b(?:v\s*\d+([._\s]\d+)*|build\s*\d+|patch\s*\d+|update\s*\d*|hotfix)\b`)
	rxStandalone   = regexp.MustCompile(`(?i)\b(repack|репак|rip|рип|portable|unpacked|steamrip|gog|xatab|fitgirl|dodi|decepticon|elamigos|codex|empress|multi\d*)\b`)
	rxTrailingPC   = regexp.MustCompile(`(?i)\s*\b(pc|mac|linux|win|windows)\s*$`)
	rxTrailingSize = regexp.MustCompile(`(?i)(?:[\s\-_]+)?(?:\[|\()?(\d+([.,]\d+)?\s*(?:gb|mb|tb|гб|мб|тб|g|m|t))(?:\)|\])?$`)
	rxMultiSpace   = regexp.MustCompile(`\s+`)
)

func cleanSteamTitle(rawTitle string) string {
	if rawTitle == "" {
		return ""
	}
	s := strings.TrimSpace(rawTitle)

	// 1. Spaced abbreviations like S T A L K E R
	s = rxSpacedAbbr7.ReplaceAllString(s, "$1.$2.$3.$4.$5.$6.$7.")
	s = rxSpacedAbbr6.ReplaceAllString(s, "$1.$2.$3.$4.$5.$6.")
	s = rxSpacedAbbr5.ReplaceAllString(s, "$1.$2.$3.$4.$5.")
	s = rxSpacedAbbr4.ReplaceAllString(s, "$1.$2.$3.$4.")
	s = rxSpacedAbbr3.ReplaceAllString(s, "$1.$2.$3.")

	// 2. Strip all square brackets [ ... ] (e.g. [DL], [В разработке], [P], [RUS / ENG], [v1.2])
	s = rxAllBrackets.ReplaceAllString(s, " ")

	// 3. Strip years in parentheses (2019) or (2003-2020)
	s = rxParenYears.ReplaceAllString(s, " ")

	// 4. Strip release notes in parentheses (repack by ...)
	s = rxParenTags.ReplaceAllString(s, " ")

	// 5. Dual titles (e.g. "Game / Игра") - done after stripping brackets so brackets with slashes don't break
	if strings.Contains(s, " / ") {
		parts := strings.Split(s, " / ")
		if len(parts) >= 2 && len(strings.TrimSpace(parts[0])) >= 3 {
			s = parts[0]
		}
	} else if strings.Contains(s, " | ") {
		parts := strings.Split(s, " | ")
		if len(parts) >= 2 && len(strings.TrimSpace(parts[0])) >= 3 {
			s = parts[0]
		}
	}

	// 6. Strip version markers (v 1 3 3, build 1234, etc.)
	s = rxVersionTag.ReplaceAllString(s, " ")

	// 7. Strip standalone release words (RePack, portable, etc.)
	s = rxStandalone.ReplaceAllString(s, " ")

	// 8. Strip trailing platform markers (PC, Linux, Windows) and sizes
	s = rxTrailingPC.ReplaceAllString(s, " ")
	s = rxTrailingSize.ReplaceAllString(s, " ")

	// 9. Strip curly braces and boundary separators
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.Trim(s, " -_:/\\")

	s = rxMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func (a *App) resolveGameSteamTitle(game *database.GameEntity) string {
	if game == nil {
		return ""
	}
	steamTitle := strings.TrimSpace(game.SteamTitle)
	if steamTitle != "" && !strings.HasPrefix(strings.ToLower(steamTitle), "steam app ") {
		return cleanSteamTitle(steamTitle)
	}

	raw := strings.TrimSpace(game.CleanTitle)
	if raw == "" {
		raw = strings.TrimSpace(game.RawName)
	}
	return cleanSteamTitle(raw)
}

func (a *App) UpdateSteamAppID(gameID int64, appID int) error {
	if appID > 0 {
		_, _ = a.steamService.FetchAppDetailsPriority(appID)
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
		a.updateTorrentCacheItem(game)
		wailsRuntime.EventsEmit(a.ctx, "game:enriched", game)
	}
	return game, err
}

func (a *App) GetMetadataProgress() metadata.MetadataProgress {
	if a.steamService == nil {
		return metadata.MetadataProgress{}
	}
	return a.steamService.GetProgress()
}

func (a *App) SearchSteamCandidates(query string) ([]metadata.SteamCandidate, error) {
	return a.steamService.SearchSteamCandidates(query)
}

// GetGameMovies retrieves fresh trailers (including modern HLS streams) for a given Steam AppID
func (a *App) GetGameMovies(appID int) ([]database.SteamMovie, error) {
	if appID <= 0 {
		return []database.SteamMovie{}, nil
	}
	meta, err := a.steamService.FetchAppDetailsPriority(appID)
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

	cleanTitle := remote.SanitizeForSteamSearch(title)
	if cleanTitle == "" {
		cleanTitle = strings.TrimSpace(title)
	}

	coverURL, err := a.steamService.GetSteamGridCover(cleanTitle)
	if (err != nil || coverURL == "") && cleanTitle != title {
		coverURL, err = a.steamService.GetSteamGridCover(title)
	}
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

	cleanTitle := remote.SanitizeForSteamSearch(title)
	if cleanTitle == "" {
		cleanTitle = strings.TrimSpace(title)
	}

	bannerURL, err := a.steamService.GetSteamGridBanner(cleanTitle)
	if (err != nil || bannerURL == "") && cleanTitle != title {
		bannerURL, err = a.steamService.GetSteamGridBanner(title)
	}
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

	cleanTitle := remote.SanitizeForSteamSearch(title)
	if cleanTitle == "" {
		cleanTitle = strings.TrimSpace(title)
	}

	// 1. Try SteamGridDB logo first (SteamGridDB logos are high quality, transparent PNGs)
	logoURL, err := a.steamService.GetSteamGridLogo(cleanTitle)
	if (err != nil || logoURL == "") && cleanTitle != title {
		logoURL, err = a.steamService.GetSteamGridLogo(title)
	}
	if err == nil && logoURL != "" {
		return logoURL, nil
	}

	// 2. If steamAppID is present, fallback to Steam store logo
	if steamAppID > 0 {
		steamLogo := fmt.Sprintf("https://shared.steamstatic.com/store_item_assets/steam/apps/%d/logo.png", steamAppID)
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
	game, err := a.db.GetGameByID(gameID)
	if err == nil && game != nil && game.SourceType == "torrent" {
		return a.downloader.StartTorrentDownload(*game, destinationPath)
	}
	return a.downloader.StartDownload(gameID, destinationPath)
}

func (a *App) StartTorrentDownload(gameID int64, destinationPath string) (string, error) {
	game, err := a.db.GetGameByID(gameID)
	if err != nil || game == nil {
		return "", fmt.Errorf("игра не найдена: %w", err)
	}
	return a.downloader.StartTorrentDownload(*game, destinationPath)
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

// GetTorrentSeedsBatch queries live or cached seeders and leechers for a list of torrent queries
func (a *App) GetTorrentSeedsBatch(queries []downloader.TorrentSeedQuery) map[int64]downloader.TorrentSeedResult {
	if a.downloader == nil {
		return map[int64]downloader.TorrentSeedResult{}
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.downloader.GetTorrentSeedsBatch(ctx, queries)
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

// ==========================================
// Torrent Sources & Catalog Methods
// ==========================================

func (a *App) GetTorrentSources() []config.TorrentSourceConfig {
	return a.cfgManager.GetSettings().TorrentSources
}

// fetchHydraSource downloads, validates, and parses a JSON source catalog
func fetchHydraSource(sourceURL string) (*database.HydraSourceFile, string, error) {
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return nil, "", fmt.Errorf("URL или путь к источнику не может быть пустым")
	}

	var body []byte
	var err error

	if strings.HasPrefix(sourceURL, "http://") || strings.HasPrefix(sourceURL, "https://") {
		client := &http.Client{Timeout: 30 * time.Second}
		req, errReq := http.NewRequest("GET", sourceURL, nil)
		if errReq != nil {
			return nil, "", fmt.Errorf("ошибка создания запроса: %w", errReq)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Ducke/1.0")

		resp, errDo := client.Do(req)
		if errDo != nil {
			return nil, "", fmt.Errorf("не удалось загрузить источник: %w", errDo)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("сервер вернул статус %d", resp.StatusCode)
		}

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", fmt.Errorf("ошибка чтения ответа: %w", err)
		}
	} else {
		filePath := strings.TrimPrefix(sourceURL, "file:///")
		filePath = strings.TrimPrefix(filePath, "file://")
		body, err = os.ReadFile(filePath)
		if err != nil {
			return nil, "", fmt.Errorf("ошибка чтения локального файла: %w", err)
		}
	}

	var hf database.HydraSourceFile
	if err := json.Unmarshal(body, &hf); err != nil {
		return nil, "", fmt.Errorf("неверный формат источника: %w", err)
	}

	if len(hf.Downloads) == 0 {
		return nil, "", fmt.Errorf("источник не содержит раздач")
	}

	sourceName := strings.TrimSpace(hf.Name)
	if sourceName == "" {
		cleanName := filepath.Base(sourceURL)
		sourceName = strings.TrimSuffix(cleanName, filepath.Ext(cleanName))
		if sourceName == "" {
			sourceName = "Пользовательский источник"
		}
	}

	return &hf, sourceName, nil
}

func (a *App) AddTorrentSource(sourceURL string) (*config.TorrentSourceConfig, error) {
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return nil, fmt.Errorf("URL источника не может быть пустым")
	}

	settings := a.cfgManager.GetSettings()
	for _, s := range settings.TorrentSources {
		if strings.EqualFold(strings.TrimSpace(s.URL), sourceURL) {
			return nil, fmt.Errorf("источник с таким URL уже добавлен (\"%s\")", s.Name)
		}
	}

	hf, sourceName, err := fetchHydraSource(sourceURL)
	if err != nil {
		return nil, err
	}

	sourceID := fmt.Sprintf("tsrc_%d", time.Now().UnixNano())

	if err := a.db.UpsertTorrentGames(sourceID, sourceName, hf.Downloads); err != nil {
		return nil, fmt.Errorf("ошибка сохранения раздач в базу: %w", err)
	}

	srcConfig := config.TorrentSourceConfig{
		ID:         sourceID,
		Name:       sourceName,
		URL:        sourceURL,
		Enabled:    true,
		ItemCount:  len(hf.Downloads),
		LastSynced: time.Now().Unix(),
	}

	settings.TorrentSources = append(settings.TorrentSources, srcConfig)
	if err := a.cfgManager.SaveSettings(settings); err != nil {
		return nil, fmt.Errorf("ошибка сохранения настроек: %w", err)
	}

	a.invalidateTorrentCache()

	log.Printf("[Torrent] Added source \"%s\" (%d items) from %s", sourceName, len(hf.Downloads), sourceURL)

	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "settings:updated", a.cfgManager.GetSettings())
		wailsRuntime.EventsEmit(a.ctx, "torrents:updated", nil)
	}

	return &srcConfig, nil
}

// ImportTorrentSourceFile opens native file dialog to select a JSON source catalog and imports it
func (a *App) ImportTorrentSourceFile() (*config.TorrentSourceConfig, error) {
	filePath, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Выберите JSON-каталог источников",
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "JSON Files (*.json)",
				Pattern:     "*.json",
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

	return a.AddTorrentSource(filePath)
}

func (a *App) RemoveTorrentSource(id string) error {
	settings := a.cfgManager.GetSettings()
	newSources := make([]config.TorrentSourceConfig, 0, len(settings.TorrentSources))
	for _, s := range settings.TorrentSources {
		if s.ID != id {
			newSources = append(newSources, s)
		}
	}
	settings.TorrentSources = newSources
	if err := a.cfgManager.SaveSettings(settings); err != nil {
		return err
	}

	if err := a.db.DeleteTorrentGamesBySource(id); err != nil {
		log.Printf("[Torrent] Error pruning games for source %s: %v", id, err)
	}

	a.invalidateTorrentCache()

	log.Printf("[Torrent] Removed source %s", id)

	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "settings:updated", a.cfgManager.GetSettings())
		wailsRuntime.EventsEmit(a.ctx, "torrents:updated", nil)
	}
	return nil
}

func (a *App) ToggleTorrentSource(id string, enabled bool) error {
	settings := a.cfgManager.GetSettings()
	for i := range settings.TorrentSources {
		if settings.TorrentSources[i].ID == id {
			settings.TorrentSources[i].Enabled = enabled
			break
		}
	}
	if err := a.cfgManager.SaveSettings(settings); err != nil {
		return err
	}

	a.invalidateTorrentCache()

	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "settings:updated", a.cfgManager.GetSettings())
		wailsRuntime.EventsEmit(a.ctx, "torrents:updated", nil)
	}
	return nil
}

func (a *App) SyncTorrentSources() error {
	settings := a.cfgManager.GetSettings()
	updated := false

	for i := range settings.TorrentSources {
		src := &settings.TorrentSources[i]
		if !src.Enabled || strings.TrimSpace(src.URL) == "" {
			continue
		}

		hf, sourceName, err := fetchHydraSource(src.URL)
		if err != nil {
			log.Printf("[Torrent] Sync error for %s (%s): %v", src.Name, src.URL, err)
			continue
		}

		if err := a.db.UpsertTorrentGames(src.ID, sourceName, hf.Downloads); err != nil {
			log.Printf("[Torrent] Sync database error for %s: %v", src.Name, err)
			continue
		}

		src.Name = sourceName
		src.ItemCount = len(hf.Downloads)
		src.LastSynced = time.Now().Unix()
		updated = true
		log.Printf("[Torrent] Synced source \"%s\": %d items", src.Name, src.ItemCount)
	}

	if updated {
		a.invalidateTorrentCache()
		_ = a.cfgManager.SaveSettings(settings)
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "settings:updated", a.cfgManager.GetSettings())
			wailsRuntime.EventsEmit(a.ctx, "torrents:updated", nil)
		}
	}

	return nil
}

func (a *App) invalidateTorrentCache() {
	a.torrentMu.Lock()
	a.torrentCache = nil
	a.torrentCacheByID = nil
	a.torrentCacheByAppID = nil
	a.torrentCacheValid = false
	a.torrentMu.Unlock()
}

func (a *App) rebuildTorrentCacheIndexLocked() {
	a.torrentCacheByID = make(map[int64]int, len(a.torrentCache)*2)
	a.torrentCacheByAppID = make(map[int][]int, len(a.torrentCache))
	for i := range a.torrentCache {
		g := &a.torrentCache[i]
		a.torrentCacheByID[g.ID] = i
		for _, v := range g.Variants {
			a.torrentCacheByID[v.ID] = i
		}
		if g.SteamAppID > 0 {
			a.torrentCacheByAppID[g.SteamAppID] = append(a.torrentCacheByAppID[g.SteamAppID], i)
		}
	}
}

func (a *App) updateTorrentCacheItem(enriched *database.GameEntity) {
	if enriched == nil {
		return
	}
	a.torrentMu.Lock()
	defer a.torrentMu.Unlock()
	if !a.torrentCacheValid || len(a.torrentCache) == 0 {
		return
	}

	targetIndices := make(map[int]bool)
	if a.torrentCacheByID != nil {
		if idx, ok := a.torrentCacheByID[enriched.ID]; ok {
			targetIndices[idx] = true
		}
	}
	if enriched.SteamAppID > 0 && a.torrentCacheByAppID != nil {
		if indices, ok := a.torrentCacheByAppID[enriched.SteamAppID]; ok {
			for _, idx := range indices {
				targetIndices[idx] = true
			}
		}
	}

	// Fallback if not found in index
	if len(targetIndices) == 0 {
		for i := range a.torrentCache {
			g := &a.torrentCache[i]
			match := g.ID == enriched.ID || (enriched.SteamAppID != 0 && g.SteamAppID == enriched.SteamAppID)
			if !match && len(g.Variants) > 0 {
				for _, v := range g.Variants {
					if v.ID == enriched.ID {
						match = true
						break
					}
				}
			}
			if match {
				targetIndices[i] = true
				break
			}
		}
	}

	for idx := range targetIndices {
		if idx < 0 || idx >= len(a.torrentCache) {
			continue
		}
		g := &a.torrentCache[idx]
		g.SteamAppID = enriched.SteamAppID
		g.SteamSynced = enriched.SteamSynced
		g.SteamTitle = enriched.SteamTitle
		g.ShortDescription = enriched.ShortDescription
		g.DetailedDescription = enriched.DetailedDescription
		g.HeaderImage = enriched.HeaderImage
		g.CapsuleImage = enriched.CapsuleImage
		g.BackgroundImage = enriched.BackgroundImage
		g.IconURL = enriched.IconURL
		g.Genres = enriched.Genres
		g.Tags = enriched.Tags
		g.Developers = enriched.Developers
		g.Publishers = enriched.Publishers
		g.ReleaseDate = enriched.ReleaseDate
		g.ControllerSupport = enriched.ControllerSupport
		g.PCRequirements = enriched.PCRequirements
		g.MetacriticScore = enriched.MetacriticScore
		g.ReviewScoreDesc = enriched.ReviewScoreDesc
		g.ReviewPercent = enriched.ReviewPercent
		g.TotalReviews = enriched.TotalReviews
		for vi := range g.Variants {
			g.Variants[vi].SteamAppID = enriched.SteamAppID
		}
		if enriched.SteamAppID > 0 && a.torrentCacheByAppID != nil {
			a.torrentCacheByAppID[enriched.SteamAppID] = append(a.torrentCacheByAppID[enriched.SteamAppID], idx)
		}
	}
}

func (a *App) GetTorrentCatalog(forceRefresh bool) ([]database.GameEntity, error) {
	start := time.Now()
	if !forceRefresh {
		a.torrentMu.RLock()
		if a.torrentCacheValid && a.torrentCache != nil {
			res := make([]database.GameEntity, len(a.torrentCache))
			copy(res, a.torrentCache)
			count := len(res)
			a.torrentMu.RUnlock()
			log.Printf("[Torrent] GetTorrentCatalog: returned %d cached games (took %v)", count, time.Since(start))
			return res, nil
		}
		a.torrentMu.RUnlock()
	}

	a.torrentMu.Lock()
	defer a.torrentMu.Unlock()

	// Double-check under write lock
	if !forceRefresh && a.torrentCacheValid && a.torrentCache != nil {
		res := make([]database.GameEntity, len(a.torrentCache))
		copy(res, a.torrentCache)
		log.Printf("[Torrent] GetTorrentCatalog: returned %d cached games under lock (took %v)", len(res), time.Since(start))
		return res, nil
	}

	log.Printf("[Torrent] GetTorrentCatalog: fetching from database (forceRefresh=%v)...", forceRefresh)
	if forceRefresh {
		_ = a.SyncTorrentSources()
		a.triggerBackgroundEnrichment()
	}

	qStart := time.Now()
	games, err := a.db.GetTorrentGames()
	log.Printf("[Torrent] db.GetTorrentGames finished: %d games returned (took %v, err=%v)", len(games), time.Since(qStart), err)

	if err == nil && len(games) == 0 && len(a.cfgManager.GetSettings().TorrentSources) > 0 {
		log.Printf("[Torrent] 0 games in DB, triggering sync of %d configured torrent sources...", len(a.cfgManager.GetSettings().TorrentSources))
		_ = a.SyncTorrentSources()
		games, err = a.db.GetTorrentGames()
		log.Printf("[Torrent] After source sync, db.GetTorrentGames returned %d games (err=%v)", len(games), err)
	}

	if err == nil {
		a.torrentCache = games
		a.rebuildTorrentCacheIndexLocked()
		a.torrentCacheValid = true
		res := make([]database.GameEntity, len(games))
		copy(res, games)
		log.Printf("[Torrent] GetTorrentCatalog: catalog cache populated with %d games (total took %v)", len(games), time.Since(start))
		return res, nil
	}

	log.Printf("[Torrent] GetTorrentCatalog: failed with error: %v (took %v)", err, time.Since(start))
	return games, err
}

// AssetHandler implements http.Handler for streaming large data directly to WebView2
// completely bypassing Windows ExecuteScript IPC message payload limits.
// It is kept separate from App so Wails does not treat it as an exported IPC method.
type AssetHandler struct {
	app *App
}

func NewAssetHandler(app *App) *AssetHandler {
	return &AssetHandler{app: app}
}

func (h *AssetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/torrent-catalog") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-cache")

		forceRefresh := r.URL.Query().Get("refresh") == "1"

		games, err := h.app.GetTorrentCatalog(forceRefresh)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_ = json.NewEncoder(w).Encode(games)
		return
	}
	http.NotFound(w, r)
}

func (a *App) GetTorrentCatalogCount() int {
	start := time.Now()
	a.torrentMu.RLock()
	if a.torrentCacheValid && a.torrentCache != nil {
		count := len(a.torrentCache)
		a.torrentMu.RUnlock()
		log.Printf("[Torrent] GetTorrentCatalogCount: returned %d (cached, took %v)", count, time.Since(start))
		return count
	}
	a.torrentMu.RUnlock()

	log.Printf("[Torrent] GetTorrentCatalogCount: cache not ready, triggering GetTorrentCatalog(false)...")
	_, _ = a.GetTorrentCatalog(false)

	a.torrentMu.RLock()
	count := len(a.torrentCache)
	a.torrentMu.RUnlock()
	log.Printf("[Torrent] GetTorrentCatalogCount: returned %d after warm-up (took %v)", count, time.Since(start))
	return count
}

func (a *App) GetTorrentCatalogChunk(offset int, limit int) []database.GameEntity {
	start := time.Now()
	a.torrentMu.RLock()
	if !a.torrentCacheValid || a.torrentCache == nil {
		a.torrentMu.RUnlock()
		log.Printf("[Torrent] GetTorrentCatalogChunk(offset=%d, limit=%d): cache not ready, warming up...", offset, limit)
		_, _ = a.GetTorrentCatalog(false)
		a.torrentMu.RLock()
	}
	defer a.torrentMu.RUnlock()

	total := len(a.torrentCache)
	if offset < 0 || offset >= total || limit <= 0 {
		log.Printf("[Torrent] GetTorrentCatalogChunk(offset=%d, limit=%d): out of bounds (total=%d)", offset, limit, total)
		return []database.GameEntity{}
	}
	end := offset + limit
	if end > total {
		end = total
	}
	res := make([]database.GameEntity, end-offset)
	copy(res, a.torrentCache[offset:end])
	log.Printf("[Torrent] GetTorrentCatalogChunk: offset=%d, limit=%d -> returned %d items (total=%d, took %v)", offset, limit, len(res), total, time.Since(start))
	return res
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
	_, _ = a.db.SyncDuplicateGamesMetadata()
	a.invalidateTorrentCache()
	a.triggerBackgroundEnrichment()
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
	Success      bool   `json:"success"`
	ProtocolUsed string `json:"protocolUsed"`
	ErrorMessage string `json:"errorMessage"`
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

// OpenLocalFolder opens native file manager at folder path, safely handling non-existent paths, files, and fallback directories
func (a *App) OpenLocalFolder(folderPath string) error {
	folderPath = strings.TrimSpace(folderPath)
	if folderPath == "" {
		return fmt.Errorf("path is empty")
	}

	cleanPath := filepath.Clean(filepath.FromSlash(folderPath))

	// Resolve the most accurate existing path on disk
	resolvedPath := cleanPath
	isFile := false

	fi, err := os.Stat(cleanPath)
	if err == nil {
		if !fi.IsDir() {
			isFile = true
		}
	} else {
		// Path does not exist directly. Look in parent directory for matching folder
		parentDir := filepath.Dir(cleanPath)
		found := false
		if entries, rErr := os.ReadDir(parentDir); rErr == nil {
			baseName := strings.ToLower(filepath.Base(cleanPath))
			for _, entry := range entries {
				entryLower := strings.ToLower(entry.Name())
				if strings.Contains(entryLower, baseName) || strings.Contains(baseName, entryLower) {
					candidate := filepath.Join(parentDir, entry.Name())
					if cFi, cErr := os.Stat(candidate); cErr == nil {
						resolvedPath = candidate
						if !cFi.IsDir() {
							isFile = true
						}
						found = true
						break
					}
				}
			}
		}

		if !found {
			// Fallback: If parent directory exists, open parent directory (e.g. user's downloads folder)
			if pFi, pErr := os.Stat(parentDir); pErr == nil && pFi.IsDir() {
				resolvedPath = parentDir
			} else if a.cfgManager != nil {
				// Fallback to configured download path
				cfgPath := a.cfgManager.GetSettings().DownloadPath
				if cFi, cErr := os.Stat(cfgPath); cErr == nil && cFi.IsDir() {
					resolvedPath = cfgPath
				} else {
					return fmt.Errorf("папка не найдена: %s", cleanPath)
				}
			} else {
				return fmt.Errorf("папка не найдена: %s", cleanPath)
			}
		}
	}

	switch runtime.GOOS {
	case "windows":
		if isFile {
			return exec.Command("explorer", "/select,", resolvedPath).Start()
		}
		return exec.Command("explorer", resolvedPath).Start()
	case "darwin":
		if isFile {
			return exec.Command("open", "-R", resolvedPath).Start()
		}
		return exec.Command("open", resolvedPath).Start()
	default: // linux, steamOS
		target := resolvedPath
		if isFile {
			target = filepath.Dir(resolvedPath)
		}
		return exec.Command("xdg-open", target).Start()
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
			return launcher.LaunchExecutable(folderOrFilePath, "")
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
			return launcher.LaunchExecutable(candidates[0], "")
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
		Version: "1.1.5",
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

// ==========================================
// Favorites & Backlog Methods
// ==========================================

// GetFavorites returns all saved favorite games with their metadata
func (a *App) GetFavorites() ([]database.FavoriteItem, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return a.db.GetFavorites()
}

// SetFavoriteStatus sets or updates the backlog status for a game (planned, playing, completed)
func (a *App) SetFavoriteStatus(gameID int64, status string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	err := a.db.SetFavorite(gameID, status)
	if err == nil && a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "favorites:updated", map[string]interface{}{
			"gameId": gameID,
			"status": status,
		})
	}
	return err
}

// RemoveFromFavorites removes a game from favorites
func (a *App) RemoveFromFavorites(gameID int64) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	err := a.db.RemoveFavorite(gameID)
	if err == nil && a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "favorites:updated", map[string]interface{}{
			"gameId":  gameID,
			"status":  "",
			"removed": true,
		})
	}
	return err
}

// GetStopGameCompilations retrieves a paginated list of compilations from StopGame
func (a *App) GetStopGameCompilations(sort string, page int) (*collections.CompilationsResponse, error) {
	if a.collectionsService == nil {
		return nil, fmt.Errorf("collections service not initialized")
	}
	return a.collectionsService.FetchCompilations(sort, page)
}

// GetStopGameCompilationDetail retrieves a single compilation with full games list and library matching
func (a *App) GetStopGameCompilationDetail(id string, forceRefresh bool) (*collections.CompilationDetail, error) {
	if a.collectionsService == nil {
		return nil, fmt.Errorf("collections service not initialized")
	}
	return a.collectionsService.FetchCompilationDetail(id, forceRefresh)
}

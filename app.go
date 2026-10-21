package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"gamevault/pkg/collections"
	"gamevault/pkg/config"
	"gamevault/pkg/database"
	"gamevault/pkg/downloader"
	"gamevault/pkg/logger"
	"gamevault/pkg/metadata"
	"gamevault/pkg/steam"
	"gamevault/pkg/updater"

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
	updater            *updater.Updater

	catalogMu            sync.Mutex
	catalogCache         []database.GameEntity
	catalogCacheValid    bool
	lastScannedServerSig string

	torrentMu           sync.RWMutex
	torrentCache        []database.GameEntity
	torrentCacheByID    map[int64]int
	torrentCacheByAppID map[int][]int
	torrentCacheValid   bool

	enrichmentInFlight sync.Map
	dominantColorCache sync.Map
	appDataDir         string
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
	log.Printf("[System] Ducke v1.2.3 starting up")
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
	a.updater = updater.NewUpdater("makswh/ducke", "1.2.0", appDataDir)

	// Initialize downloader and stream progress via Wails Events
	a.downloader = downloader.NewDownloadManager(db, cfgMgr, func(event downloader.DownloadProgressEvent) {
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "download:progress", event)
		}
	}, func() {
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "downloads:updated", nil)
		}
	})

	// Background check for updates after interface finishes initial load
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Updater] Recovered from panic in updater goroutine: %v", r)
			}
		}()
		time.Sleep(5 * time.Second)
		if a.updater != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			info, err := a.updater.CheckForUpdates(ctx)
			if err == nil && info != nil && info.HasUpdate {
				log.Printf("[Updater] New version available: %s (current: %s)", info.LatestVersion, info.CurrentVersion)
				if a.ctx != nil {
					wailsRuntime.EventsEmit(a.ctx, "update:available", info)
				}
			}
		}
	}()

	// Pre-warm torrent catalog cache into memory in background immediately
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Torrent] Recovered from panic in pre-warming goroutine: %v", r)
			}
		}()
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
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[System] Recovered from panic in background enrichment starter: %v", r)
			}
		}()
		time.Sleep(6000 * time.Millisecond) // Give frontend quiet time to fetch initial catalog
		log.Printf("[System] Starting background enrichment worker...")
		a.triggerBackgroundEnrichment()
	}()

	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "downloads:updated", nil)
	}
}

func (a *App) triggerBackgroundEnrichment() {
	if a.steamService == nil {
		return
	}
	a.steamService.StartBackgroundEnrichment(
		func(gameID int64, appID int) {
			if game, err := a.db.GetGameByID(gameID); err == nil && game != nil {
				a.updateAllCachedGameItems(game)
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

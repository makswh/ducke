package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gamevault/pkg/remote"

	_ "modernc.org/sqlite"
)

// SteamMovie represents a video trailer from Steam
type SteamMovie struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Thumbnail string `json:"thumbnail"`
	MP4       string `json:"mp4,omitempty"`
	Webm      string `json:"webm,omitempty"`
	HLS       string `json:"hls,omitempty"`
}

// GameEntity represents a game entry with remote and steam metadata joined
type GameEntity struct {
	ID                int64        `json:"id"`
	RawName           string       `json:"rawName"`
	CleanTitle        string       `json:"cleanTitle"`
	SearchTitle       string       `json:"searchTitle"`
	RemotePath        string       `json:"remotePath"`
	SizeBytes         int64        `json:"sizeBytes"`
	SizeDisplay       string       `json:"sizeDisplay"`
	IsDirectory       bool         `json:"isDirectory"`
	IsCollection      bool         `json:"isCollection"`
	ParentPath        string       `json:"parentPath"`
	SteamAppID        int          `json:"steamAppId"`
	SteamSynced       bool         `json:"steamSynced"`
	// Joined Steam Metadata
	SteamTitle        string       `json:"steamTitle,omitempty"`
	ShortDescription  string       `json:"shortDescription,omitempty"`
	DetailedDescription string     `json:"detailedDescription,omitempty"`
	HeaderImage       string       `json:"headerImage,omitempty"`
	CapsuleImage      string       `json:"capsuleImage,omitempty"`
	BackgroundImage   string       `json:"backgroundImage,omitempty"`
	IconURL           string       `json:"iconUrl,omitempty"`
	Screenshots       []string     `json:"screenshots,omitempty"`
	Movies            []SteamMovie `json:"movies,omitempty"`
	Genres            []string     `json:"genres,omitempty"`
	Developers        []string     `json:"developers,omitempty"`
	Publishers        []string     `json:"publishers,omitempty"`
	ReleaseDate       string       `json:"releaseDate,omitempty"`
	ControllerSupport string       `json:"controllerSupport,omitempty"`
	PCRequirements    string       `json:"pcRequirements,omitempty"`
	MetacriticScore   int          `json:"metacriticScore,omitempty"`
	ReviewScoreDesc   string       `json:"reviewScoreDesc,omitempty"`
	ReviewPercent     int          `json:"reviewPercent,omitempty"`
	TotalReviews      int          `json:"totalReviews,omitempty"`
	// Torrent Source Metadata
	SourceType        string       `json:"sourceType,omitempty"` // "ftp" or "torrent"
	TorrentSource     string       `json:"torrentSource,omitempty"`
	MagnetURI         string       `json:"magnetUri,omitempty"`
	UploadDate        string       `json:"uploadDate,omitempty"`
	// Favorites Metadata
	FavoriteStatus    string       `json:"favoriteStatus,omitempty"` // "planned", "playing", "completed"
	// Release variants / duplicates
	Variants          []GameVariant `json:"variants,omitempty"`
}

// FavoriteItem represents a game saved to user's favorites / backlog
type FavoriteItem struct {
	GameID    int64      `json:"gameId"`
	Status    string     `json:"status"` // "planned", "playing", "completed"
	AddedAt   string     `json:"addedAt"`
	UpdatedAt string     `json:"updatedAt"`
	Game      GameEntity `json:"game"`
}

// GameVariant represents an alternative release/repack/version of a game
type GameVariant struct {
	ID            int64  `json:"id"`
	RawName       string `json:"rawName"`
	CleanTitle    string `json:"cleanTitle"`
	SizeBytes     int64  `json:"sizeBytes"`
	SizeDisplay   string `json:"sizeDisplay"`
	SourceType    string `json:"sourceType"`
	TorrentSource string `json:"torrentSource,omitempty"`
	RemotePath    string `json:"remotePath"`
	MagnetURI     string `json:"magnetUri,omitempty"`
	UploadDate    string `json:"uploadDate,omitempty"`
	IsDirectory   bool   `json:"isDirectory"`
	SteamAppID    int    `json:"steamAppId,omitempty"`
}

// HydraDownloadItem represents a single item inside a Hydra torrent source list
type HydraDownloadItem struct {
	Title      string   `json:"title"`
	FileSize   string   `json:"fileSize"`
	URIs       []string `json:"uris"`
	UploadDate string   `json:"uploadDate"`
}

// HydraSourceFile represents the root structure of a Hydra source JSON file
type HydraSourceFile struct {
	Name      string              `json:"name"`
	Downloads []HydraDownloadItem `json:"downloads"`
}

// SteamMetadata represents detailed information fetched from Steam Store API
type SteamMetadata struct {
	AppID               int          `json:"appId"`
	Title               string       `json:"title"`
	ShortDescription    string       `json:"shortDescription"`
	DetailedDescription string       `json:"detailedDescription"`
	HeaderImage         string       `json:"headerImage"`
	CapsuleImage        string       `json:"capsuleImage"`
	BackgroundImage     string       `json:"backgroundImage"`
	IconURL             string       `json:"iconUrl,omitempty"`
	Screenshots         []string     `json:"screenshots"`
	Movies              []SteamMovie `json:"movies"`
	Genres              []string     `json:"genres"`
	Developers          []string     `json:"developers"`
	Publishers          []string     `json:"publishers"`
	ReleaseDate         string       `json:"releaseDate"`
	ControllerSupport   string       `json:"controllerSupport"` // "full", "partial", "none"
	PCRequirements      string       `json:"pcRequirements"`
	MetacriticScore     int          `json:"metacriticScore"`
	ReviewScoreDesc     string       `json:"reviewScoreDesc"`
	ReviewPercent       int          `json:"reviewPercent"`
	TotalReviews        int          `json:"totalReviews"`
	TotalPositive       int          `json:"totalPositive"`
	CachedAt            int64        `json:"cachedAt"`
}

// DownloadRecord represents a tracked download task
type DownloadRecord struct {
	ID              string `json:"id"`
	GameID          int64  `json:"gameId"`
	GameTitle       string `json:"gameTitle"`
	RemotePath      string `json:"remotePath"`
	LocalPath       string `json:"localPath"`
	TotalBytes      int64  `json:"totalBytes"`
	DownloadedBytes int64  `json:"downloadedBytes"`
	Status          string `json:"status"` // queued, downloading, paused, completed, failed, cancelled
	ErrorMessage    string `json:"errorMessage"`
	IsTorrent       bool   `json:"isTorrent"`
	MagnetURI       string `json:"magnetUri,omitempty"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
}

type Database struct {
	mu sync.RWMutex
	db *sql.DB
}

func InitDB(appDir string) (*Database, error) {
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	dbPath := filepath.Join(appDir, "gamevault.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}

	// Optimize SQLite performance & concurrency
	if _, err := db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		PRAGMA busy_timeout = 5000;
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure sqlite pragma: %w", err)
	}

	database := &Database{db: db}
	if err := database.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	return database, nil
}

func (d *Database) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *Database) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS games (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		raw_name TEXT NOT NULL,
		clean_title TEXT NOT NULL,
		search_title TEXT NOT NULL,
		remote_path TEXT NOT NULL UNIQUE,
		size_bytes INTEGER DEFAULT 0,
		size_display TEXT DEFAULT '',
		is_directory INTEGER DEFAULT 1,
		is_collection INTEGER DEFAULT 0,
		parent_path TEXT DEFAULT '',
		steam_appid INTEGER DEFAULT 0,
		steam_synced INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS steam_metadata (
		appid INTEGER PRIMARY KEY,
		title TEXT NOT NULL,
		short_description TEXT,
		detailed_description TEXT,
		header_image TEXT,
		capsule_image TEXT,
		background_image TEXT,
		screenshots TEXT,
		movies TEXT,
		genres TEXT,
		developers TEXT,
		publishers TEXT,
		release_date TEXT,
		controller_support TEXT,
		pc_requirements TEXT,
		metacritic_score INTEGER DEFAULT 0,
		cached_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS downloads (
		id TEXT PRIMARY KEY,
		game_id INTEGER DEFAULT 0,
		game_title TEXT NOT NULL,
		remote_path TEXT NOT NULL,
		local_path TEXT NOT NULL,
		total_bytes INTEGER DEFAULT 0,
		downloaded_bytes INTEGER DEFAULT 0,
		status TEXT DEFAULT 'queued',
		error_message TEXT DEFAULT '',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_games_clean_title ON games(clean_title);
	CREATE INDEX IF NOT EXISTS idx_games_steam_appid ON games(steam_appid);
	CREATE INDEX IF NOT EXISTS idx_games_steam_synced ON games(steam_synced);
	CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);
	`

	if _, err := d.db.Exec(schema); err != nil {
		return err
	}

	// Automatic schema migration for existing databases
	_, _ = d.db.Exec("ALTER TABLE steam_metadata ADD COLUMN movies TEXT DEFAULT '[]'")
	_, _ = d.db.Exec("ALTER TABLE steam_metadata ADD COLUMN review_score_desc TEXT DEFAULT ''")
	_, _ = d.db.Exec("ALTER TABLE steam_metadata ADD COLUMN review_percent INTEGER DEFAULT 0")
	_, _ = d.db.Exec("ALTER TABLE steam_metadata ADD COLUMN total_reviews INTEGER DEFAULT 0")
	_, _ = d.db.Exec("ALTER TABLE steam_metadata ADD COLUMN total_positive INTEGER DEFAULT 0")
	_, _ = d.db.Exec("ALTER TABLE steam_metadata ADD COLUMN icon_url TEXT DEFAULT ''")
	// Invalidate and delete any legacy cached steam_metadata where movies exist but lack modern HLS streams
	_, _ = d.db.Exec("DELETE FROM steam_metadata WHERE movies IS NOT NULL AND movies != '[]' AND movies != 'null' AND movies NOT LIKE '%hls%'")
	// Also mark those games as unsynced so background worker immediately refreshes them
	_, _ = d.db.Exec("UPDATE games SET steam_synced = 0 WHERE steam_appid > 0 AND steam_appid NOT IN (SELECT appid FROM steam_metadata)")
	// Invalidate horizontal capsules (e.g. 231x87, 616x353) stored in capsule_image so portrait 600x900 covers are used instead
	_, _ = d.db.Exec("UPDATE steam_metadata SET capsule_image = '' WHERE capsule_image LIKE '%capsule_231x87%' OR capsule_image LIKE '%capsule_616x353%' OR capsule_image LIKE '%capsule_467x181%' OR capsule_image LIKE '%header%'")

	// Torrent sources and downloads support
	_, _ = d.db.Exec("ALTER TABLE games ADD COLUMN source_type TEXT DEFAULT 'ftp'")
	_, _ = d.db.Exec("ALTER TABLE games ADD COLUMN torrent_source TEXT DEFAULT ''")
	_, _ = d.db.Exec("ALTER TABLE games ADD COLUMN uris TEXT DEFAULT '[]'")
	_, _ = d.db.Exec("ALTER TABLE games ADD COLUMN upload_date TEXT DEFAULT ''")
	_, _ = d.db.Exec("CREATE INDEX IF NOT EXISTS idx_games_source_type ON games(source_type)")
	_, _ = d.db.Exec("CREATE INDEX IF NOT EXISTS idx_games_torrent_source ON games(torrent_source)")

	_, _ = d.db.Exec("ALTER TABLE downloads ADD COLUMN is_torrent INTEGER DEFAULT 0")
	_, _ = d.db.Exec("ALTER TABLE downloads ADD COLUMN magnet_uri TEXT DEFAULT ''")
	_, _ = d.db.Exec("UPDATE steam_metadata SET title = '' WHERE title LIKE 'Steam App %'")

	// Favorites and offline backlog table
	_, _ = d.db.Exec(`
		CREATE TABLE IF NOT EXISTS favorites (
			game_id INTEGER PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'planned',
			game_data TEXT NOT NULL DEFAULT '',
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_favorites_status ON favorites(status);
	`)
	return nil
}

// UpsertGames updates or inserts remote game records
func (d *Database) UpsertGames(items []remote.RemoteItem) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO games (
			raw_name, clean_title, search_title, remote_path, size_bytes, size_display,
			is_directory, is_collection, parent_path, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(remote_path) DO UPDATE SET
			raw_name = excluded.raw_name,
			clean_title = excluded.clean_title,
			search_title = excluded.search_title,
			size_bytes = excluded.size_bytes,
			size_display = excluded.size_display,
			is_directory = excluded.is_directory,
			is_collection = excluded.is_collection,
			parent_path = excluded.parent_path,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		isDir := 0
		if item.IsDirectory {
			isDir = 1
		}
		isColl := 0
		if item.IsCollection {
			isColl = 1
		}

		_, err := stmt.Exec(
			item.RawName,
			item.CleanTitle,
			item.SearchTitle,
			item.RemotePath,
			item.SizeBytes,
			item.SizeDisplay,
			isDir,
			isColl,
			item.ParentPath,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ResetInvalidSteamMatches clears false steam matches (e.g. Assassins Guild)
func (d *Database) ResetInvalidSteamMatches() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		UPDATE games 
		SET steam_appid = 0, steam_synced = 0 
		WHERE id IN (
			SELECT g.id FROM games g
			JOIN steam_metadata s ON g.steam_appid = s.appid
			WHERE s.title = 'Assassins Guild' AND g.clean_title NOT LIKE '%Assassins Guild%'
		)
	`)
	return err
}

const gameSelectFields = `
	g.id, g.raw_name, g.clean_title, g.search_title, g.remote_path, g.size_bytes,
	g.size_display, g.is_directory, g.is_collection, g.parent_path, g.steam_appid, g.steam_synced,
	COALESCE(g.source_type, 'ftp'), COALESCE(g.torrent_source, ''), COALESCE(g.uris, '[]'), COALESCE(g.upload_date, ''),
	COALESCE(s.title, ''), COALESCE(s.short_description, ''), COALESCE(s.detailed_description, ''),
	COALESCE(s.header_image, ''), COALESCE(s.capsule_image, ''), COALESCE(s.background_image, ''),
	COALESCE(s.icon_url, ''),
	COALESCE(s.screenshots, '[]'), COALESCE(s.movies, '[]'), COALESCE(s.genres, '[]'), COALESCE(s.developers, '[]'),
	COALESCE(s.publishers, '[]'), COALESCE(s.release_date, ''), COALESCE(s.controller_support, ''),
	COALESCE(s.pc_requirements, ''), COALESCE(s.metacritic_score, 0),
	COALESCE(s.review_score_desc, ''), COALESCE(s.review_percent, 0), COALESCE(s.total_reviews, 0),
	COALESCE(f.status, '')
`

type rowScanner interface {
	Scan(dest ...any) error
}

func (d *Database) scanGame(scanner rowScanner) (GameEntity, error) {
	var g GameEntity
	var isDir, isColl, synced int
	var urisJSON string
	var screenshotsJSON, moviesJSON, genresJSON, devsJSON, pubsJSON string

	err := scanner.Scan(
		&g.ID, &g.RawName, &g.CleanTitle, &g.SearchTitle, &g.RemotePath, &g.SizeBytes,
		&g.SizeDisplay, &isDir, &isColl, &g.ParentPath, &g.SteamAppID, &synced,
		&g.SourceType, &g.TorrentSource, &urisJSON, &g.UploadDate,
		&g.SteamTitle, &g.ShortDescription, &g.DetailedDescription,
		&g.HeaderImage, &g.CapsuleImage, &g.BackgroundImage,
		&g.IconURL,
		&screenshotsJSON, &moviesJSON, &genresJSON, &devsJSON, &pubsJSON,
		&g.ReleaseDate, &g.ControllerSupport, &g.PCRequirements, &g.MetacriticScore,
		&g.ReviewScoreDesc, &g.ReviewPercent, &g.TotalReviews,
		&g.FavoriteStatus,
	)
	if err != nil {
		return g, err
	}

	g.IsDirectory = isDir == 1
	g.IsCollection = isColl == 1
	g.SteamSynced = synced == 1

	if g.SourceType == "" {
		g.SourceType = "ftp"
	}

	var uris []string
	if err := json.Unmarshal([]byte(urisJSON), &uris); err == nil && len(uris) > 0 {
		g.MagnetURI = uris[0]
	} else if strings.HasPrefix(g.RemotePath, "magnet:") {
		g.MagnetURI = g.RemotePath
	}

	if g.SourceType == "ftp" {
		if g.SizeDisplay == "Unknown" || (g.SizeBytes == 0 && g.RawName != "") || strings.Contains(g.CleanTitle, "GB") || strings.Contains(g.CleanTitle, "MB") {
			parsed := remote.ParseFolderName(g.RawName, g.RemotePath, g.IsDirectory)
			if parsed.SizeBytes > 0 {
				g.SizeBytes = parsed.SizeBytes
				g.SizeDisplay = parsed.SizeDisplay
			}
			if parsed.CleanTitle != "" {
				g.CleanTitle = parsed.CleanTitle
			}
		}
	}

	g.CleanTitle = remote.CleanDisplayTitle(g.CleanTitle)
	if g.CleanTitle == "" {
		g.CleanTitle = remote.CleanDisplayTitle(g.RawName)
	}
	g.SearchTitle = remote.SanitizeForSteamSearch(g.CleanTitle)

	_ = json.Unmarshal([]byte(screenshotsJSON), &g.Screenshots)
	_ = json.Unmarshal([]byte(moviesJSON), &g.Movies)
	_ = json.Unmarshal([]byte(genresJSON), &g.Genres)
	_ = json.Unmarshal([]byte(devsJSON), &g.Developers)
	_ = json.Unmarshal([]byte(pubsJSON), &g.Publishers)

	if g.CapsuleImage == "" && g.SteamAppID > 0 {
		g.CapsuleImage = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/library_600x900.jpg", g.SteamAppID)
	}
	if g.HeaderImage == "" && g.SteamAppID > 0 {
		g.HeaderImage = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/header.jpg", g.SteamAppID)
	}
	if g.BackgroundImage == "" && g.SteamAppID > 0 {
		g.BackgroundImage = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/page_bg_generated_v6b.jpg", g.SteamAppID)
	}

	if strings.HasPrefix(g.SteamTitle, "Steam App ") {
		g.SteamTitle = ""
	}

	return g, nil
}

// DeduplicateGames groups duplicates of the same game, keeping the entry with richest metadata as primary
// while merging Steam metadata and populating all versions into Variants
func DeduplicateGames(games []GameEntity) []GameEntity {
	if len(games) <= 1 {
		return games
	}

	type dsu struct {
		parent []int
	}
	newDSU := func(n int) *dsu {
		p := make([]int, n)
		for i := range p {
			p[i] = i
		}
		return &dsu{parent: p}
	}
	var find func(d *dsu, i int) int
	find = func(d *dsu, i int) int {
		if d.parent[i] == i {
			return i
		}
		d.parent[i] = find(d, d.parent[i])
		return d.parent[i]
	}
	union := func(d *dsu, i, j int) {
		rootI := find(d, i)
		rootJ := find(d, j)
		if rootI != rootJ {
			d.parent[rootI] = rootJ
		}
	}

	d := newDSU(len(games))
	keyToRoot := make(map[string]int)

	for i, g := range games {
		var keys []string
		if g.SteamAppID != 0 {
			keys = append(keys, fmt.Sprintf("appid:%d", g.SteamAppID))
		}
		if g.SteamTitle != "" && !strings.HasPrefix(g.SteamTitle, "Steam App ") {
			stk := remote.CleanCanonicalKey(g.SteamTitle)
			if stk != "" {
				keys = append(keys, fmt.Sprintf("steam:%s", stk))
			}
		}
		kClean := remote.CleanCanonicalKey(g.CleanTitle)
		if kClean != "" {
			keys = append(keys, fmt.Sprintf("title:%s", kClean))
		}
		kSearch := remote.CleanCanonicalKey(g.SearchTitle)
		if kSearch != "" && kSearch != kClean {
			keys = append(keys, fmt.Sprintf("title:%s", kSearch))
		}

		for _, k := range keys {
			if root, ok := keyToRoot[k]; ok {
				union(d, i, root)
			} else {
				keyToRoot[k] = i
			}
		}
	}

	groups := make(map[int][]GameEntity)
	var groupOrder []int
	seenRoots := make(map[int]bool)

	for i, g := range games {
		root := find(d, i)
		if !seenRoots[root] {
			seenRoots[root] = true
			groupOrder = append(groupOrder, root)
		}
		groups[root] = append(groups[root], g)
	}

	scoreGame := func(g GameEntity) int {
		score := 0
		if g.SteamAppID != 0 {
			score += 100
		}
		if g.SteamSynced {
			score += 50
		}
		if g.HeaderImage != "" {
			score += 50
		}
		if g.ShortDescription != "" {
			score += 30
		}
		if len(g.Screenshots) > 0 {
			score += 20
		}
		if g.IconURL != "" {
			score += 10
		}
		return score
	}

	result := make([]GameEntity, 0, len(groupOrder))
	for _, root := range groupOrder {
		items := groups[root]
		if len(items) == 0 {
			continue
		}

		// Find best primary game
		bestIdx := 0
		bestScore := scoreGame(items[0])
		for idx := 1; idx < len(items); idx++ {
			sc := scoreGame(items[idx])
			if sc > bestScore || (sc == bestScore && items[idx].ID > items[bestIdx].ID) {
				bestScore = sc
				bestIdx = idx
			}
		}
		primary := items[bestIdx]

		// Inherit missing metadata onto primary from siblings
		for _, item := range items {
			if primary.SteamAppID == 0 && item.SteamAppID != 0 {
				primary.SteamAppID = item.SteamAppID
				primary.SteamSynced = item.SteamSynced
				primary.SteamTitle = item.SteamTitle
				primary.ShortDescription = item.ShortDescription
				primary.DetailedDescription = item.DetailedDescription
				primary.HeaderImage = item.HeaderImage
				primary.CapsuleImage = item.CapsuleImage
				primary.BackgroundImage = item.BackgroundImage
				primary.IconURL = item.IconURL
				primary.Screenshots = item.Screenshots
				primary.Movies = item.Movies
				primary.Genres = item.Genres
				primary.Developers = item.Developers
				primary.Publishers = item.Publishers
				primary.ReleaseDate = item.ReleaseDate
				primary.ControllerSupport = item.ControllerSupport
				primary.PCRequirements = item.PCRequirements
				primary.MetacriticScore = item.MetacriticScore
				primary.ReviewScoreDesc = item.ReviewScoreDesc
				primary.ReviewPercent = item.ReviewPercent
				primary.TotalReviews = item.TotalReviews
			}
			if primary.IconURL == "" && item.IconURL != "" {
				primary.IconURL = item.IconURL
			}
			if primary.HeaderImage == "" && item.HeaderImage != "" {
				primary.HeaderImage = item.HeaderImage
			}
			if primary.CapsuleImage == "" && item.CapsuleImage != "" {
				primary.CapsuleImage = item.CapsuleImage
			}
			if primary.BackgroundImage == "" && item.BackgroundImage != "" {
				primary.BackgroundImage = item.BackgroundImage
			}
			if primary.FavoriteStatus == "" && item.FavoriteStatus != "" {
				primary.FavoriteStatus = item.FavoriteStatus
			}
		}

		if strings.HasPrefix(primary.SteamTitle, "Steam App ") {
			primary.SteamTitle = ""
		}

		// Build variants slice
		variants := make([]GameVariant, 0, len(items))
		seenVariants := make(map[string]bool)
		for _, item := range items {
			vKey := fmt.Sprintf("%d|%s|%d", item.ID, strings.TrimSpace(item.RawName), item.SizeBytes)
			if seenVariants[vKey] {
				continue
			}
			seenVariants[vKey] = true
			variants = append(variants, GameVariant{
				ID:            item.ID,
				RawName:       item.RawName,
				CleanTitle:    item.CleanTitle,
				SizeBytes:     item.SizeBytes,
				SizeDisplay:   item.SizeDisplay,
				SourceType:    item.SourceType,
				TorrentSource: item.TorrentSource,
				RemotePath:    item.RemotePath,
				MagnetURI:     item.MagnetURI,
				UploadDate:    item.UploadDate,
				IsDirectory:   item.IsDirectory,
				SteamAppID:    item.SteamAppID,
			})
		}
		primary.Variants = variants
		result = append(result, primary)
	}

	return result
}

// GetAllGames returns all FTP games with joined Steam metadata
func (d *Database) GetAllGames() ([]GameEntity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
		SELECT ` + gameSelectFields + `
		FROM games g
		LEFT JOIN steam_metadata s ON g.steam_appid = s.appid AND g.steam_appid != 0
		LEFT JOIN favorites f ON g.id = f.game_id
		WHERE g.source_type = 'ftp' OR g.source_type = '' OR g.source_type IS NULL
		ORDER BY g.id DESC
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return make([]GameEntity, 0), err
	}
	defer rows.Close()

	games := make([]GameEntity, 0)
	for rows.Next() {
		g, err := d.scanGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, g)
	}

	return DeduplicateGames(games), nil
}

// GetTorrentGames returns all Torrent games with joined Steam metadata
func (d *Database) GetTorrentGames() ([]GameEntity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
		SELECT ` + gameSelectFields + `
		FROM games g
		LEFT JOIN steam_metadata s ON g.steam_appid = s.appid AND g.steam_appid != 0
		LEFT JOIN favorites f ON g.id = f.game_id
		WHERE g.source_type = 'torrent'
		ORDER BY g.id DESC
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return make([]GameEntity, 0), err
	}
	defer rows.Close()

	games := make([]GameEntity, 0)
	for rows.Next() {
		g, err := d.scanGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, g)
	}

	return DeduplicateGames(games), nil
}

// GetGameByID returns a single game entity with full details (FTP or Torrent)
func (d *Database) GetGameByID(id int64) (*GameEntity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.getGameByIDInternal(id)
}

func (d *Database) getGameByIDInternal(id int64) (*GameEntity, error) {
	query := `
		SELECT ` + gameSelectFields + `
		FROM games g
		LEFT JOIN steam_metadata s ON g.steam_appid = s.appid AND g.steam_appid != 0
		LEFT JOIN favorites f ON g.id = f.game_id
		WHERE g.id = ?
	`

	row := d.db.QueryRow(query, id)
	g, err := d.scanGame(row)
	if err != nil {
		return nil, err
	}

	// Attach duplicate variants for selection in UI
	canonicalKey := remote.CleanCanonicalKey(g.CleanTitle)
	if canonicalKey == "" {
		canonicalKey = g.SearchTitle
	}

	var vQuery string
	var args []interface{}
	if g.SteamAppID != 0 {
		vQuery = `SELECT ` + gameSelectFields + ` FROM games g LEFT JOIN steam_metadata s ON g.steam_appid = s.appid AND g.steam_appid != 0 LEFT JOIN favorites f ON g.id = f.game_id WHERE g.steam_appid = ? OR (g.search_title = ? AND g.search_title != '') ORDER BY g.id DESC`
		args = []interface{}{g.SteamAppID, g.SearchTitle}
	} else if g.SearchTitle != "" {
		vQuery = `SELECT ` + gameSelectFields + ` FROM games g LEFT JOIN steam_metadata s ON g.steam_appid = s.appid AND g.steam_appid != 0 LEFT JOIN favorites f ON g.id = f.game_id WHERE g.search_title = ? ORDER BY g.id DESC`
		args = []interface{}{g.SearchTitle}
	}

	if vQuery != "" {
		if vRows, vErr := d.db.Query(vQuery, args...); vErr == nil {
			var siblingGames []GameEntity
			for vRows.Next() {
				if sg, sgErr := d.scanGame(vRows); sgErr == nil {
					sKey := remote.CleanCanonicalKey(sg.CleanTitle)
					sameAppID := g.SteamAppID != 0 && sg.SteamAppID == g.SteamAppID
					sameKey := canonicalKey != "" && (sKey == canonicalKey || remote.CleanCanonicalKey(sg.SearchTitle) == canonicalKey)
					sameSteamTitle := g.SteamTitle != "" && !strings.HasPrefix(g.SteamTitle, "Steam App ") &&
						remote.CleanCanonicalKey(sg.SteamTitle) == remote.CleanCanonicalKey(g.SteamTitle)
					if sameAppID || sameKey || sameSteamTitle {
						siblingGames = append(siblingGames, sg)
					}
				}
			}
			vRows.Close()

			if len(siblingGames) > 1 {
				variants := make([]GameVariant, 0, len(siblingGames))
				seenVariants := make(map[string]bool)
				for _, item := range siblingGames {
					key := fmt.Sprintf("%s|%d", strings.TrimSpace(item.RawName), item.SizeBytes)
					if seenVariants[key] {
						continue
					}
					seenVariants[key] = true
					variants = append(variants, GameVariant{
						ID:            item.ID,
						RawName:       item.RawName,
						CleanTitle:    item.CleanTitle,
						SizeBytes:     item.SizeBytes,
						SizeDisplay:   item.SizeDisplay,
						SourceType:    item.SourceType,
						TorrentSource: item.TorrentSource,
						RemotePath:    item.RemotePath,
						MagnetURI:     item.MagnetURI,
						UploadDate:    item.UploadDate,
						IsDirectory:   item.IsDirectory,
						SteamAppID:    item.SteamAppID,
					})
				}
				g.Variants = variants
			}
		}
	}

	return &g, nil
}

// SetFavorite adds or updates a game in favorites with a status and cached game data JSON
func (d *Database) SetFavorite(gameID int64, status string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Get game entity snapshot for complete offline resilience
	var gameDataJSON string
	g, err := d.getGameByIDInternal(gameID)
	if err == nil && g != nil {
		g.FavoriteStatus = status
		if b, bErr := json.Marshal(g); bErr == nil {
			gameDataJSON = string(b)
		}
	}

	stmt := `
		INSERT INTO favorites (game_id, status, game_data, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(game_id) DO UPDATE SET
			status = excluded.status,
			game_data = CASE WHEN excluded.game_data != '' THEN excluded.game_data ELSE favorites.game_data END,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err = d.db.Exec(stmt, gameID, status, gameDataJSON)
	return err
}

// RemoveFavorite removes a game from favorites
func (d *Database) RemoveFavorite(gameID int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM favorites WHERE game_id = ?", gameID)
	return err
}

// GetFavorites returns all favorite items with full metadata (offline-first)
func (d *Database) GetFavorites() ([]FavoriteItem, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT game_id, status, game_data, COALESCE(added_at, ''), COALESCE(updated_at, '') 
		FROM favorites 
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return make([]FavoriteItem, 0), err
	}
	defer rows.Close()

	var results []FavoriteItem
	for rows.Next() {
		var item FavoriteItem
		var gameDataJSON, addedAt, updatedAt string
		if err := rows.Scan(&item.GameID, &item.Status, &gameDataJSON, &addedAt, &updatedAt); err != nil {
			continue
		}
		item.AddedAt = addedAt
		item.UpdatedAt = updatedAt

		// Try to fetch live game data if available
		g, err := d.getGameByIDInternal(item.GameID)
		if err == nil && g != nil {
			g.FavoriteStatus = item.Status
			item.Game = *g
		} else if gameDataJSON != "" {
			var cachedGame GameEntity
			if err := json.Unmarshal([]byte(gameDataJSON), &cachedGame); err == nil {
				cachedGame.FavoriteStatus = item.Status
				item.Game = cachedGame
			}
		}
		results = append(results, item)
	}
	return results, nil
}

// UpsertTorrentGames adds or updates games imported from a Hydra torrent source
func (d *Database) UpsertTorrentGames(sourceID, sourceName string, items []HydraDownloadItem) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO games (
			raw_name, clean_title, search_title, remote_path, size_bytes, size_display,
			is_directory, is_collection, parent_path, steam_appid, steam_synced,
			source_type, torrent_source, uris, upload_date, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 0, 0, ?, ?, ?, 'torrent', ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(remote_path) DO UPDATE SET
			raw_name = excluded.raw_name,
			clean_title = excluded.clean_title,
			search_title = excluded.search_title,
			size_bytes = excluded.size_bytes,
			size_display = excluded.size_display,
			torrent_source = excluded.torrent_source,
			uris = excluded.uris,
			upload_date = excluded.upload_date,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Pre-load known Steam metadata titles into memory for lightning-fast O(1) matching
	cachedMetadata := make(map[string]int)
	if rows, err := tx.Query(`SELECT LOWER(title), appid FROM steam_metadata WHERE appid > 0`); err == nil {
		for rows.Next() {
			var t string
			var aid int
			if err := rows.Scan(&t, &aid); err == nil && aid > 0 {
				cachedMetadata[t] = aid
			}
		}
		rows.Close()
	}

	for _, item := range items {
		if len(item.URIs) == 0 || strings.TrimSpace(item.URIs[0]) == "" {
			continue
		}
		primaryURI := strings.TrimSpace(item.URIs[0])
		urisJSON, _ := json.Marshal(item.URIs)

		cleanTitle := remote.CleanDisplayTitle(item.Title)
		searchTitle := remote.SanitizeForSteamSearch(cleanTitle)
		sizeBytes := remote.ParseFileSize(item.FileSize)
		sizeDisplay := strings.TrimSpace(item.FileSize)
		if sizeDisplay == "" && sizeBytes > 0 {
			sizeDisplay = formatBytes(sizeBytes)
		}

		// Fast in-memory check if steam_metadata already has this title to reuse cache immediately
		steamAppID := 0
		steamSynced := 0
		if aid, ok := cachedMetadata[strings.ToLower(cleanTitle)]; ok && aid > 0 {
			steamAppID = aid
			steamSynced = 1
		} else if aid, ok := cachedMetadata[strings.ToLower(searchTitle)]; ok && aid > 0 {
			steamAppID = aid
			steamSynced = 1
		}

		if _, err := stmt.Exec(
			item.Title, cleanTitle, searchTitle, primaryURI, sizeBytes, sizeDisplay,
			sourceName, steamAppID, steamSynced,
			sourceID, string(urisJSON), item.UploadDate,
		); err != nil {
			log.Printf("[Database] Failed to upsert torrent item %s: %v", item.Title, err)
			continue
		}
	}

	return tx.Commit()
}

// DeleteTorrentGamesBySource prunes games belonging to a specific torrent source
func (d *Database) DeleteTorrentGamesBySource(sourceID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`DELETE FROM games WHERE source_type = 'torrent' AND torrent_source = ?`, sourceID)
	return err
}


// GetUnsyncedGames returns list of games needing Steam metadata (skipping duplicate releases to optimize API requests)
func (d *Database) GetUnsyncedGames() ([]GameEntity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
		SELECT g.id, g.clean_title, g.search_title, g.steam_appid
		FROM games g
		LEFT JOIN steam_metadata s ON g.steam_appid = s.appid
		WHERE g.steam_synced = 0
		   OR (g.steam_appid > 0 AND (s.appid IS NULL OR s.title = '' OR (s.short_description = '' AND s.detailed_description = '')))
		ORDER BY g.id ASC
	`
	rows, err := d.db.Query(query)
	if err != nil {
		return make([]GameEntity, 0), err
	}
	defer rows.Close()

	seenKeys := make(map[string]bool)
	games := make([]GameEntity, 0)
	for rows.Next() {
		var g GameEntity
		if err := rows.Scan(&g.ID, &g.CleanTitle, &g.SearchTitle, &g.SteamAppID); err != nil {
			return nil, err
		}
		key := remote.CleanCanonicalKey(g.CleanTitle)
		if key == "" {
			key = g.SearchTitle
		}
		if key != "" {
			if seenKeys[key] {
				continue
			}
			seenKeys[key] = true
		}
		games = append(games, g)
	}
	return games, nil
}

// SetGameAppID assigns a Steam AppID to a game and all its duplicate releases in DB
func (d *Database) SetGameAppID(gameID int64, appID int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE games SET steam_appid = ?, steam_synced = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, appID, gameID)
	if err != nil {
		return err
	}

	// Also propagate AppID and synced status to all duplicate sibling rows in DB
	var clean, search string
	_ = d.db.QueryRow(`SELECT clean_title, search_title FROM games WHERE id = ?`, gameID).Scan(&clean, &search)
	if clean != "" || search != "" {
		if search != "" {
			_, _ = d.db.Exec(`UPDATE games SET steam_appid = ?, steam_synced = 1, updated_at = CURRENT_TIMESTAMP WHERE search_title = ? AND (steam_appid = 0 OR steam_synced = 0)`, appID, search)
		}
		canonicalKey := remote.CleanCanonicalKey(clean)
		if canonicalKey != "" {
			if rows, sErr := d.db.Query(`SELECT id, clean_title FROM games WHERE steam_appid <= 0 OR steam_synced = 0`); sErr == nil {
				var siblingIDs []int64
				for rows.Next() {
					var sID int64
					var sClean string
					if rows.Scan(&sID, &sClean) == nil {
						if remote.CleanCanonicalKey(sClean) == canonicalKey {
							siblingIDs = append(siblingIDs, sID)
						}
					}
				}
				rows.Close()
				if len(siblingIDs) > 0 {
					placeholders := make([]string, len(siblingIDs))
					args := make([]interface{}, len(siblingIDs)+1)
					args[0] = appID
					for i, sID := range siblingIDs {
						placeholders[i] = "?"
						args[i+1] = sID
					}
					query := fmt.Sprintf("UPDATE games SET steam_appid = ?, steam_synced = 1, updated_at = CURRENT_TIMESTAMP WHERE id IN (%s)", strings.Join(placeholders, ","))
					_, _ = d.db.Exec(query, args...)
				}
			}
		}
	}

	return nil
}

// SyncDuplicateGamesMetadata propagates Steam AppID and synced status to all duplicate rows in DB
func (d *Database) SyncDuplicateGamesMetadata() (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(`SELECT clean_title, search_title, steam_appid FROM games WHERE steam_appid != 0`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	keyToAppID := make(map[string]int)
	searchToAppID := make(map[string]int)
	for rows.Next() {
		var clean, search string
		var appID int
		if err := rows.Scan(&clean, &search, &appID); err == nil && appID != 0 {
			key := remote.CleanCanonicalKey(clean)
			if key != "" {
				keyToAppID[key] = appID
			}
			if search != "" {
				searchToAppID[search] = appID
			}
		}
	}

	unsyncedRows, err := d.db.Query(`SELECT id, clean_title, search_title FROM games WHERE steam_appid = 0 OR steam_synced = 0`)
	if err != nil {
		return 0, err
	}
	defer unsyncedRows.Close()

	var toUpdate []struct {
		id    int64
		appID int
	}

	for unsyncedRows.Next() {
		var id int64
		var clean, search string
		if err := unsyncedRows.Scan(&id, &clean, &search); err == nil {
			appID := 0
			if a, ok := keyToAppID[remote.CleanCanonicalKey(clean)]; ok {
				appID = a
			} else if a, ok := searchToAppID[search]; ok {
				appID = a
			}
			if appID != 0 {
				toUpdate = append(toUpdate, struct {
					id    int64
					appID int
				}{id: id, appID: appID})
			}
		}
	}

	var syncedCount int64
	for _, item := range toUpdate {
		res, err := d.db.Exec(`UPDATE games SET steam_appid = ?, steam_synced = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, item.appID, item.id)
		if err == nil {
			if aff, _ := res.RowsAffected(); aff > 0 {
				syncedCount += aff
			}
		}
	}

	return syncedCount, nil
}

// MarkGameSynced marks a game as synced (even if no Steam AppID was found)
func (d *Database) MarkGameSynced(gameID int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE games SET steam_synced = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, gameID)
	return err
}

// ResetGameMetadata unlinks steam metadata from a game and marks it unsynced for fresh lookup
func (d *Database) ResetGameMetadata(gameID int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE games SET steam_appid = 0, steam_synced = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, gameID)
	return err
}

// ResetUnmatchedGames resets all games without a Steam AppID back to unsynced so the new engine can match them
func (d *Database) ResetUnmatchedGames() (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Re-sanitize search_title for all games in DB to ensure newly added sanitization rules take effect
	if rows, err := d.db.Query(`SELECT id, clean_title FROM games`); err == nil {
		for rows.Next() {
			var id int64
			var clean string
			if err := rows.Scan(&id, &clean); err == nil {
				sanitized := remote.SanitizeForSteamSearch(clean)
				if sanitized != "" {
					_, _ = d.db.Exec(`UPDATE games SET search_title = ? WHERE id = ? AND search_title != ?`, sanitized, id, sanitized)
				}
			}
		}
		rows.Close()
	}

	res, err := d.db.Exec(`
		UPDATE games SET steam_synced = 0, updated_at = CURRENT_TIMESTAMP
		WHERE steam_appid = 0
		   OR id IN (
			   SELECT g.id FROM games g
			   LEFT JOIN steam_metadata s ON g.steam_appid = s.appid
			   WHERE g.steam_appid > 0 AND (s.appid IS NULL OR s.title = '' OR (s.short_description = '' AND s.detailed_description = ''))
		   )
	`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PurgeMismatchedMetadata detects and resets any existing corrupt or false metadata matches in DB
func (d *Database) PurgeMismatchedMetadata(similarityChecker func(query, candidate string) float64, threshold float64) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(`
		SELECT g.id, g.clean_title, COALESCE(s.title, ''), g.steam_appid
		FROM games g
		LEFT JOIN steam_metadata s ON g.steam_appid = s.appid
		WHERE g.steam_appid > 0
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type mismatch struct {
		id    int64
		clean string
		steam string
		appID int
	}

	var mismatches []mismatch
	for rows.Next() {
		var m mismatch
		if err := rows.Scan(&m.id, &m.clean, &m.steam, &m.appID); err == nil {
			if strings.TrimSpace(m.steam) == "" {
				// Metadata was never successfully fetched or is empty -> mark for fresh re-sync
				mismatches = append(mismatches, m)
				continue
			}
			scoreClean := similarityChecker(m.clean, m.steam)
			sanitized := remote.SanitizeForSteamSearch(m.clean)
			scoreSanitized := similarityChecker(sanitized, m.steam)
			bestScore := scoreClean
			if scoreSanitized > bestScore {
				bestScore = scoreSanitized
			}

			// If the game title was sanitized to a base game, but the matched Steam item is an addon/DLC,
			// scoreSanitized will heavily penalize the DLC. Reject if scoreSanitized < threshold.
			if scoreSanitized < threshold || bestScore < threshold {
				mismatches = append(mismatches, m)
			}
		}
	}

	purged := 0
	for _, m := range mismatches {
		_, err := d.db.Exec(`UPDATE games SET steam_appid = 0, steam_synced = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, m.id)
		if err == nil {
			purged++
		}
	}

	return purged, nil
}

// SaveSteamMetadata saves Steam store details to cache
func (d *Database) SaveSteamMetadata(meta SteamMetadata) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	screenshotsJSON, _ := json.Marshal(meta.Screenshots)
	moviesJSON, _ := json.Marshal(meta.Movies)
	genresJSON, _ := json.Marshal(meta.Genres)
	devsJSON, _ := json.Marshal(meta.Developers)
	pubsJSON, _ := json.Marshal(meta.Publishers)

	_, err := d.db.Exec(`
		INSERT INTO steam_metadata (
			appid, title, short_description, detailed_description, header_image, capsule_image,
			background_image, icon_url, screenshots, movies, genres, developers, publishers, release_date,
			controller_support, pc_requirements, metacritic_score,
			review_score_desc, review_percent, total_reviews, total_positive, cached_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(appid) DO UPDATE SET
			title = CASE WHEN excluded.title != '' THEN excluded.title ELSE steam_metadata.title END,
			short_description = CASE WHEN excluded.short_description != '' THEN excluded.short_description ELSE steam_metadata.short_description END,
			detailed_description = CASE WHEN excluded.detailed_description != '' THEN excluded.detailed_description ELSE steam_metadata.detailed_description END,
			header_image = CASE WHEN excluded.header_image != '' THEN excluded.header_image ELSE steam_metadata.header_image END,
			capsule_image = CASE WHEN excluded.capsule_image != '' THEN excluded.capsule_image ELSE steam_metadata.capsule_image END,
			background_image = CASE WHEN excluded.background_image != '' THEN excluded.background_image ELSE steam_metadata.background_image END,
			icon_url = CASE WHEN excluded.icon_url != '' THEN excluded.icon_url ELSE steam_metadata.icon_url END,
			screenshots = CASE WHEN excluded.screenshots != '[]' AND excluded.screenshots != '' THEN excluded.screenshots ELSE steam_metadata.screenshots END,
			movies = CASE WHEN excluded.movies != '[]' AND excluded.movies != '' THEN excluded.movies ELSE steam_metadata.movies END,
			genres = CASE WHEN excluded.genres != '[]' AND excluded.genres != '' THEN excluded.genres ELSE steam_metadata.genres END,
			developers = CASE WHEN excluded.developers != '[]' AND excluded.developers != '' THEN excluded.developers ELSE steam_metadata.developers END,
			publishers = CASE WHEN excluded.publishers != '[]' AND excluded.publishers != '' THEN excluded.publishers ELSE steam_metadata.publishers END,
			release_date = CASE WHEN excluded.release_date != '' THEN excluded.release_date ELSE steam_metadata.release_date END,
			controller_support = CASE WHEN excluded.controller_support != '' THEN excluded.controller_support ELSE steam_metadata.controller_support END,
			pc_requirements = CASE WHEN excluded.pc_requirements != '' THEN excluded.pc_requirements ELSE steam_metadata.pc_requirements END,
			metacritic_score = CASE WHEN excluded.metacritic_score > 0 THEN excluded.metacritic_score ELSE steam_metadata.metacritic_score END,
			review_score_desc = CASE WHEN excluded.review_score_desc != '' THEN excluded.review_score_desc ELSE steam_metadata.review_score_desc END,
			review_percent = CASE WHEN excluded.review_percent > 0 THEN excluded.review_percent ELSE steam_metadata.review_percent END,
			total_reviews = CASE WHEN excluded.total_reviews > 0 THEN excluded.total_reviews ELSE steam_metadata.total_reviews END,
			total_positive = CASE WHEN excluded.total_positive > 0 THEN excluded.total_positive ELSE steam_metadata.total_positive END,
			cached_at = excluded.cached_at
	`,
		meta.AppID, meta.Title, meta.ShortDescription, meta.DetailedDescription,
		meta.HeaderImage, meta.CapsuleImage, meta.BackgroundImage, meta.IconURL,
		string(screenshotsJSON), string(moviesJSON), string(genresJSON), string(devsJSON), string(pubsJSON),
		meta.ReleaseDate, meta.ControllerSupport, meta.PCRequirements, meta.MetacriticScore,
		meta.ReviewScoreDesc, meta.ReviewPercent, meta.TotalReviews, meta.TotalPositive,
		time.Now().Unix(),
	)

	return err
}

// GetSteamMetadataFromCache retrieves cached Steam metadata for an AppID
func (d *Database) GetSteamMetadataFromCache(appID int) (*SteamMetadata, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	row := d.db.QueryRow(`
		SELECT appid, title, short_description, detailed_description, header_image, capsule_image,
		       background_image, COALESCE(icon_url, ''), screenshots, COALESCE(movies, '[]'), genres, developers, publishers, release_date,
		       controller_support, pc_requirements, metacritic_score,
		       COALESCE(review_score_desc, ''), COALESCE(review_percent, 0), COALESCE(total_reviews, 0), COALESCE(total_positive, 0),
		       cached_at
		FROM steam_metadata WHERE appid = ?
	`, appID)

	var m SteamMetadata
	var screenshotsJSON, moviesJSON, genresJSON, devsJSON, pubsJSON string

	err := row.Scan(
		&m.AppID, &m.Title, &m.ShortDescription, &m.DetailedDescription,
		&m.HeaderImage, &m.CapsuleImage, &m.BackgroundImage,
		&m.IconURL,
		&screenshotsJSON, &moviesJSON, &genresJSON, &devsJSON, &pubsJSON,
		&m.ReleaseDate, &m.ControllerSupport, &m.PCRequirements, &m.MetacriticScore,
		&m.ReviewScoreDesc, &m.ReviewPercent, &m.TotalReviews, &m.TotalPositive,
		&m.CachedAt,
	)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(screenshotsJSON), &m.Screenshots)
	_ = json.Unmarshal([]byte(moviesJSON), &m.Movies)
	_ = json.Unmarshal([]byte(genresJSON), &m.Genres)
	_ = json.Unmarshal([]byte(devsJSON), &m.Developers)
	_ = json.Unmarshal([]byte(pubsJSON), &m.Publishers)

	// If cached movies exist but lack modern HLS streams, treat cache as expired so fresh streams are fetched
	if len(m.Movies) > 0 {
		hasHLS := false
		for _, mov := range m.Movies {
			if strings.TrimSpace(mov.HLS) != "" {
				hasHLS = true
				break
			}
		}
		if !hasHLS {
			return nil, nil
		}
	}

	return &m, nil
}

// UpdateSteamMetadataCover updates the capsule_image for a given appID
func (d *Database) UpdateSteamMetadataCover(appID int, coverURL string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE steam_metadata SET capsule_image = ? WHERE appid = ?`, coverURL, appID)
	return err
}

// UpdateSteamMetadataIcon updates the icon_url for a given appID
func (d *Database) UpdateSteamMetadataIcon(appID int, iconURL string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE steam_metadata SET icon_url = ? WHERE appid = ?`, iconURL, appID)
	return err
}

// UpdateSteamMetadataTitle updates the title for a given appID
func (d *Database) UpdateSteamMetadataTitle(appID int, title string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE steam_metadata SET title = ? WHERE appid = ?`, title, appID)
	return err
}

type MissingIconItem struct {
	GameID int64
	AppID  int
	Title  string
}

// GetGamesMissingIcons returns distinct games that have SteamAppID but lack an icon or title
func (d *Database) GetGamesMissingIcons() ([]MissingIconItem, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT g.id, g.steam_appid, COALESCE(NULLIF(s.title, ''), g.clean_title)
		FROM games g
		JOIN steam_metadata s ON g.steam_appid = s.appid
		WHERE (s.icon_url IS NULL OR s.icon_url = '' OR s.title = '') AND g.steam_appid != 0
		GROUP BY g.steam_appid
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []MissingIconItem
	for rows.Next() {
		var it MissingIconItem
		if err := rows.Scan(&it.GameID, &it.AppID, &it.Title); err == nil {
			items = append(items, it)
		}
	}
	return items, nil
}

// UpdateSteamReviewSummary updates review score description, percent and counts for an appID
func (d *Database) UpdateSteamReviewSummary(appID int, desc string, percent, total, positive int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		UPDATE steam_metadata
		SET review_score_desc = ?, review_percent = ?, total_reviews = ?, total_positive = ?
		WHERE appid = ?
	`, desc, percent, total, positive, appID)
	return err
}


// ==========================================
// Downloads Storage
// ==========================================

func (d *Database) SaveDownloadRecord(rec DownloadRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().Unix()
	if rec.CreatedAt == 0 {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now

	isTorrentInt := 0
	if rec.IsTorrent {
		isTorrentInt = 1
	}

	_, err := d.db.Exec(`
		INSERT INTO downloads (
			id, game_id, game_title, remote_path, local_path, total_bytes, downloaded_bytes,
			status, error_message, is_torrent, magnet_uri, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			local_path = excluded.local_path,
			total_bytes = excluded.total_bytes,
			downloaded_bytes = excluded.downloaded_bytes,
			status = excluded.status,
			error_message = excluded.error_message,
			is_torrent = excluded.is_torrent,
			magnet_uri = excluded.magnet_uri,
			updated_at = excluded.updated_at
	`,
		rec.ID, rec.GameID, rec.GameTitle, rec.RemotePath, rec.LocalPath,
		rec.TotalBytes, rec.DownloadedBytes, rec.Status, rec.ErrorMessage,
		isTorrentInt, rec.MagnetURI,
		rec.CreatedAt, rec.UpdatedAt,
	)
	return err
}

func (d *Database) GetAllDownloads() ([]DownloadRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, game_id, game_title, remote_path, local_path, total_bytes, downloaded_bytes,
		       status, error_message, COALESCE(is_torrent, 0), COALESCE(magnet_uri, ''), created_at, updated_at
		FROM downloads
		ORDER BY created_at DESC
	`)
	if err != nil {
		return make([]DownloadRecord, 0), err
	}
	defer rows.Close()

	records := make([]DownloadRecord, 0)
	for rows.Next() {
		var r DownloadRecord
		var isTorrentInt int
		if err := rows.Scan(
			&r.ID, &r.GameID, &r.GameTitle, &r.RemotePath, &r.LocalPath,
			&r.TotalBytes, &r.DownloadedBytes, &r.Status, &r.ErrorMessage,
			&isTorrentInt, &r.MagnetURI,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		r.IsTorrent = isTorrentInt == 1
		records = append(records, r)
	}

	return records, nil
}

func (d *Database) DeleteDownloadRecord(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`DELETE FROM downloads WHERE id = ?`, id)
	return err
}

func (d *Database) ClearCompletedDownloads() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`DELETE FROM downloads WHERE status IN ('completed', 'cancelled')`)
	return err
}

// GetDownloadRecordByGameID retrieves the latest download record for a game
func (d *Database) GetDownloadRecordByGameID(gameID int64) (*DownloadRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	row := d.db.QueryRow(`
		SELECT id, game_id, game_title, remote_path, local_path, total_bytes, downloaded_bytes,
		       status, error_message, COALESCE(is_torrent, 0), COALESCE(magnet_uri, ''), created_at, updated_at
		FROM downloads
		WHERE game_id = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, gameID)

	var r DownloadRecord
	var isTorrentInt int
	if err := row.Scan(
		&r.ID, &r.GameID, &r.GameTitle, &r.RemotePath, &r.LocalPath,
		&r.TotalBytes, &r.DownloadedBytes, &r.Status, &r.ErrorMessage,
		&isTorrentInt, &r.MagnetURI,
		&r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		return nil, err
	}
	r.IsTorrent = isTorrentInt == 1
	return &r, nil
}

func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
		tb = 1024 * gb
	)
	switch {
	case bytes >= tb:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(tb))
	case bytes >= gb:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}


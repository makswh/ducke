package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	// Invalidate and delete any legacy cached steam_metadata where movies exist but lack modern HLS streams
	_, _ = d.db.Exec("DELETE FROM steam_metadata WHERE movies IS NOT NULL AND movies != '[]' AND movies != 'null' AND movies NOT LIKE '%hls%'")
	// Also mark those games as unsynced so background worker immediately refreshes them
	_, _ = d.db.Exec("UPDATE games SET steam_synced = 0 WHERE steam_appid > 0 AND steam_appid NOT IN (SELECT appid FROM steam_metadata)")
	// Invalidate horizontal capsules (e.g. 231x87, 616x353) stored in capsule_image so portrait 600x900 covers are used instead
	_, _ = d.db.Exec("UPDATE steam_metadata SET capsule_image = '' WHERE capsule_image LIKE '%capsule_231x87%' OR capsule_image LIKE '%capsule_616x353%' OR capsule_image LIKE '%capsule_467x181%' OR capsule_image LIKE '%header%'")
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

// GetAllGames returns all games with joined Steam metadata
func (d *Database) GetAllGames() ([]GameEntity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
		SELECT 
			g.id, g.raw_name, g.clean_title, g.search_title, g.remote_path, g.size_bytes,
			g.size_display, g.is_directory, g.is_collection, g.parent_path, g.steam_appid, g.steam_synced,
			COALESCE(s.title, ''), COALESCE(s.short_description, ''), COALESCE(s.detailed_description, ''),
			COALESCE(s.header_image, ''), COALESCE(s.capsule_image, ''), COALESCE(s.background_image, ''),
			COALESCE(s.screenshots, '[]'), COALESCE(s.movies, '[]'), COALESCE(s.genres, '[]'), COALESCE(s.developers, '[]'),
			COALESCE(s.publishers, '[]'), COALESCE(s.release_date, ''), COALESCE(s.controller_support, ''),
			COALESCE(s.pc_requirements, ''), COALESCE(s.metacritic_score, 0),
			COALESCE(s.review_score_desc, ''), COALESCE(s.review_percent, 0), COALESCE(s.total_reviews, 0)
		FROM games g
		LEFT JOIN steam_metadata s ON g.steam_appid = s.appid AND g.steam_appid != 0
		ORDER BY g.id DESC
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return make([]GameEntity, 0), err
	}
	defer rows.Close()

	games := make([]GameEntity, 0)
	for rows.Next() {
		var g GameEntity
		var isDir, isColl, synced int
		var screenshotsJSON, moviesJSON, genresJSON, devsJSON, pubsJSON string

		err := rows.Scan(
			&g.ID, &g.RawName, &g.CleanTitle, &g.SearchTitle, &g.RemotePath, &g.SizeBytes,
			&g.SizeDisplay, &isDir, &isColl, &g.ParentPath, &g.SteamAppID, &synced,
			&g.SteamTitle, &g.ShortDescription, &g.DetailedDescription,
			&g.HeaderImage, &g.CapsuleImage, &g.BackgroundImage,
			&screenshotsJSON, &moviesJSON, &genresJSON, &devsJSON, &pubsJSON,
			&g.ReleaseDate, &g.ControllerSupport, &g.PCRequirements, &g.MetacriticScore,
			&g.ReviewScoreDesc, &g.ReviewPercent, &g.TotalReviews,
		)
		if err != nil {
			return nil, err
		}

		g.IsDirectory = isDir == 1
		g.IsCollection = isColl == 1
		g.SteamSynced = synced == 1

		// Auto-heal title and size from RawName if previous parser left artifacts or "Unknown"
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

		// Fallback capsule & background image if empty but SteamAppID exists
		if g.CapsuleImage == "" && g.SteamAppID > 0 {
			g.CapsuleImage = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/library_600x900.jpg", g.SteamAppID)
		}
		if g.HeaderImage == "" && g.SteamAppID > 0 {
			g.HeaderImage = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/header.jpg", g.SteamAppID)
		}
		if g.BackgroundImage == "" && g.SteamAppID > 0 {
			g.BackgroundImage = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/page_bg_generated_v6b.jpg", g.SteamAppID)
		}

		games = append(games, g)
	}

	return games, nil
}

// GetGameByID returns a single game entity with full details
func (d *Database) GetGameByID(id int64) (*GameEntity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
		SELECT 
			g.id, g.raw_name, g.clean_title, g.search_title, g.remote_path, g.size_bytes,
			g.size_display, g.is_directory, g.is_collection, g.parent_path, g.steam_appid, g.steam_synced,
			COALESCE(s.title, ''), COALESCE(s.short_description, ''), COALESCE(s.detailed_description, ''),
			COALESCE(s.header_image, ''), COALESCE(s.capsule_image, ''), COALESCE(s.background_image, ''),
			COALESCE(s.screenshots, '[]'), COALESCE(s.movies, '[]'), COALESCE(s.genres, '[]'), COALESCE(s.developers, '[]'),
			COALESCE(s.publishers, '[]'), COALESCE(s.release_date, ''), COALESCE(s.controller_support, ''),
			COALESCE(s.pc_requirements, ''), COALESCE(s.metacritic_score, 0),
			COALESCE(s.review_score_desc, ''), COALESCE(s.review_percent, 0), COALESCE(s.total_reviews, 0)
		FROM games g
		LEFT JOIN steam_metadata s ON g.steam_appid = s.appid AND g.steam_appid != 0
		WHERE g.id = ?
	`

	var g GameEntity
	var isDir, isColl, synced int
	var screenshotsJSON, moviesJSON, genresJSON, devsJSON, pubsJSON string

	err := d.db.QueryRow(query, id).Scan(
		&g.ID, &g.RawName, &g.CleanTitle, &g.SearchTitle, &g.RemotePath, &g.SizeBytes,
		&g.SizeDisplay, &isDir, &isColl, &g.ParentPath, &g.SteamAppID, &synced,
		&g.SteamTitle, &g.ShortDescription, &g.DetailedDescription,
		&g.HeaderImage, &g.CapsuleImage, &g.BackgroundImage,
		&screenshotsJSON, &moviesJSON, &genresJSON, &devsJSON, &pubsJSON,
		&g.ReleaseDate, &g.ControllerSupport, &g.PCRequirements, &g.MetacriticScore,
		&g.ReviewScoreDesc, &g.ReviewPercent, &g.TotalReviews,
	)
	if err != nil {
		return nil, err
	}

	g.IsDirectory = isDir == 1
	g.IsCollection = isColl == 1
	g.SteamSynced = synced == 1

	// Auto-heal title and size from RawName
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

	return &g, nil
}

// GetUnsyncedGames returns list of games needing Steam metadata
func (d *Database) GetUnsyncedGames() ([]GameEntity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT id, clean_title, search_title, steam_appid FROM games WHERE steam_synced = 0 LIMIT 50`)
	if err != nil {
		return make([]GameEntity, 0), err
	}
	defer rows.Close()

	games := make([]GameEntity, 0)
	for rows.Next() {
		var g GameEntity
		if err := rows.Scan(&g.ID, &g.CleanTitle, &g.SearchTitle, &g.SteamAppID); err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, nil
}

// SetGameAppID assigns a Steam AppID to a game and marks it synced
func (d *Database) SetGameAppID(gameID int64, appID int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE games SET steam_appid = ?, steam_synced = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, appID, gameID)
	return err
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

	res, err := d.db.Exec(`UPDATE games SET steam_synced = 0, updated_at = CURRENT_TIMESTAMP WHERE steam_appid = 0`)
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
			score := similarityChecker(m.clean, m.steam)
			if score < threshold {
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
			background_image, screenshots, movies, genres, developers, publishers, release_date,
			controller_support, pc_requirements, metacritic_score,
			review_score_desc, review_percent, total_reviews, total_positive, cached_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(appid) DO UPDATE SET
			title = excluded.title,
			short_description = excluded.short_description,
			detailed_description = excluded.detailed_description,
			header_image = excluded.header_image,
			capsule_image = excluded.capsule_image,
			background_image = excluded.background_image,
			screenshots = excluded.screenshots,
			movies = excluded.movies,
			genres = excluded.genres,
			developers = excluded.developers,
			publishers = excluded.publishers,
			release_date = excluded.release_date,
			controller_support = excluded.controller_support,
			pc_requirements = excluded.pc_requirements,
			metacritic_score = excluded.metacritic_score,
			review_score_desc = excluded.review_score_desc,
			review_percent = excluded.review_percent,
			total_reviews = excluded.total_reviews,
			total_positive = excluded.total_positive,
			cached_at = excluded.cached_at
	`,
		meta.AppID, meta.Title, meta.ShortDescription, meta.DetailedDescription,
		meta.HeaderImage, meta.CapsuleImage, meta.BackgroundImage,
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
		       background_image, screenshots, COALESCE(movies, '[]'), genres, developers, publishers, release_date,
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

	_, err := d.db.Exec(`
		INSERT INTO downloads (
			id, game_id, game_title, remote_path, local_path, total_bytes, downloaded_bytes,
			status, error_message, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			downloaded_bytes = excluded.downloaded_bytes,
			status = excluded.status,
			error_message = excluded.error_message,
			updated_at = excluded.updated_at
	`,
		rec.ID, rec.GameID, rec.GameTitle, rec.RemotePath, rec.LocalPath,
		rec.TotalBytes, rec.DownloadedBytes, rec.Status, rec.ErrorMessage,
		rec.CreatedAt, rec.UpdatedAt,
	)
	return err
}

func (d *Database) GetAllDownloads() ([]DownloadRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, game_id, game_title, remote_path, local_path, total_bytes, downloaded_bytes,
		       status, error_message, created_at, updated_at
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
		if err := rows.Scan(
			&r.ID, &r.GameID, &r.GameTitle, &r.RemotePath, &r.LocalPath,
			&r.TotalBytes, &r.DownloadedBytes, &r.Status, &r.ErrorMessage,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
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
		       status, error_message, created_at, updated_at
		FROM downloads
		WHERE game_id = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, gameID)

	var r DownloadRecord
	if err := row.Scan(
		&r.ID, &r.GameID, &r.GameTitle, &r.RemotePath, &r.LocalPath,
		&r.TotalBytes, &r.DownloadedBytes, &r.Status, &r.ErrorMessage,
		&r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &r, nil
}

package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestLegacyDatabaseMigration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ducke_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "gamevault.db")

	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open raw sqlite: %v", err)
	}

	legacySchema := `
	CREATE TABLE games (
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
	CREATE INDEX idx_games_clean_title ON games(clean_title);
	CREATE INDEX idx_games_steam_appid ON games(steam_appid);
	CREATE INDEX idx_games_steam_synced ON games(steam_synced);

	INSERT INTO games (raw_name, clean_title, search_title, remote_path) 
	VALUES ('Test.Game.v1.0-Repack', 'Test Game', 'Test Game', 'magnet:?xt=urn:btih:123');
	`
	if _, err := rawDB.Exec(legacySchema); err != nil {
		rawDB.Close()
		t.Fatalf("failed to create legacy db: %v", err)
	}
	rawDB.Close()

	db, err := InitDB(tempDir)
	if err != nil {
		t.Fatalf("InitDB failed on legacy database: %v", err)
	}
	defer db.Close()

	db.BackfillCanonicalKeys()

	var canonicalKey string
	err = db.db.QueryRow(`SELECT canonical_key FROM games WHERE id = 1`).Scan(&canonicalKey)
	if err != nil {
		t.Fatalf("failed to query canonical_key: %v", err)
	}
	if canonicalKey == "" {
		t.Errorf("expected canonical_key to be populated, got empty string")
	}

	_, err = db.GetAllGames()
	if err != nil {
		t.Fatalf("GetAllGames failed: %v", err)
	}

	_, err = db.GetTorrentGames()
	if err != nil {
		t.Fatalf("GetTorrentGames failed: %v", err)
	}
}

func TestUpsertTorrentGames(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ducke_upsert_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := InitDB(tempDir)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	items := []HydraDownloadItem{
		{
			Title:      "Half-Life 2: Episode Two (2007) PC [RePack]",
			URIs:       []string{"magnet:?xt=urn:btih:456abc&dn=HL2"},
			FileSize:   "4.5 GB",
			UploadDate: "2024-01-01",
		},
		{
			Title:      "Portal 2 Complete Edition",
			URIs:       []string{"magnet:?xt=urn:btih:789def&dn=Portal2"},
			FileSize:   "6.8 GB",
			UploadDate: "2024-02-01",
		},
	}

	err = db.UpsertTorrentGames("src_test", "Test Source", items)
	if err != nil {
		t.Fatalf("UpsertTorrentGames failed: %v", err)
	}

	torrents, err := db.GetTorrentGames()
	if err != nil {
		t.Fatalf("GetTorrentGames failed: %v", err)
	}

	if len(torrents) != 2 {
		t.Fatalf("expected 2 torrents, got %d", len(torrents))
	}

	// Verify canonical key and size display
	for _, game := range torrents {
		if game.CanonicalKey == "" {
			t.Errorf("expected non-empty canonical key for game %s", game.CleanTitle)
		}
		if game.SizeDisplay == "" {
			t.Errorf("expected non-empty size display for game %s", game.CleanTitle)
		}
	}
}

func TestMatchLibraryGame(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ducke_match_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := InitDB(tempDir)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	items := []HydraDownloadItem{
		{
			Title:      "Zuma Deluxe (2003) [P] [RUS / ENG]",
			URIs:       []string{"magnet:?xt=urn:btih:111aaa"},
			FileSize:   "150 MB",
			UploadDate: "2024-01-01",
		},
		{
			Title:      "Cave Story+: Doukutsu Monogatari",
			URIs:       []string{"magnet:?xt=urn:btih:222bbb"},
			FileSize:   "250 MB",
			UploadDate: "2024-01-01",
		},
	}

	if err := db.UpsertTorrentGames("src_test", "Test Source", items); err != nil {
		t.Fatalf("failed to upsert games: %v", err)
	}

	// 1. Exact match
	m1 := db.MatchLibraryGame("Zuma Deluxe")
	if m1 == nil {
		t.Fatalf("expected Zuma Deluxe to match")
	}

	// 2. Subtitle / punctuation variation match
	m2 := db.MatchLibraryGame("Cave Story: Doukutsu Monogatari")
	if m2 == nil {
		t.Fatalf("expected Cave Story: Doukutsu Monogatari to match Cave Story+: Doukutsu Monogatari")
	}

	// 3. Negative match
	m3 := db.MatchLibraryGame("Half-Life 3")
	if m3 != nil {
		t.Fatalf("expected Half-Life 3 to not match")
	}
}


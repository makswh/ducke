package metadata

import (
	"strings"
	"testing"
)

func TestSteamGridDB_FindNeedForSpeedCarbon(t *testing.T) {
	service := NewSteamGridDBService("")

	// User test case: Need for Speed Carbon
	assets, err := service.FindAssetsForGame("Need for Speed Carbon")
	if err != nil {
		t.Fatalf("expected to find Need for Speed Carbon on SteamGridDB, got error: %v", err)
	}

	if assets == nil {
		t.Fatalf("expected non-nil assets")
	}

	t.Logf("Found Game: %s (ID: %d)", assets.GameTitle, assets.GameID)
	t.Logf("Cover: %s", assets.CoverURL)
	t.Logf("Background: %s", assets.BackgroundURL)
	t.Logf("Header: %s", assets.HeaderURL)

	if !strings.Contains(strings.ToLower(assets.GameTitle), "need for speed") {
		t.Errorf("expected title to contain 'need for speed', got %q", assets.GameTitle)
	}

	if assets.CoverURL == "" {
		t.Errorf("expected valid CoverURL from SteamGridDB")
	}

	if assets.BackgroundURL == "" {
		t.Errorf("expected valid BackgroundURL from SteamGridDB")
	}

	if !strings.HasPrefix(assets.CoverURL, "https://cdn2.steamgriddb.com/") {
		t.Errorf("expected CoverURL from steamgriddb CDN, got %q", assets.CoverURL)
	}

	if !strings.HasPrefix(assets.BackgroundURL, "https://cdn2.steamgriddb.com/") {
		t.Errorf("expected BackgroundURL from steamgriddb CDN, got %q", assets.BackgroundURL)
	}
}

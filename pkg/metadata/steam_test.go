package metadata

import (
	"strings"
	"testing"
)

func TestFetchAppDetails_RegionLockedGame(t *testing.T) {
	service := NewSteamService(nil)

	// Helldivers 2 (AppID 553850) is restricted in many countries including RU
	meta, err := service.FetchAppDetails(553850)
	if err != nil {
		t.Fatalf("expected successful metadata fetch for region-locked game 553850, got err: %v", err)
	}

	if meta == nil {
		t.Fatalf("expected non-nil metadata")
	}

	if meta.AppID != 553850 {
		t.Errorf("expected AppID 553850, got %d", meta.AppID)
	}

	if !strings.Contains(strings.ToLower(meta.Title), "helldivers") {
		t.Errorf("expected title to contain 'helldivers', got %q", meta.Title)
	}

	if meta.HeaderImage == "" {
		t.Errorf("expected non-empty HeaderImage")
	}

	if meta.CapsuleImage == "" {
		t.Errorf("expected non-empty CapsuleImage")
	}

	if len(meta.Screenshots) == 0 {
		t.Errorf("expected screenshots to be populated")
	}

	if meta.TotalReviews == 0 {
		t.Errorf("expected TotalReviews > 0, got %d", meta.TotalReviews)
	}

	if meta.ReviewPercent == 0 {
		t.Errorf("expected ReviewPercent > 0, got %d", meta.ReviewPercent)
	}

	if meta.ReviewScoreDesc == "" {
		t.Errorf("expected non-empty ReviewScoreDesc")
	}
}

func TestFetchAppDetails_StoreUnlistedGame(t *testing.T) {
	service := NewSteamService(nil)

	// Spec Ops: The Line (AppID 50300) is unlisted from normal store browsing,
	// but fully accessible via cc=US query
	meta, err := service.FetchAppDetails(50300)
	if err != nil {
		t.Fatalf("expected metadata for unlisted game 50300, got err: %v", err)
	}

	if meta == nil {
		t.Fatalf("expected non-nil metadata")
	}

	if meta.AppID != 50300 {
		t.Errorf("expected AppID 50300, got %d", meta.AppID)
	}

	if !strings.Contains(strings.ToLower(meta.Title), "spec ops") {
		t.Errorf("expected title to contain 'spec ops', got %q", meta.Title)
	}

	if meta.HeaderImage == "" {
		t.Errorf("expected valid header image")
	}

	if meta.CapsuleImage == "" {
		t.Errorf("expected valid capsule image")
	}
}

func TestFetchAppDetails_DelistedGameFallback(t *testing.T) {
	service := NewSteamService(nil)

	// Deadpool (AppID 224060) is completely delisted from Steam Store API in all regions,
	// but public community hub and CDN assets exist
	meta, err := service.FetchAppDetails(224060)
	if err != nil {
		t.Fatalf("expected fallback metadata for delisted game 224060, got err: %v", err)
	}

	if meta == nil {
		t.Fatalf("expected non-nil metadata")
	}

	if meta.AppID != 224060 {
		t.Errorf("expected AppID 224060, got %d", meta.AppID)
	}

	if !strings.Contains(strings.ToLower(meta.Title), "deadpool") {
		t.Errorf("expected title to contain 'deadpool', got %q", meta.Title)
	}

	if meta.HeaderImage == "" || !strings.Contains(meta.HeaderImage, "224060/header.jpg") {
		t.Errorf("expected valid CDN header image, got %q", meta.HeaderImage)
	}

	if meta.CapsuleImage == "" || !strings.Contains(meta.CapsuleImage, "224060/capsule_616x353.jpg") {
		t.Errorf("expected valid CDN capsule image, got %q", meta.CapsuleImage)
	}
}

func TestVerifyCDNAsset(t *testing.T) {
	service := NewSteamService(nil)

	// Existing asset on CDN
	validURL := "https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/50300/header.jpg"
	if !service.verifyCDNAsset(validURL) {
		t.Errorf("expected CDN asset %s to exist", validURL)
	}

	// Non-existent asset on CDN
	invalidURL := "https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/999999999999/header.jpg"
	if service.verifyCDNAsset(invalidURL) {
		t.Errorf("expected non-existent CDN asset %s to return false", invalidURL)
	}
}

func TestCommunityGameTitleScraping(t *testing.T) {
	service := NewSteamService(nil)

	title := service.fetchCommunityGameTitle(224060)
	if !strings.Contains(strings.ToLower(title), "deadpool") {
		t.Errorf("expected community title to contain 'deadpool', got %q", title)
	}
}

func TestIsHorizontalAsset(t *testing.T) {
	horizontalURLs := []string{
		"https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/3596700/capsule_231x87.jpg",
		"https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/3596700/capsule_616x353.jpg",
		"https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/3596700/capsule_467x181.jpg",
		"https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/3596700/header.jpg",
		"https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/3596700/header_alt_assets_3_russian.jpg",
	}
	for _, u := range horizontalURLs {
		if !IsHorizontalAsset(u) {
			t.Errorf("expected %s to be recognized as horizontal asset", u)
		}
	}

	verticalURLs := []string{
		"https://shared.steamstatic.com/store_item_assets/steam/apps/50300/library_600x900.jpg",
		"https://cdn2.steamgriddb.com/grid/f1c6016367172d511769a8651cbde007.png",
	}
	for _, u := range verticalURLs {
		if IsHorizontalAsset(u) {
			t.Errorf("expected %s NOT to be recognized as horizontal asset", u)
		}
	}
}

func TestSteamGridDB_ResolveMissingSteamCovers(t *testing.T) {
	service := NewSteamService(nil)

	// Games from user screenshot where Steam lacks 600x900 portrait covers
	games := []string{
		"TerraTech Legion",
		"Lootbound",
		"How to Fish",
	}

	for _, g := range games {
		cover, err := service.GetSteamGridCover(g)
		if err != nil {
			t.Fatalf("failed to resolve SteamGridDB cover for %q: %v", g, err)
		}
		if cover == "" {
			t.Fatalf("empty SteamGridDB cover for %q", g)
		}
		if IsHorizontalAsset(cover) {
			t.Fatalf("cover for %q must be vertical, got horizontal URL: %s", g, cover)
		}
		t.Logf("Game %q -> SGDB Cover: %s", g, cover)
	}
}

func TestSteamGridDB_Banner(t *testing.T) {
	service := NewSteamService(nil)
	banner, err := service.GetSteamGridBanner("TerraTech Legion")
	if err != nil {
		t.Fatalf("failed to resolve SteamGridDB banner for TerraTech Legion: %v", err)
	}
	if banner == "" {
		t.Fatalf("empty banner for TerraTech Legion")
	}
	t.Logf("TerraTech Legion -> SGDB Banner: %s", banner)
}

func TestSteamGridDB_Logo(t *testing.T) {
	service := NewSteamService(nil)
	logo, err := service.GetSteamGridLogo("TerraTech Legion")
	if err != nil {
		t.Logf("TerraTech Legion SGDB logo note: %v", err)
	} else {
		t.Logf("TerraTech Legion -> SGDB Logo: %s", logo)
	}

	// Also test with a well-known title that definitely has a logo on SteamGridDB
	logoWk, err := service.GetSteamGridLogo("Cyberpunk 2077")
	if err != nil {
		t.Fatalf("Cyberpunk 2077 SGDB logo failed: %v", err)
	}
	if logoWk == "" {
		t.Fatalf("empty logo for Cyberpunk 2077")
	}
	t.Logf("Cyberpunk 2077 -> SGDB Logo: %s", logoWk)
}


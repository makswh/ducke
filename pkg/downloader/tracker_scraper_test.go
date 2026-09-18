package downloader

import (
	"context"
	"testing"
	"time"
)

func TestTrackerScraper_CacheAndBatch(t *testing.T) {
	scraper := NewTrackerScraper()

	// 1. Invalid or empty magnet
	res := scraper.ScrapeMagnet(context.Background(), "")
	if res.Success || res.Seeders != 0 {
		t.Errorf("expected false/0 for empty magnet, got %+v", res)
	}

	invalidRes := scraper.ScrapeMagnet(context.Background(), "invalid-magnet-string")
	if invalidRes.Success || invalidRes.Seeders != 0 {
		t.Errorf("expected false/0 for invalid magnet, got %+v", invalidRes)
	}

	// 2. Pre-fill cache manually to test cache hits
	ihHex := "3b245504cf5f11bbdbe1201cea6a6bf45a0b7758"
	scraper.cache[ihHex] = cachedScrape{
		seeders:   42,
		leechers:  7,
		fetchedAt: time.Now(),
	}

	magnet := "magnet:?xt=urn:btih:3b245504cf5f11bbdbe1201cea6a6bf45a0b7758&dn=Ubuntu"
	cachedRes := scraper.ScrapeMagnet(context.Background(), magnet)
	if !cachedRes.Success || cachedRes.Seeders != 42 || cachedRes.Leechers != 7 {
		t.Errorf("expected cached result seeds=42, leech=7, got %+v", cachedRes)
	}

	// 3. Batch scraping with duplicates and empty items
	queries := []TorrentSeedQuery{
		{ID: 101, MagnetURI: magnet},
		{ID: 102, MagnetURI: magnet}, // duplicate hash
		{ID: 103, MagnetURI: ""},     // empty
	}

	batchRes := scraper.ScrapeBatch(context.Background(), queries)
	if len(batchRes) != 3 {
		t.Fatalf("expected 3 results, got %d", len(batchRes))
	}
	if batchRes[101].Seeders != 42 || !batchRes[101].Success {
		t.Errorf("ID 101 expected 42 seeds, got %+v", batchRes[101])
	}
	if batchRes[102].Seeders != 42 || !batchRes[102].Success {
		t.Errorf("ID 102 expected 42 seeds, got %+v", batchRes[102])
	}
	if batchRes[103].Success {
		t.Errorf("ID 103 expected success=false, got %+v", batchRes[103])
	}
}

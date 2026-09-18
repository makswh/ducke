package downloader

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/tracker"
	"github.com/anacrolix/torrent/types/infohash"
)

// TorrentSeedQuery specifies a variant or game ID and its magnet URI to query
type TorrentSeedQuery struct {
	ID        int64  `json:"id"`
	MagnetURI string `json:"magnetUri"`
}

// TorrentSeedResult holds seeder and leecher counts for a queried torrent
type TorrentSeedResult struct {
	Seeders  int  `json:"seeders"`
	Leechers int  `json:"leechers"`
	Success  bool `json:"success"`
}

type cachedScrape struct {
	seeders   int
	leechers  int
	fetchedAt time.Time
}

// TrackerScraper queries BitTorrent trackers via BEP 15 UDP/HTTP scrape protocol
// and caches results in-memory with a TTL to avoid tracker spamming.
type TrackerScraper struct {
	mu    sync.RWMutex
	cache map[string]cachedScrape // infohash hex -> cached result
	ttl   time.Duration
}

// NewTrackerScraper creates a new TrackerScraper with default 5-minute TTL
func NewTrackerScraper() *TrackerScraper {
	return &TrackerScraper{
		cache: make(map[string]cachedScrape),
		ttl:   5 * time.Minute,
	}
}

// DefaultScrapeTrackers are top reliable public trackers to supplement magnet trackers
var DefaultScrapeTrackers = []string{
	"http://bt2.t-ru.org/ann",
	"udp://zer0day.ch:1337/announce",
	"udp://tracker2.dler.org:80/announce",
	"udp://tracker.wildkat.net:6969/announce",
	"udp://tracker.torrent.eu.org:451/announce",
	"udp://tracker.qu.ax:6969/announce",
	"udp://tracker.publictracker.xyz:6969/announce",
	"udp://tracker.peerfect.org:6969/announce",
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://tracker.opentrackr.com:6969/announce",
	"udp://tracker.ilibr.org:6969/announce",
	"udp://tracker.gmi.gd:6969/announce",
	"udp://tracker.ducks.party:1984/announce",
	"udp://tracker.dler.org:6969/announce",
	"udp://tracker.corpscorp.online:80/announce",
	"udp://tracker.bittor.pw:1337/announce",
	"udp://tracker.auctor.tv:6969/announce",
	"udp://tracker.0x7c0.com:6969/announce",
	"udp://tracker-udp.gbitt.info:80/announce",
	"udp://tr4ck3r.duckdns.org:6969/announce",
	"udp://torrentclub.online:54123/announce",
	"udp://open.tracker.cl:1337/announce",
	"udp://open.stealth.si:80/announce",
	"udp://open.demonii.com:1337/announce",
	"http://tracker.opentrackr.org:1337/announce",
	"http://open.tracker.cl:1337/announce",
	"udp://opentor.net:6969/announce",
	"udp://explodie.org:6969/announce",
	"udp://tracker.openbittorrent.com:6969/announce",
	"udp://retracker.lanta-net.ru:2710/announce",
}

// ScrapeMagnet performs a non-blocking tracker scrape for the given magnet URI
func (ts *TrackerScraper) ScrapeMagnet(ctx context.Context, magnetURI string) TorrentSeedResult {
	magnetURI = strings.TrimSpace(magnetURI)
	if magnetURI == "" || !strings.HasPrefix(magnetURI, "magnet:") {
		return TorrentSeedResult{Seeders: 0, Leechers: 0, Success: false}
	}

	mag, err := metainfo.ParseMagnetUri(magnetURI)
	if err != nil {
		return TorrentSeedResult{Seeders: 0, Leechers: 0, Success: false}
	}

	ih := infohash.T(mag.InfoHash)
	if ih.IsZero() {
		return TorrentSeedResult{Seeders: 0, Leechers: 0, Success: false}
	}

	ihHex := ih.HexString()

	// Check cache
	ts.mu.RLock()
	if item, found := ts.cache[ihHex]; found {
		if time.Since(item.fetchedAt) < ts.ttl {
			ts.mu.RUnlock()
			return TorrentSeedResult{
				Seeders:  item.seeders,
				Leechers: item.leechers,
				Success:  true,
			}
		}
	}
	ts.mu.RUnlock()

	// Gather tracker list: magnet's trackers first, then defaults
	seen := make(map[string]bool)
	var trackerList []string

	addTracker := func(u string) {
		u = strings.TrimSpace(u)
		if u != "" && !seen[u] {
			seen[u] = true
			trackerList = append(trackerList, u)
		}
	}

	for _, tr := range mag.Trackers {
		addTracker(tr)
	}
	for _, tr := range DefaultScrapeTrackers {
		addTracker(tr)
	}

	// Limit to max 12 trackers per torrent to prevent excessive network spam while ensuring good coverage
	if len(trackerList) > 12 {
		trackerList = trackerList[:12]
	}

	// Query trackers in parallel with per-tracker timeout
	var (
		resMu       sync.Mutex
		maxSeeds    int
		maxLeech    int
		successResp bool
		wg          sync.WaitGroup
	)

	// Scrape timeout of 2.2s per tracker or caller's ctx
	scrapeCtx, cancel := context.WithTimeout(ctx, 2200*time.Millisecond)
	defer cancel()

	for _, trURL := range trackerList {
		wg.Add(1)
		go func(urlStr string) {
			defer wg.Done()

			cl, err := tracker.NewClient(urlStr, tracker.NewClientOpts{})
			if err != nil {
				return
			}
			defer cl.Close()

			resp, err := cl.Scrape(scrapeCtx, []infohash.T{ih})
			if err != nil || len(resp) == 0 {
				return
			}

			seeds := int(resp[0].Seeders)
			leech := int(resp[0].Leechers)

			resMu.Lock()
			successResp = true
			if seeds > maxSeeds {
				maxSeeds = seeds
			}
			if leech > maxLeech {
				maxLeech = leech
			}
			resMu.Unlock()
		}(trURL)
	}

	wg.Wait()

	// Store in cache if succeeded, or even if 0 seeds reported by responsive trackers
	if successResp {
		ts.mu.Lock()
		ts.cache[ihHex] = cachedScrape{
			seeders:   maxSeeds,
			leechers:  maxLeech,
			fetchedAt: time.Now(),
		}
		ts.mu.Unlock()
	}

	return TorrentSeedResult{
		Seeders:  maxSeeds,
		Leechers: maxLeech,
		Success:  successResp,
	}
}

// ScrapeBatch queries tracker seeds for a batch of magnet queries in parallel
func (ts *TrackerScraper) ScrapeBatch(ctx context.Context, queries []TorrentSeedQuery) map[int64]TorrentSeedResult {
	results := make(map[int64]TorrentSeedResult, len(queries))
	if len(queries) == 0 {
		return results
	}

	// Overall batch timeout: 3.5 seconds
	batchCtx, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
	defer cancel()

	// Deduplicate by MagnetURI to avoid duplicate tracker scrapes for same hash
	type hashGroup struct {
		magnet string
		ids    []int64
	}
	grouped := make(map[string]*hashGroup)
	for _, q := range queries {
		mag := strings.TrimSpace(q.MagnetURI)
		if mag == "" {
			results[q.ID] = TorrentSeedResult{Seeders: 0, Leechers: 0, Success: false}
			continue
		}
		if g, exists := grouped[mag]; exists {
			g.ids = append(g.ids, q.ID)
		} else {
			grouped[mag] = &hashGroup{
				magnet: mag,
				ids:    []int64{q.ID},
			}
		}
	}

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, 4) // max 4 distinct torrents scraped concurrently
	)

	for _, g := range grouped {
		wg.Add(1)
		go func(grp *hashGroup) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-batchCtx.Done():
				return
			}

			res := ts.ScrapeMagnet(batchCtx, grp.magnet)

			mu.Lock()
			for _, id := range grp.ids {
				results[id] = res
			}
			mu.Unlock()
		}(g)
	}

	wg.Wait()

	// Ensure all requested IDs have an entry
	for _, q := range queries {
		if _, ok := results[q.ID]; !ok {
			results[q.ID] = TorrentSeedResult{Seeders: 0, Leechers: 0, Success: false}
		}
	}

	return results
}

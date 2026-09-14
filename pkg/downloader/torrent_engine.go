package downloader

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"
)

// PublicTier1Trackers is a curated list of reliable, high-availability public BitTorrent trackers
// that dramatically expands swarm discovery and ensures steady seed redundancy across all regions.
var PublicTier1Trackers = [][]string{
	{"udp://tracker.opentrackr.org:1337/announce"},
	{"udp://open.stealth.si:80/announce"},
	{"udp://tracker.torrent.eu.org:451/announce"},
	{"udp://tracker.openbittorrent.com:6969/announce"},
	{"udp://explodie.org:6969/announce"},
	{"udp://tracker.coppersurfer.tk:6969/announce"},
	{"udp://tracker.internetwarriors.net:1337/announce"},
	{"udp://tracker.moeking.me:6969/announce"},
	{"udp://opentor.net:6969/announce"},
	{"udp://retracker.lanta-net.ru:2710/announce"},
	{"udp://tracker.cyberia.is:6969/announce"},
	{"http://tracker.openbittorrent.com:80/announce"},
}

// engineCompletionStores maps *TorrentEngine -> *sync.Map of downloadID -> storage.PieceCompletion.
// This avoids modifying the TorrentEngine struct layout while still tracking completion handles.
var (
	engineStoresMu sync.Mutex
	engineStores   = map[*TorrentEngine]*sync.Map{}
)

func getCompletionStore(te *TorrentEngine) *sync.Map {
	engineStoresMu.Lock()
	defer engineStoresMu.Unlock()
	if m, ok := engineStores[te]; ok {
		return m
	}
	m := &sync.Map{}
	engineStores[te] = m
	return m
}

// TorrentEngine encapsulates anacrolix/torrent client lifecycle, storage and rate limiting
type TorrentEngine struct {
	client      *torrent.Client
	rateLimiter *rate.Limiter
	dataDir     string
	mu          sync.RWMutex
	torrents    map[string]*torrent.Torrent // downloadID -> Torrent
}

// NewTorrentEngine initializes a background BitTorrent client with dynamic speed limiter
func NewTorrentEngine(dataDir string, initialSpeedLimitKBps int) (*TorrentEngine, error) {
	if dataDir == "" {
		dataDir = "."
	}
	_ = os.MkdirAll(dataDir, 0755)

	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = dataDir

	// 1. Swarm Connection Limits & Peer Watermarks
	// Modern clients use 80-150 connections per torrent to aggregate bandwidth across many seeders.
	cfg.EstablishedConnsPerTorrent = 120
	cfg.HalfOpenConnsPerTorrent = 50
	cfg.TotalHalfOpenConns = 150
	cfg.TorrentPeersHighWater = 1000
	cfg.TorrentPeersLowWater = 150
	cfg.HandshakesTimeout = 4 * time.Second
	cfg.NominalDialTimeout = 10 * time.Second
	cfg.MinDialTimeout = 2 * time.Second

	// 2. Multi-core Piece Verification & In-flight Data Buffering
	numWorkers := runtime.NumCPU()
	if numWorkers < 4 {
		numWorkers = 4
	} else if numWorkers > 8 {
		numWorkers = 8
	}
	cfg.PieceHashersPerTorrent = numWorkers
	cfg.MaxUnverifiedBytes = 64 * 1024 * 1024

	// 3. DHT Continuous Discovery & Re-announcing
	// Periodically re-announce to DHT to maintain an active, fresh swarm as peers disconnect
	cfg.PeriodicallyAnnounceTorrentsToDht = true

	// 4. Rate Limiter with generous burst to prevent thread sleep stutters
	var limiter *rate.Limiter
	if initialSpeedLimitKBps > 0 {
		burst := initialSpeedLimitKBps * 1024 * 2
		if burst < 4*1024*1024 {
			burst = 4 * 1024 * 1024
		}
		limiter = rate.NewLimiter(rate.Limit(initialSpeedLimitKBps*1024), burst)
	} else {
		limiter = rate.NewLimiter(rate.Inf, 16*1024*1024)
	}
	cfg.DownloadRateLimiter = limiter
	cfg.NoUpload = false
	cfg.DisableTCP = false
	cfg.DisableUTP = false
	cfg.DropDuplicatePeerIds = true

	client, err := torrent.NewClient(cfg)
	if err != nil {
		log.Printf("[Torrent] Failed on default port (%v), retrying with dynamic port :0...", err)
		cfg.SetListenAddr(":0")
		client, err = torrent.NewClient(cfg)
	}
	if err != nil {
		log.Printf("[Torrent] Failed to initialize anacrolix/torrent client: %v", err)
		return nil, fmt.Errorf("failed to init torrent client: %w", err)
	}

	log.Printf("[Torrent] BitTorrent engine online (DataDir: %s, SpeedLimit: %d KB/s, Hashers: %d, MaxConns: %d)",
		dataDir, initialSpeedLimitKBps, cfg.PieceHashersPerTorrent, cfg.EstablishedConnsPerTorrent)

	return &TorrentEngine{
		client:      client,
		rateLimiter: limiter,
		dataDir:     dataDir,
		torrents:    make(map[string]*torrent.Torrent),
	}, nil
}

// AddMagnet creates or re-attaches a magnet torrent with custom target storage directory.
// Uses a SQLite-backed PieceCompletion store so downloads can resume across app restarts.
func (te *TorrentEngine) AddMagnet(downloadID string, magnetURI string, destDir string) (*torrent.Torrent, error) {
	magnetURI = strings.TrimSpace(magnetURI)
	if magnetURI == "" {
		return nil, fmt.Errorf("empty magnet URI")
	}

	te.mu.Lock()
	defer te.mu.Unlock()

	if te.client == nil {
		return nil, fmt.Errorf("torrent client is closed or uninitialized")
	}

	if existing, ok := te.torrents[downloadID]; ok && existing != nil {
		return existing, nil
	}

	if destDir == "" {
		destDir = te.dataDir
	}
	_ = os.MkdirAll(destDir, 0755)

	// Per-download state directory: stores SQLite piece-completion DB across sessions
	stateDir := te.stateDir(downloadID)
	_ = os.MkdirAll(stateDir, 0755)

	completion, err := storage.NewDefaultPieceCompletionForDir(stateDir)
	if err != nil {
		log.Printf("[Torrent] [%s] Failed to open piece-completion DB, falling back to in-memory: %v", downloadID, err)
		completion = storage.NewMapPieceCompletion()
	}
	getCompletionStore(te).Store(downloadID, completion)

	spec, err := torrent.TorrentSpecFromMagnetUri(magnetURI)
	if err != nil {
		completion.Close()
		getCompletionStore(te).Delete(downloadID)
		return nil, fmt.Errorf("invalid magnet URI: %w", err)
	}

	// Merge reliable Tier-1 public trackers to maximize seeder discovery
	existingTrackers := make(map[string]bool)
	for _, tier := range spec.Trackers {
		for _, tr := range tier {
			existingTrackers[strings.ToLower(strings.TrimSpace(tr))] = true
		}
	}
	for _, tier := range PublicTier1Trackers {
		var newTier []string
		for _, tr := range tier {
			normalized := strings.ToLower(strings.TrimSpace(tr))
			if !existingTrackers[normalized] {
				existingTrackers[normalized] = true
				newTier = append(newTier, tr)
			}
		}
		if len(newTier) > 0 {
			spec.Trackers = append(spec.Trackers, newTier)
		}
	}

	spec.Storage = storage.NewFileWithCompletion(destDir, completion)

	t, _, err := te.client.AddTorrentSpec(spec)
	if err != nil {
		completion.Close()
		getCompletionStore(te).Delete(downloadID)
		return nil, fmt.Errorf("failed to add torrent spec: %w", err)
	}

	// Ensure trackers and connection limits are applied to active torrent handle
	t.AddTrackers(PublicTier1Trackers)
	t.SetMaxEstablishedConns(120)

	te.torrents[downloadID] = t
	return t, nil
}

// stateDir returns the per-download SQLite completion directory path.
func (te *TorrentEngine) stateDir(downloadID string) string {
	return fmt.Sprintf("%s/.torrent_state/%s", te.dataDir, downloadID)
}

// GetTorrent returns an existing torrent handle by downloadID
func (te *TorrentEngine) GetTorrent(downloadID string) (*torrent.Torrent, bool) {
	te.mu.RLock()
	defer te.mu.RUnlock()
	t, ok := te.torrents[downloadID]
	return t, ok
}

// PauseTorrent suspends peer chunk downloads
func (te *TorrentEngine) PauseTorrent(downloadID string) {
	te.mu.RLock()
	t, ok := te.torrents[downloadID]
	te.mu.RUnlock()
	if ok && t != nil {
		t.DisallowDataDownload()
	}
}

// ResumeTorrent permits peer chunk downloads
func (te *TorrentEngine) ResumeTorrent(downloadID string) {
	te.mu.RLock()
	t, ok := te.torrents[downloadID]
	te.mu.RUnlock()
	if ok && t != nil {
		t.AllowDataDownload()
	}
}

// DropTorrent releases torrent handle from the client but preserves piece-completion state on disk
// so the download can resume from where it left off.
func (te *TorrentEngine) DropTorrent(downloadID string) {
	te.mu.Lock()
	t, ok := te.torrents[downloadID]
	if ok && t != nil {
		t.Drop()
		delete(te.torrents, downloadID)
	}
	te.mu.Unlock()

	// Close the completion DB handle (state files stay on disk for resume)
	te.closeCompletion(downloadID)
}

// PurgeState drops the torrent and deletes the piece-completion state directory from disk.
// Call this only when the user explicitly cancels/deletes a download and wants a fresh start.
func (te *TorrentEngine) PurgeState(downloadID string) {
	te.DropTorrent(downloadID)
	stateDir := te.stateDir(downloadID)
	if err := os.RemoveAll(stateDir); err != nil {
		log.Printf("[Torrent] [%s] Failed to remove state dir %s: %v", downloadID, stateDir, err)
	} else {
		log.Printf("[Torrent] [%s] State dir removed: %s", downloadID, stateDir)
	}
}

// SetSpeedLimit dynamically adjusts the global download rate limiter
func (te *TorrentEngine) SetSpeedLimit(kbps int) {
	if te.rateLimiter == nil {
		return
	}
	if kbps <= 0 {
		te.rateLimiter.SetLimit(rate.Inf)
		te.rateLimiter.SetBurst(16 * 1024 * 1024)
		log.Printf("[Torrent] Speed limit set to unlimited (Burst: 16 MB)")
	} else {
		burst := kbps * 1024 * 2
		if burst < 4*1024*1024 {
			burst = 4 * 1024 * 1024
		}
		te.rateLimiter.SetLimit(rate.Limit(kbps * 1024))
		te.rateLimiter.SetBurst(burst)
		log.Printf("[Torrent] Speed limit set to %d KB/s (Burst: %d KB)", kbps, burst/1024)
	}
}

// Close gracefully terminates all active torrents and closes the engine
func (te *TorrentEngine) Close() error {
	te.mu.Lock()
	defer te.mu.Unlock()

	// Close all piece-completion DB handles before closing the client
	cs := getCompletionStore(te)
	cs.Range(func(key, val any) bool {
		if c, ok := val.(storage.PieceCompletion); ok {
			_ = c.Close()
		}
		cs.Delete(key)
		return true
	})

	// Remove engine entry from global map
	engineStoresMu.Lock()
	delete(engineStores, te)
	engineStoresMu.Unlock()

	if te.client != nil {
		errs := te.client.Close()
		te.client = nil
		if len(errs) > 0 {
			return fmt.Errorf("errors closing torrent client: %v", errs)
		}
	}
	return nil
}

// closeCompletion closes the PieceCompletion handle for a single download ID.
func (te *TorrentEngine) closeCompletion(downloadID string) {
	if val, ok := getCompletionStore(te).LoadAndDelete(downloadID); ok {
		if c, ok := val.(storage.PieceCompletion); ok {
			_ = c.Close()
		}
	}
}

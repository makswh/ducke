package metadata

import (
	"sync"
	"time"

	"gamevault/pkg/database"
)

type cacheEntry[T any] struct {
	val       T
	expiresAt time.Time
}

func (e cacheEntry[T]) isExpired() bool {
	return time.Now().After(e.expiresAt)
}

// MemoryCache provides an in-memory, thread-safe TTL cache for Steam metadata and candidates.
type MemoryCache struct {
	mu           sync.RWMutex
	metaEntries  map[int]cacheEntry[*database.SteamMetadata]
	candEntries  map[string]cacheEntry[[]SteamCandidate]
	iconEntries  map[int]cacheEntry[string]
	reviewScores map[int]cacheEntry[reviewSummaryData]
	appTags      map[int]cacheEntry[[]string]
}

type reviewSummaryData struct {
	Desc    string
	Percent int
	Total   int
	Pos     int
}

// NewMemoryCache initializes an empty MemoryCache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		metaEntries:  make(map[int]cacheEntry[*database.SteamMetadata]),
		candEntries:  make(map[string]cacheEntry[[]SteamCandidate]),
		iconEntries:  make(map[int]cacheEntry[string]),
		reviewScores: make(map[int]cacheEntry[reviewSummaryData]),
		appTags:      make(map[int]cacheEntry[[]string]),
	}
}

// GetMetadata retrieves cached SteamMetadata for appID if not expired.
func (c *MemoryCache) GetMetadata(appID int) (*database.SteamMetadata, bool) {
	if appID <= 0 {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.metaEntries[appID]
	if !ok || e.isExpired() {
		return nil, false
	}
	return e.val, true
}

// SetMetadata caches SteamMetadata for appID with the given TTL.
func (c *MemoryCache) SetMetadata(appID int, meta *database.SteamMetadata, ttl time.Duration) {
	if appID <= 0 || meta == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Keep cache size bounded
	if len(c.metaEntries) > 2000 {
		c.purgeExpiredLocked()
	}

	c.metaEntries[appID] = cacheEntry[*database.SteamMetadata]{
		val:       meta,
		expiresAt: time.Now().Add(ttl),
	}
}

// GetCandidates retrieves cached search candidates for a normalized query.
func (c *MemoryCache) GetCandidates(query string) ([]SteamCandidate, bool) {
	if query == "" {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.candEntries[query]
	if !ok || e.isExpired() {
		return nil, false
	}
	return e.val, true
}

// SetCandidates caches search candidates for a normalized query with the given TTL.
func (c *MemoryCache) SetCandidates(query string, candidates []SteamCandidate, ttl time.Duration) {
	if query == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.candEntries) > 2000 {
		c.purgeExpiredLocked()
	}

	c.candEntries[query] = cacheEntry[[]SteamCandidate]{
		val:       candidates,
		expiresAt: time.Now().Add(ttl),
	}
}

// GetIcon retrieves cached game icon URL for appID.
func (c *MemoryCache) GetIcon(appID int) (string, bool) {
	if appID <= 0 {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.iconEntries[appID]
	if !ok || e.isExpired() {
		return "", false
	}
	return e.val, true
}

// SetIcon caches game icon URL for appID.
func (c *MemoryCache) SetIcon(appID int, iconURL string, ttl time.Duration) {
	if appID <= 0 || iconURL == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.iconEntries[appID] = cacheEntry[string]{
		val:       iconURL,
		expiresAt: time.Now().Add(ttl),
	}
}

// GetReviewSummary retrieves cached Steam reviews.
func (c *MemoryCache) GetReviewSummary(appID int) (string, int, int, int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.reviewScores[appID]
	if !ok || e.isExpired() {
		return "", 0, 0, 0, false
	}
	return e.val.Desc, e.val.Percent, e.val.Total, e.val.Pos, true
}

// SetReviewSummary caches Steam reviews.
func (c *MemoryCache) SetReviewSummary(appID int, desc string, percent, total, pos int, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.reviewScores[appID] = cacheEntry[reviewSummaryData]{
		val:       reviewSummaryData{Desc: desc, Percent: percent, Total: total, Pos: pos},
		expiresAt: time.Now().Add(ttl),
	}
}

// GetTags retrieves cached tags.
func (c *MemoryCache) GetTags(appID int) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.appTags[appID]
	if !ok || e.isExpired() {
		return nil, false
	}
	return e.val, true
}

// SetTags caches tags.
func (c *MemoryCache) SetTags(appID int, tags []string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.appTags[appID] = cacheEntry[[]string]{
		val:       tags,
		expiresAt: time.Now().Add(ttl),
	}
}

// InvalidateAppID removes cached metadata for a specific appID.
func (c *MemoryCache) InvalidateAppID(appID int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.metaEntries, appID)
	delete(c.iconEntries, appID)
	delete(c.reviewScores, appID)
	delete(c.appTags, appID)
}

// Clear wipes all cached entries.
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metaEntries = make(map[int]cacheEntry[*database.SteamMetadata])
	c.candEntries = make(map[string]cacheEntry[[]SteamCandidate])
	c.iconEntries = make(map[int]cacheEntry[string])
	c.reviewScores = make(map[int]cacheEntry[reviewSummaryData])
	c.appTags = make(map[int]cacheEntry[[]string])
}

func (c *MemoryCache) purgeExpiredLocked() {
	now := time.Now()
	for k, e := range c.metaEntries {
		if now.After(e.expiresAt) {
			delete(c.metaEntries, k)
		}
	}
	for k, e := range c.candEntries {
		if now.After(e.expiresAt) {
			delete(c.candEntries, k)
		}
	}
	for k, e := range c.iconEntries {
		if now.After(e.expiresAt) {
			delete(c.iconEntries, k)
		}
	}
}

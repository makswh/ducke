package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"gamevault/pkg/remote"
)

const (
	sgdbBaseURL   = "https://www.steamgriddb.com"
	sgdbUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// SteamGridDBGridItem represents a box art / cover grid
type SteamGridDBGridItem struct {
	ID     int    `json:"id"`
	Style  string `json:"style"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	URL    string `json:"url"`
	Thumb  string `json:"thumb"`
	NSFW   bool   `json:"nsfw"`
	Humor  bool   `json:"humor"`
}

// SteamGridDBHeroItem represents a widescreen hero banner / backdrop
type SteamGridDBHeroItem struct {
	ID     int    `json:"id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	URL    string `json:"url"`
	Thumb  string `json:"thumb"`
	NSFW   bool   `json:"nsfw"`
	Humor  bool   `json:"humor"`
}

// SteamGridDBLogoItem represents a transparent game logo
type SteamGridDBLogoItem struct {
	ID    int    `json:"id"`
	URL   string `json:"url"`
	Thumb string `json:"thumb"`
	NSFW  bool   `json:"nsfw"`
	Humor bool   `json:"humor"`
}

// SteamGridDBIconItem represents a game icon
type SteamGridDBIconItem struct {
	ID    int    `json:"id"`
	URL   string `json:"url"`
	Thumb string `json:"thumb"`
}

// SteamGridDBGameHomeData represents the data payload returned by /api/public/game/:id/home
type SteamGridDBGameHomeData struct {
	Game struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		ReleaseDate int64  `json:"release_date"`
		Verified    bool   `json:"verified"`
	} `json:"game"`
	Grids  []SteamGridDBGridItem `json:"grids"`
	Heroes []SteamGridDBHeroItem `json:"heroes"`
	Logos  []SteamGridDBLogoItem `json:"logos"`
	Icons  []SteamGridDBIconItem `json:"icons"`
}

// SteamGridDBGameHomeResponse is the response wrapper
type SteamGridDBGameHomeResponse struct {
	Success bool                     `json:"success"`
	Data    *SteamGridDBGameHomeData `json:"data"`
}

// SteamGridDBSearchItem represents an autocomplete result item
type SteamGridDBSearchItem struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Verified    bool   `json:"verified"`
	ReleaseDate int64  `json:"release_date"`
}

// SteamGridDBAutocompleteResponse is the autocomplete response
type SteamGridDBAutocompleteResponse struct {
	Success bool                    `json:"success"`
	Data    []SteamGridDBSearchItem `json:"data"`
}

// SteamGridDBAssets contains extracted cover, backdrop, and metadata
type SteamGridDBAssets struct {
	GameID        int    `json:"gameId"`
	GameTitle     string `json:"gameTitle"`
	CoverURL      string `json:"coverUrl"`      // Vertical 600x900 poster
	BackgroundURL string `json:"backgroundUrl"` // Widescreen 1920x620 hero
	HeaderURL     string `json:"headerUrl"`     // Landscape 920x430 or 460x215 capsule
	LogoURL       string `json:"logoUrl"`
	IconURL       string `json:"iconUrl"`
	ReleaseDate   string `json:"releaseDate"`
}

// SteamGridDBService handles searching and fetching assets from SteamGridDB
type SteamGridDBService struct {
	httpClient  *http.Client
	apiKey      string
	assetCache  sync.Map // cleanTitle string -> *SteamGridDBAssets
	searchCache sync.Map // cleanTitle string -> *SteamGridDBSearchItem
}

// NewSteamGridDBService creates a new SteamGridDB service
func NewSteamGridDBService(apiKey string) *SteamGridDBService {
	tr := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}
	return &SteamGridDBService{
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: tr,
		},
		apiKey: strings.TrimSpace(apiKey),
	}
}

// SearchGame searches SteamGridDB for the best matching game
func (s *SteamGridDBService) SearchGame(title string) (*SteamGridDBSearchItem, error) {
	clean := remote.SanitizeForSteamSearch(title)
	if clean == "" {
		clean = strings.TrimSpace(title)
	}
	if clean == "" {
		return nil, fmt.Errorf("empty search title")
	}

	if val, ok := s.searchCache.Load(clean); ok {
		item := val.(*SteamGridDBSearchItem)
		if item == nil {
			return nil, fmt.Errorf("no games found on steamgriddb for %q", title)
		}
		return item, nil
	}

	endpoint := fmt.Sprintf("%s/api/public/search/autocomplete?term=%s", sgdbBaseURL, url.QueryEscape(clean))
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", sgdbUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,ru;q=0.8")
	req.Header.Set("Referer", "https://www.steamgriddb.com/")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("steamgriddb search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steamgriddb search returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var autoResp SteamGridDBAutocompleteResponse
	if err := json.Unmarshal(body, &autoResp); err != nil {
		return nil, fmt.Errorf("failed to parse steamgriddb search json: %w", err)
	}

	if !autoResp.Success || len(autoResp.Data) == 0 {
		return nil, fmt.Errorf("no games found on steamgriddb for %q", title)
	}

	// Score candidates using title similarity
	type scoredItem struct {
		item  SteamGridDBSearchItem
		score float64
	}
	var scored []scoredItem

	for _, item := range autoResp.Data {
		sc := CalculateTitleSimilarity(title, item.Name)
		scored = append(scored, scoredItem{item: item, score: sc})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	best := scored[0]
	if best.score < 0.35 {
		// Even if similarity is slightly lower, if first word matches, accept
		f1 := strings.Fields(strings.ToLower(title))
		f2 := strings.Fields(strings.ToLower(best.item.Name))
		if len(f1) > 0 && len(f2) > 0 && f1[0] == f2[0] {
			s.searchCache.Store(clean, &best.item)
			return &best.item, nil
		}
		return nil, fmt.Errorf("similarity too low (%.2f) for %q vs %q", best.score, title, best.item.Name)
	}

	s.searchCache.Store(clean, &best.item)
	return &best.item, nil
}

// FetchGameAssets fetches covers, heroes, and logos for a given SteamGridDB game ID
func (s *SteamGridDBService) FetchGameAssets(gameID int) (*SteamGridDBAssets, error) {
	if gameID <= 0 {
		return nil, fmt.Errorf("invalid steamgriddb game ID: %d", gameID)
	}

	endpoint := fmt.Sprintf("%s/api/public/game/%d/home", sgdbBaseURL, gameID)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", sgdbUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,ru;q=0.8")
	req.Header.Set("Referer", fmt.Sprintf("https://www.steamgriddb.com/game/%d", gameID))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("steamgriddb game assets request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steamgriddb game assets returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var homeResp SteamGridDBGameHomeResponse
	if err := json.Unmarshal(body, &homeResp); err != nil {
		return nil, fmt.Errorf("failed to parse steamgriddb game home json: %w", err)
	}

	if !homeResp.Success || homeResp.Data == nil {
		return nil, fmt.Errorf("no assets returned for steamgriddb game ID %d", gameID)
	}

	data := homeResp.Data
	assets := &SteamGridDBAssets{
		GameID:    data.Game.ID,
		GameTitle: data.Game.Name,
	}

	if data.Game.ReleaseDate > 0 {
		t := time.Unix(data.Game.ReleaseDate, 0)
		assets.ReleaseDate = t.Format("02.01.2006")
	}

	// 1. Extract Vertical Cover (2:3 aspect ratio, prefer 600x900)
	for _, g := range data.Grids {
		if g.NSFW || g.Humor {
			continue
		}
		if (g.Width == 600 && g.Height == 900) || (g.Height > g.Width) {
			assets.CoverURL = g.URL
			break
		}
	}
	// Fallback to vertical/square grid if no strict 600x900 found (NEVER landscape!)
	if assets.CoverURL == "" && len(data.Grids) > 0 {
		for _, g := range data.Grids {
			if !g.NSFW && g.Height >= g.Width {
				assets.CoverURL = g.URL
				break
			}
		}
	}

	// 2. Extract Landscape Header Capsule (prefer width > height)
	for _, g := range data.Grids {
		if g.NSFW || g.Humor {
			continue
		}
		if g.Width > g.Height {
			assets.HeaderURL = g.URL
			break
		}
	}

	// 3. Extract Background Hero (1920x620)
	for _, h := range data.Heroes {
		if h.NSFW || h.Humor {
			continue
		}
		assets.BackgroundURL = h.URL
		break
	}
	// If no header found, hero can serve as header too
	if assets.HeaderURL == "" && assets.BackgroundURL != "" {
		assets.HeaderURL = assets.BackgroundURL
	}

	// 4. Extract Logo
	for _, l := range data.Logos {
		if !l.NSFW && !l.Humor {
			assets.LogoURL = l.URL
			break
		}
	}

	// 5. Extract Icon
	if len(data.Icons) > 0 {
		assets.IconURL = data.Icons[0].URL
	}

	return assets, nil
}

// FindAssetsForGame searches for a game by title and retrieves its assets
func (s *SteamGridDBService) FindAssetsForGame(title string) (*SteamGridDBAssets, error) {
	clean := remote.SanitizeForSteamSearch(title)
	if clean == "" {
		clean = strings.TrimSpace(title)
	}
	if clean == "" {
		return nil, fmt.Errorf("empty title")
	}

	if val, ok := s.assetCache.Load(clean); ok {
		assets := val.(*SteamGridDBAssets)
		if assets == nil {
			return nil, fmt.Errorf("no assets found on steamgriddb for %q", title)
		}
		return assets, nil
	}

	item, err := s.SearchGame(title)
	if err != nil {
		return nil, err
	}

	assets, err := s.FetchGameAssets(item.ID)
	if err == nil && assets != nil {
		s.assetCache.Store(clean, assets)
	}
	return assets, err
}

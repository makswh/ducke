package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gamevault/pkg/database"
	"gamevault/pkg/remote"
)

// SteamReviewSummaryResponse matches Steam's appreviews query_summary schema
type SteamReviewSummaryResponse struct {
	Success      int `json:"success"`
	QuerySummary struct {
		NumReviews      int    `json:"num_reviews"`
		ReviewScore     int    `json:"review_score"`
		ReviewScoreDesc string `json:"review_score_desc"`
		TotalPositive   int    `json:"total_positive"`
		TotalNegative   int    `json:"total_negative"`
		TotalReviews    int    `json:"total_reviews"`
	} `json:"query_summary"`
}

const (
	steamUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	steamAgeCookie = "birthtime=283993201; mature_content=1; wants_mature_content=1; lastagecheckage=1-January-1990; path=/;"
)

var (
	fallbackRegions = []string{"US", "KZ", "DE"}

	suggestAppIDRegex   = regexp.MustCompile(`data-ds-appid="(\d+)"`)
	suggestNameRegex    = regexp.MustCompile(`<div class="match_name">([^<]+)</div>`)
	suggestImgRegex     = regexp.MustCompile(`<div class="match_img">\s*<img\s+src="([^"]+)"`)
	communityTitleRegex = regexp.MustCompile(`(?i)<title>(?:Steam Community|Сообщество Steam|\S+)\s*::\s*([^<]+)</title>`)
	appHubNameRegex     = regexp.MustCompile(`(?i)<div[^>]*class="[^"]*apphub_AppName[^"]*"[^>]*>([^<]+)</div>`)
)

// SteamCommunitySearchItem represents an item returned by Steam Community SearchApps endpoint
type SteamCommunitySearchItem struct {
	AppID string `json:"appid"`
	Name  string `json:"name"`
	Logo  string `json:"logo"`
	Icon  string `json:"icon"`
}

// SteamSearchResultItem represents an item returned by Steam Store Search
type SteamSearchResultItem struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	TinyImage string `json:"tiny_image"`
}

type SteamSearchResponse struct {
	Total int                     `json:"total"`
	Items []SteamSearchResultItem `json:"items"`
}

// SteamCandidate represents a scored matching candidate for UI or auto-matching
type SteamCandidate struct {
	AppID     int     `json:"appId"`
	Name      string  `json:"name"`
	TinyImage string  `json:"tinyImage"`
	Score     float64 `json:"score"`
}

// SteamAppDetailsData matches Steam's appdetails API schema
type SteamAppDetailsData struct {
	Type                string      `json:"type"`
	Name                string      `json:"name"`
	SteamAppID          int         `json:"steam_appid"`
	RequiredAge         interface{} `json:"required_age"`
	IsFree              bool        `json:"is_free"`
	DetailedDescription string      `json:"detailed_description"`
	ShortDescription    string      `json:"short_description"`
	HeaderImage         string      `json:"header_image"`
	CapsuleImage        string      `json:"capsule_image"`
	BackgroundImage     string      `json:"background"`
	BackgroundRaw       string      `json:"background_raw"`
	Website             string      `json:"website"`
	PCRequirements      struct {
		Minimum     string `json:"minimum"`
		Recommended string `json:"recommended"`
	} `json:"pc_requirements"`
	Developers []string `json:"developers"`
	Publishers []string `json:"publishers"`
	Metacritic struct {
		Score int    `json:"score"`
		URL   string `json:"url"`
	} `json:"metacritic"`
	Genres []struct {
		ID          string `json:"id"`
		Description string `json:"description"`
	} `json:"genres"`
	Screenshots []struct {
		ID            int    `json:"id"`
		PathThumbnail string `json:"path_thumbnail"`
		PathFull      string `json:"path_full"`
	} `json:"screenshots"`
	Movies []struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Thumbnail string `json:"thumbnail"`
		HLSH264   string `json:"hls_h264"`
		DashH264  string `json:"dash_h264"`
		DashAV1   string `json:"dash_av1"`
		Webm      struct {
			SD  string `json:"480"`
			Max string `json:"max"`
		} `json:"webm"`
		MP4 struct {
			SD  string `json:"480"`
			Max string `json:"max"`
		} `json:"mp4"`
		Highlight bool `json:"highlight"`
	} `json:"movies"`
	ReleaseDate struct {
		ComingSoon bool   `json:"coming_soon"`
		Date       string `json:"date"`
	} `json:"release_date"`
	ControllerSupport string `json:"controller_support"` // "full", "partial"
}

type SteamAppDetailsResponse map[string]struct {
	Success bool                `json:"success"`
	Data    SteamAppDetailsData `json:"data"`
}

type SteamService struct {
	db          *database.Database
	httpClient  *http.Client
	rateLimiter *time.Ticker
	sgdb        *SteamGridDBService
	workerCtx   context.Context
	cancelFunc  context.CancelFunc
	mu          sync.Mutex
	isEnriching bool
}

func NewSteamService(db *database.Database) *SteamService {
	tr := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     60 * time.Second,
		DisableKeepAlives:   false,
	}
	return &SteamService{
		db: db,
		httpClient: &http.Client{
			Timeout:   8 * time.Second,
			Transport: tr,
		},
		// Fast responsive rate limit: 1 request every 150ms
		rateLimiter: time.NewTicker(150 * time.Millisecond),
		sgdb:        NewSteamGridDBService(""),
	}
}

// querySteamSuggest queries Steam's modern store search endpoint which indexes all games including unreleased/upcoming/AAA
func (s *SteamService) querySteamSuggest(searchTitle, query, lang string) ([]SteamCandidate, error) {
	<-s.rateLimiter.C

	if lang == "" {
		lang = "english"
	}

	endpoint := fmt.Sprintf(
		"https://store.steampowered.com/search/suggest?term=%s&f=games&cc=US&l=%s",
		url.QueryEscape(query),
		lang,
	)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", steamUserAgent)
	req.Header.Set("Cookie", steamAgeCookie)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam suggest returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	htmlContent := string(body)
	if strings.TrimSpace(htmlContent) == "" {
		return nil, nil
	}

	// Split items by match class
	blocks := strings.Split(htmlContent, `<a class="match`)
	var candidates []SteamCandidate

	for _, block := range blocks {
		if !strings.Contains(block, "data-ds-appid") {
			continue
		}

		appIDMatches := suggestAppIDRegex.FindStringSubmatch(block)
		nameMatches := suggestNameRegex.FindStringSubmatch(block)
		imgMatches := suggestImgRegex.FindStringSubmatch(block)

		if len(appIDMatches) < 2 || len(nameMatches) < 2 {
			continue
		}

		appID, err := strconv.Atoi(appIDMatches[1])
		if err != nil || appID == 0 {
			continue
		}

		name := strings.TrimSpace(html.UnescapeString(nameMatches[1]))
		img := ""
		if len(imgMatches) >= 2 {
			img = imgMatches[1]
		}

		score := CalculateTitleSimilarity(searchTitle, name)
		candidates = append(candidates, SteamCandidate{
			AppID:     appID,
			Name:      name,
			TinyImage: img,
			Score:     score,
		})
	}

	return candidates, nil
}

// queryLegacyStoreSearch queries Steam's legacy storesearch API endpoint as secondary source
func (s *SteamService) queryLegacyStoreSearch(searchTitle, query string) ([]SteamCandidate, error) {
	<-s.rateLimiter.C

	endpoint := fmt.Sprintf(
		"https://store.steampowered.com/api/storesearch/?term=%s&cc=US&l=english",
		url.QueryEscape(query),
	)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", steamUserAgent)
	req.Header.Set("Cookie", steamAgeCookie)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam storesearch returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResp SteamSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, err
	}

	var candidates []SteamCandidate
	for _, item := range searchResp.Items {
		score := CalculateTitleSimilarity(searchTitle, item.Name)
		candidates = append(candidates, SteamCandidate{
			AppID:     item.ID,
			Name:      item.Name,
			TinyImage: item.TinyImage,
			Score:     score,
		})
	}

	return candidates, nil
}

// queryCommunitySearch queries Steam Community's SearchApps endpoint as a third-tier search fallback
func (s *SteamService) queryCommunitySearch(searchTitle, query string) ([]SteamCandidate, error) {
	<-s.rateLimiter.C

	endpoint := fmt.Sprintf(
		"https://steamcommunity.com/actions/SearchApps/%s",
		url.PathEscape(query),
	)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", steamUserAgent)
	req.Header.Set("Cookie", steamAgeCookie)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam community search returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var items []SteamCommunitySearchItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}

	var candidates []SteamCandidate
	for _, item := range items {
		appID, err := strconv.Atoi(item.AppID)
		if err != nil || appID == 0 {
			continue
		}
		score := CalculateTitleSimilarity(searchTitle, item.Name)
		img := item.Logo
		if img == "" {
			img = item.Icon
		}
		candidates = append(candidates, SteamCandidate{
			AppID:     appID,
			Name:      item.Name,
			TinyImage: img,
			Score:     score,
		})
	}

	return candidates, nil
}

// SearchSteamCandidates searches Steam Store across both modern suggest and legacy APIs and ranks results by similarity
func (s *SteamService) SearchSteamCandidates(searchTitle string) ([]SteamCandidate, error) {
	if strings.TrimSpace(searchTitle) == "" {
		return nil, nil
	}

	queries := GenerateSearchQueries(searchTitle)
	candidateMap := make(map[int]SteamCandidate)

QueryLoop:
	for _, query := range queries {
		// 1. Primary query: Steam English Suggest API (matches canonical title e.g. "Hogwarts Legacy", "Assassin's Creed Unity")
		suggestEnglish, err := s.querySteamSuggest(searchTitle, query, "english")
		if err == nil {
			for _, c := range suggestEnglish {
				if existing, exists := candidateMap[c.AppID]; !exists || c.Score > existing.Score {
					candidateMap[c.AppID] = c
				}
			}
		}

		// If we found a high confidence match (score >= 0.80), stop immediately!
		for _, c := range candidateMap {
			if c.Score >= 0.80 {
				break QueryLoop
			}
		}

		// 2. Multilingual query: Steam Russian Suggest API (for Russian title releases)
		suggestRussian, err := s.querySteamSuggest(searchTitle, query, "russian")
		if err == nil {
			for _, c := range suggestRussian {
				if existing, exists := candidateMap[c.AppID]; !exists || c.Score > existing.Score {
					candidateMap[c.AppID] = c
				}
			}
		}

		for _, c := range candidateMap {
			if c.Score >= 0.80 {
				break QueryLoop
			}
		}

		// 3. Fallback: StoreSearch API (only if no acceptable match >= 0.70)
		hasAcceptable := false
		for _, c := range candidateMap {
			if c.Score >= 0.70 {
				hasAcceptable = true
				break
			}
		}
		if !hasAcceptable {
			legacyResults, err := s.queryLegacyStoreSearch(searchTitle, query)
			if err == nil {
				for _, c := range legacyResults {
					if existing, exists := candidateMap[c.AppID]; !exists || c.Score > existing.Score {
						candidateMap[c.AppID] = c
					}
				}
			}
		}

		for _, c := range candidateMap {
			if c.Score >= 0.80 {
				break QueryLoop
			}
		}

		// 4. Additional Fallback: Steam Community Search (finds unlisted/restricted games)
		hasHighConfidence := false
		for _, c := range candidateMap {
			if c.Score >= 0.70 {
				hasHighConfidence = true
				break
			}
		}
		if !hasHighConfidence {
			commResults, err := s.queryCommunitySearch(searchTitle, query)
			if err == nil {
				for _, c := range commResults {
					if existing, exists := candidateMap[c.AppID]; !exists || c.Score > existing.Score {
						candidateMap[c.AppID] = c
					}
				}
			}
		}

		for _, c := range candidateMap {
			if c.Score >= 0.80 {
				break QueryLoop
			}
		}
	}

	var candidates []SteamCandidate
	for _, c := range candidateMap {
		candidates = append(candidates, c)
	}

	// Sort by highest similarity score first
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	return candidates, nil
}

// SearchGame searches Steam Store for a game title and returns matching AppID and Name if confidence >= 0.70
func (s *SteamService) SearchGame(searchTitle string) (int, string, error) {
	candidates, err := s.SearchSteamCandidates(searchTitle)
	if err != nil {
		return 0, "", err
	}

	if len(candidates) == 0 {
		return 0, "", nil
	}

	best := candidates[0]
	if best.Score >= 0.70 {
		return best.AppID, best.Name, nil
	}

	log.Printf("[Steam] Best candidate for %q was %q with score %.2f (rejected: below 0.70 threshold)", searchTitle, best.Name, best.Score)
	return 0, "", nil
}

// FetchAppDetails retrieves full game details for a given Steam AppID.
// Implements multi-region fallback (US -> KZ -> DE) to bypass regional restrictions,
// includes age-verification headers, and falls back to deterministic CDN metadata for delisted games.
func (s *SteamService) FetchAppDetails(appID int) (*database.SteamMetadata, error) {
	// First check database cache
	if s.db != nil {
		if cached, err := s.db.GetSteamMetadataFromCache(appID); err == nil && cached != nil {
			// If cached metadata lacks Steam review score, fetch it on demand
			if cached.ReviewScoreDesc == "" && cached.TotalReviews == 0 {
				desc, pct, tot, pos := s.FetchSteamReviewSummary(appID)
				if tot > 0 {
					cached.ReviewScoreDesc = desc
					cached.ReviewPercent = pct
					cached.TotalReviews = tot
					cached.TotalPositive = pos
					_ = s.db.SaveSteamMetadata(*cached)
				}
			}
			return cached, nil
		}
	}

	var lastErr error

	// Try store API across fallback regions (US -> KZ -> DE) with Russian localization
	for _, cc := range fallbackRegions {
		<-s.rateLimiter.C // Respect rate limit

		endpoint := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%d&cc=%s&l=russian", appID, cc)

		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", steamUserAgent)
		req.Header.Set("Cookie", steamAgeCookie)

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("steam appdetails request failed (cc=%s): %w", cc, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("steam appdetails returned HTTP %d for cc=%s", resp.StatusCode, cc)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		var appResp SteamAppDetailsResponse
		if err := json.Unmarshal(body, &appResp); err != nil {
			lastErr = fmt.Errorf("failed to decode appdetails json (cc=%s): %w", cc, err)
			continue
		}

		appKey := fmt.Sprintf("%d", appID)
		entry, exists := appResp[appKey]
		if !exists || !entry.Success {
			lastErr = fmt.Errorf("steam app details not found or unlisted for appid %d in region %s", appID, cc)
			continue
		}

		data := entry.Data

		var genres []string
		for _, g := range data.Genres {
			genres = append(genres, g.Description)
		}

		var screenshots []string
		for _, sc := range data.Screenshots {
			screenshots = append(screenshots, sc.PathFull)
		}

		var movies []database.SteamMovie
		for _, m := range data.Movies {
			hlsUrl := strings.TrimSpace(m.HLSH264)
			if hlsUrl != "" {
				hlsUrl = strings.ReplaceAll(hlsUrl, "http://", "https://")
				hlsUrl = strings.ReplaceAll(hlsUrl, "video.akamai.steamstatic.com", "video.fastly.steamstatic.com")
			}

			mp4Url := m.MP4.Max
			if mp4Url == "" {
				mp4Url = m.MP4.SD
			}
			if mp4Url != "" {
				mp4Url = strings.ReplaceAll(mp4Url, "http://", "https://")
				mp4Url = strings.ReplaceAll(mp4Url, "video.akamai.steamstatic.com", "video.fastly.steamstatic.com")
				mp4Url = strings.ReplaceAll(mp4Url, "shared.akamai.steamstatic.com", "video.fastly.steamstatic.com")
			}

			webmUrl := m.Webm.Max
			if webmUrl == "" {
				webmUrl = m.Webm.SD
			}
			if webmUrl != "" {
				webmUrl = strings.ReplaceAll(webmUrl, "http://", "https://")
				webmUrl = strings.ReplaceAll(webmUrl, "video.akamai.steamstatic.com", "video.fastly.steamstatic.com")
				webmUrl = strings.ReplaceAll(webmUrl, "shared.akamai.steamstatic.com", "video.fastly.steamstatic.com")
			}

			thumb := m.Thumbnail
			if thumb != "" {
				thumb = strings.ReplaceAll(thumb, "http://", "https://")
				thumb = strings.ReplaceAll(thumb, "shared.akamai.steamstatic.com", "shared.fastly.steamstatic.com")
			}

			movies = append(movies, database.SteamMovie{
				ID:        m.ID,
				Name:      m.Name,
				Thumbnail: thumb,
				MP4:       mp4Url,
				Webm:      webmUrl,
				HLS:       hlsUrl,
			})
		}

		ctrlSupport := data.ControllerSupport
		if ctrlSupport == "" {
			ctrlSupport = "none"
		}

		bgImage := data.BackgroundRaw
		if bgImage == "" {
			bgImage = data.BackgroundImage
		}
		if bgImage == "" {
			bgImage = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/page_bg_generated_v6b.jpg", appID)
		}

		// Check Steam CDN for portrait vertical cover (600x900)
		var capsuleImg string
		steamCover := fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/library_600x900.jpg", appID)
		if s.verifyCDNAsset(steamCover) {
			capsuleImg = steamCover
		} else if s.sgdb != nil {
			// Steam does not have a 600x900 portrait cover. Look up SteamGridDB!
			if sgdbAssets, err := s.sgdb.FindAssetsForGame(data.Name); err == nil && sgdbAssets != nil {
				if sgdbAssets.CoverURL != "" {
					capsuleImg = sgdbAssets.CoverURL
				}
				if (bgImage == "" || strings.Contains(bgImage, "page_bg_generated")) && sgdbAssets.BackgroundURL != "" {
					bgImage = sgdbAssets.BackgroundURL
				}
			}
		}

		headerImg := data.HeaderImage
		if headerImg == "" {
			headerImg = data.CapsuleImage
		}
		if headerImg == "" {
			headerImg = fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d/header.jpg", appID)
		}

		// Fetch authentic Steam user reviews
		reviewDesc, reviewPercent, totalReviews, totalPositive := s.FetchSteamReviewSummary(appID)

		meta := database.SteamMetadata{
			AppID:               appID,
			Title:               data.Name,
			ShortDescription:    data.ShortDescription,
			DetailedDescription: data.DetailedDescription,
			HeaderImage:         headerImg,
			CapsuleImage:        capsuleImg,
			BackgroundImage:     bgImage,
			Screenshots:         screenshots,
			Movies:              movies,
			Genres:              genres,
			Developers:          data.Developers,
			Publishers:          data.Publishers,
			ReleaseDate:         data.ReleaseDate.Date,
			ControllerSupport:   ctrlSupport,
			PCRequirements:      data.PCRequirements.Minimum,
			MetacriticScore:     data.Metacritic.Score,
			ReviewScoreDesc:     reviewDesc,
			ReviewPercent:       reviewPercent,
			TotalReviews:        totalReviews,
			TotalPositive:       totalPositive,
			CachedAt:            time.Now().Unix(),
		}

		// Persist in local database cache
		if s.db != nil {
			_ = s.db.SaveSteamMetadata(meta)
		}

		return &meta, nil
	}

	// All store regions returned unlisted/failure. Attempt fallback metadata via Community & Steam CDN for delisted games
	if fallbackMeta := s.fetchFallbackMetadata(appID); fallbackMeta != nil {
		if s.db != nil {
			_ = s.db.SaveSteamMetadata(*fallbackMeta)
		}
		return fallbackMeta, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("steam app details not found or unlisted for appid %d", appID)
}

// fetchFallbackMetadata generates deterministic high-res CDN metadata for unlisted or delisted games
func (s *SteamService) fetchFallbackMetadata(appID int) *database.SteamMetadata {
	baseCDN := fmt.Sprintf("https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/%d", appID)
	headerImage := fmt.Sprintf("%s/header.jpg", baseCDN)
	capsuleImage := fmt.Sprintf("%s/capsule_616x353.jpg", baseCDN)
	backgroundImage := fmt.Sprintf("%s/page_bg_generated_v6b.jpg", baseCDN)

	// Verify if assets actually exist on Steam CDN using a lightweight HEAD request
	if !s.verifyCDNAsset(headerImage) && !s.verifyCDNAsset(capsuleImage) {
		return nil
	}

	title := s.fetchCommunityGameTitle(appID)
	if title == "" {
		title = fmt.Sprintf("Steam App %d", appID)
	}

	return &database.SteamMetadata{
		AppID:           appID,
		Title:           title,
		HeaderImage:     headerImage,
		CapsuleImage:    capsuleImage,
		BackgroundImage: backgroundImage,
		Screenshots:     []string{},
		Movies:          []database.SteamMovie{},
		Genres:          []string{},
		Developers:      []string{},
		Publishers:      []string{},
		CachedAt:        time.Now().Unix(),
	}
}

// fetchCommunityGameTitle attempts to scrape the game title from the public Steam Community hub page
func (s *SteamService) fetchCommunityGameTitle(appID int) string {
	endpoint := fmt.Sprintf("https://steamcommunity.com/app/%d/", appID)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", steamUserAgent)
	req.Header.Set("Cookie", steamAgeCookie)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	content := string(body)

	// Try apphub_AppName first (most accurate on Community Hub)
	if nameMatch := appHubNameRegex.FindStringSubmatch(content); len(nameMatch) >= 2 {
		name := strings.TrimSpace(html.UnescapeString(nameMatch[1]))
		if name != "" {
			return name
		}
	}

	// Fallback to <title>
	titleMatch := communityTitleRegex.FindStringSubmatch(content)
	if len(titleMatch) >= 2 {
		title := strings.TrimSpace(html.UnescapeString(titleMatch[1]))
		title = strings.TrimPrefix(title, "Steam Community :: ")
		title = strings.TrimPrefix(title, "Сообщество Steam :: ")
		return strings.TrimSpace(title)
	}

	return ""
}

// FetchSteamReviewSummary queries Steam's official appreviews API for community score & review count
func (s *SteamService) FetchSteamReviewSummary(appID int) (string, int, int, int) {
	<-s.rateLimiter.C

	endpoint := fmt.Sprintf("https://store.steampowered.com/appreviews/%d?json=1&language=all&purchase_type=all&l=russian", appID)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", 0, 0, 0
	}
	req.Header.Set("User-Agent", steamUserAgent)
	req.Header.Set("Cookie", steamAgeCookie)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", 0, 0, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, 0, 0
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, 0, 0
	}

	var revResp SteamReviewSummaryResponse
	if err := json.Unmarshal(body, &revResp); err != nil {
		return "", 0, 0, 0
	}

	total := revResp.QuerySummary.TotalReviews
	pos := revResp.QuerySummary.TotalPositive
	desc := revResp.QuerySummary.ReviewScoreDesc

	percent := 0
	if total > 0 {
		percent = int(math.Round(float64(pos) / float64(total) * 100))
	}

	return desc, percent, total, pos
}

// verifyCDNAsset performs a lightweight check to confirm asset presence on Steam CDN
func (s *SteamService) verifyCDNAsset(assetURL string) bool {
	// Try fast HEAD first
	req, err := http.NewRequest("HEAD", assetURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", steamUserAgent)
		resp, err := s.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
	}

	// Fallback to GET with Range: bytes=0-0 (supported by all Steam CDNs, fetches only 1 byte)
	reqRange, err := http.NewRequest("GET", assetURL, nil)
	if err != nil {
		return false
	}
	reqRange.Header.Set("User-Agent", steamUserAgent)
	reqRange.Header.Set("Range", "bytes=0-0")

	respRange, err := s.httpClient.Do(reqRange)
	if err != nil {
		return false
	}
	defer respRange.Body.Close()

	return respRange.StatusCode == http.StatusOK || respRange.StatusCode == http.StatusPartialContent
}

// EnrichGame provides on-demand instant enrichment for a single game
func (s *SteamService) EnrichGame(gameID int64) (*database.GameEntity, error) {
	game, err := s.db.GetGameByID(gameID)
	if err != nil {
		return nil, err
	}

	targetAppID := game.SteamAppID
	searchTerm := game.SearchTitle
	if searchTerm == "" {
		searchTerm = game.CleanTitle
	}

	if targetAppID == 0 {
		sanitized := remote.SanitizeForSteamSearch(searchTerm)
		query := searchTerm
		if sanitized != "" {
			query = sanitized
		}

		appID, _, _ := s.SearchGame(query)
		targetAppID = appID
	}

	if targetAppID > 0 {
		meta, _ := s.FetchAppDetails(targetAppID)
		if meta != nil && (meta.CapsuleImage == "" || IsHorizontalAsset(meta.CapsuleImage) || meta.BackgroundImage == "" || strings.Contains(meta.BackgroundImage, "page_bg_generated")) {
			// Fallback missing cover or background from SteamGridDB
			if s.sgdb != nil {
				sgdbTitle := searchTerm
				if meta.Title != "" {
					sgdbTitle = meta.Title
				}
				if sgdbAssets, err := s.sgdb.FindAssetsForGame(sgdbTitle); err == nil && sgdbAssets != nil {
					updated := false
					if (meta.CapsuleImage == "" || IsHorizontalAsset(meta.CapsuleImage)) && sgdbAssets.CoverURL != "" {
						meta.CapsuleImage = sgdbAssets.CoverURL
						updated = true
					}
					if (meta.BackgroundImage == "" || strings.Contains(meta.BackgroundImage, "page_bg_generated")) && sgdbAssets.BackgroundURL != "" {
						meta.BackgroundImage = sgdbAssets.BackgroundURL
						updated = true
					}
					if meta.HeaderImage == "" && sgdbAssets.HeaderURL != "" {
						meta.HeaderImage = sgdbAssets.HeaderURL
						updated = true
					}
					if updated {
						_ = s.db.SaveSteamMetadata(*meta)
					}
				}
			}
		}
		_ = s.db.SetGameAppID(gameID, targetAppID)
	} else {
		// Game not found on Steam (e.g. Need for Speed Carbon, non-Steam games).
		// Fallback to SteamGridDB for box art, hero backdrop, and clean title!
		foundSGDB := false
		if s.sgdb != nil {
			if sgdbAssets, err := s.sgdb.FindAssetsForGame(searchTerm); err == nil && sgdbAssets != nil {
				sgdbAppID := -sgdbAssets.GameID
				meta := database.SteamMetadata{
					AppID:           sgdbAppID,
					Title:           sgdbAssets.GameTitle,
					CapsuleImage:    sgdbAssets.CoverURL,
					BackgroundImage: sgdbAssets.BackgroundURL,
					HeaderImage:     sgdbAssets.HeaderURL,
					ReleaseDate:     sgdbAssets.ReleaseDate,
					CachedAt:        time.Now().Unix(),
				}
				_ = s.db.SaveSteamMetadata(meta)
				_ = s.db.SetGameAppID(gameID, sgdbAppID)
				foundSGDB = true
			}
		}
		if !foundSGDB {
			_ = s.db.MarkGameSynced(gameID)
		}
	}

	return s.db.GetGameByID(gameID)
}

// StartBackgroundEnrichment runs an async worker that enriches unsynced games progressively
func (s *SteamService) StartBackgroundEnrichment(onGameUpdated func(gameID int64, appID int)) {
	s.mu.Lock()
	if s.isEnriching {
		s.mu.Unlock()
		return
	}
	s.isEnriching = true
	s.workerCtx, s.cancelFunc = context.WithCancel(context.Background())
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.isEnriching = false
			s.mu.Unlock()
		}()

		for {
			select {
			case <-s.workerCtx.Done():
				return
			default:
			}

			unsynced, err := s.db.GetUnsyncedGames()
			if err != nil || len(unsynced) == 0 {
				time.Sleep(5 * time.Second)
				continue
			}

			for _, game := range unsynced {
				select {
				case <-s.workerCtx.Done():
					return
				default:
				}

				targetAppID := game.SteamAppID
				searchTerm := game.SearchTitle
				if searchTerm == "" {
					searchTerm = game.CleanTitle
				}

				if targetAppID == 0 {
					sanitized := remote.SanitizeForSteamSearch(searchTerm)
					query := searchTerm
					if sanitized != "" {
						query = sanitized
					}

					appID, _, err := s.SearchGame(query)
					if err == nil {
						targetAppID = appID
					}
				}

				if targetAppID > 0 {
					meta, err := s.FetchAppDetails(targetAppID)
					if err != nil {
						log.Printf("[Steam] AppDetails error for %d: %v", targetAppID, err)
					}
					if meta != nil && (meta.CapsuleImage == "" || IsHorizontalAsset(meta.CapsuleImage) || meta.BackgroundImage == "" || strings.Contains(meta.BackgroundImage, "page_bg_generated")) {
						if s.sgdb != nil {
							sgdbTitle := searchTerm
							if meta.Title != "" {
								sgdbTitle = meta.Title
							}
							if sgdbAssets, err := s.sgdb.FindAssetsForGame(sgdbTitle); err == nil && sgdbAssets != nil {
								updated := false
								if (meta.CapsuleImage == "" || IsHorizontalAsset(meta.CapsuleImage)) && sgdbAssets.CoverURL != "" {
									meta.CapsuleImage = sgdbAssets.CoverURL
									updated = true
								}
								if (meta.BackgroundImage == "" || strings.Contains(meta.BackgroundImage, "page_bg_generated")) && sgdbAssets.BackgroundURL != "" {
									meta.BackgroundImage = sgdbAssets.BackgroundURL
									updated = true
								}
								if meta.HeaderImage == "" && sgdbAssets.HeaderURL != "" {
									meta.HeaderImage = sgdbAssets.HeaderURL
									updated = true
								}
								if updated {
									_ = s.db.SaveSteamMetadata(*meta)
								}
							}
						}
					}
					_ = s.db.SetGameAppID(game.ID, targetAppID)
					if onGameUpdated != nil {
						onGameUpdated(game.ID, targetAppID)
					}
				} else {
					// Fallback to SteamGridDB for non-Steam games (e.g. Need for Speed Carbon)
					foundSGDB := false
					if s.sgdb != nil {
						if sgdbAssets, err := s.sgdb.FindAssetsForGame(searchTerm); err == nil && sgdbAssets != nil {
							sgdbAppID := -sgdbAssets.GameID
							meta := database.SteamMetadata{
								AppID:           sgdbAppID,
								Title:           sgdbAssets.GameTitle,
								CapsuleImage:    sgdbAssets.CoverURL,
								BackgroundImage: sgdbAssets.BackgroundURL,
								HeaderImage:     sgdbAssets.HeaderURL,
								ReleaseDate:     sgdbAssets.ReleaseDate,
								CachedAt:        time.Now().Unix(),
							}
							_ = s.db.SaveSteamMetadata(meta)
							_ = s.db.SetGameAppID(game.ID, sgdbAppID)
							foundSGDB = true
							if onGameUpdated != nil {
								onGameUpdated(game.ID, sgdbAppID)
							}
						}
					}
					if !foundSGDB {
						_ = s.db.MarkGameSynced(game.ID)
					}
				}
			}
		}
	}()
}

// StopBackgroundEnrichment stops the background worker
func (s *SteamService) StopBackgroundEnrichment() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancelFunc != nil {
		s.cancelFunc()
	}
}

// IsHorizontalAsset returns true if an image URL is known to be a horizontal banner or capsule
func IsHorizontalAsset(url string) bool {
	if url == "" {
		return false
	}
	u := strings.ToLower(url)
	return strings.Contains(u, "capsule_231x87") ||
		strings.Contains(u, "capsule_616x353") ||
		strings.Contains(u, "capsule_467x181") ||
		strings.Contains(u, "header.jpg") ||
		strings.Contains(u, "header_alt")
}

// GetSteamGridCover returns a portrait 600x900 cover from SteamGridDB for the given title
func (s *SteamService) GetSteamGridCover(title string) (string, error) {
	if s.sgdb == nil {
		return "", fmt.Errorf("steamgriddb service not initialized")
	}
	clean := remote.SanitizeForSteamSearch(title)
	if clean == "" {
		clean = strings.TrimSpace(title)
	}
	if clean == "" {
		return "", fmt.Errorf("empty title")
	}
	assets, err := s.sgdb.FindAssetsForGame(clean)
	if err != nil {
		return "", err
	}
	if assets == nil || assets.CoverURL == "" {
		return "", fmt.Errorf("no portrait cover found on steamgriddb for %q", title)
	}
	return assets.CoverURL, nil
}

// GetSteamGridBanner returns a landscape or portrait banner/cover from SteamGridDB for the given title
func (s *SteamService) GetSteamGridBanner(title string) (string, error) {
	if s.sgdb == nil {
		return "", fmt.Errorf("steamgriddb service not initialized")
	}
	clean := remote.SanitizeForSteamSearch(title)
	if clean == "" {
		clean = strings.TrimSpace(title)
	}
	if clean == "" {
		return "", fmt.Errorf("empty title")
	}
	assets, err := s.sgdb.FindAssetsForGame(clean)
	if err != nil {
		return "", err
	}
	if assets == nil {
		return "", fmt.Errorf("no assets found on steamgriddb for %q", title)
	}
	if assets.BackgroundURL != "" {
		return assets.BackgroundURL, nil
	}
	if assets.HeaderURL != "" {
		return assets.HeaderURL, nil
	}
	if assets.CoverURL != "" {
		return assets.CoverURL, nil
	}
	return "", fmt.Errorf("no artwork found on steamgriddb for %q", title)
}

// GetSteamGridLogo returns a transparent logo from SteamGridDB for the given title
func (s *SteamService) GetSteamGridLogo(title string) (string, error) {
	if s.sgdb == nil {
		return "", fmt.Errorf("steamgriddb service not initialized")
	}
	clean := remote.SanitizeForSteamSearch(title)
	if clean == "" {
		clean = strings.TrimSpace(title)
	}
	if clean == "" {
		return "", fmt.Errorf("empty title")
	}
	assets, err := s.sgdb.FindAssetsForGame(clean)
	if err != nil {
		return "", err
	}
	if assets == nil || assets.LogoURL == "" {
		return "", fmt.Errorf("no logo found on steamgriddb for %q", title)
	}
	return assets.LogoURL, nil
}


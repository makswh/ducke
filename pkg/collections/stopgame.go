package collections

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gamevault/pkg/database"
)

var (
	// Catalog regexes
	reCompCard      = regexp.MustCompile(`(?s)<div data-key="(\d+)">\s*<article class="_card_[^"]*">(.*?)</article>`)
	reCompImg       = regexp.MustCompile(`(?s)<picture class="_game-image_[^"]*"[^>]*>[\s\S]*?<img src="([^"]+)"`)
	reCompGamepad   = regexp.MustCompile(`(?s)href=['"]#sg-new/gamepad[^'"]*['"][^>]*>[\s\S]*?</svg>\s*(\d+)`)
	reCompComments  = regexp.MustCompile(`(?s)href=['"]#sg-new/comments[^'"]*['"][^>]*>[\s\S]*?</svg>\s*(\d+)`)
	reCompRating    = regexp.MustCompile(`(?s)<span class="_rating-spinner__rating[^"]*">([^<]+)</span>`)
	reCompAuthorAv  = regexp.MustCompile(`(?s)<picture class="_user-info__avatar[^"]*"[^>]*>[\s\S]*?<img src="([^"]+)"`)
	reCompAuthorNm  = regexp.MustCompile(`(?s)<span class="_user-info__name[^"]*"[^>]*>([^<]+)</span>`)
	reCompAuthorURL = regexp.MustCompile(`(?s)<a class="_user-info[^"]*"\s+href="([^"]+)"`)
	reCompTitle     = regexp.MustCompile(`(?s)<a href="/games/compilation/\d+" class="_title_[^"]*">\s*([^<]+)\s*</a>`)
	reCompDesc      = regexp.MustCompile(`(?s)<span class="_content_[^"]*">\s*([\s\S]*?)\s*</span>`)
	rePageLink      = regexp.MustCompile(`page=(\d+)`)

	// Detail regexes
	reDetailH1     = regexp.MustCompile(`(?s)<h1[^>]*>\s*([\s\S]*?)\s*<span class="_count_[^"]*">\s*\((\d+)\)\s*</span>\s*</h1>`)
	reDetailDesc   = regexp.MustCompile(`(?s)<p class="_description_[^"]*">([\s\S]*?)</p>`)
	reDetailUpdate = regexp.MustCompile(`(?s)<span class="_last-update_[^"]*">([^<]+)</span>`)
	reGameCard     = regexp.MustCompile(`(?s)<div data-key="(\d+)">\s*<a\s+class="_card_[^"]*"[^>]*href="(/game/[^"]+)"[^>]*title="([^"]+)"[^>]*>(.*?)</a>\s*</div>`)
	reGameImg      = regexp.MustCompile(`<img class="_image_[^"]*"\s+src="([^"]+)"`)
	reGameRating   = regexp.MustCompile(`<button type="button" class="_rating_[^"]*"[^>]*>([^<]+)</button>`)
	reHTMLTags     = regexp.MustCompile(`<[^>]+>`)
)

type StopGameService struct {
	db     *database.Database
	client *http.Client
}

func NewStopGameService(db *database.Database) *StopGameService {
	tr := &http.Transport{
		MaxIdleConns:        20,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	return &StopGameService{
		db: db,
		client: &http.Client{
			Transport: tr,
			Timeout:   15 * time.Second,
		},
	}
}

func (s *StopGameService) fetchHTML(targetURL string) (string, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("stopgame returned HTTP %d for %s", resp.StatusCode, targetURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// FetchCompilations retrieves a paginated list of StopGame compilations
func (s *StopGameService) FetchCompilations(sort string, page int) (*CompilationsResponse, error) {
	if sort == "" {
		sort = "best"
	}
	if page < 1 {
		page = 1
	}

	cacheKey := fmt.Sprintf("catalog:%s:%d", sort, page)
	if s.db != nil {
		if cachedJSON, ok := s.db.GetStopGameCache(cacheKey, 2*time.Hour); ok {
			var resp CompilationsResponse
			if err := json.Unmarshal([]byte(cachedJSON), &resp); err == nil && len(resp.Items) > 0 {
				return &resp, nil
			}
		}
	}

	targetURL := fmt.Sprintf("https://stopgame.ru/games/compilations?sort=%s&page=%d", sort, page)
	htmlContent, err := s.fetchHTML(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch compilations: %w", err)
	}

	resp := s.ParseCompilationsCatalog(htmlContent, sort, page)

	if s.db != nil && len(resp.Items) > 0 {
		if b, err := json.Marshal(resp); err == nil {
			_ = s.db.SetStopGameCache(cacheKey, string(b))
		}
	}

	return resp, nil
}

// ParseCompilationsCatalog parses HTML of StopGame compilations catalog
func (s *StopGameService) ParseCompilationsCatalog(htmlContent, sort string, page int) *CompilationsResponse {
	resp := &CompilationsResponse{
		Items:       make([]CompilationSummary, 0),
		TotalPages:  1,
		CurrentPage: page,
		Sort:        sort,
	}

	// Detect max page number
	pageMatches := rePageLink.FindAllStringSubmatch(htmlContent, -1)
	maxPage := 1
	for _, m := range pageMatches {
		if len(m) > 1 {
			if p, err := strconv.Atoi(m[1]); err == nil && p > maxPage {
				maxPage = p
			}
		}
	}
	if maxPage > 1 {
		resp.TotalPages = maxPage
	}

	// Extract compilation cards
	cards := reCompCard.FindAllStringSubmatch(htmlContent, -1)
	for _, cardMatch := range cards {
		if len(cardMatch) < 3 {
			continue
		}
		compID := cardMatch[1]
		body := cardMatch[2]

		item := CompilationSummary{
			ID:            compID,
			URL:           fmt.Sprintf("/games/compilation/%s", compID),
			PreviewImages: make([]string, 0),
		}

		// Title
		if tm := reCompTitle.FindStringSubmatch(body); len(tm) > 1 {
			item.Title = strings.TrimSpace(html.UnescapeString(tm[1]))
		}

		// Images
		imgMatches := reCompImg.FindAllStringSubmatch(body, -1)
		for _, im := range imgMatches {
			if len(im) > 1 && im[1] != "" {
				item.PreviewImages = append(item.PreviewImages, im[1])
			}
		}

		// Gamepad count
		if gm := reCompGamepad.FindStringSubmatch(body); len(gm) > 1 {
			if count, err := strconv.Atoi(gm[1]); err == nil {
				item.GamesCount = count
			}
		}

		// Comments count
		if cm := reCompComments.FindStringSubmatch(body); len(cm) > 1 {
			if count, err := strconv.Atoi(cm[1]); err == nil {
				item.CommentsCount = count
			}
		}

		// Rating
		if rm := reCompRating.FindStringSubmatch(body); len(rm) > 1 {
			item.Rating = strings.TrimSpace(rm[1])
		}

		// Author
		if am := reCompAuthorNm.FindStringSubmatch(body); len(am) > 1 {
			item.AuthorName = strings.TrimSpace(html.UnescapeString(am[1]))
		}
		if avm := reCompAuthorAv.FindStringSubmatch(body); len(avm) > 1 {
			item.AuthorAvatar = avm[1]
		}
		if aum := reCompAuthorURL.FindStringSubmatch(body); len(aum) > 1 {
			item.AuthorURL = aum[1]
		}

		// Description
		if dm := reCompDesc.FindStringSubmatch(body); len(dm) > 1 {
			desc := reHTMLTags.ReplaceAllString(dm[1], " ")
			desc = strings.TrimSpace(html.UnescapeString(desc))
			item.Description = desc
		}

		resp.Items = append(resp.Items, item)
	}

	return resp
}

// FetchCompilationDetail retrieves full compilation info and all its games
func (s *StopGameService) FetchCompilationDetail(id string, forceRefresh bool) (*CompilationDetail, error) {
	cacheKey := fmt.Sprintf("detail:%s", id)

	if !forceRefresh && s.db != nil {
		if cachedJSON, ok := s.db.GetStopGameCache(cacheKey, 24*time.Hour); ok {
			var detail CompilationDetail
			if err := json.Unmarshal([]byte(cachedJSON), &detail); err == nil && len(detail.Games) > 0 {
				// Re-match against library in memory so dynamic downloads/new games are recognized
				s.matchGamesAgainstLibrary(&detail)
				return &detail, nil
			}
		}
	}

	// Fetch Page 1
	page1URL := fmt.Sprintf("https://stopgame.ru/games/compilation/%s", id)
	html1, err := s.fetchHTML(page1URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch compilation detail %s: %w", id, err)
	}

	detail := s.ParseCompilationDetail(id, html1)

	// Fetch remaining pages concurrently if compilation spans multiple pages
	if detail.GamesCount > len(detail.Games) {
		totalPages := (detail.GamesCount + 29) / 30
		if totalPages > 1 {
			// Cap at 10 pages (300 games) to keep response fast
			if totalPages > 10 {
				totalPages = 10
			}

			type pageResult struct {
				pageNum int
				games   []CompilationGame
			}

			resultsChan := make(chan pageResult, totalPages)
			var wg sync.WaitGroup

			for p := 2; p <= totalPages; p++ {
				wg.Add(1)
				go func(pageNum int) {
					defer wg.Done()
					pURL := fmt.Sprintf("https://stopgame.ru/games/compilation/%s/p%d", id, pageNum)
					if pHtml, err := s.fetchHTML(pURL); err == nil {
						gList := s.ParseGamesGrid(pHtml)
						resultsChan <- pageResult{pageNum: pageNum, games: gList}
					}
				}(p)
			}

			wg.Wait()
			close(resultsChan)

			// Collect results in ordered map
			pageGames := make(map[int][]CompilationGame)
			for res := range resultsChan {
				pageGames[res.pageNum] = res.games
			}

			for p := 2; p <= totalPages; p++ {
				if gList, ok := pageGames[p]; ok {
					detail.Games = append(detail.Games, gList...)
				}
			}
		}
	}

	// Match games against Ducke library
	s.matchGamesAgainstLibrary(detail)

	// Cache result
	if s.db != nil && len(detail.Games) > 0 {
		if b, err := json.Marshal(detail); err == nil {
			_ = s.db.SetStopGameCache(cacheKey, string(b))
		}
	}

	return detail, nil
}

// ParseCompilationDetail parses detail page header and page 1 games
func (s *StopGameService) ParseCompilationDetail(id, htmlContent string) *CompilationDetail {
	detail := &CompilationDetail{
		ID:    id,
		Games: make([]CompilationGame, 0),
	}

	// Title and count
	if tm := reDetailH1.FindStringSubmatch(htmlContent); len(tm) > 1 {
		rawTitle := reHTMLTags.ReplaceAllString(tm[1], "")
		detail.Title = strings.TrimSpace(html.UnescapeString(rawTitle))
		if len(tm) > 2 {
			if count, err := strconv.Atoi(tm[2]); err == nil {
				detail.GamesCount = count
			}
		}
	}

	// Author
	if am := reCompAuthorNm.FindStringSubmatch(htmlContent); len(am) > 1 {
		detail.AuthorName = strings.TrimSpace(html.UnescapeString(am[1]))
	}
	if avm := reCompAuthorAv.FindStringSubmatch(htmlContent); len(avm) > 1 {
		detail.AuthorAvatar = avm[1]
	}
	if aum := reCompAuthorURL.FindStringSubmatch(htmlContent); len(aum) > 1 {
		detail.AuthorURL = aum[1]
	}

	// Description
	if dm := reDetailDesc.FindStringSubmatch(htmlContent); len(dm) > 1 {
		cleanDesc := strings.ReplaceAll(dm[1], "<br />", "\n")
		cleanDesc = strings.ReplaceAll(cleanDesc, "<br/>", "\n")
		cleanDesc = strings.ReplaceAll(cleanDesc, "<br>", "\n")
		cleanDesc = reHTMLTags.ReplaceAllString(cleanDesc, "")
		detail.Description = strings.TrimSpace(html.UnescapeString(cleanDesc))
	}

	// Rating
	if rm := reCompRating.FindStringSubmatch(htmlContent); len(rm) > 1 {
		detail.Rating = strings.TrimSpace(rm[1])
	}

	// Last update
	if um := reDetailUpdate.FindStringSubmatch(htmlContent); len(um) > 1 {
		detail.LastUpdated = strings.TrimSpace(html.UnescapeString(um[1]))
	}

	// Games
	detail.Games = s.ParseGamesGrid(htmlContent)
	if detail.GamesCount == 0 && len(detail.Games) > 0 {
		detail.GamesCount = len(detail.Games)
	}

	return detail
}

// ParseGamesGrid parses game cards in a compilation grid
func (s *StopGameService) ParseGamesGrid(htmlContent string) []CompilationGame {
	games := make([]CompilationGame, 0)
	matches := reGameCard.FindAllStringSubmatch(htmlContent, -1)

	for _, m := range matches {
		if len(m) < 5 {
			continue
		}
		gameID := m[1]
		gameURL := m[2]
		rawTitle := m[3]
		cardBody := m[4]

		cleanTitle := strings.TrimSpace(html.UnescapeString(rawTitle))

		g := CompilationGame{
			StopGameID: gameID,
			Title:      cleanTitle,
			URL:        gameURL,
		}

		if im := reGameImg.FindStringSubmatch(cardBody); len(im) > 1 {
			g.PosterURL = im[1]
		}
		if rm := reGameRating.FindStringSubmatch(cardBody); len(rm) > 1 {
			g.StopGameScore = strings.TrimSpace(rm[1])
		}

		games = append(games, g)
	}

	return games
}

func (s *StopGameService) matchGamesAgainstLibrary(detail *CompilationDetail) {
	if s.db == nil || detail == nil {
		return
	}

	matchedCount := 0
	for i := range detail.Games {
		game := &detail.Games[i]
		if matched := s.db.MatchLibraryGame(game.Title); matched != nil {
			game.InLibrary = true
			game.DuckeGame = matched
			matchedCount++
		} else {
			game.InLibrary = false
			game.DuckeGame = nil
		}
	}
	detail.MatchedCount = matchedCount
}

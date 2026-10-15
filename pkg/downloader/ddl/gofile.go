package ddl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

var (
	gofileRegex = regexp.MustCompile(`(?i)gofile\.io/(?:d|f)/([a-zA-Z0-9_-]+)`)

	gofileTokenMu   sync.Mutex
	gofileToken     string
	gofileTokenTime time.Time
)

const (
	// gofilePrimarySalt is the latest salt extracted from gofile-downloader / wt.obf.js
	gofilePrimarySalt   = "12af056dacea0b"
	gofileSecondarySalt = "9844d94d963d30"
	gofileTimeBucket    = 14400 // 4 hours in seconds
	// gofileUserAgent must exactly match the string used in hashing
	gofileUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	gofileLang      = "en-US"
)

// GoFileResolver extracts direct download links from gofile.io
type GoFileResolver struct {
	client *http.Client
}

// NewGoFileResolver creates a new GoFile resolver with DoH resilient client
func NewGoFileResolver() *GoFileResolver {
	return &GoFileResolver{
		client: NewResilientHTTPClient(25 * time.Second),
	}
}

func (r *GoFileResolver) Name() string {
	return HosterGoFile
}

func (r *GoFileResolver) CanHandle(rawURL string) bool {
	return gofileRegex.MatchString(rawURL)
}

func parseGoFileContentID(rawURL string) (string, error) {
	matches := gofileRegex.FindStringSubmatch(rawURL)
	if len(matches) < 2 {
		return "", fmt.Errorf("invalid GoFile URL: %s", rawURL)
	}
	return matches[1], nil
}

func getActiveSalt() string {
	if s := os.Getenv("GOFILE_SALT"); s != "" {
		return s
	}
	return gofilePrimarySalt
}

// generateWebsiteToken calculates SHA256 signature required by GoFile API
func generateWebsiteToken(userAgent, apiToken, salt string) string {
	bucket := time.Now().Unix() / gofileTimeBucket
	raw := fmt.Sprintf("%s::%s::%s::%d::%s", userAgent, gofileLang, apiToken, bucket, salt)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// getGuestToken fetches or returns a cached guest account token
func (r *GoFileResolver) getGuestToken(ctx context.Context) (string, error) {
	gofileTokenMu.Lock()
	defer gofileTokenMu.Unlock()

	// Token is valid for 12 hours
	if gofileToken != "" && time.Since(gofileTokenTime) < 12*time.Hour {
		return gofileToken, nil
	}

	salt := getActiveSalt()
	wt := generateWebsiteToken(gofileUserAgent, "", salt)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.gofile.io/accounts", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", gofileUserAgent)
	req.Header.Set("X-Website-Token", wt)
	req.Header.Set("X-BL", gofileLang)
	req.Header.Set("Origin", "https://gofile.io")
	req.Header.Set("Referer", "https://gofile.io/")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create GoFile guest account: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var res struct {
		Status string `json:"status"`
		Data   struct {
			Token string `json:"token"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("failed to parse GoFile account response: %w", err)
	}

	if res.Status != "ok" || res.Data.Token == "" {
		return "", fmt.Errorf("GoFile account creation returned status %s", res.Status)
	}

	gofileToken = res.Data.Token
	gofileTokenTime = time.Now()
	return gofileToken, nil
}

type gofileChild struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	Link string `json:"link"`
}

// Resolve extracts the direct download URL for a gofile link
func (r *GoFileResolver) Resolve(ctx context.Context, rawURL string) (*ResolvedItem, error) {
	contentID, err := parseGoFileContentID(rawURL)
	if err != nil {
		return nil, err
	}

	token, err := r.getGuestToken(ctx)
	if err != nil {
		return nil, err
	}

	// Try primary salt first, then fallback to secondary salt if status is not ok
	salts := []string{getActiveSalt()}
	if salts[0] != gofileSecondarySalt {
		salts = append(salts, gofileSecondarySalt)
	}

	var lastErr error
	for _, salt := range salts {
		item, sErr := r.fetchContent(ctx, rawURL, contentID, token, salt)
		if sErr == nil && item != nil {
			return item, nil
		}
		lastErr = sErr
		log.Printf("[GoFile] Fetch with salt %s returned error: %v, trying fallback salt...", salt, sErr)
	}

	return nil, lastErr
}

func (r *GoFileResolver) fetchContent(ctx context.Context, rawURL, contentID, token, salt string) (*ResolvedItem, error) {
	wt := generateWebsiteToken(gofileUserAgent, token, salt)

	apiURL := fmt.Sprintf("https://api.gofile.io/contents/%s?cache=true&sortField=createTime&sortDirection=1", contentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", gofileUserAgent)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Cookie", "accountToken="+token)
	req.Header.Set("X-Website-Token", wt)
	req.Header.Set("X-BL", gofileLang)
	req.Header.Set("Origin", "https://gofile.io")
	req.Header.Set("Referer", "https://gofile.io/")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GoFile API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GoFile API returned HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res struct {
		Status string `json:"status"`
		Data   struct {
			Name     string                 `json:"name"`
			Children map[string]gofileChild `json:"children"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GoFile content: %w", err)
	}

	if res.Status != "ok" {
		return nil, fmt.Errorf("GoFile returned non-ok status: %s", res.Status)
	}

	if len(res.Data.Children) == 0 {
		return nil, fmt.Errorf("GoFile folder %s has no files or is empty", contentID)
	}

	// Pick the first file entry
	var selectedFile *gofileChild
	for _, child := range res.Data.Children {
		c := child
		if c.Type == "file" && c.Link != "" {
			selectedFile = &c
			break
		}
	}

	if selectedFile == nil {
		return nil, fmt.Errorf("no valid downloadable file found in GoFile folder %s", contentID)
	}

	headers := map[string]string{
		"User-Agent":    gofileUserAgent,
		"Authorization": "Bearer " + token,
		"Cookie":        "accountToken=" + token,
		"Referer":       "https://gofile.io/",
	}

	return &ResolvedItem{
		DirectURL:   selectedFile.Link,
		Filename:    selectedFile.Name,
		SizeBytes:   selectedFile.Size,
		Headers:     headers,
		Hoster:      HosterGoFile,
		OriginalURL: rawURL,
	}, nil
}

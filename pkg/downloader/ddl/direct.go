package ddl

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	fileditchRegex  = regexp.MustCompile(`(?i)(?:fileditchstuff\.me|fileditchfiles\.me|fileditch\.com|theditch\.st)`)
	archiveExtRegex = regexp.MustCompile(`(?i)\.(rar|zip|7z|tar|gz|iso|bin|exe|001|part\d+\.rar)$`)
)

// DirectResolver handles Fileditch and direct HTTP file URLs
type DirectResolver struct {
	client *http.Client
}

// NewDirectResolver creates a new direct link resolver with resilient client
func NewDirectResolver() *DirectResolver {
	return &DirectResolver{
		client: NewResilientHTTPClient(12 * time.Second),
	}
}

func (r *DirectResolver) Name() string {
	return HosterFileditch
}

func (r *DirectResolver) CanHandle(rawURL string) bool {
	clean := strings.TrimSpace(rawURL)
	if strings.HasPrefix(strings.ToLower(clean), "magnet:") {
		return false
	}
	if fileditchRegex.MatchString(clean) {
		return true
	}
	return archiveExtRegex.MatchString(clean) || strings.HasPrefix(clean, "http://") || strings.HasPrefix(clean, "https://")
}

// getFileditchCandidateURLs returns URLs to probe including alternate Fileditch domains if applicable
func getFileditchCandidateURLs(rawURL string) []string {
	urls := []string{rawURL}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return urls
	}

	hostLower := strings.ToLower(parsed.Host)
	if strings.Contains(hostLower, "fileditchstuff.me") {
		// If small.fileditchstuff.me, also add fileditchfiles.me and fileditch.com candidates
		cand1 := *parsed
		cand1.Host = "fileditchfiles.me"
		urls = append(urls, cand1.String())

		cand2 := *parsed
		cand2.Host = "fileditch.com"
		urls = append(urls, cand2.String())
	}

	return urls
}

// Resolve inspects a direct URL to determine file size and filename
func (r *DirectResolver) Resolve(ctx context.Context, rawURL string) (*ResolvedItem, error) {
	const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	candidateURLs := getFileditchCandidateURLs(rawURL)
	var lastErr error

	for _, targetURL := range candidateURLs {
		item, err := r.probeSingleURL(ctx, targetURL, rawURL, userAgent)
		if err == nil && item != nil {
			return item, nil
		}
		lastErr = err
	}

	// If probe failed on all candidates (server offline/purged), return item with filename parsed from URL
	// so that failover/next mirrors can still proceed or report descriptive status
	parsed, err := url.Parse(rawURL)
	defaultFilename := "download.bin"
	if err == nil {
		defaultFilename = path.Base(parsed.Path)
	}

	return &ResolvedItem{
		DirectURL:   rawURL,
		Filename:    defaultFilename,
		SizeBytes:   0,
		Headers:     map[string]string{"User-Agent": userAgent, "Referer": "https://fileditch.com/"},
		Hoster:      r.determineHoster(rawURL),
		OriginalURL: rawURL,
	}, lastErr
}

func (r *DirectResolver) probeSingleURL(ctx context.Context, targetURL, originalURL, userAgent string) (*ResolvedItem, error) {
	var sizeBytes int64 = 0
	filename := ""

	// 1. Try fast HEAD probe with 2.5s sub-timeout
	headCtx, headCancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	req, err := http.NewRequestWithContext(headCtx, http.MethodHead, targetURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Referer", "https://fileditch.com/")
		req.Header.Set("Accept", "*/*")
		resp, err := r.client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
				if cl := resp.Header.Get("Content-Length"); cl != "" {
					sizeBytes, _ = strconv.ParseInt(cl, 10, 64)
				}
				if cd := resp.Header.Get("Content-Disposition"); cd != "" {
					if _, params, err := mime.ParseMediaType(cd); err == nil {
						filename = params["filename"]
					}
				}
			}
		}
	}
	headCancel()

	// 2. If HEAD failed or gave 0 bytes, try a range probe (Range: bytes=0-0)
	if sizeBytes <= 0 {
		probeCtx, probeCancel := context.WithTimeout(ctx, 4*time.Second)
		defer probeCancel()
		probeReq, err := http.NewRequestWithContext(probeCtx, http.MethodGet, targetURL, nil)
		if err == nil {
			probeReq.Header.Set("User-Agent", userAgent)
			probeReq.Header.Set("Referer", "https://fileditch.com/")
			probeReq.Header.Set("Accept", "*/*")
			probeReq.Header.Set("Range", "bytes=0-0")
			if probeResp, err := r.client.Do(probeReq); err == nil {
				defer probeResp.Body.Close()
				if probeResp.StatusCode == http.StatusOK || probeResp.StatusCode == http.StatusPartialContent {
					if cr := probeResp.Header.Get("Content-Range"); cr != "" {
						if slashIdx := strings.LastIndex(cr, "/"); slashIdx != -1 {
							totalStr := strings.TrimSpace(cr[slashIdx+1:])
							sizeBytes, _ = strconv.ParseInt(totalStr, 10, 64)
						}
					} else if cl := probeResp.Header.Get("Content-Length"); cl != "" {
						sizeBytes, _ = strconv.ParseInt(cl, 10, 64)
					}
					if filename == "" {
						if cd := probeResp.Header.Get("Content-Disposition"); cd != "" {
							if _, params, err := mime.ParseMediaType(cd); err == nil {
								filename = params["filename"]
							}
						}
					}
				}
			}
		}
	}

	if sizeBytes <= 0 && filename == "" {
		return nil, fmt.Errorf("probe failed to retrieve file metadata from %s", targetURL)
	}

	if filename == "" {
		if parsed, err := url.Parse(targetURL); err == nil {
			filename = path.Base(parsed.Path)
		}
	}

	return &ResolvedItem{
		DirectURL:   targetURL,
		Filename:    filename,
		SizeBytes:   sizeBytes,
		Headers:     map[string]string{"User-Agent": userAgent, "Referer": "https://fileditch.com/"},
		Hoster:      r.determineHoster(originalURL),
		OriginalURL: originalURL,
	}, nil
}

func (r *DirectResolver) determineHoster(rawURL string) string {
	if fileditchRegex.MatchString(rawURL) {
		return HosterFileditch
	}
	return HosterDirect
}

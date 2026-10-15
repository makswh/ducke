package ddl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

var (
	pixeldrainFileRegex = regexp.MustCompile(`(?i)pixeldrain\.com/(?:u|api/file)/([a-zA-Z0-9_-]+)`)
	pixeldrainListRegex = regexp.MustCompile(`(?i)pixeldrain\.com/l/([a-zA-Z0-9_-]+)`)
)

// PixelDrainResolver extracts direct download links from pixeldrain.com
type PixelDrainResolver struct {
	client *http.Client
}

// NewPixelDrainResolver creates a new PixelDrain resolver
func NewPixelDrainResolver() *PixelDrainResolver {
	return &PixelDrainResolver{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (r *PixelDrainResolver) Name() string {
	return HosterPixelDrain
}

func (r *PixelDrainResolver) CanHandle(rawURL string) bool {
	return pixeldrainFileRegex.MatchString(rawURL) || pixeldrainListRegex.MatchString(rawURL)
}

type pixeldrainFileInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Success bool   `json:"success,omitempty"`
}

type pixeldrainListInfo struct {
	ID      string               `json:"id"`
	Title   string               `json:"title"`
	Files   []pixeldrainFileInfo `json:"files"`
	Success bool                 `json:"success,omitempty"`
}

// Resolve extracts the direct download URL for a pixeldrain link
func (r *PixelDrainResolver) Resolve(ctx context.Context, rawURL string) (*ResolvedItem, error) {
	// 1. Single file check
	if m := pixeldrainFileRegex.FindStringSubmatch(rawURL); len(m) >= 2 {
		fileID := m[1]
		infoURL := fmt.Sprintf("https://pixeldrain.com/api/file/%s/info", fileID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, infoURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 Ducke/1.0")

		resp, err := r.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("PixelDrain API request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("PixelDrain file info returned HTTP %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var info pixeldrainFileInfo
		if err := json.Unmarshal(body, &info); err != nil {
			return nil, fmt.Errorf("failed to parse PixelDrain file info: %w", err)
		}

		directURL := fmt.Sprintf("https://pixeldrain.com/api/file/%s?download", fileID)
		filename := info.Name
		if filename == "" {
			filename = fileID
		}

		return &ResolvedItem{
			DirectURL:   directURL,
			Filename:    filename,
			SizeBytes:   info.Size,
			Hoster:      HosterPixelDrain,
			OriginalURL: rawURL,
		}, nil
	}

	// 2. List check
	if m := pixeldrainListRegex.FindStringSubmatch(rawURL); len(m) >= 2 {
		listID := m[1]
		listURL := fmt.Sprintf("https://pixeldrain.com/api/list/%s", listID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 Ducke/1.0")

		resp, err := r.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("PixelDrain list API request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("PixelDrain list API returned HTTP %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var listInfo pixeldrainListInfo
		if err := json.Unmarshal(body, &listInfo); err != nil {
			return nil, fmt.Errorf("failed to parse PixelDrain list info: %w", err)
		}

		if len(listInfo.Files) == 0 {
			return nil, fmt.Errorf("PixelDrain list %s contains no files", listID)
		}

		first := listInfo.Files[0]
		directURL := fmt.Sprintf("https://pixeldrain.com/api/file/%s?download", first.ID)
		return &ResolvedItem{
			DirectURL:   directURL,
			Filename:    first.Name,
			SizeBytes:   first.Size,
			Hoster:      HosterPixelDrain,
			OriginalURL: rawURL,
		}, nil
	}

	return nil, fmt.Errorf("unrecognized PixelDrain URL: %s", rawURL)
}

package ddl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"
)

var (
	oneFichierRegex = regexp.MustCompile(`(?i)1fichier\.com/\?([a-zA-Z0-9_-]+)`)
)

// OneFichierResolver handles 1fichier links via API or web ticket
type OneFichierResolver struct {
	client *http.Client
	apiKey string
}

// NewOneFichierResolver creates a 1fichier resolver
func NewOneFichierResolver(apiKey string) *OneFichierResolver {
	if apiKey == "" {
		apiKey = os.Getenv("ONEFICHIER_API_KEY")
	}
	return &OneFichierResolver{
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
		apiKey: apiKey,
	}
}

func (r *OneFichierResolver) Name() string {
	return Hoster1fichier
}

func (r *OneFichierResolver) CanHandle(rawURL string) bool {
	return oneFichierRegex.MatchString(rawURL)
}

// Resolve extracts the direct link if API key is present or provides ticket info
func (r *OneFichierResolver) Resolve(ctx context.Context, rawURL string) (*ResolvedItem, error) {
	if r.apiKey != "" {
		endpoint := "https://api.1fichier.com/v1/download/get_token.pl"
		payload, _ := json.Marshal(map[string]any{
			"url":            rawURL,
			"amilgotthelock": 1,
		})

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+r.apiKey)

		resp, err := r.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("1fichier API error: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var res struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			URL     string `json:"url"`
		}
		if err := json.Unmarshal(body, &res); err != nil {
			return nil, fmt.Errorf("failed to parse 1fichier response: %w", err)
		}

		if res.Status != "OK" || res.URL == "" {
			return nil, fmt.Errorf("1fichier error: %s", res.Message)
		}

		return &ResolvedItem{
			DirectURL:   res.URL,
			Filename:    "1fichier_download",
			SizeBytes:   0,
			Hoster:      Hoster1fichier,
			OriginalURL: rawURL,
		}, nil
	}

	// Without API key, 1fichier free tier has wait queues and captchas.
	// We report that free download requires mirrors or API key.
	return nil, fmt.Errorf("1fichier free downloads require waiting queues; please use alternative mirrors (PixelDrain, GoFile, Fileditch)")
}

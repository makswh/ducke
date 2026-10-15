package ddl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestLivePixelDrainResolver(t *testing.T) {
	resolver := NewPixelDrainResolver()
	urls := []string{
		"https://pixeldrain.com/u/2QYQVULw",
		"https://pixeldrain.com/u/DssFWXh4",
	}

	for _, u := range urls {
		t.Run(u, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			item, err := resolver.Resolve(ctx, u)
			if err != nil {
				t.Fatalf("PixelDrain Resolve failed for %s: %v", u, err)
			}

			t.Logf("[PixelDrain] OK! Filename: %s, SizeBytes: %d (%s), DirectURL: %s",
				item.Filename, item.SizeBytes, formatBytes(item.SizeBytes), item.DirectURL)

			if item.DirectURL == "" {
				t.Fatalf("PixelDrain returned empty DirectURL")
			}

			// Test downloading first 1024 bytes via Range request
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.DirectURL, nil)
			if err != nil {
				t.Fatalf("failed to create download request: %v", err)
			}
			req.Header.Set("Range", "bytes=0-1023")
			req.Header.Set("User-Agent", "Mozilla/5.0 Ducke/1.0")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("PixelDrain download probe failed: %v", err)
			}
			defer resp.Body.Close()

			t.Logf("[PixelDrain] Download status: %s, Content-Range: %s, Content-Length: %s",
				resp.Status, resp.Header.Get("Content-Range"), resp.Header.Get("Content-Length"))

			buf := make([]byte, 1024)
			n, err := io.ReadFull(resp.Body, buf)
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				t.Fatalf("PixelDrain read probe failed: %v", err)
			}
			t.Logf("[PixelDrain] Successfully read %d bytes from stream", n)
		})
	}
}

func TestLiveFileditchResolver(t *testing.T) {
	resolver := NewDirectResolver()
	urls := []string{
		"https://small.fileditchstuff.me/s16/dLITTKHFdhziMVEMuyrn.rar",
		"https://small.fileditchstuff.me/s16/nSVyQTMnbjLNDpdYuClK.rar",
	}

	for _, u := range urls {
		t.Run(u, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			item, err := resolver.Resolve(ctx, u)
			if err != nil {
				t.Logf("[Fileditch] Note: Resolve error (server may be offline): %v", err)
				return
			}

			t.Logf("[Fileditch] OK! Filename: %s, SizeBytes: %d, DirectURL: %s",
				item.Filename, item.SizeBytes, item.DirectURL)

			// Probe stream using resolver's resilient client and headers
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.DirectURL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			for k, v := range item.Headers {
				req.Header.Set(k, v)
			}
			req.Header.Set("Range", "bytes=0-1023")

			resp, err := resolver.client.Do(req)
			if err != nil {
				t.Logf("[Fileditch] Download request error: %v", err)
				return
			}
			defer resp.Body.Close()

			t.Logf("[Fileditch] Download status: %s, Content-Range: %s",
				resp.Status, resp.Header.Get("Content-Range"))
		})
	}
}

func TestLiveGoFileResolver(t *testing.T) {
	resolver := NewGoFileResolver()
	urls := []string{
		"https://gofile.io/d/7vkx6I",
		"https://gofile.io/d/2ElBtJ",
	}

	for _, u := range urls {
		t.Run(u, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			item, err := resolver.Resolve(ctx, u)
			if err != nil {
				t.Logf("[GoFile] Resolve note: %v", err)
				return
			}

			t.Logf("[GoFile] OK! Filename: %s, SizeBytes: %d (%s), DirectURL: %s",
				item.Filename, item.SizeBytes, formatBytes(item.SizeBytes), item.DirectURL)

			if item.DirectURL == "" {
				t.Fatalf("GoFile returned empty DirectURL")
			}

			// Probe direct CDN download
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.DirectURL, nil)
			if err != nil {
				t.Fatalf("failed to create download request: %v", err)
			}
			for k, v := range item.Headers {
				req.Header.Set(k, v)
			}
			req.Header.Set("Range", "bytes=0-1023")

			resp, err := resolver.client.Do(req)
			if err != nil {
				t.Logf("[GoFile] CDN probe note: %v", err)
				return
			}
			defer resp.Body.Close()

			t.Logf("[GoFile] Download status: %s, Content-Range: %s",
				resp.Status, resp.Header.Get("Content-Range"))

			buf := make([]byte, 1024)
			n, _ := io.ReadFull(resp.Body, buf)
			t.Logf("[GoFile] Read %d bytes from CDN stream", n)
		})
	}
}

func TestLive1fichierResolver(t *testing.T) {
	resolver := NewOneFichierResolver("")
	urls := []string{
		"https://1fichier.com/?d79rdlypo3qdf4tfyhfv",
	}

	for _, u := range urls {
		t.Run(u, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			item, err := resolver.Resolve(ctx, u)
			if err != nil {
				t.Logf("[1fichier] Expected free tier response: %v", err)
			} else {
				t.Logf("[1fichier] Resolved item: %+v", item)
			}
		})
	}
}

func TestDoHHostResolution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	hosts := []string{"api.gofile.io", "pixeldrain.com", "fileditchfiles.me"}
	for _, h := range hosts {
		ips, err := ResolveHostDoH(ctx, h)
		if err != nil {
			t.Fatalf("Failed to resolve %s via DoH: %v", h, err)
		}
		if len(ips) == 0 {
			t.Fatalf("Expected at least one IP for %s, got none", h)
		}
		t.Logf("[DoH] Host %s resolved successfully to: %v", h, ips)
	}
}

func TestLiveGrouperAndPartPlan(t *testing.T) {
	uris := []string{
		"https://gofile.io/d/7vkx6I",
		"https://small.fileditchstuff.me/s16/dLITTKHFdhziMVEMuyrn.rar",
		"https://pixeldrain.com/u/2QYQVULw",
		"https://1fichier.com/?d79rdlypo3qdf4tfyhfv",
		"https://gofile.io/d/2ElBtJ",
		"https://small.fileditchstuff.me/s16/nSVyQTMnbjLNDpdYuClK.rar",
		"https://pixeldrain.com/u/DssFWXh4",
		"https://1fichier.com/?qxanthc19ntz7a2k4hcr",
	}

	grouper := NewGrouper("")
	plan, err := grouper.BuildPlan(uris, "")
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	t.Logf("Plan SelectedHoster: %s", plan.SelectedHoster)
	t.Logf("Plan TotalParts: %d", len(plan.Parts))

	if plan.SelectedHoster != HosterPixelDrain {
		t.Fatalf("Expected selected hoster to be %s, got %s", HosterPixelDrain, plan.SelectedHoster)
	}

	if len(plan.Parts) != 2 {
		t.Fatalf("Expected 2 parts, got %d", len(plan.Parts))
	}

	for i, part := range plan.Parts {
		t.Logf("Part %d: %d mirrors", part.PartIndex, len(part.Mirrors))
		for _, m := range part.Mirrors {
			t.Logf("  - [%s] %s", m.Hoster, m.OriginalURL)
		}

		// Verify that the preferred mirror resolves
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resolved, rErr := grouper.ResolvePartDirectURL(ctx, part.Mirrors[0])
		cancel()

		if rErr != nil {
			t.Fatalf("Failed to resolve preferred mirror for part %d: %v", i+1, rErr)
		}

		t.Logf("Part %d Resolved: Name=%s, Size=%s, URL=%s",
			part.PartIndex, resolved.Filename, formatBytes(resolved.SizeBytes), resolved.DirectURL)
	}
}

func formatBytes(b int64) string {
	if b >= 1024*1024*1024 {
		return fmt.Sprintf("%.2f GB", float64(b)/(1024*1024*1024))
	}
	if b >= 1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(b)/(1024*1024))
	}
	if b >= 1024 {
		return fmt.Sprintf("%.2f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%d B", b)
}


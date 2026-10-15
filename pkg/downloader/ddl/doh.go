package ddl

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	dohCache   = make(map[string]dohEntry)
	dohCacheMu sync.RWMutex
)

type dohEntry struct {
	ips     []string
	expires time.Time
}

// ResolveHostDoH resolves a hostname using DNS-over-HTTPS (Cloudflare / Google)
// with in-memory caching and fallback to standard system resolver.
func ResolveHostDoH(ctx context.Context, host string) ([]string, error) {
	// 1. Check in-memory cache
	dohCacheMu.RLock()
	if entry, found := dohCache[host]; found && time.Now().Before(entry.expires) {
		dohCacheMu.RUnlock()
		return entry.ips, nil
	}
	dohCacheMu.RUnlock()

	// 2. Try fast system DNS first (1.5s timeout)
	sysCtx, sysCancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	ips, err := net.DefaultResolver.LookupHost(sysCtx, host)
	sysCancel()
	if err == nil && len(ips) > 0 {
		cacheIPs(host, ips, 10*time.Minute)
		return ips, nil
	}

	// 3. Fallback to Cloudflare DoH
	dohIPs, dohErr := queryCloudflareDoH(ctx, host)
	if dohErr == nil && len(dohIPs) > 0 {
		cacheIPs(host, dohIPs, 30*time.Minute)
		return dohIPs, nil
	}

	// 4. Fallback to Google DoH
	dohIPs, dohErr = queryGoogleDoH(ctx, host)
	if dohErr == nil && len(dohIPs) > 0 {
		cacheIPs(host, dohIPs, 30*time.Minute)
		return dohIPs, nil
	}

	if len(ips) > 0 {
		return ips, nil
	}
	return nil, fmt.Errorf("failed to resolve %s via DNS and DoH: %v", host, err)
}

func cacheIPs(host string, ips []string, ttl time.Duration) {
	dohCacheMu.Lock()
	defer dohCacheMu.Unlock()
	dohCache[host] = dohEntry{
		ips:     ips,
		expires: time.Now().Add(ttl),
	}
}

// queryCloudflareDoH queries Cloudflare's 1.1.1.1 DoH endpoint
func queryCloudflareDoH(ctx context.Context, host string) ([]string, error) {
	reqURL := fmt.Sprintf("https://cloudflare-dns.com/dns-query?name=%s&type=A", host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-json")

	// Direct client for DoH query with direct transport to avoid recursive loop
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cloudflare DoH returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res struct {
		Status int `json:"Status"`
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	var ips []string
	for _, a := range res.Answer {
		if a.Type == 1 && net.ParseIP(a.Data) != nil {
			ips = append(ips, a.Data)
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no A records returned by Cloudflare DoH")
	}
	return ips, nil
}

// queryGoogleDoH queries Google's 8.8.8.8 DoH endpoint
func queryGoogleDoH(ctx context.Context, host string) ([]string, error) {
	reqURL := fmt.Sprintf("https://dns.google/resolve?name=%s&type=A", host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google DoH returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res struct {
		Status int `json:"Status"`
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	var ips []string
	for _, a := range res.Answer {
		if a.Type == 1 && net.ParseIP(a.Data) != nil {
			ips = append(ips, a.Data)
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no A records returned by Google DoH")
	}
	return ips, nil
}

// NewResilientHTTPClient creates an HTTP client with DoH resolution fallback,
// proxy support, and modern TLS cipher suites.
func NewResilientHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return dialer.DialContext(ctx, network, addr)
			}

			// If it's already an IP address, dial directly
			if net.ParseIP(host) != nil {
				return dialer.DialContext(ctx, network, addr)
			}

			// Resolve host using DoH fallback
			ips, err := ResolveHostDoH(ctx, host)
			if err != nil || len(ips) == 0 {
				return dialer.DialContext(ctx, network, addr)
			}

			// Try dialed IPs in sequence with fast per-IP timeout
			var lastErr error
			for _, ip := range ips {
				targetAddr := net.JoinHostPort(ip, port)
				ipCtx, ipCancel := context.WithTimeout(ctx, 2500*time.Millisecond)
				conn, dErr := dialer.DialContext(ipCtx, network, targetAddr)
				ipCancel()
				if dErr == nil {
					return conn, nil
				}
				lastErr = dErr
			}
			return nil, lastErr
		},
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

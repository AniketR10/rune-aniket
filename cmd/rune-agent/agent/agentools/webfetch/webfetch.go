// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package webfetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Fetcher fetches and extracts content from a URL.
type Fetcher interface {
	Fetch(ctx context.Context, url string) (FetchResult, error)
}

// FetchResult holds the extracted content from a fetched URL.
type FetchResult struct {
	URL         string
	Title       string
	Content     string
	ContentType string
	ExtractMode string
}

// Config holds HTTPFetcher configuration.
type Config struct {
	MaxContentBytes int
	MaxRedirects    int
	Timeout         time.Duration
	CacheTTL        time.Duration
	CacheMaxEntries int
	UserAgent       string
	MaxContentChars int
	// AllowLoopback disables SSRF checks for loopback IPs.
	// Only for testing with httptest servers.
	AllowLoopback bool
}

// DefaultConfig returns the default fetcher configuration.
func DefaultConfig() Config {
	return Config{
		MaxContentBytes: 5 * 1024 * 1024,
		MaxRedirects:    10,
		Timeout:         30 * time.Second,
		CacheTTL:        15 * time.Minute,
		CacheMaxEntries: 100,
		MaxContentChars: 50000,
		UserAgent: "Mozilla/5.0 (compatible; RuneAgent/1.0;" +
			" +https://rune.build)",
	}
}

// HTTPFetcher is the concrete Fetcher backed by HTTP.
type HTTPFetcher struct {
	client *SafeClient
	cache  *Cache
	config Config
}

// NewHTTPFetcher creates a new HTTPFetcher with the given config.
func NewHTTPFetcher(cfg Config) *HTTPFetcher {
	return &HTTPFetcher{
		client: NewSafeClient(cfg),
		cache:  NewCache(cfg.CacheMaxEntries, cfg.CacheTTL),
		config: cfg,
	}
}

// Fetch retrieves the URL, extracts content, and caches results.
func (f *HTTPFetcher) Fetch(
	ctx context.Context, rawURL string,
) (FetchResult, error) {
	if cached, ok := f.cache.Get(rawURL); ok {
		return cached, nil
	}

	resp, err := f.client.Do(ctx, rawURL)
	if err != nil {
		return FetchResult{}, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return FetchResult{}, fmt.Errorf(
			"HTTP %d %s", resp.StatusCode, resp.Status,
		)
	}

	body, err := io.ReadAll(
		io.LimitReader(resp.Body, int64(f.config.MaxContentBytes)),
	)
	if err != nil {
		return FetchResult{}, fmt.Errorf("read body: %w", err)
	}

	ct := resp.Header.Get("Content-Type")
	title, content, mode := Extract(
		body, ct, resp.Request.URL.String(),
		f.config.MaxContentChars,
	)

	result := FetchResult{
		URL:         finalURL(resp),
		Title:       title,
		Content:     content,
		ContentType: ct,
		ExtractMode: mode,
	}
	f.cache.Set(rawURL, result)
	return result, nil
}

func finalURL(resp *http.Response) string {
	if resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL.String()
	}
	return ""
}

// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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

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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPFetcher(t *testing.T) {
	t.Run("fetch plain text", func(t *testing.T) {
		srv := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = fmt.Fprint(w, "Hello, world!")
			}),
		)
		defer srv.Close()

		cfg := testConfig()
		fetcher := NewHTTPFetcher(cfg)
		result, err := fetcher.Fetch(
			context.Background(), srv.URL,
		)

		assert.NoError(t, err)
		assert.Equal(t, "Hello, world!", result.Content)
		assert.Equal(t, "plaintext", result.ExtractMode)
	})

	t.Run("fetch HTML", func(t *testing.T) {
		srv := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				_, _ = fmt.Fprint(w, "<html><body><p>Content</p></body></html>")
			}),
		)
		defer srv.Close()

		cfg := testConfig()
		fetcher := NewHTTPFetcher(cfg)
		result, err := fetcher.Fetch(
			context.Background(), srv.URL,
		)

		assert.NoError(t, err)
		assert.Contains(t, result.Content, "Content")
		assert.Equal(t, "readability", result.ExtractMode)
	})

	t.Run("fetch JSON", func(t *testing.T) {
		srv := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(
					"Content-Type", "application/json",
				)
				_, _ = fmt.Fprint(w, `{"key":"value"}`)
			}),
		)
		defer srv.Close()

		cfg := testConfig()
		fetcher := NewHTTPFetcher(cfg)
		result, err := fetcher.Fetch(
			context.Background(), srv.URL,
		)

		assert.NoError(t, err)
		assert.Equal(t, "json", result.ExtractMode)
		assert.Contains(t, result.Content, `"key": "value"`)
	})

	t.Run("cache hit", func(t *testing.T) {
		calls := 0
		srv := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Content-Type", "text/plain")
				_, _ = fmt.Fprint(w, "cached data")
			}),
		)
		defer srv.Close()

		cfg := testConfig()
		fetcher := NewHTTPFetcher(cfg)
		ctx := context.Background()

		_, err := fetcher.Fetch(ctx, srv.URL)
		assert.NoError(t, err)

		_, err = fetcher.Fetch(ctx, srv.URL)
		assert.NoError(t, err)

		assert.Equal(t, 1, calls, "second call should hit cache")
	})

	t.Run("HTTP error status", func(t *testing.T) {
		srv := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}),
		)
		defer srv.Close()

		cfg := testConfig()
		fetcher := NewHTTPFetcher(cfg)
		_, err := fetcher.Fetch(
			context.Background(), srv.URL,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("redirect followed", func(t *testing.T) {
		srv := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/redirect" {
					http.Redirect(
						w, r, "/dest", http.StatusFound,
					)
					return
				}
				w.Header().Set("Content-Type", "text/plain")
				_, _ = fmt.Fprint(w, "destination")
			}),
		)
		defer srv.Close()

		cfg := testConfig()
		fetcher := NewHTTPFetcher(cfg)
		result, err := fetcher.Fetch(
			context.Background(), srv.URL+"/redirect",
		)

		assert.NoError(t, err)
		assert.Equal(t, "destination", result.Content)
	})
}

// Verify interface compliance.
var _ Fetcher = &HTTPFetcher{}

func testConfig() Config {
	cfg := DefaultConfig()
	cfg.AllowLoopback = true
	return cfg
}

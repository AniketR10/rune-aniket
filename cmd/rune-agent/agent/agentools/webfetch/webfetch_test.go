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

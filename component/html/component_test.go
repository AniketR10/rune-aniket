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

package html

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/component/markdown"
)

func waitFor(t *testing.T, fn func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return u
}

func TestNew(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "<h1>Hello</h1><p>World</p>")
	}))
	defer srv.Close()

	interrupter := term.NopInterrupter()
	c := New(interrupter, mustParseURL(t, srv.URL))
	defer func() { _ = c.Close() }()

	require.NotNil(t, c)
	assert.Equal(t, StateLoading, c.State())
}

func TestLoadContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "<h1>Hello</h1><p>World</p>")
	}))
	defer srv.Close()

	interrupter := term.NopInterrupter()
	c := New(interrupter, mustParseURL(t, srv.URL))
	defer func() { _ = c.Close() }()

	c.Resize(40, 10)

	waitFor(t, func() bool {
		return c.State() == StateLoaded
	}, 5*time.Second)

	assert.Equal(t, StateLoaded, c.State())
	assert.NotNil(t, c.Resolved())
}

func TestCancel(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()

	interrupter := term.NopInterrupter()
	c := New(interrupter, mustParseURL(t, srv.URL))
	defer func() { _ = c.Close() }()

	c.Resize(40, 10)

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("server handler not called")
	}

	c.Cancel()

	waitFor(t, func() bool {
		return c.State() == StateCanceled
	}, 5*time.Second)

	assert.Equal(t, StateCanceled, c.State())
	assert.Nil(t, c.Resolved())
}

func TestDrawDoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "<h1>Hello</h1>")
	}))
	defer srv.Close()

	interrupter := term.NopInterrupter()
	c := New(interrupter, mustParseURL(t, srv.URL))
	defer func() { _ = c.Close() }()

	c.Resize(40, 10)

	// Draw during loading.
	w := term.NewStringWriter(40, 10)
	require.NotPanics(t, func() { c.Draw(w) })

	waitFor(t, func() bool {
		return c.State() == StateLoaded
	}, 5*time.Second)

	// Draw after loading.
	w = term.NewStringWriter(40, 10)
	require.NotPanics(t, func() { c.Draw(w) })
}

func TestWithOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "<h1>Hello</h1>")
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	cfg := markdown.DefaultConfig()
	cfg.HeaderPrefix = false

	interrupter := term.NopInterrupter()
	c := New(interrupter, mustParseURL(t, srv.URL),
		WithHTTPClient(client),
		WithMarkdownConfig(cfg),
	)
	defer func() { _ = c.Close() }()

	c.Resize(40, 10)

	waitFor(t, func() bool {
		return c.State() == StateLoaded
	}, 5*time.Second)

	assert.NotNil(t, c.Resolved())
}

func TestFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	interrupter := term.NopInterrupter()
	c := New(interrupter, mustParseURL(t, srv.URL))
	defer func() { _ = c.Close() }()

	c.Resize(40, 10)

	time.Sleep(200 * time.Millisecond)

	assert.NotEqual(t, StateLoaded, c.State())
	assert.Nil(t, c.Resolved())

	w := term.NewStringWriter(40, 10)
	require.NotPanics(t, func() { c.Draw(w) })
}

func TestDimensions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "<h1>Hello</h1>")
	}))
	defer srv.Close()

	interrupter := term.NopInterrupter()
	c := New(interrupter, mustParseURL(t, srv.URL))
	defer func() { _ = c.Close() }()

	// Before resize, placeholder dimensions.
	w, h := c.Dimensions()
	assert.Equal(t, 2, w)
	assert.Equal(t, 1, h)
}

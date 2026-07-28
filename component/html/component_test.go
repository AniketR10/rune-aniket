// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
	"unstable.build/go-tui/component/markdown"
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

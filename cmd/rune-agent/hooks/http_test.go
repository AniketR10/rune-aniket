// Copyright (C) 2017-2026 The Rune Authors
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

package hooks

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunHTTP_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(`{"decision":"block","reason":"because"}`))
	}))
	defer srv.Close()
	r := &Runner{}
	res := r.runHTTP(context.Background(), Hook{
		Type: HookTypeHTTP, URL: srv.URL, Timeout: 2 * time.Second,
	}, []byte(`{}`))
	if res.warn != nil {
		t.Fatalf("unexpected warn: %v", res.warn)
	}
	if res.output.Decision != "block" || res.output.Reason != "because" {
		t.Fatalf("output = %+v", res.output)
	}
}

func TestRunHTTP_Non2xxWarns(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	r := &Runner{}
	res := r.runHTTP(context.Background(), Hook{
		Type: HookTypeHTTP, URL: srv.URL, Timeout: 2 * time.Second,
	}, []byte(`{}`))
	if res.warn == nil || !strings.Contains(res.warn.Error(), "status 500") {
		t.Fatalf("expected non-2xx warn, got %v", res.warn)
	}
	if res.output.Decision == "block" {
		t.Fatalf("non-2xx must not block")
	}
}

func TestRunHTTP_AllowedEnvVarHeader(t *testing.T) {
	t.Setenv("HOOK_TOKEN", "secret-xyz")
	t.Setenv("HOOK_OTHER", "should-not-leak")
	var gotAuth, gotOther string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotOther = r.Header.Get("X-Other")
	}))
	defer srv.Close()
	r := &Runner{}
	res := r.runHTTP(context.Background(), Hook{
		Type:    HookTypeHTTP,
		URL:     srv.URL,
		Headers: map[string]string{"Authorization": "Bearer ${HOOK_TOKEN}", "X-Other": "${HOOK_OTHER}"},
		// Only HOOK_TOKEN is allow-listed; HOOK_OTHER must remain literal.
		AllowedEnvVars: []string{"HOOK_TOKEN"},
		Timeout:        2 * time.Second,
	}, []byte(`{}`))
	if res.warn != nil {
		t.Fatalf("unexpected warn: %v", res.warn)
	}
	if gotAuth != "Bearer secret-xyz" {
		t.Fatalf("Authorization = %q, want Bearer secret-xyz", gotAuth)
	}
	if gotOther != "${HOOK_OTHER}" {
		t.Fatalf("X-Other = %q, want literal ${HOOK_OTHER}", gotOther)
	}
}

func TestRunHTTP_Timeout(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()
	r := &Runner{}
	res := r.runHTTP(context.Background(), Hook{
		Type: HookTypeHTTP, URL: srv.URL, Timeout: 50 * time.Millisecond,
	}, []byte(`{}`))
	if res.warn == nil {
		t.Fatalf("expected timeout warn")
	}
}

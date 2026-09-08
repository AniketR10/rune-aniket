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

package ollama_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/rune/internal/llm/ollama"
)

func collectModels(t *testing.T, it iterator.Iterator[llmapi.ModelEntry]) []llmapi.ModelEntry {
	t.Helper()
	entries, err := iterator.ToSlice(context.Background(), it)
	if err != nil {
		t.Fatalf("collecting models: %v", err)
	}
	return entries
}

func TestRegistry_Models(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3:latest"},{"name":"codellama:7b"}]}`))
	}))
	defer srv.Close()

	reg := ollama.NewRegistry(srv.URL)
	models := collectModels(t, reg.Models())
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	e, ok := reg.Get(context.Background(), "llama3:latest")
	if !ok {
		t.Fatal("expected to find llama3:latest")
	}
	if e.Provider != ollama.LLMProvider {
		t.Fatalf("expected ollama provider, got %s", e.Provider)
	}
	if e.BaseURL != srv.URL+"/v1/" {
		t.Fatalf("unexpected base URL: %s", e.BaseURL)
	}
}

func TestRegistry_fetches_every_call(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"m1"}]}`))
	}))
	defer srv.Close()

	reg := ollama.NewRegistry(srv.URL)
	collectModels(t, reg.Models())
	collectModels(t, reg.Models())
	collectModels(t, reg.Models())
	if calls != 3 {
		t.Fatalf("expected 3 HTTP calls (no cache), got %d", calls)
	}
}

func TestRegistry_unreachable_returns_empty(t *testing.T) {
	reg := ollama.NewRegistry("http://127.0.0.1:1") // port 1 won't respond
	models := collectModels(t, reg.Models())
	if len(models) != 0 {
		t.Fatalf("expected 0 models for unreachable server, got %d", len(models))
	}
}

func TestRegistry_Close_cancels_hanging_fetch(t *testing.T) {
	requestReceived := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestReceived)
		// Block until the request context is cancelled.
		<-r.Context().Done()
	}))
	defer srv.Close()

	reg := ollama.NewRegistry(srv.URL)
	it := reg.Models()

	// Wait for the HTTP request to arrive at the server.
	select {
	case <-requestReceived:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for HTTP request")
	}

	// Close the iterator — should cancel the in-flight fetch.
	if err := it.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Next should return immediately with no entries (fetch was cancelled).
	_, ok := it.Next(context.Background())
	if ok {
		t.Fatal("expected no entries after Close")
	}
}

func TestRegistry_Get_not_found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"m1"}]}`))
	}))
	defer srv.Close()

	reg := ollama.NewRegistry(srv.URL)
	_, ok := reg.Get(context.Background(), "nonexistent")
	if ok {
		t.Fatal("expected Get to return false for nonexistent model")
	}
}

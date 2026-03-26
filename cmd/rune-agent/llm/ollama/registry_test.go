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

package ollama_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"unstable.build/go-tui/cmd/rune-agent/llm/llmregistry"
	"unstable.build/go-tui/cmd/rune-agent/llm/ollama"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

func collectModels(t *testing.T, it iterator.Iterator[llmregistry.ModelEntry]) []llmregistry.ModelEntry {
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

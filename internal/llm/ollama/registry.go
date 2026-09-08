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

package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/rune/internal/debug"
)

// LLMProvider identifies the Ollama provider in the model registry.
const LLMProvider = "ollama"

const (
	// DefaultBaseURL is the default local Ollama API base URL.
	DefaultBaseURL       = "http://localhost:11434"
	defaultContextWindow = 128000
	openAICompatiblePath = "/v1/"
)

// tagsResponse is the JSON shape returned by GET /api/tags.
type tagsResponse struct {
	Models []modelInfo `json:"models"`
}

type modelInfo struct {
	Name string `json:"name"`
}

// Registry is a dynamic registry that queries an Ollama instance
// for available models. Each call to Models or Get fetches fresh
// data from the Ollama API.
type Registry struct {
	baseURL    string
	httpClient *http.Client
}

// NewRegistry creates an Ollama model registry.
func NewRegistry(baseURL string) *Registry {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Registry{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 500 * time.Millisecond},
	}
}

// Models returns an iterator that asynchronously fetches available
// models from the Ollama API. If the server is unreachable, the
// iterator yields zero entries. Closing the iterator cancels the
// in-flight HTTP request if the fetch has not completed yet.
func (r *Registry) Models() iterator.Iterator[llmapi.ModelEntry] {
	type result struct {
		entries []llmapi.ModelEntry
		err     error
	}
	fetchCtx, cancelFetch := context.WithCancel(context.Background())
	ch := make(chan result, 1)
	go debug.CapturePanicReport(func() {

		entries, err := r.fetchCtx(fetchCtx)
		ch <- result{entries, err}

	})

	var entries []llmapi.ModelEntry
	fetched := false
	idx := 0

	return iterator.FromFunc(
		func(ctx context.Context) (llmapi.ModelEntry, bool, error) {
			if !fetched {
				select {
				case <-ctx.Done():
					return llmapi.ModelEntry{}, false, ctx.Err()
				case res := <-ch:
					fetched = true
					if res.err != nil {
						return llmapi.ModelEntry{}, false, nil
					}
					entries = res.entries
				}
			}
			if idx >= len(entries) {
				return llmapi.ModelEntry{}, false, nil
			}
			e := entries[idx]
			idx++
			return e, true, nil
		},
		func() error {
			cancelFetch()
			return nil
		},
	)
}

// Get returns the entry for the given model name.
func (r *Registry) Get(ctx context.Context, model string) (llmapi.ModelEntry, bool) {
	entries, err := r.fetchCtx(ctx)
	if err != nil {
		return llmapi.ModelEntry{}, false
	}
	for _, e := range entries {
		if e.Name == model {
			return e, true
		}
	}
	return llmapi.ModelEntry{}, false
}

func (r *Registry) fetchCtx(ctx context.Context) ([]llmapi.ModelEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("ollama tags: %w", err)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama tags: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama tags: status %d", resp.StatusCode)
	}

	var tags tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("ollama tags decode: %w", err)
	}

	entries := make([]llmapi.ModelEntry, len(tags.Models))
	for i, m := range tags.Models {
		entries[i] = llmapi.ModelEntry{
			Name:          m.Name,
			Provider:      LLMProvider,
			ContextWindow: defaultContextWindow,
			BaseURL:       r.baseURL + openAICompatiblePath,
		}
	}
	return entries, nil
}

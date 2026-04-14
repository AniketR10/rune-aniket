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

package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmregistry"
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
func (r *Registry) Models() iterator.Iterator[llmregistry.ModelEntry] {
	type result struct {
		entries []llmregistry.ModelEntry
		err     error
	}
	fetchCtx, cancelFetch := context.WithCancel(context.Background())
	ch := make(chan result, 1)
	go func() {
		entries, err := r.fetchCtx(fetchCtx)
		ch <- result{entries, err}
	}()

	var entries []llmregistry.ModelEntry
	fetched := false
	idx := 0

	return iterator.FromFunc(
		func(ctx context.Context) (llmregistry.ModelEntry, bool, error) {
			if !fetched {
				select {
				case <-ctx.Done():
					return llmregistry.ModelEntry{}, false, ctx.Err()
				case res := <-ch:
					fetched = true
					if res.err != nil {
						return llmregistry.ModelEntry{}, false, nil
					}
					entries = res.entries
				}
			}
			if idx >= len(entries) {
				return llmregistry.ModelEntry{}, false, nil
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
func (r *Registry) Get(ctx context.Context, model string) (llmregistry.ModelEntry, bool) {
	entries, err := r.fetchCtx(ctx)
	if err != nil {
		return llmregistry.ModelEntry{}, false
	}
	for _, e := range entries {
		if e.Name == model {
			return e, true
		}
	}
	return llmregistry.ModelEntry{}, false
}

func (r *Registry) fetchCtx(ctx context.Context) ([]llmregistry.ModelEntry, error) {
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

	entries := make([]llmregistry.ModelEntry, len(tags.Models))
	for i, m := range tags.Models {
		entries[i] = llmregistry.ModelEntry{
			Name:          m.Name,
			Provider:      LLMProvider,
			ContextWindow: defaultContextWindow,
			BaseURL:       r.baseURL + openAICompatiblePath,
		}
	}
	return entries, nil
}

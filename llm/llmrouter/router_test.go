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

package llmrouter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"unstable.build/go-tui/llm"
)

// newTestRouter constructs a router with sensible defaults rooted in
// a fresh temp directory.
func newTestRouter(t *testing.T) *Router {
	t.Helper()
	r, err := New(llm.DefaultConfig(), t.TempDir(), storagestub.NewInMemoryService())
	require.NoError(t, err)
	return r
}

// TestNew_ConstructsLocalRegistry verifies the router exposes the
// llama.cpp registry it owns.
func TestNew_ConstructsLocalRegistry(t *testing.T) {
	r := newTestRouter(t)
	require.NotNil(t, r.LocalRegistry())
}

// TestRouter_UnknownProvider rejects models with an unknown provider.
func TestRouter_UnknownProvider(t *testing.T) {
	r := newTestRouter(t)
	_, err := r.CreateCompletion(context.Background(),
		llmapi.ModelEntry{Name: "foo", Provider: "unknown"}, llmapi.Request{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no provider registered for "unknown"`)

	_, err = r.CountTokens(llmapi.ModelEntry{Provider: "unknown"}, nil)
	require.Error(t, err)
}

// TestRouter_CustomDisabledWithoutURL returns an error when the
// custom provider is referenced but not configured.
func TestRouter_CustomDisabledWithoutURL(t *testing.T) {
	r := newTestRouter(t)
	_, err := r.CreateCompletion(context.Background(),
		llmapi.ModelEntry{Name: "x", Provider: ProviderCustom}, llmapi.Request{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "custom provider not configured")
}

// TestRouter_CustomCatalogSurfacesEntries lists user-configured custom
// models in Models() and routes them via the custom client.
func TestRouter_CustomCatalogSurfacesEntries(t *testing.T) {
	cfg := llm.DefaultConfig()
	cfg.Custom = llm.CustomConfig{
		URL:    "https://example.test/v1",
		APIKey: "k",
		AvailableModels: map[string]int{
			"my-model": 8192,
		},
	}
	r, err := New(cfg, t.TempDir(), storagestub.NewInMemoryService())
	require.NoError(t, err)

	got, ok := r.GetModel(context.Background(),
		llmapi.ModelEntry{Provider: ProviderCustom, Name: "my-model"})
	require.True(t, ok)
	assert.Equal(t, 8192, got.ContextWindow)
	assert.Equal(t, ProviderCustom, got.Provider)
}

// TestRouter_StaticCatalog includes the OpenAI/Anthropic/Codex/Gemini
// model lists.
func TestRouter_StaticCatalog(t *testing.T) {
	r := newTestRouter(t)
	names := map[string]string{}
	ctx := context.Background()
	it := r.Models()
	for {
		entry, ok := it.Next(ctx)
		if !ok {
			break
		}
		names[entry.Name] = entry.Provider
	}
	// At least one entry per static provider should be present.
	hasProvider := func(p string) bool {
		for _, v := range names {
			if v == p {
				return true
			}
		}
		return false
	}
	assert.True(t, hasProvider(ProviderOpenAI), "no openai models")
	assert.True(t, hasProvider(ProviderAnthropic), "no anthropic models")
	assert.True(t, hasProvider(ProviderCodex), "no codex models")
	assert.True(t, hasProvider(ProviderGemini), "no gemini models")
}

func TestRouter_GetModel_RequiresProvider(t *testing.T) {
	r := newTestRouter(t)
	// gpt-5.5 collides across openai and codex catalogs.
	_, ok := r.GetModel(context.Background(), llmapi.ModelEntry{Name: "gpt-5.5"})
	assert.False(t, ok, "GetModel must reject empty Provider")
}

func TestRouter_GetModel_QualifiedDisambiguates(t *testing.T) {
	r := newTestRouter(t)
	ctx := context.Background()

	got, ok := r.GetModel(ctx, llmapi.ModelEntry{Provider: ProviderCodex, Name: "gpt-5.5"})
	require.True(t, ok)
	assert.Equal(t, ProviderCodex, got.Provider)

	got, ok = r.GetModel(ctx, llmapi.ModelEntry{Provider: ProviderOpenAI, Name: "gpt-5.5"})
	require.True(t, ok)
	assert.Equal(t, ProviderOpenAI, got.Provider)
}

func TestRouter_Resolve_RejectsEmptyProvider(t *testing.T) {
	r := newTestRouter(t)
	_, err := r.resolve(context.Background(), llmapi.ModelEntry{Name: "gpt-5.5"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ModelEntry.Provider must be set")
}

func TestRouter_CreateCompletion_RejectsEmptyProvider(t *testing.T) {
	r := newTestRouter(t)
	_, err := r.CreateCompletion(context.Background(),
		llmapi.ModelEntry{Name: "gpt-5.5"}, llmapi.Request{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ModelEntry.Provider must be set")

	_, err = r.CountTokens(llmapi.ModelEntry{Name: "gpt-5.5"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ModelEntry.Provider must be set")
}

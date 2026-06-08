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
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

func me(provider, name string) llmapi.ModelEntry {
	return llmapi.ModelEntry{Provider: provider, Name: name}
}

func TestAliasStore_SetGet(t *testing.T) {
	ctx := context.Background()
	s := newAliasStore(storagestub.NewInMemoryService())

	require.NoError(t, s.set(ctx, "default", me("openai", "gpt-5.5")))
	target, ok, err := s.get(ctx, "default")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, me("openai", "gpt-5.5"), target)

	_, ok, err = s.get(ctx, "missing")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestAliasStore_SetOverwrites(t *testing.T) {
	ctx := context.Background()
	s := newAliasStore(storagestub.NewInMemoryService())

	require.NoError(t, s.set(ctx, "default", me("openai", "gpt-5.5")))
	require.NoError(t, s.set(ctx, "default", me("anthropic", "claude-opus-4-8")))
	target, ok, err := s.get(ctx, "default")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, me("anthropic", "claude-opus-4-8"), target)
}

func TestAliasStore_Remove(t *testing.T) {
	ctx := context.Background()
	s := newAliasStore(storagestub.NewInMemoryService())
	require.NoError(t, s.set(ctx, "default", me("openai", "gpt-5.5")))

	require.NoError(t, s.remove(ctx, "default"))
	_, ok, err := s.get(ctx, "default")
	require.NoError(t, err)
	assert.False(t, ok)

	err = s.remove(ctx, "default")
	assert.ErrorIs(t, err, ErrAliasNotFound)
}

func TestAliasStore_All(t *testing.T) {
	ctx := context.Background()
	s := newAliasStore(storagestub.NewInMemoryService())
	require.NoError(t, s.set(ctx, "default", me("openai", "gpt-5.5")))
	require.NoError(t, s.set(ctx, "query", me("gemini", "gemini-3.1-pro-preview")))

	all, err := s.all(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]llmapi.ModelEntry{
		"default": me("openai", "gpt-5.5"),
		"query":   me("gemini", "gemini-3.1-pro-preview"),
	}, all)
}

func TestAliasStore_EmptyNameOrTarget(t *testing.T) {
	ctx := context.Background()
	s := newAliasStore(storagestub.NewInMemoryService())
	assert.Error(t, s.set(ctx, "", me("openai", "gpt-5.5")))
	assert.Error(t, s.set(ctx, "default", llmapi.ModelEntry{}))
}

func TestAliasStore_NilStoragePanics(t *testing.T) {
	assert.PanicsWithValue(t,
		"llmrouter: newAliasStore: storage must not be nil",
		func() { newAliasStore(nil) })
}

func TestAliasStore_ConcurrentSetsNoLostWrites(t *testing.T) {
	ctx := context.Background()
	s := newAliasStore(storagestub.NewInMemoryService())

	const n = 16
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			assert.NoError(t, s.set(ctx, fmt.Sprintf("a%02d", i), me("openai", fmt.Sprintf("m%02d", i))))
		}(i)
	}
	wg.Wait()

	all, err := s.all(ctx)
	require.NoError(t, err)
	assert.Len(t, all, n)
}

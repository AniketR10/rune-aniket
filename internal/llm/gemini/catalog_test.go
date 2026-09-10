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

package gemini

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// stubIterator yields entries then ends with an optional error.
type stubIterator struct {
	entries []llmapi.ModelEntry
	i       int
	err     error
}

func (s *stubIterator) Next(context.Context) (llmapi.ModelEntry, bool) {
	if s.i >= len(s.entries) {
		return llmapi.ModelEntry{}, false
	}
	e := s.entries[s.i]
	s.i++
	return e, true
}
func (s *stubIterator) Err() error   { return s.err }
func (s *stubIterator) Close() error { return nil }

func drainCatalog(t *testing.T, it iterator.Iterator[llmapi.ModelEntry]) []llmapi.ModelEntry {
	t.Helper()
	var out []llmapi.ModelEntry
	for {
		e, ok := it.Next(context.Background())
		if !ok {
			break
		}
		out = append(out, e)
	}
	return out
}

func TestFallbackIterator(t *testing.T) {
	fallback := []llmapi.ModelEntry{{Name: "static-a"}, {Name: "static-b"}}
	liveErr := errors.New("permission denied")
	live := []llmapi.ModelEntry{{Name: "live-a"}}

	t.Run("live success serves live, no error", func(t *testing.T) {
		it := withStaticFallback(&stubIterator{entries: live}, fallback)
		got := drainCatalog(t, it)
		assert.Equal(t, live, got)
		require.NoError(t, it.Err())
	})

	t.Run("error before any entry serves fallback and surfaces error", func(t *testing.T) {
		it := withStaticFallback(&stubIterator{err: liveErr}, fallback)
		got := drainCatalog(t, it)
		assert.Equal(t, fallback, got)
		require.ErrorIs(t, it.Err(), liveErr)
	})

	t.Run("successful empty list stays empty", func(t *testing.T) {
		it := withStaticFallback(&stubIterator{}, fallback)
		got := drainCatalog(t, it)
		assert.Empty(t, got)
		require.NoError(t, it.Err())
	})

	t.Run("error after partial yield keeps partial and surfaces error", func(t *testing.T) {
		it := withStaticFallback(&stubIterator{entries: live, err: liveErr}, fallback)
		got := drainCatalog(t, it)
		assert.Equal(t, live, got, "must not append the fallback to partial live results")
		require.ErrorIs(t, it.Err(), liveErr)
	})
}

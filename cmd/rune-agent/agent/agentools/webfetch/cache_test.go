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

package webfetch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	t.Run("set and get", func(t *testing.T) {
		c := NewCache(10, time.Minute)
		r := FetchResult{URL: "https://example.com", Content: "hello"}
		c.Set("key1", r)

		got, ok := c.Get("key1")
		assert.True(t, ok)
		assert.Equal(t, r, got)
	})

	t.Run("miss on unknown key", func(t *testing.T) {
		c := NewCache(10, time.Minute)
		_, ok := c.Get("missing")
		assert.False(t, ok)
	})

	t.Run("TTL expiry", func(t *testing.T) {
		now := time.Now()
		c := NewCache(10, time.Minute)
		c.nowFunc = func() time.Time { return now }

		r := FetchResult{URL: "https://example.com", Content: "data"}
		c.Set("k", r)

		// Still valid.
		got, ok := c.Get("k")
		assert.True(t, ok)
		assert.Equal(t, r, got)

		// Advance past TTL.
		c.nowFunc = func() time.Time {
			return now.Add(2 * time.Minute)
		}
		_, ok = c.Get("k")
		assert.False(t, ok)
	})

	t.Run("LRU eviction", func(t *testing.T) {
		c := NewCache(2, time.Hour)
		c.Set("a", FetchResult{Content: "A"})
		c.Set("b", FetchResult{Content: "B"})

		// Access "a" to make it most recent.
		_, _ = c.Get("a")

		// Adding "c" should evict "b" (least recently used).
		c.Set("c", FetchResult{Content: "C"})

		_, ok := c.Get("b")
		assert.False(t, ok, "b should be evicted")

		got, ok := c.Get("a")
		assert.True(t, ok)
		assert.Equal(t, "A", got.Content)

		got, ok = c.Get("c")
		assert.True(t, ok)
		assert.Equal(t, "C", got.Content)
	})

	t.Run("update existing key", func(t *testing.T) {
		c := NewCache(10, time.Minute)
		c.Set("k", FetchResult{Content: "old"})
		c.Set("k", FetchResult{Content: "new"})

		got, ok := c.Get("k")
		assert.True(t, ok)
		assert.Equal(t, "new", got.Content)
	})
}

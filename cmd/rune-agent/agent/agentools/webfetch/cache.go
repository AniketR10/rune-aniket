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
	"container/list"
	"sync"
	"time"
)

// Cache is an in-memory LRU cache with TTL-based expiry.
type Cache struct {
	mu      sync.Mutex
	maxSize int
	ttl     time.Duration
	items   map[string]*list.Element
	order   *list.List
	nowFunc func() time.Time
}

// NewCache creates a new LRU cache.
func NewCache(maxSize int, ttl time.Duration) *Cache {
	return &Cache{
		maxSize: maxSize,
		ttl:     ttl,
		items:   make(map[string]*list.Element, maxSize),
		order:   list.New(),
		nowFunc: time.Now,
	}
}

// Get retrieves a cached result. Returns false if not found
// or expired.
func (c *Cache) Get(key string) (FetchResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return FetchResult{}, false
	}

	entry := el.Value.(*cacheEntry)
	if c.nowFunc().After(entry.expiresAt) {
		c.removeLocked(el, key)
		return FetchResult{}, false
	}

	c.order.MoveToFront(el)
	return entry.result, true
}

// Set stores a result in the cache, evicting the oldest
// entry if at capacity.
func (c *Cache) Set(key string, result FetchResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		entry := el.Value.(*cacheEntry)
		entry.result = result
		entry.expiresAt = c.nowFunc().Add(c.ttl)
		return
	}

	if c.order.Len() >= c.maxSize {
		c.evictLocked()
	}

	entry := &cacheEntry{
		key:       key,
		result:    result,
		expiresAt: c.nowFunc().Add(c.ttl),
	}
	el := c.order.PushFront(entry)
	c.items[key] = el
}

func (c *Cache) removeLocked(
	el *list.Element, key string,
) {
	c.order.Remove(el)
	delete(c.items, key)
}

func (c *Cache) evictLocked() {
	el := c.order.Back()
	if el == nil {
		return
	}
	entry := el.Value.(*cacheEntry)
	c.removeLocked(el, entry.key)
}

type cacheEntry struct {
	key       string
	result    FetchResult
	expiresAt time.Time
}

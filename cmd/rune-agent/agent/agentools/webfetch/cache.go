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

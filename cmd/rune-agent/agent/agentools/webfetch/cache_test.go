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

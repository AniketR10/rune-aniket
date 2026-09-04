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

package storagecache

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

func TestCacheService(t *testing.T) {
	t.Run("creates hit the underlying service", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		cache := New[testStruct](svc, storagestub.NewInMemoryService())
		ctx := context.Background()

		id := "myId"
		err := cache.Create(ctx, id, &testStruct{Id: id, Content: "1234"})
		require.NoError(t, err)

		var temp testStruct
		err = svc.Get(ctx, "myId", &temp)
		require.NoError(t, err)
		assert.Equal(t, "1234", temp.Content)
	})

	t.Run("get do not hit the underlying service, if already cached", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		cache := New[testStruct](svc, storagestub.NewInMemoryService())
		ctx := context.Background()

		id := "myId"
		err := cache.Create(ctx, id, &testStruct{Id: id, Content: "1234"})
		require.NoError(t, err)

		var temp testStruct
		it, err := cache.List(ctx, nil) // force caching
		require.NoError(t, err)
		require.NoError(t, it.Close())

		// override
		err = svc.Set(ctx, id, &testStruct{Id: id, Content: "AAAA"})
		require.NoError(t, err)

		temp = testStruct{}
		err = cache.Get(ctx, id, &temp)
		require.NoError(t, err)
		assert.Equal(t, "1234", temp.Content)
	})

	t.Run("Set forces next get to hit underlying service", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		cache := New[testStruct](svc, storagestub.NewInMemoryService())
		ctx := context.Background()

		id := "myId"
		err := cache.Create(ctx, id, &testStruct{Id: id, Content: "1234"})
		require.NoError(t, err)

		err = cache.Set(ctx, id, &testStruct{Id: id, Content: "ZZZZ"})
		require.NoError(t, err)

		err = svc.Set(ctx, id, &testStruct{Id: id, Content: "AAAA"})
		require.NoError(t, err)

		var temp testStruct
		err = cache.Get(ctx, "myId", &temp)
		require.NoError(t, err)
		assert.Equal(t, "AAAA", temp.Content)
	})

	t.Run("EvictAll forces next get to hit underlying service", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		cache := New[testStruct](svc, storagestub.NewInMemoryService())
		ctx := context.Background()

		id := "myId"
		err := cache.Create(ctx, id, &testStruct{Id: id, Content: "1234"})
		require.NoError(t, err)

		err = svc.Set(ctx, id, &testStruct{Id: id, Content: "AAAA"})
		require.NoError(t, err)

		cache.EvictAll(ctx)

		var temp testStruct
		err = cache.Get(ctx, "myId", &temp)
		require.NoError(t, err)
		assert.Equal(t, "AAAA", temp.Content)
	})

	t.Run("first List returns all documents from the underlying service", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		cache := New[testStruct](svc, storagestub.NewInMemoryService())
		ctx := context.Background()
		n := prepareServiceForListTest(t, cache, "first")

		it, err := cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n)
	})

	t.Run("second call to list List returns all documents cached from the previous list", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		cache := New[testStruct](svc, storagestub.NewInMemoryService())
		ctx := context.Background()
		n := prepareServiceForListTest(t, cache, "first")

		it, err := cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n)

		err = svc.Set(ctx, "otherID", &testStruct{Id: "otherID", Content: "AAAA"})
		require.NoError(t, err)

		it, err = cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n)
	})

	t.Run("writes should NOT force next List to call the underlying service", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		cache := New[testStruct](svc, storagestub.NewInMemoryService())
		ctx := context.Background()
		n := prepareServiceForListTest(t, cache, "first")

		it, err := cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n)

		err = cache.Set(ctx, "otherID", &testStruct{Id: "otherID", Content: "AAAA"})
		require.NoError(t, err)

		// set a diff value on service, so we can see below if cached is used
		err = svc.Set(ctx, "otherID", &testStruct{Id: "otherID", Content: "UIUIUI"})
		require.NoError(t, err)

		it, err = cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n+1)

		var actualOut testStruct
		err = cache.Get(ctx, "otherID", &actualOut)
		require.NoError(t, err)

		assert.Equal(t, "otherID", actualOut.Id)
		assert.Equal(t, "AAAA", actualOut.Content)
	})

	t.Run("EvictAll should force next List to call the underlying service", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		cache := New[testStruct](svc, storagestub.NewInMemoryService())
		ctx := context.Background()
		n := prepareServiceForListTest(t, cache, "first")

		it, err := cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n)

		err = svc.Set(ctx, "otherID", &testStruct{Id: "otherID", Content: "AAAA"})
		require.NoError(t, err)

		cache.EvictAll(ctx)

		it, err = cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n+1)
	})

	t.Run("List with filters is not cached", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		cache := New[testStruct](svc, storagestub.NewInMemoryService())
		ctx := context.Background()
		n := prepareServiceForListTest(t, cache, "first")

		it, err := cache.List(ctx, []storageapi.Filter{
			{Field: storageapi.Field{FieldPath: []string{"OtherField"}}, Op: storageapi.OpEqual},
		})
		require.NoError(t, err)
		assertListResults(t, it, n)

		err = svc.Set(ctx, "otherID", &testStruct{Id: "otherID", Content: "AAAA"})
		require.NoError(t, err)

		it, err = cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n+1)
	})
}

type testStruct struct {
	Id        string
	Content   string
	UpdatedAt time.Time
	UpdatedBy string
}

func (t testStruct) ID() string {
	return t.Id
}

func (t testStruct) WithID(id string) testStruct {
	t.Id = id
	return t
}

func (t testStruct) UpdatedTime() time.Time {
	return t.UpdatedAt
}

func (t testStruct) WithUpdatedTime(tt time.Time) testStruct {
	t.UpdatedAt = tt
	return t
}

func (t testStruct) WithUpdatedBy(author string) testStruct {
	t.UpdatedBy = author
	return t
}

func prepareServiceForListTest(
	t *testing.T, s *Service[testStruct],
	name string,
) int {
	a := testStruct{Content: "bob"}
	b := testStruct{Content: "alice"}
	ctx := context.Background()
	for i := range 10 {
		myID := fmt.Sprintf("%s_list_bob_%d", name, i)
		a.Id = myID
		err := s.Create(ctx, myID, a)
		require.NoError(t, err)
	}

	for i := range 2 {
		myID := fmt.Sprintf("%s_list_alice_%d", name, i)
		b.Id = myID
		err := s.Create(ctx, myID, b)
		require.NoError(t, err)
	}

	return 12
}

func assertListResults(t *testing.T, it storageapi.Iterator, n int) {
	var i int
	for it.HasNext() {
		i++
		var tmp testStruct
		err := it.NextTo(&tmp)
		require.NoError(t, err)
		if strings.Contains(tmp.ID(), "bob") {
			assert.Equal(t, "bob", tmp.Content)
		} else if strings.Contains(tmp.ID(), "bob") {
			assert.Equal(t, "alice", tmp.Content)
		} // else test created some other doc, skip
	}
	assert.Equal(t, n, i)
}

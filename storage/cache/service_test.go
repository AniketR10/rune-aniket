// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package cache

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
)

func TestCacheService(t *testing.T) {
	t.Run("creates hit the underlying service", func(t *testing.T) {
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc, document.NewInMemoryService())
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
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc, document.NewInMemoryService())
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
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc, document.NewInMemoryService())
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
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc, document.NewInMemoryService())
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
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc, document.NewInMemoryService())
		ctx := context.Background()
		n := prepareServiceForListTest(t, cache, "first")

		it, err := cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n)
	})

	t.Run("second call to list List returns all documents cached from the previous list", func(t *testing.T) {
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc, document.NewInMemoryService())
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
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc, document.NewInMemoryService())
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
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc, document.NewInMemoryService())
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
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc, document.NewInMemoryService())
		ctx := context.Background()
		n := prepareServiceForListTest(t, cache, "first")

		it, err := cache.List(ctx, []document.Filter{
			{Field: document.Field{FieldPath: []string{"OtherField"}}, Op: document.OpEqual},
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
	for i := 0; i < 10; i++ {
		myID := fmt.Sprintf("%s_list_bob_%d", name, i)
		a.Id = myID
		err := s.Create(ctx, myID, a)
		require.NoError(t, err)
	}

	for i := 0; i < 2; i++ {
		myID := fmt.Sprintf("%s_list_alice_%d", name, i)
		b.Id = myID
		err := s.Create(ctx, myID, b)
		require.NoError(t, err)
	}

	return 12
}

func assertListResults(t *testing.T, it document.Iterator, n int) {
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

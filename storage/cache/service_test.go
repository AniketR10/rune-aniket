package cache

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheService(t *testing.T) {
	t.Run("creates hit the underlying service", func(t *testing.T) {
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc)
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
		cache := New[testStruct](svc)
		ctx := context.Background()

		id := "myId"
		err := cache.Create(ctx, id, &testStruct{Id: id, Content: "1234"})
		require.NoError(t, err)

		var temp testStruct
		err = cache.Get(ctx, id, &temp) // force caching
		require.NoError(t, err)
		assert.Equal(t, "1234", temp.Content)

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
		cache := New[testStruct](svc)
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
		cache := New[testStruct](svc)
		ctx := context.Background()

		id := "myId"
		err := cache.Create(ctx, id, &testStruct{Id: id, Content: "1234"})
		require.NoError(t, err)

		err = svc.Set(ctx, id, &testStruct{Id: id, Content: "AAAA"})
		require.NoError(t, err)

		cache.EvictAll()

		var temp testStruct
		err = cache.Get(ctx, "myId", &temp)
		require.NoError(t, err)
		assert.Equal(t, "AAAA", temp.Content)
	})

	t.Run("first List returns all documents from the underlying service", func(t *testing.T) {
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc)
		ctx := context.Background()
		n := prepareServiceForListTest(t, cache, "first")

		it, err := cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n)
	})

	t.Run("second call to list List returns all documents cached from the previous list", func(t *testing.T) {
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc)
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

	t.Run("writes should force next List to call the underlying service", func(t *testing.T) {
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc)
		ctx := context.Background()
		n := prepareServiceForListTest(t, cache, "first")

		it, err := cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n)

		err = cache.Set(ctx, "otherID", &testStruct{Id: "otherID", Content: "AAAA"})
		require.NoError(t, err)

		it, err = cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n+1)
	})

	t.Run("List with filters always calls the underlying service", func(t *testing.T) {
		svc := document.NewInMemoryService()
		cache := New[testStruct](svc)
		ctx := context.Background()
		n := prepareServiceForListTest(t, cache, "first")

		it, err := cache.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, n)

		err = svc.Set(ctx, "otherID", &testStruct{Id: "otherID", Content: "AAAA"})
		require.NoError(t, err)

		it, err = cache.List(ctx, []document.Filter{
			{Field: document.Field{FieldPath: []string{"OtherField"}}, Op: document.OpEqual},
		})
		require.NoError(t, err)
		assertListResults(t, it, n+1)
	})
}

type testStruct struct {
	Id        string
	Content   string
	UpdatedAt time.Time
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

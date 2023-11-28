package dialogue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/extension_ai/backend"
)

func TestManager(t *testing.T) {
	ctx := context.Background()
	t.Run("surfaces error if openai client returns error", func(t *testing.T) {
		store := newTestStore(t)
		client := testClient{err: errors.New("boom")}
		manager := newTestManager(client, store)
		_, err := manager.CreateCompletion(ctx, "myId", []string{"hello"})
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "boom"), err.Error())
	})

	t.Run("returns error if input len is 0", func(t *testing.T) {
		store := newTestStore(t)
		client := testClient{err: errors.New("boom")}
		manager := newTestManager(client, store)
		_, err := manager.CreateCompletion(ctx, "myId", nil)
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "input"), err.Error())
	})

	t.Run("surfaces error if openai stream returns error", func(t *testing.T) {
		store := newTestStore(t)
		client := testClient{streamErr: errors.New("boom")}
		manager := newTestManager(client, store)
		it, err := manager.CreateCompletion(ctx, "myId", []string{"hello"})
		require.NoError(t, err)
		_, ok := it.Next()
		require.False(t, ok)
		err = it.Err()
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "boom"), err.Error())
	})

	t.Run("streams completion back and consumes entire original stream", func(t *testing.T) {
		store := newTestStore(t)
		client := testClient{streamRes: []backend.ChatCompletionResponse{
			{Message: backend.ChatCompletionMessage{Content: "0"}},
			{Message: backend.ChatCompletionMessage{Content: "1"}},
			{Message: backend.ChatCompletionMessage{Content: "2"}},
		}}
		manager := newTestManager(client, store)
		it, err := manager.CreateCompletion(ctx, "myId", []string{"hello"})
		require.NoError(t, err)
		for i := 0; i < 3; i++ {
			next, ok := it.Next()
			require.True(t, ok)
			assert.Equal(t, strconv.Itoa(i), next)
		}

		_, ok := it.Next()
		require.False(t, ok)

		err = it.Err()
		require.NoError(t, err)

		assert.True(t, it.(*completionStreamIterator).it.(*testStream).closed)
	})

	t.Run("stores messages in store", func(t *testing.T) {
		store := newTestStore(t)
		messages := []backend.ChatCompletionResponse{
			{Message: backend.ChatCompletionMessage{Content: "0"}},
			{Message: backend.ChatCompletionMessage{Content: "1"}},
			{Message: backend.ChatCompletionMessage{Content: "2"}},
		}
		client := testClient{streamRes: messages}
		manager := newTestManager(client, store)
		m := len(messages)
		n := 2

		for i := 0; i < n; i++ {
			it, err := manager.CreateCompletion(ctx, "myId", []string{fmt.Sprintf("hello:%d", i)})
			require.NoError(t, err)
			l, err := iterator.ToSlice(it)
			require.NoError(t, err)
			require.Len(t, l, m)
		}

		dialogue, err := store.Get(context.Background(), "myId")
		require.NoError(t, err)

		// client streamed messages should be conflated
		require.Len(t, dialogue.Messages, n*2)
		for i, msg := range dialogue.Messages {
			if i%2 == 0 {
				assert.Equal(t, backend.ChatCompletionMessage{
					Role: "user", Content: fmt.Sprintf("hello:%d", i/2)}, msg)
			} else {
				assert.Equal(t, backend.ChatCompletionMessage{
					Role: "assistant", Content: "012"}, msg)
			}
		}
	})
}

var _ backend.Service = testClient{}

type testClient struct {
	err       error
	streamErr error
	streamRes []backend.ChatCompletionResponse
}

func (t testClient) CreateChatCompletion(
	ctx context.Context,
	request backend.ChatCompletionRequest,
) (iterator.Iterator[backend.ChatCompletionResponse], error) {
	return &testStream{res: t.streamRes, err: t.streamErr}, t.err
}

func (t testClient) CountTokens([]backend.ChatCompletionMessage) int {
	return 0
}

type testStream struct {
	err    error
	res    []backend.ChatCompletionResponse
	closed bool
}

func (t *testStream) Next() (ret backend.ChatCompletionResponse, ok bool) {
	if t.err != nil {
		return
	}
	if len(t.res) == 0 {
		t.closed = true
		return
	}
	ok = true
	ret = t.res[0]
	t.res = t.res[1:]
	return
}

func (t *testStream) Err() error {
	return t.err
}

func newTestStore(t *testing.T) Store {
	svc := document.NewInMemoryService()
	store := NewStore(svc)
	return store
}

func newTestManager(client backend.Service, store Store) Manager {
	return Manager{
		store: store,
		svc:   client,
		model: "testModel",
	}
}

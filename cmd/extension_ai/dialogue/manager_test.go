package dialogue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
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
			{Message: backend.ChatCompletionMessage{Content: "2"},
				FinishReason: backend.FinishReasonStop},
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

	t.Run("dispatches Completer.Complete when configured to do so", func(t *testing.T) {
		store := newTestStore(t)
		myMetadata := "myMeta"
		client := testClient{streamRes: []backend.ChatCompletionResponse{
			{Message: backend.ChatCompletionMessage{Content: "0"}},
			{Message: backend.ChatCompletionMessage{Content: "1"}},
			{
				ID: "1234",
				Message: backend.ChatCompletionMessage{
					Content:  "2",
					Metadata: myMetadata,
				},
				FinishReason: backend.FinishReasonToolCall,
			},
		}}
		var called int
		completer := FuncCompleter(func(ctx context.Context, dialogueID, completionID string,
			reason backend.FinishReason, msg backend.ChatCompletionMessage) {
			expectedMsg := backend.ChatCompletionMessage{
				Content:  "012",
				Role:     backend.RoleAssistant,
				Metadata: myMetadata,
			}
			assert.Equal(t, expectedMsg, msg)
			assert.Equal(t, backend.FinishReasonToolCall, reason)
			assert.Equal(t, "1234", completionID)
			assert.Equal(t, "myId", dialogueID)
			called++
		})
		manager := newTestManager(client, store,
			WithCompleter(completer), WithCompleter(completer))
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
		assert.Equal(t, 2, called)
	})

	t.Run("returns error for non-ok finish reasons", func(t *testing.T) {
		finishReasons := []backend.FinishReason{
			backend.FinishReasonLength,
			backend.FinishReasonContentFilter,
			backend.FinishReasonNull,
			backend.FinishReason(""),
			backend.FinishReason("somethingElse"),
		}
		for _, finishReason := range finishReasons {
			t.Run(string(finishReason), func(t *testing.T) {
				store := newTestStore(t)
				client := testClient{streamRes: []backend.ChatCompletionResponse{
					{
						ID:      "inconsistent IDs, should use last",
						Message: backend.ChatCompletionMessage{Content: "X"},
					},
					{
						ID:           "1234",
						Message:      backend.ChatCompletionMessage{Content: "0"},
						FinishReason: finishReason,
					},
				}}
				var called bool
				completer := func(ctx context.Context, dialogueID, completionID string,
					reason backend.FinishReason, msg backend.ChatCompletionMessage) {
					expectedMsg := backend.ChatCompletionMessage{
						Content: "X0",
						Role:    backend.RoleAssistant,
					}
					assert.Equal(t, expectedMsg, msg)
					assert.Equal(t, finishReason, reason)
					assert.Equal(t, "1234", completionID)
					assert.Equal(t, "myId", dialogueID)
					called = true
				}
				manager := newTestManager(client, store, WithCompleter(FuncCompleter(completer)))
				it, err := manager.CreateCompletion(ctx, "myId", []string{"hello"})
				require.NoError(t, err)

				for i := 0; i < 2; i++ {
					_, ok := it.Next()
					require.True(t, ok)
				}

				_, ok := it.Next()
				require.False(t, ok)

				err = it.Err()
				require.Error(t, err)

				assert.True(t, called)
			})
		}
	})

	t.Run("stores messages in store", func(t *testing.T) {
		store := newTestStore(t)
		messages := []backend.ChatCompletionResponse{
			{Message: backend.ChatCompletionMessage{Content: "0"}},
			{Message: backend.ChatCompletionMessage{Content: "1"}},
			{Message: backend.ChatCompletionMessage{Content: "2"},
				FinishReason: backend.FinishReasonStop},
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
					Role: backend.RoleUser, Content: fmt.Sprintf("hello:%d", i/2)}, msg)
			} else {
				assert.Equal(t, backend.ChatCompletionMessage{
					Role: backend.RoleAssistant, Content: "012"}, msg)
			}
		}
	})

	t.Run("trims messages if prompt with no history count of tokens is > context window", func(t *testing.T) {
		contextWindow := 100
		store := newTestStore(t)
		client := testClient{
			exceedsContextWindow: contextWindow,
			streamRes: []backend.ChatCompletionResponse{
				{Message: backend.ChatCompletionMessage{Content: "0"}},
				{Message: backend.ChatCompletionMessage{Content: "1"}},
				{Message: backend.ChatCompletionMessage{Content: "2"},
					FinishReason: backend.FinishReasonStop},
			},
		}
		input, _ := makeContentTokens(contextWindow)
		manager := newTestManager(client, store)
		it, err := manager.CreateCompletion(ctx, "myId", input)
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

		dialogue, err := store.Get(context.Background(), "myId")
		require.NoError(t, err)

		// 100-1 -> (below window) + 1 answer
		require.Len(t, dialogue.Messages, 100)
	})

	t.Run("trims messages if prompt + history count of tokens is > context window", func(t *testing.T) {
		contextWindow := 100
		store := newTestStore(t)
		client := testClient{
			exceedsContextWindow: contextWindow,
			streamRes: []backend.ChatCompletionResponse{
				{Message: backend.ChatCompletionMessage{Content: "0"}},
				{Message: backend.ChatCompletionMessage{Content: "1"}},
				{Message: backend.ChatCompletionMessage{Content: "2"},
					FinishReason: backend.FinishReasonStop},
			},
		}
		manager := newTestManager(client, store)

		var msgs []backend.ChatCompletionMessage
		for {
			exceeds, _ := client.ExceedsContextWindow(msgs)
			if exceeds {
				break
			}
			input, cmsgs := makeContentTokens(contextWindow / 8)
			msgs = append(msgs, cmsgs...)
			it, err := manager.CreateCompletion(ctx, "myId", input)
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
		}

		dialogue, err := store.Get(context.Background(), "myId")
		require.NoError(t, err)

		// 100-2 -> (below window) + 1 answer
		require.Len(t, dialogue.Messages, 99)
	})
}

var _ backend.Service = testClient{}

type testClient struct {
	exceedsContextWindow int
	err                  error
	streamErr            error
	streamRes            []backend.ChatCompletionResponse
}

func (t testClient) CreateChatCompletion(
	ctx context.Context,
	request backend.ChatCompletionRequest,
) (iterator.Iterator[backend.ChatCompletionResponse], error) {
	if exceeds, _ := t.ExceedsContextWindow(request.Messages); exceeds {
		return nil, &backend.ErrContextWindowExceeded{}
	}
	return &testStream{res: t.streamRes, err: t.streamErr}, t.err
}

func (t testClient) ExceedsContextWindow(
	msgs []backend.ChatCompletionMessage,
) (bool, error) {
	if t.exceedsContextWindow == 0 {
		return false, t.err
	}
	return len(msgs) >= t.exceedsContextWindow, t.err
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

func newTestManager(client backend.Service, store Store, options ...Option) *Manager {
	return NewManager(client, store, options...)
}

func makeContentTokens(greaterThan int) ([]string, []backend.ChatCompletionMessage) {
	var ret []string
	var msgs []backend.ChatCompletionMessage
	for i := 0; len(msgs) < greaterThan; i++ {
		msgs = append(msgs, backend.ChatCompletionMessage{Content: strconv.Itoa(i)})
		ret = append(ret, strconv.Itoa(i))
	}
	return ret, msgs
}

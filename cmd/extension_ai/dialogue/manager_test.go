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
	"github.com/unstablebuild/blue/ai/llm"
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
		_, ok := it.Next(context.Background())
		require.False(t, ok)
		err = it.Err()
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "boom"), err.Error())
	})

	t.Run("streams completion back and consumes entire original stream", func(t *testing.T) {
		store := newTestStore(t)
		client := testClient{streamRes: []llm.ChatCompletionResponse{
			{Message: llm.ChatCompletionMessage{Content: "0"}},
			{Message: llm.ChatCompletionMessage{Content: "1"}},
			{Message: llm.ChatCompletionMessage{Content: "2"},
				FinishReason: llm.FinishReasonStop},
		}}
		manager := newTestManager(client, store)
		it, err := manager.CreateCompletion(ctx, "myId", []string{"hello"})
		require.NoError(t, err)
		for i := 0; i < 3; i++ {
			next, ok := it.Next(context.Background())
			require.True(t, ok)
			assert.Equal(t, strconv.Itoa(i), next)
		}

		_, ok := it.Next(context.Background())
		require.False(t, ok)

		err = it.Err()
		require.NoError(t, err)

		require.NoError(t, it.Close())

		assert.True(t, it.(*completionStreamIterator).it.(*testStream).closed)
	})

	t.Run("dispatches Completer.Complete when configured to do so", func(t *testing.T) {
		store := newTestStore(t)
		myMetadata := "myMeta"
		client := testClient{streamRes: []llm.ChatCompletionResponse{
			{Message: llm.ChatCompletionMessage{Content: "0"}},
			{Message: llm.ChatCompletionMessage{Content: "1"}},
			{
				ID: "1234",
				Message: llm.ChatCompletionMessage{
					Content:  "2",
					Metadata: myMetadata,
				},
				FinishReason: llm.FinishReasonToolCall,
			},
		}}
		var called int
		completer := FuncCompleter(func(ctx context.Context, dialogueID, completionID string,
			reason llm.FinishReason, msg llm.ChatCompletionMessage) {
			expectedMsg := llm.ChatCompletionMessage{
				Content:  "012",
				Role:     llm.RoleAssistant,
				Metadata: myMetadata,
			}
			assert.Equal(t, expectedMsg, msg)
			assert.Equal(t, llm.FinishReasonToolCall, reason)
			assert.Equal(t, "1234", completionID)
			assert.Equal(t, "myId", dialogueID)
			called++
		})
		manager := newTestManager(client, store,
			WithCompleter(completer), WithCompleter(completer))
		it, err := manager.CreateCompletion(ctx, "myId", []string{"hello"})
		require.NoError(t, err)
		for i := 0; i < 3; i++ {
			next, ok := it.Next(context.Background())
			require.True(t, ok)
			assert.Equal(t, strconv.Itoa(i), next)
		}

		_, ok := it.Next(context.Background())
		require.False(t, ok)

		err = it.Err()
		require.NoError(t, err)

		require.NoError(t, it.Close())

		assert.True(t, it.(*completionStreamIterator).it.(*testStream).closed)
		assert.Equal(t, 2, called)
	})

	t.Run("returns error for non-ok finish reasons", func(t *testing.T) {
		finishReasons := []llm.FinishReason{
			llm.FinishReasonLength,
			llm.FinishReasonContentFilter,
			llm.FinishReasonNull,
			llm.FinishReason(""),
			llm.FinishReason("somethingElse"),
		}
		for _, finishReason := range finishReasons {
			t.Run(string(finishReason), func(t *testing.T) {
				store := newTestStore(t)
				client := testClient{streamRes: []llm.ChatCompletionResponse{
					{
						ID:      "inconsistent IDs, should use last",
						Message: llm.ChatCompletionMessage{Content: "X"},
					},
					{
						ID:           "1234",
						Message:      llm.ChatCompletionMessage{Content: "0"},
						FinishReason: finishReason,
					},
				}}
				var called bool
				completer := func(ctx context.Context, dialogueID, completionID string,
					reason llm.FinishReason, msg llm.ChatCompletionMessage) {
					expectedMsg := llm.ChatCompletionMessage{
						Content: "X0",
						Role:    llm.RoleAssistant,
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
					_, ok := it.Next(context.Background())
					require.True(t, ok)
				}

				_, ok := it.Next(context.Background())
				require.False(t, ok)

				err = it.Err()
				require.Error(t, err)

				assert.True(t, called)
			})
		}
	})

	t.Run("stores messages in store", func(t *testing.T) {
		store := newTestStore(t)
		messages := []llm.ChatCompletionResponse{
			{Message: llm.ChatCompletionMessage{Content: "0"}},
			{Message: llm.ChatCompletionMessage{Content: "1"}},
			{Message: llm.ChatCompletionMessage{Content: "2"},
				FinishReason: llm.FinishReasonStop},
		}
		client := testClient{streamRes: messages}
		manager := newTestManager(client, store)
		m := len(messages)
		n := 2

		for i := 0; i < n; i++ {
			it, err := manager.CreateCompletion(ctx, "myId", []string{fmt.Sprintf("hello:%d", i)})
			require.NoError(t, err)
			l, err := iterator.ToSlice(context.Background(), it)
			require.NoError(t, err)
			require.Len(t, l, m)
		}

		dialogue, err := store.Get(context.Background(), "myId")
		require.NoError(t, err)

		// client streamed messages should be conflated
		require.Len(t, dialogue.Messages, n*2)
		for i, msg := range dialogue.Messages {
			if i%2 == 0 {
				assert.Equal(t, llm.ChatCompletionMessage{
					Role: llm.RoleUser, Content: fmt.Sprintf("hello:%d", i/2)}, msg)
			} else {
				assert.Equal(t, llm.ChatCompletionMessage{
					Role: llm.RoleAssistant, Content: "012"}, msg)
			}
		}
	})

	t.Run("trims messages if prompt with no history count of tokens is > context window", func(t *testing.T) {
		contextWindow := 100
		store := newTestStore(t)
		client := testClient{
			exceedsContextWindow: contextWindow,
			streamRes: []llm.ChatCompletionResponse{
				{Message: llm.ChatCompletionMessage{Content: "0"}},
				{Message: llm.ChatCompletionMessage{Content: "1"}},
				{Message: llm.ChatCompletionMessage{Content: "2"},
					FinishReason: llm.FinishReasonStop},
			},
		}
		input, _ := makeContentTokens(contextWindow)
		manager := newTestManager(client, store)
		it, err := manager.CreateCompletion(ctx, "myId", input)
		require.NoError(t, err)
		for i := 0; i < 3; i++ {
			next, ok := it.Next(context.Background())
			require.True(t, ok)
			assert.Equal(t, strconv.Itoa(i), next)
		}

		_, ok := it.Next(context.Background())
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
			streamRes: []llm.ChatCompletionResponse{
				{Message: llm.ChatCompletionMessage{Content: "0"}},
				{Message: llm.ChatCompletionMessage{Content: "1"}},
				{Message: llm.ChatCompletionMessage{Content: "2"},
					FinishReason: llm.FinishReasonStop},
			},
		}
		manager := newTestManager(client, store)

		var msgs []llm.ChatCompletionMessage
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
				next, ok := it.Next(context.Background())
				require.True(t, ok)
				assert.Equal(t, strconv.Itoa(i), next)
			}

			_, ok := it.Next(context.Background())
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

var _ llm.Service = testClient{}

type testClient struct {
	exceedsContextWindow int
	err                  error
	streamErr            error
	streamRes            []llm.ChatCompletionResponse
}

func (t testClient) CreateChatCompletion(
	ctx context.Context,
	request llm.ChatCompletionRequest,
) (iterator.Iterator[llm.ChatCompletionResponse], error) {
	if exceeds, _ := t.ExceedsContextWindow(request.Messages); exceeds {
		return nil, &llm.ErrContextWindowExceeded{}
	}
	return &testStream{res: t.streamRes, err: t.streamErr}, t.err
}

func (t testClient) ExceedsContextWindow(
	msgs []llm.ChatCompletionMessage,
) (bool, error) {
	if t.exceedsContextWindow == 0 {
		return false, t.err
	}
	return len(msgs) >= t.exceedsContextWindow, t.err
}

type testStream struct {
	err    error
	res    []llm.ChatCompletionResponse
	closed bool
}

func (t *testStream) Next(context.Context) (ret llm.ChatCompletionResponse, ok bool) {
	if t.err != nil {
		return
	}
	if len(t.res) == 0 {
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

func (t *testStream) Close() error {
	t.closed = true
	return nil
}

func newTestStore(t *testing.T) Store {
	svc := document.NewInMemoryService()
	store := NewStore(svc)
	return store
}

func newTestManager(client llm.Service, store Store, options ...Option) *Manager {
	return NewManager(client, store, options...)
}

func makeContentTokens(greaterThan int) ([]string, []llm.ChatCompletionMessage) {
	var ret []string
	var msgs []llm.ChatCompletionMessage
	for i := 0; len(msgs) < greaterThan; i++ {
		msgs = append(msgs, llm.ChatCompletionMessage{Content: strconv.Itoa(i)})
		ret = append(ret, strconv.Itoa(i))
	}
	return ret, msgs
}

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
	"sync"

	"github.com/unstablebuild/blue/ai/llm"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

// Manager implements chat completion via a llm.Service.
// It uses Store to persist context window across sessions.
//
// Manager implements an interface that is goroutine-safe
// if and only if the given Store is also goroutine-safe.
type Manager struct {
	config
	store     Store
	svc       llm.Service
	resources sync.Map
}

// NewManager allocates storage for a new Manager and initializes it.
func NewManager(svc llm.Service, store Store, opts ...Option) *Manager {
	ret := new(Manager)
	ret.Init(svc, store, opts...)
	return ret
}

// Init initializes this Manager with the given backend service, store and options.
func (m *Manager) Init(svc llm.Service, store Store, opts ...Option) {
	m.store = store
	m.svc = svc

	for _, o := range opts {
		o(&m.config)
	}
}

// AddContextResource adds a resource that is passed as context
// on every completion request or if resource has already previously
// been set, it updates it.
func (m *Manager) AddContextResource(
	ctx context.Context, uri workspaceapi.URI, data string,
) (err error) {
	m.resources.Store(uri, data)
	return nil
}

// RemoveContextResource removes a resource previously created
// via AddContextResource or returns an error if no such resource exists.
func (m *Manager) RemoveContextResource(
	ctx context.Context, uri workspaceapi.URI,
) (err error) {
	_, loaded := m.resources.LoadAndDelete(uri)
	if !loaded {
		err = fmt.Errorf("resource with URI %q not found", uri.String())
	}
	return
}

// CreateCompletion uses the context stored for the given
// dialogue ID to query a backend for a new chat completion
// and returns the model's response.
func (m *Manager) CreateCompletion(
	ctx context.Context, dialogueID string, messages []string,
) (ret iterator.Iterator[string], err error) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := []logging.Field{
		{Key: logging.KeyClass, Value: "dialogue.Manager"},
		{Key: "DialogueID", Value: dialogueID},
		{Key: "MessageCount", Value: strconv.Itoa(len(messages))},
	}
	attemptAt := logging.LogAttempt(traceID, "CreateCompletion", fields...)
	ret, n, err := m.doCreateCompletion(ctx, dialogueID, messages)
	fields = append(fields, logging.Field{
		Key:   "Messages",
		Value: strconv.Itoa(n),
	})
	logging.LogResultInfo(err, attemptAt, traceID, "CreateCompletion", fields...)
	return ret, err
}

type config struct {
	initialContext []llm.ChatCompletionMessage
	completer      Completer
}

func (m *Manager) doCreateCompletion(
	ctx context.Context, dialogueID string, input []string,
) (ret iterator.Iterator[string], n int, err error) {
	if len(input) == 0 {
		err = errors.New("empty completion input")
		return
	}
	dialogue, err := m.store.Get(ctx, dialogueID)
	if err != nil {
		if !errors.Is(err, document.ErrNotFound) {
			err = fmt.Errorf("get stored dialogue: %w", err)
			return
		}
		dialogue.Messages = make([]llm.ChatCompletionMessage, len(m.config.initialContext))
		copy(dialogue.Messages, m.config.initialContext)
	}

	var prompt []llm.ChatCompletionMessage

	m.resources.Range(func(k, v any) bool {
		prompt = append(prompt, llm.ChatCompletionMessage{
			Role: llm.RoleSystem,
			Content: fmt.Sprintf("The file with URI %s is "+
				"in the user's context:\n```\n%s\n```", k, v),
		})
		return true
	})

	for _, msg := range input {
		prompt = append(prompt,
			llm.ChatCompletionMessage{Role: llm.RoleUser, Content: msg})
	}

	// append new message to chat context, but keep msgs
	// len intact so at the end of the stream when we call AppendMessages
	// dialogue.Messages still reflects the previous version
	totalMessages := make([]llm.ChatCompletionMessage, len(dialogue.Messages)+len(prompt))
	copy(totalMessages, dialogue.Messages)
	copy(totalMessages[len(dialogue.Messages):], prompt)

	// NOTE: reduce number of messages until it's below context window.
	var exceedsContextWindow bool
	for len(totalMessages) > 2 {
		exceeds, err := m.svc.ExceedsContextWindow(totalMessages)
		if err != nil {
			return nil, 0, fmt.Errorf("could not verify context window: %v", err)
		}
		if !exceeds {
			break
		}
		exceedsContextWindow = true
		if len(dialogue.Messages) == 0 {
			prompt = prompt[1:]
			totalMessages = totalMessages[1:]
		} else {
			// -2 so we remove a single user/assistant interaction
			totalMessages = totalMessages[2:]
		}
	}

	// if len(totalMessages) is smaller than 2, but requests is
	// still exceeding context window, something must be wrong
	// with ExceedsContextWindow implementation so proceed and
	// let CreateChatCompletion return the appropiate error.

	req := llm.ChatCompletionRequest{
		Messages: totalMessages,
	}
	it, err := m.svc.CreateChatCompletion(ctx, req)
	if err != nil {
		err = fmt.Errorf("create chat completion: %v", err)
		return
	}

	n = len(totalMessages)
	ret = &completionStreamIterator{
		exceedsContextWindow: exceedsContextWindow,
		totalMessages:        totalMessages,
		dialogueID:           dialogueID,
		userPrompt:           prompt,
		dialogue:             dialogue,
		store:                m.store,
		it:                   it,
		ctx:                  ctx,
		completer:            m.config.completer,
	}
	return
}

// stores full message in store at the end of stream
type completionStreamIterator struct {
	ctx                  context.Context
	dialogueID           string
	userPrompt           []llm.ChatCompletionMessage
	totalMessages        []llm.ChatCompletionMessage
	dialogue             Dialogue
	exceedsContextWindow bool
	store                Store
	it                   iterator.Iterator[llm.ChatCompletionResponse]
	completer            Completer

	finishReason llm.FinishReason
	completionID string
	metadata     any
	response     strings.Builder
	err          error
}

func (s *completionStreamIterator) Next(ctx context.Context) (string, bool) {
	resp, ok := s.it.Next(ctx)
	if ok {
		s.completionID = resp.ID
		// last ok responsive should contain the finish reason
		// and complete Metadata.
		s.finishReason = resp.FinishReason
		s.metadata = resp.Message.Metadata
		s.response.WriteString(resp.Message.Content)
		return resp.Message.Content, true
	}
	if err := s.it.Err(); err != nil {
		s.err = err
		return "", false
	}
	response := llm.ChatCompletionMessage{
		Role:     llm.RoleAssistant,
		Content:  s.response.String(),
		Metadata: s.metadata,
	}
	if s.completer != nil {
		// we want to dispatch it for all finish reasons, but only after we have
		// finished appending new messages to the dialogue store
		defer s.completer.Complete(ctx, s.dialogueID,
			s.completionID, s.finishReason, response)
	}
	switch s.finishReason {
	case llm.FinishReasonStop, llm.FinishReasonToolCall:
		/* ok */
	case llm.FinishReasonContentFilter:
		s.err = errors.New("omitted content due to a flag from the backend's content filters")
	case llm.FinishReasonNull:
		s.err = errors.New("unexpected end of stream")
	case llm.FinishReasonLength:
		s.err = errors.New("incomplete model output due to configuration parameter or token limit")
	default:
		s.err = errors.New("unknown stream finish reason")
	}
	if s.err != nil {
		return "", false
	}
	newMsgs := append(s.userPrompt, response)
	err := s.store.Create(ctx, s.dialogueID, newMsgs)
	if errors.Is(err, document.ErrAlreadyExists) {
		// Calls to ExceedsContextWindow might be producing network requests
		// so we must ensure that we also trim the messages persisted
		// in store so next call to store.Get above gets the trimmed dialogue,
		// and so we call ExceedsContextWindow at most twice.
		if s.exceedsContextWindow {
			totalMessages := append(s.totalMessages, response)
			err = s.store.Set(ctx, s.dialogueID, totalMessages)
		} else {
			err = s.store.AppendMessages(ctx, s.dialogue, newMsgs)
		}
	}
	if err != nil {
		s.err = err
	}
	return "", false
}

// Err returns the first error or an aggreation of the errors
// encountered by the Iterator.
func (s *completionStreamIterator) Err() error {
	return s.err
}

func (s *completionStreamIterator) Close() error {
	return s.it.Close()
}

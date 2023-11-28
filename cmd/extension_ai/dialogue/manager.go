package dialogue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
	"unstable.build/go-tui/cmd/extension_ai/backend"
	"unstable.build/go-tui/cmd/extension_ai/backend/openai"
)

// Manager implements chat completion via a backend.Service.
// It uses Store to persist context window across sessions.
type Manager struct {
	store Store
	svc   backend.Service
	model string
}

// NewManager initializes a Manager with the given auth token
// model and Store.
func NewManager(openaiAuthToken string, model string, store Store) Manager {
	client := openai.NewClient(openaiAuthToken, openai.Config{
		Model: model,
	})
	return Manager{
		store: store,
		svc:   client,
		model: model,
	}
}

// CreateCompletion uses the context stored for the given
// dialogue ID to query openai for a new chat completion
// and returns the model's response.
func (m Manager) CreateCompletion(
	ctx context.Context, dialogueID string, messages []string,
) (ret iterator.Iterator[string], err error) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := []logging.Field{
		{Key: logging.KeyClass, Value: "dialogue.Manager"},
		{Key: "DialogueID", Value: dialogueID},
		{Key: "MessageCount", Value: strconv.Itoa(len(messages))},
		{Key: "Model", Value: m.model},
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

func (m Manager) doCreateCompletion(
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
	}

	var prompt []backend.ChatCompletionMessage
	for _, msg := range input {
		prompt = append(prompt,
			backend.ChatCompletionMessage{Role: "user", Content: msg})
	}

	// append new message to chat context, but keep msgs
	// len intact so at the end of the stream when we call AppendMessages
	// dialogue.Messages still reflects the previous version
	totalMessages := make([]backend.ChatCompletionMessage, len(dialogue.Messages)+len(prompt))
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

	req := backend.ChatCompletionRequest{
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
	}
	return
}

// stores full message in store at the end of stream
type completionStreamIterator struct {
	ctx                  context.Context
	dialogueID           string
	userPrompt           []backend.ChatCompletionMessage
	totalMessages        []backend.ChatCompletionMessage
	dialogue             Dialogue
	exceedsContextWindow bool
	store                Store
	it                   iterator.Iterator[backend.ChatCompletionResponse]

	response strings.Builder
	err      error
}

func (s *completionStreamIterator) Next() (string, bool) {
	resp, ok := s.it.Next()
	if ok {
		s.response.WriteString(resp.Message.Content)
		return resp.Message.Content, true
	}
	if err := s.it.Err(); err != nil {
		s.err = err
		return "", false
	}
	response := backend.ChatCompletionMessage{
		Role:    string(openai.RoleAssistant),
		Content: s.response.String(),
		// Metadata: ignore function calls for now
		// Name: not usually defined for an assistant
	}
	newMsgs := append(s.userPrompt, response)
	err := s.store.Create(s.ctx, s.dialogueID, newMsgs)
	if errors.Is(err, document.ErrAlreadyExists) {
		// Calls to ExceedsContextWindow might be producing network requests
		// so we must ensure that we also trim the messages persisted
		// in store so next call to store.Get above gets the trimmed dialogue,
		// and so we call ExceedsContextWindow at most twice.
		if s.exceedsContextWindow {
			totalMessages := append(s.totalMessages, response)
			err = s.store.Set(s.ctx, s.dialogueID, totalMessages)
		} else {
			err = s.store.AppendMessages(s.ctx, s.dialogue, newMsgs)
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

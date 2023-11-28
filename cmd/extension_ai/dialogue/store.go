package dialogue

import (
	"context"
	"fmt"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
	"github.com/ernestrc/blue/retry"
	"unstable.build/go-tui/cmd/extension_ai/backend"
)

const (
	createCallType         = "Create"
	updateCallType         = "Update"
	getCallType            = "Get"
	deleteCallType         = "Delete"
	appendMessagesCallType = "AppendMessages"
	storeLoggingClass      = "dialogue.store"
)

var retryStrategy = retry.CombinedStrategy(
	retry.SequentialStrategy(30*time.Millisecond),
	retry.LimitStrategy(5),
)

// Store abstracts dialogue persistence to durable storage.
type Store interface {
	Health(context.Context) error
	Create(context.Context, string, []backend.ChatCompletionMessage) error
	Set(context.Context, string, []backend.ChatCompletionMessage) error
	Get(context.Context, string) (Dialogue, error)
	Delete(context.Context, string) error
	AppendMessages(context.Context, Dialogue, []backend.ChatCompletionMessage) error
	List(context.Context) (iterator.Iterator[Dialogue], error)
}

// NewStore allocates storage for a new Store and
// initializes it with the given backend.
func NewStore(backend document.Service) Store {
	return store{backend: backend}
}

type store struct {
	backend document.Service
}

// Dialogue holds a dialogue's data as stored in durable storage.
type Dialogue struct {
	ID        string
	Version   int
	Messages  []backend.ChatCompletionMessage
	UpdatedAt time.Time
}

func (s store) Health(ctx context.Context) error {
	err := s.backend.Delete(ctx, "IDThatWillNeverExist")
	if err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	return nil
}

func (s store) Create(
	ctx context.Context, id string,
	msgs []backend.ChatCompletionMessage,
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := makeStoreLoggingFields(storeLoggingClass, id)
	attemptAt := logging.LogAttempt(traceID, createCallType, fields...)

	doc := newDialogue(id, msgs)
	err := s.backend.Create(ctx, id, &doc)
	logging.LogResultTrace(err, attemptAt, traceID, createCallType, fields...)
	if err != nil {
		return fmt.Errorf("document service create: %w", err)
	}
	return err
}

func (s store) Set(
	ctx context.Context, ID string, msgs []backend.ChatCompletionMessage,
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := makeStoreLoggingFields(storeLoggingClass, ID)
	attemptAt := logging.LogAttempt(traceID, updateCallType, fields...)

	doc := newDialogue(ID, msgs)

	err := s.backend.Set(ctx, ID, &doc)
	logging.LogResultTrace(err, attemptAt, traceID, updateCallType, fields...)
	if err != nil {
		return fmt.Errorf("document service set: %w", err)
	}
	return nil
}

func (s store) Get(
	ctx context.Context, ID string,
) (Dialogue, error) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := makeStoreLoggingFields(storeLoggingClass, ID)
	attemptAt := logging.LogAttempt(traceID, getCallType, fields...)

	var doc Dialogue
	err := s.backend.Get(ctx, ID, &doc)
	logging.LogResultTrace(err, attemptAt, traceID, getCallType, fields...)
	if err != nil {
		return Dialogue{}, fmt.Errorf("document service get: %w", err)
	}
	return doc, nil
}

func (s store) Delete(
	ctx context.Context, ID string,
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := makeStoreLoggingFields(storeLoggingClass, ID)
	attemptAt := logging.LogAttempt(traceID, deleteCallType, fields...)

	err := s.backend.Delete(ctx, ID)
	logging.LogResultTrace(err, attemptAt, traceID, deleteCallType, fields...)
	if err != nil {
		return fmt.Errorf("document service delete: %w", err)
	}
	return nil
}

func (s store) AppendMessages(
	ctx context.Context, d Dialogue, msgs []backend.ChatCompletionMessage,
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := makeStoreLoggingFields(storeLoggingClass, d.ID)
	attemptAt := logging.LogAttempt(traceID, appendMessagesCallType, fields...)

	err := document.ConsistentUpdate(ctx, s.backend, d.ID, &d, retryStrategy,
		func() ([]document.Update, []document.Precondition) {
			d.Messages = append(d.Messages, msgs...)

			updatedAt := time.Now()
			return []document.Update{
					{FieldPath: []string{"Messages"}, Value: d.Messages},
					{FieldPath: []string{"UpdatedAt"}, Value: updatedAt},
					{FieldPath: []string{"Version"}, Value: d.Version + 1},
				}, []document.Precondition{
					{FieldPath: []string{"Version"}, Value: d.Version},
				}
		})
	logging.LogResultTrace(err, attemptAt, traceID, appendMessagesCallType, fields...)
	if err != nil {
		return fmt.Errorf("consistent update : %w", err)
	}
	return nil
}

func (s store) List(ctx context.Context) (iterator.Iterator[Dialogue], error) {
	it, err := s.backend.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	return iterator.FromDocumentIterator[Dialogue](it), nil
}

func makeStoreLoggingFields(class string, id string) []logging.Field {
	return []logging.Field{
		{Key: logging.KeyClass, Value: class},
		{Key: "DialogueID", Value: id},
	}
}

func newDialogue(id string, msgs []backend.ChatCompletionMessage) Dialogue {
	return Dialogue{
		ID:        id,
		Messages:  msgs,
		UpdatedAt: time.Now(),
		Version:   1,
	}
}

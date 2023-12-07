package dialogue

import (
	"context"

	"unstable.build/go-tui/cmd/extension_ai/backend"
)

// Completer wraps the Complete method.
type Completer interface {
	// Complete is called by a Manager when a chat completion has streamed
	// successfully. It enables clients of Manager to access the
	// complete Metadata field returned by backend.Service at the end
	// of the stream and also allows them to handle backend.FinishReason.
	Complete(ctx context.Context, dialogueID, completionID string,
		reason backend.FinishReason, msg backend.ChatCompletionMessage)
}

// FuncCompleter satisfies Completer by wrapping a function.
type FuncCompleter func(context.Context, string, string,
	backend.FinishReason, backend.ChatCompletionMessage)

// Complete satisfies Completer.
func (f FuncCompleter) Complete(ctx context.Context, dialogueID, completionID string,
	reason backend.FinishReason, msg backend.ChatCompletionMessage) {
	f(ctx, dialogueID, completionID, reason, msg)
}

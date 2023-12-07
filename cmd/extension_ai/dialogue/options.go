package dialogue

import (
	"context"

	"unstable.build/go-tui/cmd/extension_ai/backend"
)

// Option is an optional configuration passed when initialigin a Manager.
type Option func(*config)

// WithInitialContext returns an option that sets the given messages
// as the initial messages of every conversation.
func WithInitialContext(msgs []backend.ChatCompletionMessage) Option {
	return func(cfg *config) {
		cfg.initialContext = msgs
	}
}

// WithCompleter returns an option that sets a Manager's Completer.
func WithCompleter(completer Completer) Option {
	return func(cfg *config) {
		// aggregate completers
		if cfg.completer != nil {
			prev := cfg.completer
			cfg.completer = FuncCompleter(func(ctx context.Context, dialogueID, completionID string,
				reason backend.FinishReason, msg backend.ChatCompletionMessage) {
				prev.Complete(ctx, dialogueID, completionID, reason, msg)
				completer.Complete(ctx, dialogueID, completionID, reason, msg)
			})
		} else {
			cfg.completer = completer
		}
	}
}

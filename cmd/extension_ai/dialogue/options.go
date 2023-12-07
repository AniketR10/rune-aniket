package dialogue

import "unstable.build/go-tui/cmd/extension_ai/backend"

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
		cfg.completer = completer
	}
}

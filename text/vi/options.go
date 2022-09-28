package vi

import (
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/text"
)

// viConfig holds configuration for Vi.
type viConfig struct {
	resAttr         term.Attributes
	clipboard       text.Clipboard
	defaultRegister string
	messenger       text.Messenger
	debug           bool
	wrap            bool
}

// Option represents a Vi handler configuration option.
type Option func(*viConfig)

// WithResAttr sets the search result cell attributes to be rendered.
func WithResAttr(attr term.Attributes) Option {
	return func(cfg *viConfig) {
		cfg.resAttr = attr
	}
}

// WithClipboard sets the editor.Clipboard implementation to use.
func WithClipboard(clip text.Clipboard) Option {
	return func(cfg *viConfig) {
		cfg.clipboard = clip
	}
}

// WithMessenger sets the editor.Messenger to use.
func WithMessenger(m text.Messenger) Option {
	return func(cfg *viConfig) {
		cfg.messenger = m
	}
}

// WithDebug disables cursor position correction to aid with cursor debugging.
func WithDebug(debug bool) Option {
	return func(cfg *viConfig) {
		cfg.debug = debug
	}
}

// WithWrap enables or disables word wrapping mode.
func WithWrap(wrap bool) Option {
	return func(cfg *viConfig) {
		cfg.wrap = wrap
	}
}

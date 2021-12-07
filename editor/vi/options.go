package vi

import (
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

// viConfig holds configuration for Vi.
type viConfig struct {
	resAttr         term.Attributes
	clipboard       editor.Clipboard
	defaultRegister string
	logger          *log.Logger
	messenger       editor.Messenger
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
func WithClipboard(clip editor.Clipboard) Option {
	return func(cfg *viConfig) {
		cfg.clipboard = clip
	}
}

// WithLogger sets the editor.Clipboard to use.
func WithLogger(l *log.Logger) Option {
	return func(cfg *viConfig) {
		cfg.logger = l
	}
}

// WithMessenger sets the editor.Messenger to use.
func WithMessenger(m editor.Messenger) Option {
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

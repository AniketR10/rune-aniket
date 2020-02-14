package vi

import (
	"github.com/ernestrc/fractal/editor"
	"github.com/ernestrc/fractal/term"
	log "github.com/sirupsen/logrus"
)

// TODO add keyMap and others.
// viConfig holds configuration for Vi.
type viConfig struct {
	ResAttr   term.Attributes
	Clipboard editor.Clipboard
	Logger    *log.Logger
}

// Option represents a Vi handler configuration option.
type Option func(*viConfig)

// WithResAttr sets the search result cell attributes to be rendered.
func WithResAttr(attr term.Attributes) Option {
	return func(cfg *viConfig) {
		cfg.ResAttr = attr
	}
}

// WithClipboard sets the editor.Clipboard implementation to use.
func WithClipboard(clip editor.Clipboard) Option {
	return func(cfg *viConfig) {
		cfg.Clipboard = clip
	}
}

// WithLogger sets the editor.Clipboard implementation to use.
func WithLogger(l *log.Logger) Option {
	return func(cfg *viConfig) {
		cfg.Logger = l
	}
}

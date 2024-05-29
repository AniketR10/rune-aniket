package vi

import (
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/clipboard"
)

// viConfig holds configuration for Vi.
type viConfig struct {
	attr                 term.Attributes
	resAttr              term.Attributes
	barAttr              term.Attributes
	clipboard            clipboard.Register
	defaultRegister      string
	superimposedMessages bool
	debug                bool
	wrap                 bool
	cursorCorrections    bool
	barHidden            bool
	skipNulls            bool
}

// defaultviHandlerImplConfig is a sane configuration defaults for viHandlerImpl.
func defaultviHandlerImplConfig() viConfig {
	return viConfig{
		resAttr: term.Attributes{
			Attrs: tcell.AttrReverse,
		},
		clipboard:         clipboard.NewInMemory(),
		defaultRegister:   clipboard.DefaultRegisterID,
		skipNulls:         true,
		cursorCorrections: true,
	}
}

// Option represents a Vi handler configuration option.
type Option func(*viConfig)

// WithResAttr sets the search result cell attributes to be rendered.
func WithResAttr(attr term.Attributes) Option {
	return func(cfg *viConfig) {
		cfg.resAttr = attr
	}
}

// WithBarAttr sets the command bar cell attributes to be rendered.
func WithBarAttr(attr term.Attributes) Option {
	return func(cfg *viConfig) {
		cfg.barAttr = attr
	}
}

// WithBarHidden sets the command bar to be hidden.
func WithBarHidden(hide bool) Option {
	return func(cfg *viConfig) {
		cfg.barHidden = hide
	}
}

// WithSuperimposedMessages changes the behaviour to instead of drawing
// a bottom bar permanently on which messages are written,
// messages are superimposed on the last row of the scroll content.
//
// The default value is false, so a full bar is drawn.
func WithSuperimposedMessages(value bool) Option {
	return func(cfg *viConfig) {
		cfg.superimposedMessages = value
	}
}

// WithAttr sets the default cell attributes to be rendered.
func WithAttr(attr term.Attributes) Option {
	return func(cfg *viConfig) {
		cfg.attr = attr
	}
}

// WithClipboard sets the editor.Clipboard implementation to use.
func WithClipboard(clip clipboard.Register) Option {
	return func(cfg *viConfig) {
		cfg.clipboard = clip
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

// WithCursorCorrections enables or disables cursor out of bounds corrections.
// By default it's enabled, unless this option is passed; when disabled, clients
// must manage it themselves.
//
// This behaviour is force disabled if WithDebug Option is used.
func WithCursorCorrections(enabled bool) Option {
	return func(cfg *viConfig) {
		cfg.cursorCorrections = enabled
	}
}

// WithAutoSkipNullCells determines whether vi should automatically
// shift the cursor on top a null cell (no content) in
// normal, yank, search, g and delete modes. Default is on.
//
// This behaviour is force disabled if WithDebug Option is used,
// or if WithCursorCorrections disables cursor corrections.
func WithAutoSkipNullCells(skip bool) Option {
	return func(cfg *viConfig) {
		cfg.skipNulls = skip
	}
}

package browser

import (
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

// DefaultConfig returns the default Config.
func DefaultConfig() Config {
	return Config{
		Tabspaces:           4,
		Logger:              nil,
		SwapDir:             "",
		Filepaths:           nil,
		RecoveryFilepath:    "",
		CommandEvent:        term.Event{Ch: ':', Type: term.EventKey},
		MessageBarAttr:      term.Attributes{Bg: term.ColorRed, Fg: term.ColorWhite},
		FocusTabAttr:        term.Attributes{Fg: term.ColorWhite},
		NonFocusTabAttr:     term.Attributes{Fg: term.ColorRed},
		StartTextAttr:       term.Attributes{Fg: term.ColorRed | term.AttrBold},
		FrameUnionCharSet:   component.DefaultFrameUnionCharSet(),
		WindowManagerConfig: component.DefaultWindowManagerConfig(),
	}
}

// Option represents a configuration option for a browser.Handler.
type Option func(*Config)

// TODO all this options should be moved to editor package for use with ex.
// Config holds configuration for an browser.Component.
type Config struct {
	Tabspaces        int
	Logger           *log.Logger
	SwapDir          string
	Filepaths        []string
	RecoveryFilepath string
	CommandEvent     term.Event
	StartText        string

	StartTextAttr           term.Attributes
	StartTextBackgroundAttr term.Attributes
	MessageBarAttr          term.Attributes
	FocusTabAttr            term.Attributes
	NonFocusTabAttr         term.Attributes

	component.FrameUnionCharSet
	component.WindowManagerConfig
}

// WithLogger sets a logger that the editor can use to log debugging data.
func WithLogger(l *log.Logger) Option {
	return func(cfg *Config) {
		cfg.Logger = l
	}
}

// WithStartText sets the starting buffer default text.
func WithStartText(text string) Option {
	return func(cfg *Config) {
		cfg.StartText = text
	}
}

// WithTabspaces sets the number of spaces used to render a tab.
func WithTabspaces(tabspaces int) Option {
	return func(cfg *Config) {
		cfg.Tabspaces = tabspaces
	}
}

// WithSwapDir defines the swap directory to use if WithFilepath option is set.
// The swap directory is used to keep persist recovery files. If this option is not
// defined, the directory of WithFilepath is used as a swap directory.
func WithSwapDir(dir string) Option {
	return func(cfg *Config) {
		cfg.SwapDir = dir
	}
}

// WithFrameUnionCharSet configures the characters used to draw the frame union
// between the browser tabs and the window manager.
func WithFrameUnionCharSet(cs component.FrameUnionCharSet) Option {
	return func(cfg *Config) {
		cfg.FrameUnionCharSet = cs
	}
}

// WithWindowManagerConfig returns an Option that defines
// the underlying's WindowManager initialization configuration.
// See handler.WindowManagerConfig for more info. If this option is not passed
// DefaultWindowManagerConfig is utilized.
func WithWindowManagerConfig(config component.WindowManagerConfig) Option {
	return func(cfg *Config) {
		cfg.WindowManagerConfig = config
	}
}

// WithRecoveryFile indicates that an Editor is to be initialized
// from recovery file swapFilePath. This option overrides WithSwapDir because
// the swap directory of swapFilePath is used instead.
func WithRecoveryFile(swapFilePath string) Option {
	return func(cfg *Config) {
		cfg.RecoveryFilepath = swapFilePath
	}
}

// WithFilepath returns an Option that sets the filepath of the file to open with
// a Editor handler.
func WithFilepath(filepath string) Option {
	return func(cfg *Config) {
		cfg.Filepaths = append(cfg.Filepaths, filepath)
	}
}

// WithCommandEvent returns an Option that defines what event triggers the editor's
// command mode.
func WithCommandEvent(event term.Event) Option {
	return func(cfg *Config) {
		cfg.CommandEvent = event
	}
}

// WithMessageBarAttr returns an Option that configures
// the browser's message bar attr.
func WithMessageBarAttr(attr term.Attributes) Option {
	return func(cfg *Config) {
		cfg.MessageBarAttr = attr
	}
}

// WithFocusTabAttr returns an Option that configures the attributes of the
// browser's tab in focus.
func WithFocusTabAttr(attr term.Attributes) Option {
	return func(cfg *Config) {
		cfg.FocusTabAttr = attr
	}
}

// WithNonFocusTabAttr returns an Option that configures the attributes of the
// browser's tabs that are not in focus.
func WithNonFocusTabAttr(attr term.Attributes) Option {
	return func(cfg *Config) {
		cfg.NonFocusTabAttr = attr
	}
}

// WithStartTextAttr returns an Option that configures the attributes of the
// text passed to WithStartText.
func WithStartTextAttr(attr term.Attributes) Option {
	return func(cfg *Config) {
		cfg.StartTextAttr = attr
	}
}

// WithStartTextBackgroundAttr returns an Option that configures the attributes of the
// padded background around text passed to WithStartText.
func WithStartTextBackgroundAttr(attr term.Attributes) Option {
	return func(cfg *Config) {
		cfg.StartTextBackgroundAttr = attr
	}
}

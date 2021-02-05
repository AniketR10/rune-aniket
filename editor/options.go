package editor

import (
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

// Config holds configuration for an browser.Component.
type Config struct {
	Tabspaces          int
	SwapDir            string
	Filepaths          []string
	RecoveryFilepath   string
	CommandEvent       term.Event
	CommandKeyBindings map[term.Event]string

	browser.Config
}

// DefaultConfig returns the default Config.
func DefaultConfig() Config {
	return Config{
		Tabspaces:          4,
		SwapDir:            "",
		Filepaths:          nil,
		RecoveryFilepath:   "",
		CommandEvent:       term.Event{Ch: ':', Type: term.EventKey},
		Config:             browser.DefaultConfig(),
		CommandKeyBindings: make(map[term.Event]string),
	}
}

// Option represents a configuration option for a browser.Handler.
type Option func(*Config)

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

// WithCommandKeyBinding maps key to issue cmd.
func WithCommandKeyBinding(ev term.Event, cmd string) Option {
	return func(cfg *Config) {
		if ev.Type != term.EventKey {
			panic("invalid command key binding")
		}
		sum := term.Event{Type: term.EventKey, Mod: ev.Mod, Ch: ev.Ch, Key: ev.Key}
		cfg.CommandKeyBindings[sum] = cmd
	}
}

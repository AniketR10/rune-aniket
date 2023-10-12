package text

import (
	"time"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

// CommandOverlayConfig holds configuration for the
// command's interface.
type CommandOverlayConfig struct {
	MatchedTextAttr  term.Attributes
	FocusElementAttr term.Attributes
	ElementAttr      term.Attributes
	ShowManualAfter  time.Duration
}

// Config holds configuration for an editor.Component.
type Config struct {
	Tabspaces               int
	Filepaths               []workspaceapi.URI
	RecoveryFilepath        workspaceapi.URI
	CommandEvent            term.KeyComb
	CommandMaxHistory       int
	CommandKeyBindings      map[term.KeyComb][]string
	CommandSequenceBindings map[handler.Sequence][]string
	CommandAliases          map[string][]string
	SequencerTimeout        time.Duration
	DirtyTabAttr            term.Attributes

	Interrupter   term.Interrupter
	SendEventNone func()

	CommandOverlay CommandOverlayConfig
	browser.Config
}

// DefaultCommandOverlayConfig returns the default Config's CommandOverlayConfig.
func DefaultCommandOverlayConfig() (cfg CommandOverlayConfig) {
	// NOTE: cannot use handler/command config: dependency cycle
	cfg.MatchedTextAttr = term.Attributes{Fg: term.ColorRed}
	cfg.FocusElementAttr = term.Attributes{Fg: term.AttrBold | term.AttrUnderline | term.ColorRed}
	cfg.ElementAttr = term.Attributes{}
	cfg.ShowManualAfter = 1 * time.Second
	return
}

// DefaultConfig returns the default Config.
func DefaultConfig() Config {
	cfg := Config{
		Tabspaces:               4,
		Filepaths:               nil,
		RecoveryFilepath:        workspaceapi.URI{},
		CommandEvent:            term.KeyComb{Ch: ':'},
		CommandMaxHistory:       2000,
		Config:                  browser.DefaultConfig(),
		DirtyTabAttr:            term.Attributes{Fg: term.AttrBold},
		CommandKeyBindings:      make(map[term.KeyComb][]string),
		CommandSequenceBindings: make(map[handler.Sequence][]string),
		CommandAliases:          make(map[string][]string),
		SequencerTimeout:        400 * time.Millisecond,
		CommandOverlay:          DefaultCommandOverlayConfig(),
		Interrupter:             term.NopInterrupter(),
		SendEventNone:           func() {},
	}
	return cfg
}

// Option represents a configuration option for Component.
type Option func(*Config)

// WithTabspaces sets the number of spaces used to render a tab.
func WithTabspaces(tabspaces int) Option {
	return func(cfg *Config) {
		cfg.Tabspaces = tabspaces
	}
}

// WithRecoveryFile indicates that an Editor is to be initialized
// from recovery file swapFilePath. This option overrides WithSwapDir because
// the swap directory of swapFilePath is used instead.
func WithRecoveryFile(swapFilePath workspaceapi.URI) Option {
	return func(cfg *Config) {
		cfg.RecoveryFilepath = swapFilePath
	}
}

// WithFilepath returns an Option that sets the filepath of the file to open with
// a Editor handler.
func WithFile(file workspaceapi.URI) Option {
	return func(cfg *Config) {
		cfg.Filepaths = append(cfg.Filepaths, file)
	}
}

// WithCommandKey returns an Option that defines what key triggers the editor's
// command mode.
func WithCommandKey(event term.KeyComb) Option {
	return func(cfg *Config) {
		cfg.CommandEvent = event
	}
}

// WithWallpaper sets the starting buffer default text wallpaper.
func WithWallpaper(text string) Option {
	return func(cfg *Config) {
		cfg.Wallpaper = text
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
func WithWindowManagerConfig(config handler.WindowManagerConfig) Option {
	return func(cfg *Config) {
		cfg.WindowManagerConfig = config
	}
}

// WithNotificationsConfig returns an Option that configures
// a Component's notifications.
func WithNotificationsConfig(c notifications.Config) Option {
	return func(cfg *Config) {
		cfg.Notifications = c
	}
}

// WithFocusTabAttr returns an Option that configures the attributes of a
// Components's tab in focus.
func WithFocusTabAttr(attr term.Attributes) Option {
	return func(cfg *Config) {
		cfg.FocusTabAttr = attr
	}
}

// WithNonFocusTabAttr returns an Option that configures the attributes of a
// Components's tabs that are not in focus.
func WithNonFocusTabAttr(attr term.Attributes) Option {
	return func(cfg *Config) {
		cfg.NonFocusTabAttr = attr
	}
}

// WithWallpaperAttr returns an Option that configures the attributes of the
// text passed to WithWallpaper.
func WithWallpaperAttr(attr term.Attributes) Option {
	return func(cfg *Config) {
		cfg.WallpaperAttr = attr
	}
}

// WithWallpaperBackgroundAttr returns an Option that configures the attributes of the
// padded background around text passed to WithWallpaper.
func WithWallpaperBackgroundAttr(attr term.Attributes) Option {
	return func(cfg *Config) {
		cfg.WallpaperBackgroundAttr = attr
	}
}

// WithCommandKeyBinding maps key to issue cmd.
func WithCommandKeyBinding(key term.KeyComb, cmdAndArgs []string) Option {
	return func(cfg *Config) {
		sum := term.KeyComb{Mod: key.Mod, Ch: key.Ch, Key: key.Key}
		cfg.CommandKeyBindings[sum] = cmdAndArgs
	}
}

// WithCommandMaxHistory sets the max command history to store for searching back through it.
func WithCommandMaxHistory(max int) Option {
	return func(cfg *Config) {
		cfg.CommandMaxHistory = max
	}
}

// WithCommandSequenceBinding configures an editor to trigger
// cmd when key sequence is pressed.
func WithCommandSequenceBinding(sequence handler.Sequence, cmdAndArgs []string) Option {
	return func(cfg *Config) {
		seq := handler.Sequence{
			First: term.KeyComb{
				Mod: sequence.First.Mod,
				Ch:  sequence.First.Ch,
				Key: sequence.First.Key,
			},
			Last: term.KeyComb{
				Mod: sequence.Last.Mod,
				Ch:  sequence.Last.Ch,
				Key: sequence.Last.Key,
			},
		}
		cfg.CommandSequenceBindings[seq] = cmdAndArgs
	}
}

// WithSequencerTimeout configures the time span during which two key events
// can be considered as a sequence.
func WithSequencerTimeout(t time.Duration) Option {
	return func(cfg *Config) {
		cfg.SequencerTimeout = t
	}
}

// WithDirtyTabAttr defines the attributes to use to indicate that
// the buffer in a tab has been modified but not flushed the changes to disk yet.
func WithDirtyTabAttr(attr term.Attributes) Option {
	return func(cfg *Config) {
		cfg.DirtyTabAttr = attr
	}
}

// WithCommandOverlayConfig defines the command overlay interface properties.
func WithCommandOverlayConfig(c CommandOverlayConfig) Option {
	return func(cfg *Config) {
		cfg.CommandOverlay = c
	}
}

// WithCommandAliases defines command aliases.
func WithCommandAliases(aliases map[string][]string) Option {
	return func(cfg *Config) {
		cfg.CommandAliases = aliases
	}
}

// WithPromptConfig sets the Components's prompt properties.
func WithPromptConfig(c browser.PromptConfig) Option {
	return func(cfg *Config) {
		cfg.Config.PromptConfig = c
	}
}

// WithInterrupter sets the Component's term.Interrupter.
func WithInterrupter(i term.Interrupter) Option {
	return func(cfg *Config) {
		cfg.Interrupter = i
	}
}

// WithSendNone sets the Component's send EventNone function.
// By default this is set to term.SendNoneEvent.
func WithSendNone(fn func()) Option {
	return func(cfg *Config) {
		cfg.SendEventNone = fn
	}
}

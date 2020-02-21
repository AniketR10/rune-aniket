package editor

import (
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

var defaultEditorConfig = editorConfig{
	Tabspaces:        4,
	WindowBorder:     true,
	Logger:           nil,
	SwapDir:          "",
	Filepath:         "",
	RecoveryFilepath: "",
	CommandEvent:     term.Event{Ch: ':', Type: term.EventKey},
}

// Option represents a configuration option for a Editor.
type Option func(*editorConfig)

// editorConfig holds configuration for an Editor.
type editorConfig struct {
	Tabspaces        int
	Logger           *log.Logger
	SwapDir          string
	WindowBorder     bool
	Filepath         string
	RecoveryFilepath string
	CommandEvent     term.Event
}

// WithLogger sets a logger that the editor can use to log debugging data.
func WithLogger(l *log.Logger) Option {
	return func(cfg *editorConfig) {
		cfg.Logger = l
	}
}

// WithTabspaces sets the number of spaces used to render a tab.
func WithTabspaces(tabspaces int) Option {
	return func(cfg *editorConfig) {
		cfg.Tabspaces = tabspaces
	}
}

// WithSwapDir defines the swap directory to use if WithFilepath option is set.
// The swap directory is used to keep persist recovery files. If this option is not
// defined, the directory of WithFilepath is used as a swap directory.
func WithSwapDir(dir string) Option {
	return func(cfg *editorConfig) {
		cfg.SwapDir = dir
	}
}

// WithWindowBorder is an Option that defines whether WindowManager should draw
// windows with a border or not.
func WithWindowBorder(border bool) Option {
	return func(cfg *editorConfig) {
		cfg.WindowBorder = border
	}
}

// WithRecoveryFile indicates that an Editor is to be initialized
// from recovery file swapFilePath. This option overrides WithSwapDir because
// the swap directory of swapFilePath is used instead.
func WithRecoveryFile(swapFilePath string) Option {
	return func(cfg *editorConfig) {
		cfg.RecoveryFilepath = swapFilePath
	}
}

// WithFilepath is an Option that sets the filepath of the file to open with
// a Editor handler.
func WithFilepath(filepath string) Option {
	return func(cfg *editorConfig) {
		cfg.Filepath = filepath
	}
}

// WithCommandEvent is an Option that defines what event triggers the editor's
// command mode.
func WithCommandEvent(event term.Event) Option {
	return func(cfg *editorConfig) {
		cfg.CommandEvent = event
	}
}

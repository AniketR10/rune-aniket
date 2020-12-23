package browser

import (
	"github.com/ernestrc/go-tui/handler"
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
		WindowManagerConfig: handler.DefaultWindowManagerConfig(),
	}
}

// Option represents a configuration option for a browser.Handler.
type Option func(*Config)

// Config holds configuration for an browser.Component.
type Config struct {
	Tabspaces        int
	Logger           *log.Logger
	SwapDir          string
	Filepaths        []string
	RecoveryFilepath string
	CommandEvent     term.Event
	StartText        string
	handler.WindowManagerConfig
}

// WithLogger sets a logger that the editor can use to log debugging data.
func WithLogger(l *log.Logger) Option {
	return func(cfg *Config) {
		cfg.Logger = l
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

// WithWindowManagerConfig is an Option that defines
// the underlying's WindowManager initialization configuration.
// See handler.WindowManagerConfig for more info. If this option is not passed
// DefaultWindowManagerConfig is utilized.
func WithWindowManagerConfig(config handler.WindowManagerConfig) Option {
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

// WithFilepath is an Option that sets the filepath of the file to open with
// a Editor handler.
func WithFilepath(filepath string) Option {
	return func(cfg *Config) {
		cfg.Filepaths = append(cfg.Filepaths, filepath)
	}
}

// WithCommandEvent is an Option that defines what event triggers the editor's
// command mode.
func WithCommandEvent(event term.Event) Option {
	return func(cfg *Config) {
		cfg.CommandEvent = event
	}
}

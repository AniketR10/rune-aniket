package vi

import (
	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/editor"
	"github.com/ernestrc/fractal/term"
	log "github.com/sirupsen/logrus"
)

// viConfig holds configuration for Vi.
type viConfig struct {
	Filepath         string
	Buffer           *cell.Buffer
	ResAttr          term.Attributes
	Tabspaces        int
	Clipboard        editor.Clipboard
	Logger           *log.Logger
	SwapDir          string
	RecoverySwapFile string
}

// Option represents a Vi handler configuration option.
type Option func(*viConfig)

// WithSwapDir defines the swap directory to use if WithFilepath option is set.
// The swap directory is used to keep persist recovery files. If this option is not
// defined, the directory of WithFilepath is used as a swap directory.
func WithSwapDir(dir string) Option {
	return func(cfg *viConfig) {
		cfg.SwapDir = dir
	}
}

// WithRecoveryFile indicates that a Vi handler is to be initialized
// from recovery file swapFilePath. This option overrides WithSwapDir because
// the swap directory of swapFilePath is used instead.
func WithRecoveryFile(swapFilePath string) Option {
	return func(cfg *viConfig) {
		cfg.RecoverySwapFile = swapFilePath
	}
}

// WithLogger sets the editor.Clipboard implementation to use.
func WithLogger(l *log.Logger) Option {
	return func(cfg *viConfig) {
		cfg.Logger = l
	}
}

// WithClipboard sets the editor.Clipboard implementation to use.
func WithClipboard(clip editor.Clipboard) Option {
	return func(cfg *viConfig) {
		cfg.Clipboard = clip
	}
}

// WithTabspaces sets the number of spaces used to render a tab.
func WithTabspaces(tabspaces int) Option {
	return func(cfg *viConfig) {
		cfg.Tabspaces = tabspaces
	}
}

// WithResAttr sets the search result cell attributes to be rendered.
func WithResAttr(attr term.Attributes) Option {
	return func(cfg *viConfig) {
		cfg.ResAttr = attr
	}
}

// WithBuffer is a Option that sets the buffer to use with
// Vi handler. If this option is set, note that it overrides
// WithFilepath so some functions (like Saving to disk) will be disabled.
// This option also overrides WithRecoveryFile.
func WithBuffer(buf *cell.Buffer) Option {
	return func(cfg *viConfig) {
		cfg.Buffer = buf
	}
}

// WithFilepath is a Option that sets the filepath of the file to open with
// a Vi handler.
func WithFilepath(filepath string) Option {
	return func(cfg *viConfig) {
		cfg.Filepath = filepath
	}
}

package vte

import (
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/clipboard"
)

// DefaultConfig returns a sane default Config.
func DefaultConfig() Config {
	return Config{
		Clipboard: clipboard.NewInMemory(),
	}
}

// Config configures Handler.
type Config struct {
	// Shell is the default shell to use. Otherwise whatever is set
	// on the $SHELL environment variable is used.
	Shell     string
	Clipboard clipboard.Register
	Watcher   workspaceapi.Watcher

	Attributes          term.Attributes
	SelectionAttributes term.Attributes

	// WidthHint and HeightHint hint allows emulator.Handler to better configure the
	// initial buffer size.
	WidthHint  int
	HeightHint int
}

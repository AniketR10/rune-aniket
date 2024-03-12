package vte

import (
	"github.com/ernestrc/tcell/v3"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/clipboard"
)

// DefaultConfig returns a sane default Config.
func DefaultConfig() Config {
	return Config{
		Clipboard:                clipboard.NewInMemory(),
		Bell:                     func() {},
		SelectionAttributes:      term.Attributes{Attrs: tcell.AttrReverse},
		NeedsAttentionAttributes: term.Attributes{Attrs: tcell.AttrBlink},
	}
}

// Config configures Handler.
type Config struct {
	// Shell is the default shell to use. Otherwise whatever is set
	// on the $SHELL environment variable is used.
	Shell     string
	Clipboard clipboard.Register
	Bell      func()
	Watcher   workspaceapi.Watcher

	Attributes               term.Attributes
	SelectionAttributes      term.Attributes
	NeedsAttentionAttributes term.Attributes

	// WidthHint and HeightHint hint allows emulator.Handler to better configure the
	// initial buffer size.
	WidthHint  int
	HeightHint int
}

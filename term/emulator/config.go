package emulator

import (
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/term"
)

// Config configures Handler.
type Config struct {
	// Shell is the default shell to use. Otherwise whatever is set
	// on the $SHELL environment variable is used.
	Shell string

	Watcher workspaceapi.Watcher

	Attributes          term.Attributes
	SelectionAttributes term.Attributes

	// WidthHint and HeightHint hint allows emulator.Handler to better configure the
	// initial buffer size.
	WidthHint  int
	HeightHint int
}

package emulator

import "unstable.build/go-tui/term"

// Config configures Handler.
type Config struct {
	// Shell is the default shell to use. Otherwise whatever is set
	// on the $SHELL environment variable is used.
	Shell string

	Attributes          term.Attributes
	SelectionAttributes term.Attributes
}

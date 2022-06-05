package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

type nop struct{}

// Nop returns a tui.Component that does nothing.
func Nop() tui.Component {
	return nop{}
}

func (n nop) Resize(width, height int) {
}

func (n nop) Draw(term.Writer) {
}

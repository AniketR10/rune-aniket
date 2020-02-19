package editor

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
)

// Editor is the interface that wraps the method Edit.
//
// Edit opens a file and returns a tui.Handler to edit it or an error
// if there was an error opening it.
type Editor interface {
	Edit(buf *cell.Buffer) tui.Handler
}

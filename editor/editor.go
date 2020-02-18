package editor

import (
	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/cell"
)

// Editor is the interface that wraps the method Edit.
//
// Edit opens a file and returns a fractal.Handler to edit it or an error
// if there was an error opening it.
type Editor interface {
	Edit(buf *cell.Buffer) fractal.Handler
}

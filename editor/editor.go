package editor

//go:generate mockgen -destination=./editor_gomock.go -package editor -self_package editor -source editor.go

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
)

// Editor is the interface that wraps an API to manage a text editor.
type Editor interface {
	// Edit opens a file and returns a tui.Handler to edit it or an error
	// if there was an error opening it.
	Edit(name string, buf *cell.Buffer) (tui.Handler, error)
}

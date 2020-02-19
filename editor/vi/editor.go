package vi

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor"
)

type viEditor struct {
	opts []Option
}

// Editor returns a Vi editor.Editor.
func Editor(opts ...Option) editor.Editor {
	return &viEditor{opts: opts}
}

func (e *viEditor) Edit(buf *cell.Buffer) tui.Handler {
	return New(buf, e.opts...)
}

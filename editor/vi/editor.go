package vi

import (
	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/editor"
)

type viEditor struct {
	opts []Option
}

// Editor returns a Vi editor.Editor.
func Editor(opts ...Option) editor.Editor {
	return &viEditor{opts: opts}
}

func (e *viEditor) Edit(buf *cell.Buffer) fractal.Handler {
	return New(buf, e.opts...)
}

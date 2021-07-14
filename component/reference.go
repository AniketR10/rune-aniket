package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// Reference can be used to dynamically swap a tui.Component. If Component is nil,
// then Resize and Draw do nothing.
type Reference struct {
	Component tui.Component
}

func (r *Reference) Resize(width, height int) {
	if r.Component == nil {
		return
	}
	r.Component.Resize(width, height)
}

func (r *Reference) Draw(w term.Writer) {
	if r.Component == nil {
		return
	}
	r.Component.Draw(w)
}

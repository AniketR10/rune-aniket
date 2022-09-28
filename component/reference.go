package component

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

var _ tui.Component = (*Reference)(nil)

// Reference can be used to dynamically swap a tui.Component.
// If the underlying tui.Component is nil, then Resize and Draw do nothing.
type Reference struct {
	component tui.Component
	dirty     bool
	height    int
	width     int
}

// NewReference allocates storage for a new Reference and initializes it with ref.
func NewReference(ref tui.Component) *Reference {
	ret := new(Reference)
	ret.Init(ref)
	return ret
}

// Init initializes this reference with ref.
// It can be used subsequently to override the underlying tui.Component reference.
func (r *Reference) Init(ref tui.Component) {
	r.component = ref
	r.dirty = true
}

func (r *Reference) Resize(width, height int) {
	r.height, r.width = height, width
	if r.component == nil {
		return
	}
	r.dirty = false
	r.component.Resize(width, height)
}

func (r *Reference) Draw(w term.Writer) {
	if r.component == nil {
		return
	}
	if r.dirty {
		r.Resize(r.width, r.height)
	}
	r.component.Draw(w)
}

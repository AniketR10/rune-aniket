package handler

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
)

// AttrSetter provides an API like component.AttrSetter for a tui.Handler.
type AttrSetter struct {
	comp *component.AttrSetter
	tui.Handler
}

// WithAttrSetter wraps h with a component.AttrSetter.
func WithAttrSetter(h tui.Handler) AttrSetter {
	return AttrSetter{
		comp:    component.WithAttrSetter(h),
		Handler: h,
	}
}

// Resize satisfies tui.Component
func (s AttrSetter) Resize(width, height int) {
	s.comp.Resize(width, height)
}

// Draw satisfies tui.Component
func (s AttrSetter) Draw(w term.Writer) {
	s.comp.Draw(w)
}

// Reset resets all the previous calls to SetAttrAt and SetAttr.
func (s AttrSetter) Reset() {
	s.comp.Reset()
}

// SetAttr sets the default attributes drawn by this component.
// This attributes will be set in each of the final term.Cells drawn.
func (s AttrSetter) SetAttr(attr term.Attributes) {
	s.comp.SetAttr(attr)
}

// SetAttrAt sets the attributes at the given coordinates.
// If coordinates are out of bounds, this method will NOT panic, but
// the next call to Draw will.
func (s AttrSetter) SetAttrAt(at term.Coordinates, attr term.Attributes) {
	s.comp.SetAttrAt(at, attr)
}

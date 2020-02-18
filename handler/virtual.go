package handler

import (
	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
	"github.com/ernestrc/fractal/term"
)

// Virtual wraps another fractal.Handler to provide cursor event coordinates.
type Virtual struct {
	component.Virtual
}

// Handle fractal.Handler
func (v *Virtual) Handle(ev term.Event) bool {
	if ev.Type == term.EventMouse {
		offset := v.Position()
		ev.MouseX -= offset.X
		ev.MouseY -= offset.Y
	}
	return v.C.(fractal.Handler).Handle(ev)
}

// Cursor fractal.Handler
func (v *Virtual) Cursor() (pos term.Coordinates, show bool) {
	pos, show = v.C.(fractal.Handler).Cursor()
	offset := v.Position()
	pos.X += offset.X
	pos.Y += offset.Y
	return
}

// Man fractal.Handler
func (v *Virtual) Man() fractal.Manual {
	return v.C.(fractal.Handler).Man()
}

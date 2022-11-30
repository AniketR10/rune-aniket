package handler

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/component"
)

// Floating handlers are not in principle confined to a predetermined
// space and so are allowed certain degree of freedom.
// See component.Floating for more details.
type Floating interface {
	tui.Handler
	component.Floating
}

// StaticFloating wraps a tui.Handler and returns a Floating that always
// return the same Dimensions values.
func StaticFloating(h tui.Handler, width, height int) Floating {
	return staticFloating{width: width, height: height, Handler: h}
}

type staticFloating struct {
	tui.Handler
	width, height int
}

func (s staticFloating) Dimensions() (int, int) {
	return s.width, s.height
}

// PaddedFloating wraps a Floating component and adds a pre-determined
// amount of x axis and y axis padding.
func PaddedFloating(f Floating, padx, pady int) Floating {
	return paddedFloating{padx: padx, pady: pady, Floating: f}
}

type paddedFloating struct {
	Floating
	padx, pady int
}

func (p paddedFloating) Dimensions() (width, height int) {
	width, height = p.Floating.Dimensions()
	width += p.padx
	height += p.pady
	return
}

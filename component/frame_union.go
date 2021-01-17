package component

import (
	"github.com/ernestrc/go-tui/term"
)

// FrameUnionCharSet configures the characters used to draw the frame union
// between the top and bottom components.
type FrameUnionCharSet struct {
	Left  rune
	Right rune
}

// DefaultFrameUnionCharSet returns the default FrameUnionCharSet used.
func DefaultFrameUnionCharSet() (ret FrameUnionCharSet) {
	ret.Left = '├'
	ret.Right = '┤'
	return
}

// FrameUnion is a tui.Component which Draws two other (Frame) components
// one on top of each other. It respects the top components height
// and draws the second component below such that the two frames are
// connected and that they fit within the given space.
type FrameUnion struct {
	top, bottom   *Virtual
	height, width int
	secondY       int

	term.Attributes
	FrameUnionCharSet
}

// NewFrameUnion allocates storage for a new FrameUnion and initializes it.
func NewFrameUnion(top, bottom *Virtual) *FrameUnion {
	ret := new(FrameUnion)
	ret.Init(top, bottom)
	return ret
}

// Init initializes this frame union with top and bottom Virtual components.
func (u *FrameUnion) Init(top, bottom *Virtual) {
	u.FrameUnionCharSet = DefaultFrameUnionCharSet()
	u.top, u.bottom = top, bottom
}

// Resize satisfies tui.Component
func (u *FrameUnion) Resize(width, height int) {
	u.height, u.width = height, width

	firstHeight := u.top.Height()

	var secondHeight int
	if firstHeight > 0 {
		secondHeight = height - firstHeight + 1
	} else {
		secondHeight = height
	}
	u.secondY = height - secondHeight

	u.top.Resize(width, firstHeight)
	u.bottom.Resize(width, secondHeight)

	u.bottom.Move(term.Coordinates{Y: u.secondY})
}

// Draw satisfies tui.Component
func (u *FrameUnion) Draw(w term.Writer) {
	u.top.Draw(w)
	u.bottom.Draw(w)
	if u.height >= 3 || u.width >= 3 {
		w.SetCell(term.Coordinates{Y: u.secondY},
			term.Cell{Ch: u.Left, Bg: u.Attributes.Bg, Fg: u.Attributes.Fg})
		if u.width > 0 {
			w.SetCell(term.Coordinates{X: u.width - 1, Y: u.secondY},
				term.Cell{Ch: u.Right, Bg: u.Attributes.Bg, Fg: u.Attributes.Fg})
		}
	}
}

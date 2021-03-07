package component

import (
	"github.com/ernestrc/go-tui/term"
)

// FrameUnionCharSet configures the characters used to draw the frame union
// between the top and bottom components.
type FrameUnionCharSet struct {
	Left   rune
	Right  rune
	Top    rune
	Bottom rune
}

// DefaultFrameUnionCharSet returns the default FrameUnionCharSet used.
func DefaultFrameUnionCharSet() (ret FrameUnionCharSet) {
	ret.Left = '├'
	ret.Right = '┤'
	ret.Top = '┬'
	ret.Bottom = '┴'
	return
}

// FrameUnion is a tui.Component which Draws a main Component
// at the max available width and height, but otherwise depending on
// how many other statically sized components are stacked either left, right
// top or bottom.
//
// It respects the stacked components height if stacked on top or bottom
// and it respects the stacked components width if stacked left or right.
// The API uses Virtual instead of tui.Component to determine what's the desired
// height or width.
type FrameUnion struct {
	main          *Virtual
	top, bottom   []*Virtual
	left, right   []*Virtual
	height, width int
	frame         bool

	term.Attributes
	FrameUnionCharSet
}

// NewFrameUnion allocates storage for a new FrameUnion and initializes it.
func NewFrameUnion(main *Virtual, frame bool) *FrameUnion {
	ret := new(FrameUnion)
	ret.Init(main, frame)
	return ret
}

// Init initializes this frame union with top and bottom Virtual components.
func (u *FrameUnion) Init(main *Virtual, frame bool) {
	u.FrameUnionCharSet = DefaultFrameUnionCharSet()
	u.main = main
	u.frame = frame
}

// UnionTop stacks top on top of the main component. This
// method panics if top is nil.
func (u *FrameUnion) UnionTop(top *Virtual) {
	if top == nil {
		panic("invalid componen.Virtual")
	}
	u.top = append(u.top, top)
}

// UnionBottom stacks bottom under of the main component. This
// method panics if bottom is nil.
func (u *FrameUnion) UnionBottom(bottom *Virtual) {
	if bottom == nil {
		panic("invalid componen.Virtual")
	}
	u.bottom = append([]*Virtual{bottom}, u.bottom...)
}

// UnionLeft stacks left to the left of the main component. This
// method panics if left is nil.
func (u *FrameUnion) UnionLeft(left *Virtual) {
	if left == nil {
		panic("invalid componen.Virtual")
	}
	u.left = append(u.left, left)
}

// UnionRight stacks right to the right of the main component. This
// method panics if right is nil.
func (u *FrameUnion) UnionRight(right *Virtual) {
	if right == nil {
		panic("invalid componen.Virtual")
	}
	u.right = append([]*Virtual{right}, u.right...)
}

func (u *FrameUnion) resizeTopBottom(width, height int) (int, int) {
	var frameOverlap int
	if u.frame {
		frameOverlap = 1
	}

	var topHeight int
	for _, top := range u.top {
		top.Move(term.Coordinates{Y: topHeight})

		height := top.Height()
		if height > 0 {
			topHeight += height - frameOverlap
			top.C.Resize(width, height)
		} else {
			top.C.Resize(0, 0)
		}
	}

	var bottomHeight int
	for _, bottom := range u.bottom {
		height := bottom.Height()
		if height > 0 {
			bottomHeight += height - frameOverlap
		}
	}

	// if there's too many union components for available height
	// do not draw them.
	mainHeight := u.height - topHeight - bottomHeight
	if mainHeight < 1 {
		for _, top := range u.top {
			top.C.Resize(0, 0)
		}
		for _, bottom := range u.bottom {
			bottom.C.Resize(0, 0)
		}
		return u.height, 0
	}

	bottomOffset := topHeight + mainHeight - frameOverlap
	for _, bottom := range u.bottom {
		bottom.Move(term.Coordinates{Y: bottomOffset})
		height := bottom.Height()
		if height > 0 {
			bottom.C.Resize(width, height)
			bottomOffset += height - frameOverlap
		} else {
			bottom.C.Resize(0, 0)
		}
	}

	return mainHeight, topHeight
}

func (u *FrameUnion) resizeLeftRight(width, height, topOffset int) (int, int) {
	var frameOverlap int
	if u.frame {
		frameOverlap = 1
	}

	var leftWidth int
	for _, left := range u.left {
		left.Move(term.Coordinates{X: leftWidth, Y: topOffset})

		width := left.Width()
		if width > 0 {
			leftWidth += width - frameOverlap
			left.C.Resize(width, height)
		} else {
			left.C.Resize(0, 0)
		}
	}

	var rightWidth int
	for _, right := range u.right {
		width := right.Width()
		if width > 0 {
			rightWidth += width - frameOverlap
		}
	}

	mainWidth := u.width - leftWidth - rightWidth
	if mainWidth < 1 {
		for _, left := range u.left {
			left.C.Resize(0, 0)
		}
		for _, right := range u.right {
			right.C.Resize(0, 0)
		}
		return u.width, 0
	}

	rightOffset := leftWidth + mainWidth - frameOverlap
	for _, right := range u.right {
		right.Move(term.Coordinates{Y: topOffset, X: rightOffset})
		width := right.Width()
		if width > 0 {
			right.C.Resize(width, height)
			rightOffset += width - frameOverlap
		} else {
			right.C.Resize(0, 0)
		}
	}

	return mainWidth, leftWidth
}

// Resize satisfies tui.Component
func (u *FrameUnion) Resize(width, height int) {
	u.height, u.width = height, width

	mainHeight, topOffset := u.resizeTopBottom(width, height)
	mainWidth, leftOffset := u.resizeLeftRight(width, mainHeight, topOffset)

	u.main.Resize(mainWidth, mainHeight)
	u.main.Move(term.Coordinates{Y: topOffset, X: leftOffset})
}

func (u *FrameUnion) setVerticalUnionFrameCells(w term.Writer, y int) {
	w.SetCell(term.Coordinates{Y: y},
		term.Cell{Ch: u.Left, Bg: u.Attributes.Bg, Fg: u.Attributes.Fg})
	if u.width > 0 {
		w.SetCell(term.Coordinates{X: u.width - 1, Y: y},
			term.Cell{Ch: u.Right, Bg: u.Attributes.Bg, Fg: u.Attributes.Fg})
	}
}

func (u *FrameUnion) setHorizontalUnionFrameCells(w term.Writer, height, x, y int) {
	w.SetCell(term.Coordinates{X: x, Y: y},
		term.Cell{Ch: u.Top, Bg: u.Attributes.Bg, Fg: u.Attributes.Fg})
	if height > 0 {
		w.SetCell(term.Coordinates{X: x, Y: y + height - 1},
			term.Cell{Ch: u.Bottom, Bg: u.Attributes.Bg, Fg: u.Attributes.Fg})
	}
}

// Draw satisfies tui.Component
func (u *FrameUnion) Draw(w term.Writer) {
	for _, top := range u.top {
		top.Draw(w)
	}
	for _, bottom := range u.bottom {
		bottom.Draw(w)
	}
	for _, left := range u.left {
		left.Draw(w)
	}
	for _, right := range u.right {
		right.Draw(w)
	}

	u.main.Draw(w)
	pos := u.main.Position()
	if u.main.Height() < 2 || u.main.Width() < 2 || pos.X == 0 && pos.Y == 0 || !u.frame {
		return
	}

	for _, left := range u.left {
		pos := left.Position()
		u.setHorizontalUnionFrameCells(w, u.main.Height(), pos.X+left.Width()-1, pos.Y)
	}

	for _, right := range u.right {
		pos := right.Position()
		u.setHorizontalUnionFrameCells(w, u.main.Height(), pos.X, pos.Y)
	}
	for _, top := range u.top {
		u.setVerticalUnionFrameCells(w, top.Position().Y+top.Height()-1)
	}

	for _, bottom := range u.bottom {
		u.setVerticalUnionFrameCells(w, bottom.Position().Y)
	}
}

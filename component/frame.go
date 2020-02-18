package component

import (
	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/term"
)

// Frame is a Component that simply draws a border around a nested component.
// By default the frame adds some padding around the component by using
// the following cells:
//
// f.Horizontal = term.Cell{Ch: '─'}
// f.Vertical = term.Cell{Ch: '│'}
// f.TopLeft = term.Cell{Ch: '┌'}
// f.TopRight = term.Cell{Ch: '┐'}
// f.BottomLeft = term.Cell{Ch: '└'}
// f.BottomRight = term.Cell{Ch: '┘'}
//
// Note that this component can achieve other effects (highlight, frame)
// by setting the rigth cell characters and/or attributes.
//
// Example frame characters (ASCII 9472-9580):
//
//   '─', '━', '│', '┃', '┄', '┅', '┆', '┇', '┈', '┉', '┊', '┋', '┌', '┍',
//   '┎', '┏', '┐', '┑', '┒', '┓', '└', '┕', '┖', '┗', '┘', '┙', '┚', '┛',
//   '├', '┝', '┞', '┟', '┠', '┡', '┢', '┣', '┤', '┥', '┦', '┧', '┨', '┩',
//   '┪', '┫', '┬', '┭', '┮', '┯', '┰', '┱', '┲', '┳', '┴', '┵', '┶', '┷',
//   '┸', '┹', '┺', '┻', '┼', '┽', '┾', '┿', '╀', '╁', '╂', '╃', '╄', '╅',
//   '╆', '╇', '╈', '╉', '╊', '╋', '╌', '╍', '╎', '╏', '═', '║', '╒', '╓',
//   '╔', '╕', '╖', '╗', '╘', '╙', '╚', '╛', '╜', '╝', '╞', '╟', '╠', '╡',
//   '╢', '╣', '╤', '╥', '╦', '╧', '╨', '╩'
type Frame struct {
	TopLeft, TopRight, BottomLeft, BottomRight term.Cell
	Horizontal, Vertical                       term.Cell

	content         Virtual
	bwidth, bheight int
	width, height   int
}

// NewFrame allocates storage and initializes a new frame with the given
// border attributes and underlying component.
func NewFrame(content fractal.Component) (f *Frame) {
	f = new(Frame)
	f.Init(content)
	return
}

// Init initializes this frame with the given Component and border attributes.
func (f *Frame) Init(content fractal.Component) {
	f.content.C = content
	f.Horizontal.Ch = '─'
	f.Vertical.Ch = '│'
	f.TopLeft.Ch = '┌'
	f.TopRight.Ch = '┐'
	f.BottomLeft.Ch = '└'
	f.BottomRight.Ch = '┘'
}

// SetAttr updates the border attributes of this Frame.
func (f *Frame) SetAttr(border term.Attributes) {
	f.Horizontal.Fg = border.Fg
	f.Horizontal.Bg = border.Bg
	f.Vertical.Fg = border.Fg
	f.Vertical.Bg = border.Bg
	f.TopLeft.Fg = border.Fg
	f.TopLeft.Bg = border.Bg
	f.TopRight.Fg = border.Fg
	f.TopRight.Bg = border.Bg
	f.BottomLeft.Fg = border.Fg
	f.BottomLeft.Bg = border.Bg
	f.BottomRight.Fg = border.Fg
	f.BottomRight.Bg = border.Bg
}

// Content returns the underlying Component.
func (f *Frame) Content() fractal.Component {
	return f.content.C
}

// SetContent updates the underlying component and resizes it
// to conform to this frame's width and height.
func (f *Frame) SetContent(content fractal.Component) {
	f.content.C = content
	f.Resize(f.width, f.height)
}

// Resize updates this frame with a new width and height. If width or height
// is smaller than 3 cells, the border will not be drawn.
func (f *Frame) Resize(width, height int) {
	// deactivate frame if there's not space for content
	if width < 3 || height < 3 {
		f.bwidth, f.bheight = 0, 0
	} else {
		f.bwidth, f.bheight = 2, 2
	}

	alignContent(&f.content, width, height, f.bwidth, f.bheight, DefaultSpanFlags)
	f.width, f.height = width, height
}

// Draw draws this frame's border and contents to the given Writer.
func (f *Frame) Draw(w fractal.Writer) {
	limitX, limitY := f.width-1, f.height-1

	for i := 0; i < limitX; i++ {
		w.SetCell(term.Coordinates{X: i, Y: 0}, f.Horizontal)
		w.SetCell(term.Coordinates{X: i, Y: limitY}, f.Horizontal)
	}

	for i := 0; i < limitY; i++ {
		w.SetCell(term.Coordinates{X: 0, Y: i}, f.Vertical)
		w.SetCell(term.Coordinates{X: limitX, Y: i}, f.Vertical)
	}

	w.SetCell(term.Coordinates{X: 0, Y: 0}, f.TopLeft)

	w.SetCell(term.Coordinates{X: limitX, Y: 0}, f.TopRight)

	w.SetCell(term.Coordinates{X: 0, Y: limitY}, f.BottomLeft)

	w.SetCell(term.Coordinates{X: limitX, Y: limitY}, f.BottomRight)

	f.content.Draw(w)
}

// ContentPosition returns the position of the content inside this frame.
func (f *Frame) ContentPosition() term.Coordinates {
	return calculateContentOffset(f.bwidth, f.bheight, DefaultSpanFlags)
}

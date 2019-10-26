package component

import (
	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/term"
)

// Frame is a Component that simply draws a border around a nested component.
type Frame struct {
	term.Attributes
	content         VirtualComponent
	bwidth, bheight int
	width, height   int
}

// NewFrame allocates storage and initializes a new frame with the given
// border attributes and underlying component.
func NewFrame(content fractal.Component, border term.Attributes) (f *Frame) {
	f = new(Frame)
	f.Init(content, border)
	return
}

// Init initializes this frame with the given Component and border attributes.
func (f *Frame) Init(content fractal.Component, border term.Attributes) {
	f.Attributes = border
	f.content.C = content
}

// SetAttr updates the border attributes of this Frame.
func (f *Frame) SetAttr(border term.Attributes) {
	f.Attributes = border
}

// Content returns the underlying Component.
func (f *Frame) Content() fractal.Component {
	return &f.content
}

// SetContent updates the underlying component and resizes it
// to conform to this frame's width and height.
func (f *Frame) SetContent(content fractal.Component) (err error) {
	f.content.C = content
	f.Resize(f.width, f.height)
	return
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

	offset := f.getContentOffset()
	contentWidth := width - f.bwidth
	contentHeight := height - f.bheight
	f.content.Resize(contentWidth, contentHeight)
	f.content.Move(offset)
	f.width, f.height = width, height
}

func (f *Frame) getContentOffset() term.Coordinates {
	return term.Coordinates{X: f.bwidth / 2, Y: f.bheight / 2}
}

// Draw draws this frame's border and contents to the given Writer.
func (f *Frame) Draw(w fractal.Writer) {
	if f.bwidth == 0 || f.bheight == 0 {
		f.content.Draw(w)
	}

	limitX, limitY := f.width-1, f.height-1

	for i := 0; i < limitX; i++ {
		cell := term.Cell{Ch: '─', Fg: f.Fg, Bg: f.Bg}
		w.SetCell(term.Coordinates{X: i, Y: 0}, cell)
		w.SetCell(term.Coordinates{X: i, Y: limitY}, cell)
	}

	for i := 0; i < limitY; i++ {
		cell := term.Cell{Ch: '│', Fg: f.Fg, Bg: f.Bg}
		w.SetCell(term.Coordinates{X: 0, Y: i}, cell)
		w.SetCell(term.Coordinates{X: limitX, Y: i}, cell)
	}

	w.SetCell(term.Coordinates{X: 0, Y: 0},
		term.Cell{Ch: '┌', Fg: f.Fg, Bg: f.Bg})

	w.SetCell(term.Coordinates{X: limitX, Y: 0},
		term.Cell{Ch: '┐', Fg: f.Fg, Bg: f.Bg})

	w.SetCell(term.Coordinates{X: 0, Y: limitY},
		term.Cell{Ch: '└', Fg: f.Fg, Bg: f.Bg})

	w.SetCell(term.Coordinates{X: limitX, Y: limitY},
		term.Cell{Ch: '┘', Fg: f.Fg, Bg: f.Bg})

	f.content.Draw(w)
}

// ContentPosition returns the position of the content inside this frame.
func (f *Frame) ContentPosition() term.Coordinates {
	return f.getContentOffset()
}

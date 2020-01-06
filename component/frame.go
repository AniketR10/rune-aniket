package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// FrameCharSet is a struct used to store the set of characters used to
// draw a frame.
//
// Example characters (ASCII 9472-9580):
//
//   '─', '━', '│', '┃', '┄', '┅', '┆', '┇', '┈', '┉', '┊', '┋', '┌', '┍',
//
//   '┎', '┏', '┐', '┑', '┒', '┓', '└', '┕', '┖', '┗', '┘', '┙', '┚', '┛',
//
//   '├', '┝', '┞', '┟', '┠', '┡', '┢', '┣', '┤', '┥', '┦', '┧', '┨', '┩',
//
//   '┪', '┫', '┬', '┭', '┮', '┯', '┰', '┱', '┲', '┳', '┴', '┵', '┶', '┷',
//
//   '┸', '┹', '┺', '┻', '┼', '┽', '┾', '┿', '╀', '╁', '╂', '╃', '╄', '╅',
//
//   '╆', '╇', '╈', '╉', '╊', '╋', '╌', '╍', '╎', '╏', '═', '║', '╒', '╓',
//
//   '╔', '╕', '╖', '╗', '╘', '╙', '╚', '╛', '╜', '╝', '╞', '╟', '╠', '╡',
//
//   '╢', '╣', '╤', '╥', '╦', '╧', '╨', '╩'
type FrameCharSet struct {
	TopLeft, TopRight       term.Cell
	BottomLeft, BottomRight term.Cell
	Horizontal, Vertical    term.Cell
}

// WithAttr sets cs term.Attributes.
func (cs FrameCharSet) WithAttr(attr term.Attributes) FrameCharSet {
	cs.Horizontal.Fg = attr.Fg
	cs.Horizontal.Bg = attr.Bg
	cs.Vertical.Fg = attr.Fg
	cs.Vertical.Bg = attr.Bg
	cs.TopLeft.Fg = attr.Fg
	cs.TopLeft.Bg = attr.Bg
	cs.TopRight.Fg = attr.Fg
	cs.TopRight.Bg = attr.Bg
	cs.BottomLeft.Fg = attr.Fg
	cs.BottomLeft.Bg = attr.Bg
	cs.BottomRight.Fg = attr.Fg
	cs.BottomRight.Bg = attr.Bg
	return cs
}

// FrameCharSetDefault returns the default FrameCharSet
// used accross the library. It produces the following frame:
//
//   ┌─┐
//   │ │
//   └─┘
//
func FrameCharSetDefault() FrameCharSet {
	return FrameCharSet{
		Horizontal:  term.Cell{Ch: '─'},
		Vertical:    term.Cell{Ch: '│'},
		TopLeft:     term.Cell{Ch: '┌'},
		TopRight:    term.Cell{Ch: '┐'},
		BottomLeft:  term.Cell{Ch: '└'},
		BottomRight: term.Cell{Ch: '┘'},
	}
}

// FrameCharSetHighlight returns a charset that produces the following frame:
//
//   ┏━┓
//   ┃ ┃
//   ┗━┛
//
func FrameCharSetHighlight() FrameCharSet {
	return FrameCharSet{
		Horizontal:  term.Cell{Ch: '━'},
		Vertical:    term.Cell{Ch: '┃'},
		TopLeft:     term.Cell{Ch: '┏'},
		TopRight:    term.Cell{Ch: '┓'},
		BottomLeft:  term.Cell{Ch: '┗'},
		BottomRight: term.Cell{Ch: '┛'},
	}
}

// FrameCharSetStack returns a charset that produces the following frame:
//
//   ├─┤
//   │ │
//   ├─┤
//
func FrameCharSetStack() FrameCharSet {
	return FrameCharSet{
		Horizontal:  term.Cell{Ch: '─'},
		Vertical:    term.Cell{Ch: '│'},
		TopLeft:     term.Cell{Ch: '├'},
		TopRight:    term.Cell{Ch: '┤'},
		BottomLeft:  term.Cell{Ch: '├'},
		BottomRight: term.Cell{Ch: '┤'},
	}
}

// FrameCharSetStackHighlight returns a charset that produces the following frame:
//
//   ┢━┪
//   ┃ ┃
//   ┡━┩
//
func FrameCharSetStackHighlight() FrameCharSet {
	return FrameCharSet{
		Horizontal:  term.Cell{Ch: '━'},
		Vertical:    term.Cell{Ch: '┃'},
		TopLeft:     term.Cell{Ch: '┢'},
		TopRight:    term.Cell{Ch: '┪'},
		BottomLeft:  term.Cell{Ch: '┡'},
		BottomRight: term.Cell{Ch: '┩'},
	}
}

// FrameCharSetStackHead returns a charset that produces the following frame:
//
//   ┌─┐
//   │ │
//   ├─┤
//
func FrameCharSetStackHead() FrameCharSet {
	return FrameCharSet{
		Horizontal:  term.Cell{Ch: '─'},
		Vertical:    term.Cell{Ch: '│'},
		TopLeft:     term.Cell{Ch: '┌'},
		TopRight:    term.Cell{Ch: '┐'},
		BottomLeft:  term.Cell{Ch: '├'},
		BottomRight: term.Cell{Ch: '┤'},
	}
}

// FrameCharSetStackHeadHighlight returns a charset that produces the following frame:
//
//   ┏━┓
//   ┃ ┃
//   ┡━┩
//
func FrameCharSetStackHeadHighlight() FrameCharSet {
	return FrameCharSet{
		Horizontal:  term.Cell{Ch: '━'},
		Vertical:    term.Cell{Ch: '┃'},
		TopLeft:     term.Cell{Ch: '┏'},
		TopRight:    term.Cell{Ch: '┓'},
		BottomLeft:  term.Cell{Ch: '┡'},
		BottomRight: term.Cell{Ch: '┩'},
	}
}

// FrameCharSetStackTail returns a charset that produces the following frame:
//
//   ├─┤
//   │ │
//   └─┘
//
func FrameCharSetStackTail() FrameCharSet {
	return FrameCharSet{
		Horizontal:  term.Cell{Ch: '─'},
		Vertical:    term.Cell{Ch: '│'},
		TopLeft:     term.Cell{Ch: '├'},
		TopRight:    term.Cell{Ch: '┤'},
		BottomLeft:  term.Cell{Ch: '└'},
		BottomRight: term.Cell{Ch: '┘'},
	}
}

// FrameCharSetStackTailHighlight returns a charset that produces the following frame:
//
//   ┢━┪
//   ┃ ┃
//   ┗━┛
//
func FrameCharSetStackTailHighlight() FrameCharSet {
	return FrameCharSet{
		Horizontal:  term.Cell{Ch: '━'},
		Vertical:    term.Cell{Ch: '┃'},
		TopLeft:     term.Cell{Ch: '┢'},
		TopRight:    term.Cell{Ch: '┪'},
		BottomLeft:  term.Cell{Ch: '┗'},
		BottomRight: term.Cell{Ch: '┛'},
	}
}

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
type Frame struct {
	FrameCharSet

	content         Virtual
	bwidth, bheight int
	width, height   int
}

// NewFrame allocates storage and initializes a new frame with the given
// border attributes and underlying component.
func NewFrame(content tui.Component) (f *Frame) {
	f = new(Frame)
	f.Init(content)
	return
}

// Init initializes this frame with the given Component and border attributes.
func (f *Frame) Init(content tui.Component) {
	f.content.C = content
	f.FrameCharSet = FrameCharSetDefault()
}

// SetAttr updates the border attributes of this Frame.
func (f *Frame) SetAttr(border term.Attributes) {
	f.FrameCharSet = f.FrameCharSet.WithAttr(border)
}

// Content returns the underlying Component.
func (f *Frame) Content() tui.Component {
	return f.content.C
}

// SetContent updates the underlying component and resizes it
// to conform to this frame's width and height.
func (f *Frame) SetContent(content tui.Component) {
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

	alignContent(&f.content, width, height, f.bwidth, f.bheight, SpanAlignmentCentered)
	f.width, f.height = width, height
}

// Draw draws this frame's border and contents to the given Writer.
func (f *Frame) Draw(w term.Writer) {
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
	return calculateContentOffset(f.bwidth, f.bheight, SpanAlignmentCentered)
}

// ContentSize returns the size and width of the inner content.
func (f *Frame) ContentSize() (int, int) {
	return f.content.Width(), f.content.Height()
}

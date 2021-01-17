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
	TopLeft, TopRight       rune
	BottomLeft, BottomRight rune
	Horizontal, Vertical    rune
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
		Horizontal:  '─',
		Vertical:    '│',
		TopLeft:     '┌',
		TopRight:    '┐',
		BottomLeft:  '└',
		BottomRight: '┘',
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
		Horizontal:  '━',
		Vertical:    '┃',
		TopLeft:     '┏',
		TopRight:    '┓',
		BottomLeft:  '┗',
		BottomRight: '┛',
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
		Horizontal:  '─',
		Vertical:    '│',
		TopLeft:     '├',
		TopRight:    '┤',
		BottomLeft:  '├',
		BottomRight: '┤',
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
		Horizontal:  '━',
		Vertical:    '┃',
		TopLeft:     '┢',
		TopRight:    '┪',
		BottomLeft:  '┡',
		BottomRight: '┩',
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
		Horizontal:  '─',
		Vertical:    '│',
		TopLeft:     '┌',
		TopRight:    '┐',
		BottomLeft:  '├',
		BottomRight: '┤',
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
		Horizontal:  '━',
		Vertical:    '┃',
		TopLeft:     '┏',
		TopRight:    '┓',
		BottomLeft:  '┡',
		BottomRight: '┩',
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
		Horizontal:  '─',
		Vertical:    '│',
		TopLeft:     '├',
		TopRight:    '┤',
		BottomLeft:  '└',
		BottomRight: '┘',
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
		Horizontal:  '━',
		Vertical:    '┃',
		TopLeft:     '┢',
		TopRight:    '┪',
		BottomLeft:  '┗',
		BottomRight: '┛',
	}
}

// Frame is a Component that simply draws a border around a nested component.
// By default the frame adds some padding around the component by using
// the following cells:
//
// f.Horizontal =  '─'
// f.Vertical =  '│'
// f.TopLeft =  '┌'
// f.TopRight =  '┐'
// f.BottomLeft =  '└'
// f.BottomRight =  '┘'
//
// Note that this component can achieve other effects (highlight, frame)
// by setting the rigth cell characters and/or attributes.
type Frame struct {
	FrameCharSet
	term.Attributes

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
		w.SetCell(term.Coordinates{X: i, Y: 0},
			term.Cell{Ch: f.Horizontal, Bg: f.Bg, Fg: f.Fg})
		w.SetCell(term.Coordinates{X: i, Y: limitY},
			term.Cell{Ch: f.Horizontal, Bg: f.Bg, Fg: f.Fg})
	}

	for i := 0; i < limitY; i++ {
		w.SetCell(term.Coordinates{X: 0, Y: i},
			term.Cell{Ch: f.Vertical, Bg: f.Bg, Fg: f.Fg})
		w.SetCell(term.Coordinates{X: limitX, Y: i},
			term.Cell{Ch: f.Vertical, Bg: f.Bg, Fg: f.Fg})
	}

	w.SetCell(term.Coordinates{X: 0, Y: 0},
		term.Cell{Ch: f.TopLeft, Bg: f.Bg, Fg: f.Fg})

	w.SetCell(term.Coordinates{X: limitX, Y: 0},
		term.Cell{Ch: f.TopRight, Bg: f.Bg, Fg: f.Fg})

	w.SetCell(term.Coordinates{X: 0, Y: limitY},
		term.Cell{Ch: f.BottomLeft, Bg: f.Bg, Fg: f.Fg})

	w.SetCell(term.Coordinates{X: limitX, Y: limitY},
		term.Cell{Ch: f.BottomRight, Bg: f.Bg, Fg: f.Fg})

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

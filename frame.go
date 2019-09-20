package fractal

import (
	"github.com/nsf/termbox-go"
)

// Frame is a Component that simply draws a border around a nested component.
type Frame struct {
	content         VirtualComponent
	bwidth, bheight int
	width, height   int
	Fg, Bg          termbox.Attribute
}

// NewFrame allocates storage and initializes a new frame with the given
// border attributes and underlying component.
func NewFrame(content Component, fg, bg termbox.Attribute) (f *Frame) {
	f = new(Frame)
	f.Init(content, fg, bg)
	return
}

// Init initializes this frame with the given Component and border attributes.
func (f *Frame) Init(content Component, fg, bg termbox.Attribute) {
	f.Fg, f.Bg = fg, bg
	f.content.C = content
}

// SetAttr updates the border attributes of this Frame.
func (f *Frame) SetAttr(fg, bg termbox.Attribute) {
	f.Fg, f.Bg = fg, bg
}

// Content returns the underlying Component.
func (f *Frame) Content() Component {
	return &f.content
}

// SetContent updates the underlying component and resizes it
// to conform to this frame's width and height.
func (f *Frame) SetContent(content Component) (err error) {
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

func (f *Frame) getContentOffset() Coordinates {
	return Coordinates{X: f.bwidth / 2, Y: f.bheight / 2}
}

// Draw draws this frame's border and contents to the given Writer.
func (f *Frame) Draw(w Writer) (err error) {
	if f.bwidth == 0 || f.bheight == 0 {
		return f.content.Draw(w)
	}

	limitX, limitY := f.width-1, f.height-1

	for i := 0; i < limitX; i++ {
		if err = w.Write(i, 0, '─', f.Fg, f.Bg); err != nil {
			return
		}
		if err = w.Write(i, limitY, '─', f.Fg, f.Bg); err != nil {
			return
		}
	}

	for i := 0; i < limitY; i++ {
		if err = w.Write(0, i, '│', f.Fg, f.Bg); err != nil {
			return
		}
		if err = w.Write(limitX, i, '│', f.Fg, f.Bg); err != nil {
			return
		}
	}

	if err = w.Write(0, 0, '┌', f.Fg, f.Bg); err != nil {
		return
	}

	if err = w.Write(limitX, 0, '┐', f.Fg, f.Bg); err != nil {
		return
	}

	if err = w.Write(0, limitY, '└', f.Fg, f.Bg); err != nil {
		return
	}

	if err = w.Write(limitX, limitY, '┘', f.Fg, f.Bg); err != nil {
		return
	}

	return f.content.Draw(w)
}

// ContentPosition returns the position of the content inside this frame.
func (f *Frame) ContentPosition() Coordinates {
	return f.getContentOffset()
}

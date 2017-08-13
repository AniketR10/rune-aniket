package fractal

import (
	"termbox"
)

type Frame struct {
	content         Component
	width, height   int
	bwidth, bheight int
	pos             Coordinates
	Fg, Bg          termbox.Attribute
}

func NewFrame(content Component, width, height, x, y int) (f *Frame, err error) {
	f = new(Frame)
	return f, f.Init(content, width, height, x, y, termbox.ColorDefault, termbox.ColorDefault)
}

func (f *Frame) Init(content Component, width, height, x, y int, fg, bg termbox.Attribute) (err error) {
	f.Fg, f.Bg = fg, bg
	f.pos.X, f.pos.Y = x, y
	f.width, f.height = width, height

	return f.SetContent(content)
}

func (f *Frame) Content() Component {
	return f.content
}

func (f *Frame) SetContent(content Component) (err error) {
	f.content = content

	if err = f.Resize(f.width, f.height); err != nil {
		return
	}

	if err = f.Move(f.pos.X, f.pos.Y); err != nil {
		return
	}

	return
}

func (f *Frame) Resize(width, height int) (err error) {
	// deactivate frame if there's not space for content
	if width < 3 || height < 3 {
		f.bwidth, f.bheight = 0, 0
	} else {
		f.bwidth, f.bheight = 2, 2
	}
	f.width, f.height = width, height

	if err = f.content.Resize(width-f.bwidth, height-f.bheight); err != nil {
		return
	}

	return f.Move(f.pos.X, f.pos.Y)
}

func (f *Frame) Move(x, y int) error {
	f.pos.X, f.pos.Y = x, y
	xoffset, yoffset := f.bwidth/2, f.bheight/2
	return f.content.Move(f.pos.X+xoffset, f.pos.Y+yoffset)
}

func (f *Frame) Draw(w Writer) (err error) {
	if f.bwidth == 0 || f.bheight == 0 {
		return f.content.Draw(w)
	}

	maxX, maxY := f.pos.X+f.width-1, f.pos.Y+f.height-1

	for i := f.pos.X; i < maxX; i++ {
		if err = w.Write(i, f.pos.Y, '─', f.Fg, f.Bg); err != nil {
			return
		}
		if err = w.Write(i, maxY, '─', f.Fg, f.Bg); err != nil {
			return
		}
	}

	for i := f.pos.Y; i < maxY; i++ {
		if err = w.Write(f.pos.X, i, '│', f.Fg, f.Bg); err != nil {
			return
		}
		if err = w.Write(maxX, i, '│', f.Fg, f.Bg); err != nil {
			return
		}
	}

	if err = w.Write(f.pos.X, f.pos.Y, '┌', f.Fg, f.Bg); err != nil {
		return
	}

	if err = w.Write(maxX, f.pos.Y, '┐', f.Fg, f.Bg); err != nil {
		return
	}

	if err = w.Write(f.pos.X, maxY, '└', f.Fg, f.Bg); err != nil {
		return
	}

	if err = w.Write(maxX, maxY, '┘', f.Fg, f.Bg); err != nil {
		return
	}

	return f.content.Draw(w)
}

func (f *Frame) Height() int {
	return f.height
}

func (f *Frame) Width() int {
	return f.width
}

func (f *Frame) Position() (int, int) {
	return f.pos.X, f.pos.Y
}

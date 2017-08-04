package fractal

import termbox "termbox"

type Writer interface {
	Write(x, y int, r rune, fg termbox.Attribute, bg termbox.Attribute) error
	Flush() error
	Clear(fg, bg termbox.Attribute) error
}

type Cursor interface {
	SetCursor(x, y int)
}

type Component interface {
	Resize(width, height int) error
	Move(x, y int) error
	Draw(Writer) error
	Height() int
	Width() int
	Position() (int, int)
}

type Coordinates struct {
	X, Y int
}

type Cell struct {
	Coordinates
	Fg, Bg termbox.Attribute
	Ch     rune
}

type Handler interface {
	Component
	Handle(termbox.Event) error
	SetCursor(Cursor) error
}

package fractal

import termbox "github.com/nsf/termbox-go"

type Writer interface {
	Write(x, y int, r rune, fg termbox.Attribute, bg termbox.Attribute) error
	Flush() error
	Clear(fg, bg termbox.Attribute) error
}

type Component interface {
	Resize(width, height int) error
	Move(x, y int) error
	Draw(w Writer) error
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

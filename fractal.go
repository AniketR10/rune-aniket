package fractal

import (
	"fmt"
	"termbox"
)

type Writer interface {
	Write(x, y int, r rune, fg termbox.Attribute, bg termbox.Attribute) error
	Flush() error
	Clear(fg, bg termbox.Attribute) error
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
	GetCursor() Coordinates
	GetAttr() (fg termbox.Attribute, bg termbox.Attribute)
	IsActive() bool
}

func Channel() <-chan termbox.Event {
	return echan
}

func Init() error {
	if err := termbox.Init(); err != nil {
		return fmt.Errorf("failed termbox init: %v", err)
	}

	return nil
}

func Run(root Handler) (err error) {
	return run(root, &TermboxWriter{})
}

func Size() (width int, height int) {
	return termbox.Size()
}

func Close() {
	termbox.Close()
}

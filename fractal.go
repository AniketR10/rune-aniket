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

type Help struct {
	Summary int
	Keys    int
}

type Handler interface {
	Component
	// TODO custom key event
	Handle(termbox.Event) (bool, error)
	GetCursor() Coordinates
	GetAttr() (fg termbox.Attribute, bg termbox.Attribute)
	Man() string
}

func Channel() <-chan termbox.Event {
	return echan
}

func Init() error {
	if err := termbox.Init(); err != nil {
		return fmt.Errorf("failed termbox init: %v", err)
	}

	echan = make(chan termbox.Event)
	ichan = make(chan termbox.Event)

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

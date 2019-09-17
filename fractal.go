package fractal

import (
	"fmt"
	"termbox"
)

type Component interface {
	Resize(width, height int)
	Draw(Writer) error
}

type Coordinates struct {
	X, Y int
}

type Handler interface {
	Component
	Handle(termbox.Event) bool
	GetCursor() Coordinates
	Man() Manual
}

type Manual struct {
	Summary string
	Keys    KeyMap
}

type KeyMap map[termbox.Event]struct {
	ID          string
	Description string
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
	foreground, background = termbox.ColorDefault, termbox.ColorDefault
	highlightfg, highlightbg = termbox.ColorRed, termbox.ColorDefault

	return nil
}

func SetAttr(fg, bg, highlightfg, highlightbg termbox.Attribute) {
	foreground, background = fg, bg
}

func Run(root Handler) (err error) {
	return run(root, &termboxWriter{})
}

func RunMode(root Handler, mode termbox.InputMode) (err error) {
	termbox.SetInputMode(mode)
	return Run(root)
}

func Size() (width int, height int) {
	return termbox.Size()
}

func Close() {
	termbox.Close()
}

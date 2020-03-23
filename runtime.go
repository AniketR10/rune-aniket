package tui

import (
	"sync"

	"github.com/ernestrc/go-tui/term"
)

var (
	attr term.Attributes
)

func redraw(root Handler, termw Writer) (err error) {
	if err = termw.Clear(attr); err != nil {
		return err
	}

	root.Draw(termw)

	cursor, show := root.Cursor()
	if show {
		termw.SetCursor(cursor)
	} else {
		termw.SetCursor(term.Coordinates{X: -1, Y: -1})
	}

	if err = termw.Flush(); err != nil {
		return err
	}

	return

}

func run(root Handler, lock sync.Locker, termw Writer) (err error) {
	width, height := term.Size()

	root.Resize(width, height)

	var hexit bool

	for !hexit && err == nil {
		if err = redraw(root, termw); err != nil {
			return
		}

		ev := term.PollEvent()
		switch ev.Type {
		case term.EventInterrupt:
		case term.EventError:
			err = ev.Err
		case term.EventResize:
			width, height := ev.Width, ev.Height
			root.Resize(width, height)
		default:
			lock.Lock()
			hexit, _ = root.Handle(ev)
			lock.Unlock()
		}
	}

	return nil
}

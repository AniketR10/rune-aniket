package tui

import (
	"sync"

	"unstable.build/go-tui/term"
)

func redraw(root Handler, lock sync.Locker, termw term.Writer) (err error) {
	// TODO Attr should be removed and Clear should no take any parameters
	if err = termw.Clear(term.Attr()); err != nil {
		return err
	}

	lock.Lock()
	root.Draw(termw)
	cursor, show := root.Cursor()
	lock.Unlock()

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

func run(root Handler, lock sync.Locker, termw term.Writer) (err error) {
	width, height := term.Size()

	lock.Lock()
	root.Resize(width, height)
	lock.Unlock()

	var exit bool

	for !exit && err == nil {
		if err = redraw(root, lock, termw); err != nil {
			return
		}

		for {
			ev := term.PollEvent()
			switch ev.Type {
			case term.EventInterrupt:
			case term.EventError:
				err = ev.Err
			case term.EventResize:
				width, height := ev.Width, ev.Height
				lock.Lock()
				root.Resize(width, height)
				lock.Unlock()
			default:
				lock.Lock()
				exit, _ = root.Handle(ev)
				lock.Unlock()
			}
			if exit || !term.HasPendingEvent() {
				break // redraw
			}
		}
	}

	return err
}

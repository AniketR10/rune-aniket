package tui

import (
	"sync"

	"github.com/ernestrc/go-tui/term"
)

var (
	ichan chan term.Event
	attr  term.Attributes
)

func resize(root Handler, width, height int) {
	root.Resize(width, height)
}

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

	resize(root, width, height)

	var hexit, texit bool

	go func() {
		for !texit {
			ichan <- term.PollEvent()
		}
	}()

	for !hexit && err == nil {
		if err = redraw(root, termw); err != nil {
			return
		}

		select {
		case ev := <-ichan:
			switch ev.Type {
			case term.EventInterrupt:
			case term.EventError:
				err = ev.Err
			case term.EventResize:
				width, height := ev.Width, ev.Height
				resize(root, width, height)
			default:
				lock.Lock()
				hexit, _ = root.Handle(ev)
				lock.Unlock()
			}
		}
	}

	// stop polling events
	texit = true
	term.Interrupt()
	<-ichan

	return nil
}

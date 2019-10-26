package fractal

import (
	"github.com/ernestrc/fractal/term"
	"github.com/nsf/termbox-go"
)

var (
	echan chan term.Event
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

	cursor := root.GetCursor()
	termw.SetCursor(cursor)

	if err = termw.Flush(); err != nil {
		return err
	}

	return

}

func run(root Handler, termw Writer) (err error) {
	width, height := termbox.Size()

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
		case ev := <-echan:
			hexit = root.Handle(ev)
		case ev := <-ichan:
			switch ev.Type {
			case term.EventInterrupt:
			case term.EventError:
				err = ev.Err
			case term.EventResize:
				width, height := ev.Width, ev.Height
				resize(root, width, height)
			default:
				hexit = root.Handle(ev)
			}
		}
	}

	// stop polling events
	texit = true
	termbox.Interrupt()
	<-ichan

	return nil
}

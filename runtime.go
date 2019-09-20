package fractal

import (
	"fmt"

	"github.com/nsf/termbox-go"
)

var (
	echan                    chan termbox.Event
	ichan                    chan termbox.Event
	foreground, background   termbox.Attribute
	highlightfg, highlightbg termbox.Attribute
)

func resize(root Handler, width, height int) {
	root.Resize(width, height)
}

func redraw(root Handler, termw Writer) (err error) {
	if err = termw.Clear(foreground, background); err != nil {
		return err
	}

	if err = root.Draw(termw); err != nil {
		return fmt.Errorf("failed to draw root handler: %v", err)
	}

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
			ichan <- termbox.PollEvent()
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
			case termbox.EventInterrupt:
			case termbox.EventError:
				err = ev.Err
			case termbox.EventResize:
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

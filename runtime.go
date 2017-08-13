package fractal

import (
	"fmt"

	"termbox"
)

var (
	echan                    chan termbox.Event
	ichan                    chan termbox.Event
	foreground, background   termbox.Attribute
	highlightfg, highlightbg termbox.Attribute
)

func resize(root Handler, width, height int) error {
	if err := root.Resize(width, height); err != nil {
		return fmt.Errorf("failed to resize root handler: %v", err)
	}

	return nil
}

func redraw(root Handler, termw Writer) (err error) {
	if err = termw.Clear(foreground, background); err != nil {
		return err
	}

	if err = root.Draw(termw); err != nil {
		return fmt.Errorf("failed to draw root handler: %v", err)
	}

	cursor := root.GetCursor()
	termbox.SetCursor(cursor.X, cursor.Y)

	if err = termw.Flush(); err != nil {
		return err
	}

	return

}

func run(root Handler, termw Writer) (err error) {
	if err = root.Move(0, 0); err != nil {
		return fmt.Errorf("failed to position root handler: %v", err)
	}

	width, height := termbox.Size()

	if err = resize(root, width, height); err != nil {
		return err
	}

	termbox.SetInputMode(termbox.InputAlt)

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
			hexit, err = root.Handle(ev)
		case ev := <-ichan:
			switch ev.Type {
			case termbox.EventInterrupt:
			case termbox.EventError:
				err = ev.Err
			case termbox.EventResize:
				width, height := ev.Width, ev.Height
				if err = resize(root, width, height); err != nil {
					return
				}
			default:
				hexit, err = root.Handle(ev)
			}
		}
	}

	// stop polling events
	texit = true
	termbox.Interrupt()
	<-ichan

	return nil
}

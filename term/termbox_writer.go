package term

import (
	"github.com/nsf/termbox-go"
)

type TermboxWriter struct{}

func (w TermboxWriter) SetCell(pos Coordinates, c Cell) {
	termbox.SetCell(pos.X, pos.Y,
		c.Ch, termbox.Attribute(c.Fg), termbox.Attribute(c.Bg))
	return
}

func (w TermboxWriter) SetAttr(pos Coordinates, attr Attributes) {
	panic("TODO")
	// FIXME
	// termbox.SetCell(x, y, ch, 0, 0)
	// termbox.Attribute(fg), termbox.Attribute(bg)
}

func (w TermboxWriter) Flush() error {
	return termbox.Flush()
}

func (w TermboxWriter) Clear(attr Attributes) (err error) {
	err = termbox.Clear(termbox.Attribute(attr.Fg), termbox.Attribute(attr.Bg))
	return
}

func (w TermboxWriter) SetCursor(pos Coordinates) {
	termbox.SetCursor(pos.X, pos.Y)
}

// SetInputMode sets termbox input mode. Termbox has two input modes:
//
// 1. Esc input mode. When ESC sequence is in the buffer and it doesn't match
// any known sequence. ESC means KeyEsc. This is the default input mode.
//
// 2. Alt input mode. When ESC sequence is in the buffer and it doesn't match
// any known sequence. ESC enables ModAlt modifier for the next keyboard event.
//
// Both input modes can be OR'ed with Mouse mode. Setting Mouse mode bit up will
// enable mouse button press/release and drag events.
//
// If 'mode' is InputCurrent, returns the current input mode. See also Input*
// constants.
func SetInputMode(mode InputMode) InputMode {
	return InputMode(termbox.SetInputMode(termbox.InputMode(mode)))
}

// Init Initializes writer.
// This function should be called before any other functions.
// After successful initialization, the writer must be finalized using 'Close'
// function.
func Init() error {
	return termbox.Init()
}

func Size() (width int, height int) {
	return termbox.Size()
}

// Wait for an event and return it. This is a blocking function call.
func PollEvent() (ev Event) {
	tev := termbox.PollEvent()
	ev.Type = EventType(tev.Type)
	ev.Mod = Modifier(tev.Mod)
	ev.Key = Key(tev.Key)
	ev.Ch = tev.Ch
	ev.Width = tev.Width
	ev.Height = tev.Height
	ev.Err = tev.Err
	ev.MouseX = tev.MouseX
	ev.MouseY = tev.MouseY
	ev.N = tev.N
	return
}

// Close writer; should be called after successful initialization
// when termbox's functionality isn't required anymore.
func Close() {
	termbox.Close()
}

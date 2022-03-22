package vi

import (
	"errors"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/text"
	"github.com/ernestrc/go-tui/term"
)

type viEditor struct {
	text.Publisher
	opts []Option
}

// Editor returns a Vi editor.Editor.
func Editor(opts ...Option) text.Editor {
	ret := &viEditor{opts: opts}
	ret.Publisher.Init()
	return ret
}

func (e *viEditor) Edit(name string, buf *cell.Buffer) (text.Handler, error) {
	root := New(buf, name, e.opts...)
	// publisher does not mutate cursor and it should never do so
	cursor := root.cursor
	return e.Publisher.PublishEdit(name, buf, root, cursor), nil
}

// SubscribeCommand is not supported.
func (e *viEditor) SubscribeCommand(cmd string, h text.CommandHandler) error {
	return errors.New("not supported")
}

// Editor is not supported
func (e *viEditor) Editor(name string) (text.Handler, error) {
	// NOTE: it would be dead code
	return nil, errors.New("not supported")
}

// SubscribeEditorEvents subsribes sub to ev. Note that this Editor is only capable
// of dispatching EventTypeOpen, EventTypeInsert and EventTypeDelete
// EventType events.
func (e *viEditor) SubscribeEditorEvents(
	evs []text.EventType, sub text.EventHandler,
) error {
	e.Publisher.SubscribeEditorEvents(evs, sub)
	return nil
}

func (e viEditor) SetLocationList(h text.Handler, ID string, loc text.LocationList) error {
	e.Publisher.Handler(h).(*Vi).SetLocationList(ID, loc)
	return nil
}

func (e *viEditor) MoveToNextLocation(h text.Handler, ID string) error {
	e.Publisher.Handler(h).(*Vi).MoveToNextLocation(ID)
	return nil
}

func (e *viEditor) MoveToPrevLocation(h text.Handler, ID string) error {
	e.Publisher.Handler(h).(*Vi).MoveToPrevLocation(ID)
	return nil
}

func (e *viEditor) Reader(h text.Handler) text.Reader {
	return text.CellReader(e.Publisher.Handler(h).(*Vi).Reader())
}

func (e *viEditor) Writer(h text.Handler) text.Writer {
	return text.CellWriter(e.Publisher.Handler(h).(*Vi).Writer())
}

func (e *viEditor) SetCursor(h text.Handler, pos term.Coordinates) error {
	ok := e.Publisher.Handler(h).(*Vi).SetCursorAtScroll(pos)
	if !ok {
		return errors.New("SetCursor: invalid cursor position")
	}
	return nil
}

func (e *viEditor) Cursor(h text.Handler) (term.Coordinates, error) {
	pos := e.Publisher.Handler(h).(*Vi).CursorAtScroll()
	return pos, nil
}

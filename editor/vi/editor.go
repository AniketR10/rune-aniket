package vi

import (
	"errors"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/term"
)

type viEditor struct {
	editor.Publisher
	opts []Option
}

// Editor returns a Vi editor.Editor.
func Editor(opts ...Option) editor.Editor {
	ret := &viEditor{opts: opts}
	ret.Publisher.Init()
	return ret
}

func (e *viEditor) Edit(name string, buf *cell.Buffer) (editor.Handler, error) {
	root := New(buf, name, e.opts...)
	return e.Publisher.PublishEdit(name, buf, root, &root.cursor), nil
}

// SubscribeCommand is not supported.
func (e *viEditor) SubscribeCommand(cmd string, h editor.CommandHandler) error {
	return errors.New("not supported")
}

// Editor is not supported
func (e *viEditor) Editor(name string) (editor.Handler, error) {
	// NOTE: it would be dead code
	return nil, errors.New("not supported")
}

// SubscribeEditorEvents subsribes sub to ev. Note that this Editor is only capable
// of dispatching EventTypeOpen, EventTypeInsert and EventTypeDelete
// EventType events.
func (e *viEditor) SubscribeEditorEvents(
	evs []editor.EventType, sub editor.EventHandler,
) error {
	e.Publisher.SubscribeEditorEvents(evs, sub)
	return nil
}

func (e viEditor) SetLocationList(h editor.Handler, ID string, loc editor.LocationList) error {
	e.Publisher.Handler(h).(*Vi).SetLocationList(ID, loc)
	return nil
}

func (e *viEditor) MoveToNextLocation(h editor.Handler, ID string) error {
	e.Publisher.Handler(h).(*Vi).MoveToNextLocation(ID)
	return nil
}

func (e *viEditor) MoveToPrevLocation(h editor.Handler, ID string) error {
	e.Publisher.Handler(h).(*Vi).MoveToPrevLocation(ID)
	return nil
}

func (e *viEditor) Reader(h editor.Handler) editor.Reader {
	return editor.CellReader(e.Publisher.Handler(h).(*Vi).less.Buffer().Reader())
}

func (e *viEditor) Writer(h editor.Handler) editor.Writer {
	return editor.CellWriter(e.Publisher.Handler(h).(*Vi).less.Buffer().Writer())
}

func (e *viEditor) SetCursor(h editor.Handler, pos term.Coordinates) error {
	ok := e.Publisher.Handler(h).(*Vi).SetCursorAtScroll(pos)
	if !ok {
		return errors.New("SetCursor: invalid cursor position")
	}
	return nil
}

func (e *viEditor) Cursor(h editor.Handler) (term.Coordinates, error) {
	pos := e.Publisher.Handler(h).(*Vi).CursorAtScroll()
	return pos, nil
}

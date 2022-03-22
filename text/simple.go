package text

import (
	"errors"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

type simpleEditor struct {
	pub  Publisher
	wrap bool
}

// SimpleEditor returns a simple to use Editor implementation.
func SimpleEditor(wrap bool) Editor {
	ret := new(simpleEditor)
	ret.wrap = wrap
	ret.pub.Init()
	return ret
}

func (e *simpleEditor) Edit(name string, buf *cell.Buffer) (Handler, error) {
	root := newSimpleEditor(buf, name, e.wrap)
	return e.pub.PublishEdit(name, buf, root, &root.cursor), nil
}

func (e *simpleEditor) SubscribeCommand(cmd string, h CommandHandler) error {
	return errors.New("not supported")
}

func (e *simpleEditor) Editor(name string) (Handler, error) {
	return nil, errors.New("not supported")
}

func (e *simpleEditor) SubscribeEditorEvents(evs []EventType, sub EventHandler) error {
	e.pub.SubscribeEditorEvents(evs, sub)
	return nil
}

func (e simpleEditor) SetLocationList(h Handler, ID string, loc LocationList) error {
	e.pub.Handler(h).(*simpleEditorHandler).cursor.SetLocationList(ID, loc)
	return nil
}

func (e *simpleEditor) MoveToNextLocation(h Handler, ID string) error {
	e.pub.Handler(h).(*simpleEditorHandler).cursor.MoveToNextLocation(ID)
	return nil
}

func (e *simpleEditor) MoveToPrevLocation(h Handler, ID string) error {
	e.pub.Handler(h).(*simpleEditorHandler).cursor.MoveToPrevLocation(ID)
	return nil
}

func (e *simpleEditor) Reader(h Handler) Reader {
	return CellReader(e.pub.Handler(h).(*simpleEditorHandler).buf.Reader())
}

func (e *simpleEditor) Writer(h Handler) Writer {
	return CellWriter(e.pub.Handler(h).(*simpleEditorHandler).buf.Writer())
}

func (e *simpleEditor) SetCursor(h Handler, pos term.Coordinates) error {
	_, ok := e.pub.Handler(h).(*simpleEditorHandler).cursor.MoveToScroll(pos)
	if !ok {
		return errors.New("MoveToScroll: invalid cursor position")
	}
	return nil
}

func (e *simpleEditor) Cursor(h Handler) (term.Coordinates, error) {
	pos := e.pub.Handler(h).(*simpleEditorHandler).cursor.CursorAtScroll()
	return pos, nil
}

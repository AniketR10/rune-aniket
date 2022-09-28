package text

import (
	"errors"

	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
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

func (e *simpleEditor) Edit(file workspace.URI, buf *cell.Buffer) (Handler, error) {
	root := newSimpleEditor(buf, file, e.wrap)
	return e.pub.PublishEdit(file, buf, root, &root.cursor), nil
}

func (e *simpleEditor) SubscribeCommand(cmd string, h CommandHandler) error {
	return errors.New("not supported")
}

func (e *simpleEditor) Editor(file workspace.URI) (Handler, error) {
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

func (e *simpleEditor) CellView(h Handler) CellView {
	return NewCellView(e.pub.Handler(h).(*simpleEditorHandler).buf.View())
}

func (e *simpleEditor) CellEditor(h Handler) CellEditor {
	return NewCellEditor(e.pub.Handler(h).(*simpleEditorHandler).buf.Editor())
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

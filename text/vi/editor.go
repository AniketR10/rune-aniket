package vi

import (
	"errors"

	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

type viEditor struct {
	text.Publisher
	opts []Option
}

// Editor returns a Vi text.Editor.
func Editor(opts ...Option) text.Editor {
	ret := &viEditor{opts: opts}
	ret.Publisher.Init()
	return ret
}

func (e *viEditor) Edit(file workspace.URI, buf *cell.Buffer) (text.Handler, error) {
	root := New(buf, file, e.opts...)
	// publisher does not mutate cursor and it should never do so
	cursor := root.cursor
	return e.Publisher.PublishEdit(file, buf, root, cursor), nil
}

// SubscribeCommand is not supported.
func (e *viEditor) SubscribeCommand(cmd string, h text.CommandHandler) error {
	return errors.New("not supported")
}

// Editor is not supported
func (e *viEditor) Editor(file workspace.URI) (text.Handler, error) {
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

func (e *viEditor) SetDefaultAttributes(h text.Handler, attrs term.Attributes) error {
	e.Publisher.Handler(h).(*Vi).SetDefaultAttributes(attrs)
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

func (e *viEditor) CellView(h text.Handler) text.CellView {
	return text.NewCellView(e.Publisher.Handler(h).(*Vi).CellView())
}

func (e *viEditor) CellEditor(h text.Handler) text.CellEditor {
	return text.NewCellEditor(e.Publisher.Handler(h).(*Vi).CellEditor())
}

func (e *viEditor) SetCursor(h text.Handler, pos term.Coordinates) error {
	vi := e.Publisher.Handler(h).(*Vi)
	ok := vi.SetCursorAtScroll(pos)
	if !ok {
		if vi.CursorAtScroll() == pos {
			// already set at position
			return nil
		}
		return errors.New("SetCursor: invalid cursor position")
	}
	return nil
}

func (e *viEditor) Cursor(h text.Handler) (term.Coordinates, error) {
	pos := e.Publisher.Handler(h).(*Vi).CursorAtScroll()
	return pos, nil
}

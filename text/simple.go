package text

import (
	"errors"

	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/clipboard"
)

// DefaultSimpleEditor returns a simple to use Editor implementation.
func DefaultSimpleEditor(clipboard clipboard.Register) Editor {
	defaultAttr := term.Attributes{Fg: term.AttrReverse}
	return NewSimpleEditor(clipboard, false, true, defaultAttr)
}

// NewSimpleEditor allocates storage for a new Editor and initializes it.
func NewSimpleEditor(
	clipboard clipboard.Register,
	wrap, commandBar bool, resultsAttr term.Attributes,
) Editor {
	ret := new(simpleEditor)
	ret.wrap = wrap
	ret.commandBar = commandBar
	ret.resAttr = resultsAttr
	ret.clipboard = clipboard
	ret.pub.Init()
	return ret
}

type simpleEditor struct {
	pub        Publisher
	wrap       bool
	commandBar bool
	resAttr    term.Attributes
	clipboard  clipboard.Register
}

func (e *simpleEditor) Edit(file workspaceapi.URI, buf *cell.Buffer) (Handler, error) {
	rootIfc := NewSimpleHandler(buf, file, e.wrap, e.commandBar, e.resAttr, e.clipboard)
	root := rootIfc.(*simpleEditorHandler)
	return e.pub.PublishEdit(file, buf, root, &root.cursor), nil
}

func (e *simpleEditor) SubscribeCommand(cmd textapi.CommandManual, h CommandHandler) error {
	return errors.New("not supported")
}

func (c *simpleEditor) UnsubscribeCommand(cmd string) error {
	return errors.New("not supported")
}

func (e *simpleEditor) Editor(file workspaceapi.URI) (Handler, error) {
	return nil, errors.New("not supported")
}

func (e *simpleEditor) SubscribeEvents(evs []textapi.EventType, sub EventHandler) error {
	e.pub.SubscribeEvents(evs, sub)
	return nil
}

func (e *simpleEditor) UnsubscribeEvents(sub EventHandler) (bool, error) {
	ok := e.pub.UnsubscribeEvents(sub)
	return ok, nil
}

func (e simpleEditor) SetLocationList(
	h Handler, pri textapi.LocationPriority, ID string, loc LocationList,
) error {
	e.pub.Handler(h).(*simpleEditorHandler).cursor.SetLocationList(pri, ID, loc)
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

func (e *simpleEditor) SetDefaultAttributes(h Handler, attr term.Attributes) error {
	e.pub.Handler(h).(*simpleEditorHandler).less.Scroll().Attributes = attr
	return nil
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

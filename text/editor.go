package text

import (
	"context"
	"fmt"

	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

// Handler just wraps a tui.Handler to indicate that this API's handlers might
// not be compatible with other APIs.
type Handler interface {
	browser.Handler

	// This is only used to differentiate editor.Handler from the rest
	// of tui.Handler in a browser.Component.
	Resource() workspace.URI
}

// CellEditor is a cell.Editor that can fail.
type CellEditor interface {
	Edit(start, end term.Coordinates, str string) (from, to term.Coordinates, old string, err error)
}

// CellView wraps a subset of cell.View behaviour with an API that can fail.
type CellView interface {
	RawCells() ([][]term.Cell, error)
}

// Command represents a command issued by the user.
type Command struct {
	Name string
	Args []string

	// optional. If command is dispatched while non-tab is in focus,
	// then these fields will be zero-valued.
	URI      workspace.URI
	Resource Handler
	Cursor   struct {
		Content term.Coordinates
		Window  term.Coordinates
	}
}

// CommandHandler is a callback interface that wraps the basic method Command.
type CommandHandler interface {
	// Handle is called when user issued a command previously registered via SubscribeCommand.
	HandleCommand(context.Context, Command) (exit bool, err error)
}

// Editor is the interface that wraps an API to manage a text editor.
type Editor interface {
	// Edit opens a file and returns an editor.Handler to edit it or an error
	// if there was an error opening it.
	Edit(file workspace.URI, buf *cell.Buffer) (Handler, error)

	// SubscribeEditorEvents subscribes EventHandler to events of type EventType.
	// Note that it's suffixed with Editor so implementors
	// can also implement browser.Subscriber.
	SubscribeEditorEvents([]EventType, EventHandler) error

	// Editor returns the editor.Handler with name or returns
	// an error if no editor with name is open via Edit.
	Editor(file workspace.URI) (Handler, error)

	// SubscribeCommand registers command to be dispatched to CommandHandler.
	SubscribeCommand(string, CommandHandler) error

	// SetLocationList sets the Handler's location list for users to
	// navigate the code. See LocationList for more details.
	// In order to remove a location list, SetLocationList must be called
	// with an empty (or nil) LocationList.
	// Locations are removed if underlying buffer is updated. It is the
	// reponsibility of the caller to recompute the list of locations
	// and call SetLocationList with the new list of locations after
	// every update. Check cell.Buffer.Subscribe for more details.
	SetLocationList(Handler, string, LocationList) error

	// Moves cursor to the next location on list with ID.
	MoveToNextLocation(h Handler, ID string) error

	// Moves cursor to the previous location on list with ID.
	MoveToPrevLocation(h Handler, ID string) error

	// Cursor gets the position of Handler's cursor in the underlying
	// content buffer.
	Cursor(Handler) (term.Coordinates, error)

	// SetCursor sets the cursor of Handler to the given Coordinates.
	SetCursor(Handler, term.Coordinates) error

	// CellView returns a CellView which allows to read the editor's internal buffer.
	CellView(Handler) CellView

	// CellEditor returns a CellEditor which allows for direct write access
	// to the editor's internal buffer.
	CellEditor(Handler) CellEditor
}

type cellEditor struct {
	c cell.Editor
}

type cellView struct {
	c cell.View
}

func (w cellEditor) Edit(
	start, end term.Coordinates, str string,
) (from, to term.Coordinates, old string, err error) {
	if start.Y < 0 || end.Y < 0 || start.X < 0 || end.X < 0 {
		err = fmt.Errorf("invalid coordinates: start=%v; end=%v", start, end)
		return
	}
	from, to, old = w.c.Edit(start, end, str)
	return
}

func (r cellView) RawCells() ([][]term.Cell, error) {
	return r.c.RawCells(), nil
}

// NewCellEditor wraps a cell.Editor with a CellEditor that detects invalid input calls
// and returns the corresponding errors.
func NewCellEditor(c cell.Editor) CellEditor {
	return cellEditor{c}
}

// NewCellView wraps a cell.Reder with a Reader that returns no errors.
func NewCellView(c cell.View) CellView {
	return cellView{c}
}

type fnCommandHandler struct {
	cb func(context.Context, Command) (bool, error)
}

func (f fnCommandHandler) HandleCommand(ctx context.Context, c Command) (bool, error) {
	return f.cb(ctx, c)
}

// FuncCommandHandler returns an CommandHandler that calls fn
// every time Handle is invoked.
func FuncCommandHandler(fn func(context.Context, Command) (bool, error)) CommandHandler {
	return fnCommandHandler{
		cb: fn,
	}
}

package api

import (
	browserapi "unstable.build/go-tui/api/browser"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

// Handler just wraps a tui.Handler to indicate that this API's handlers might
// not be compatible with other APIs.
type Handler interface {
	browserapi.Handler

	// This is only used to differentiate editor.Handler from the rest
	// of tui.Handler in a browser.Component.
	Resource() workspaceapi.URI
}

// CellEditor is a cell.Editor that can fail.
type CellEditor interface {
	Edit(start, end term.Coordinates, str string) (from, to term.Coordinates, old string, err error)
}

// CellView wraps a subset of cell.View behaviour with an API that can fail.
type CellView interface {
	RawCells() ([][]term.Cell, error)
}

type LocationPriority uint

const (
	LocationPriorityInfo LocationPriority = iota
	LocationPriorityWarning
	LocationPriorityError
	LocationPriorityCritical
)

// Editor is the interface that wraps an API to manage a text editor.
type Editor interface {
	// Edit opens a file and returns an editor.Handler to edit it or an error
	// if there was an error opening it.
	Edit(file workspaceapi.URI, buf *cell.Buffer) (Handler, error)

	// SubscribeEvents subscribes EventHandler to events of type EventType.
	SubscribeEvents([]EventType, EventHandler) error

	// Editor returns the editor.Handler with name or returns
	// an error if no editor with name is open via Edit.
	Editor(file workspaceapi.URI) (Handler, error)

	// SubscribeCommand registers command to be dispatched to CommandHandler.
	SubscribeCommand(CommandManual, CommandHandler) error

	// SetLocationList sets the Handler's location list for users to
	// navigate the code. See LocationList for more details.
	// In order to remove a location list, SetLocationList must be called
	// with an empty (or nil) LocationList.
	// Locations are removed if underlying buffer is updated. It is the
	// reponsibility of the caller to recompute the list of locations
	// and call SetLocationList with the new list of locations after
	// every update. Check cell.Buffer.Subscribe for more details.
	SetLocationList(Handler, LocationPriority, string, LocationList) error

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

	// SetDefaultAttributes sets the default attributes of the given Handler
	// before any LocationList overwrites.
	SetDefaultAttributes(Handler, term.Attributes) error
}

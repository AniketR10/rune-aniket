package api

import (
	"context"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/term"
)

// EventType is a type of editor event.
type EventType uint8

const (
	// EventTypeOpen is dispatched when an editor is called the Edit method.
	// Content represents the initial content of the underlying file.
	EventTypeOpen EventType = iota

	// EventTypeClose is dispatched when an editor buffer is closed.
	EventTypeClose

	// EventTypeFlush is dispatched when an editor buffer is Flushed.
	// Content represents the file content that was flushed.
	EventTypeFlush

	// EventTypeEdit is dispatched when new content is inserted into an editor buffer.
	// Start, End represent the input to Edit whereas
	// From, To represent output coordinates. See cell.Editor.Edit for
	// more details.
	EventTypeEdit

	// EventTypeScroll is dispatched when content is scroll to a new offset.
	// Start represents the new scroll offset, whereas From represents the
	// last offset position.
	EventTypeScroll

	// EventTypeFocus is dispatched when an editor handler is on browser.Focus.
	// Start contains the width (X) and height (Y) of the content in focus.
	// If content is resized, EventTypeFocus is sent again, with the new dimensions.
	EventTypeFocus

	// EventTypeUnfocus is dispatched when an editor handler is not
	// on browser.Focus anymore.
	EventTypeUnfocus

	// EventTypeCursor is dispatched when the position of the cursor of an
	// editor Handler changes, either in the window coordinate system or the underlying
	// content position. Event.Start will be set to the cursor's window position,
	// and Event.From will be set to the cursor's scroll position.
	EventTypeCursor
)

// Event encapsulates eventual information about a particular editor resource.
type Event struct {
	Type     EventType
	URI      workspaceapi.URI
	Resource Handler

	Start, End term.Coordinates
	From, To   term.Coordinates
	Content    string
}

// EventHandler wraps the basic method Handle.
type EventHandler interface {
	// Handle handles Event and returns true if it no longer needs to receive events,
	// in other words it returns true if it's done processing events.
	Handle(context.Context, Event) bool
}

type fnEventHandler struct {
	cb func(context.Context, Event) bool
}

func (f fnEventHandler) Handle(ctx context.Context, ev Event) bool {
	return f.cb(ctx, ev)
}

// FuncEventHandler returns an EventHandler that calls fn
// every time Handle is invoked.
func FuncEventHandler(fn func(context.Context, Event) bool) EventHandler {
	return fnEventHandler{
		cb: fn,
	}
}

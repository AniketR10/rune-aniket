package text

import (
	"context"

	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
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
	// Start represents the scroll offset.
	EventTypeScroll

	// EventTypeFocus is dispatched when an editor handler is on browser.Focus.
	EventTypeFocus

	// EventTypeUnfocus is dispatched when an editor handler is not
	// on browser.Focus anymore.
	EventTypeUnfocus

	// EventTypeCursor is dispatched when the position of the cursor of an
	// editor Handler changes, either in the window coordinate system or the underlying
	// content position. Event.Start will be set to the cursor's window position,
	// and Event.From will be set to the cursor's scroll position.
	EventTypeCursor

	// used internally by server/client to re-use EventHandler logic for CommandHandler
	EventTypeCommand
)

// Event encapsulates eventual information about a particular editor resource.
type Event struct {
	Type     EventType
	URI      workspace.URI
	Resource Handler

	Start, End term.Coordinates
	From, To   term.Coordinates
	Content    string

	Args []string
}

type cellSubscriber struct {
	uri workspace.URI
	h   Handler
	eh  EventHandler

	onWillEditStr   string
	onWillEditStart term.Coordinates
	onWillEditEnd   term.Coordinates
}

func (s *cellSubscriber) OnWillEdit(start, end term.Coordinates, str string) {
	s.onWillEditStr = str
	s.onWillEditStart = start
	s.onWillEditEnd = end
}

func (s *cellSubscriber) OnDidEdit(from, to term.Coordinates, old string) {
	s.eh.Handle(context.Background(), Event{
		Type:     EventTypeEdit,
		Resource: s.h,
		URI:      s.uri,
		From:     from,
		To:       to,
		Start:    s.onWillEditStart,
		End:      s.onWillEditEnd,
		Content:  s.onWillEditStr,
	})
}

// CellSubscriber returns a cell.Subscriber which forwards Insert/Delete events to evHandler
func CellSubscriber(uri workspace.URI, h Handler, evHandler EventHandler) cell.Subscriber {
	return &cellSubscriber{uri: uri, h: h, eh: evHandler}
}

type scrollSubscriber struct {
	uri workspace.URI
	h   Handler
	eh  EventHandler
}

func (s scrollSubscriber) OnSeek(at term.Coordinates) {
	s.eh.Handle(context.Background(), Event{
		Type:     EventTypeScroll,
		Resource: s.h,
		URI:      s.uri,
		Start:    at,
	})
}

// ScrollSubscriber returns a component.ScrollSubscriber which forwarsd Scroll events to evHandler
func ScrollSubscriber(resource workspace.URI, h Handler, evHandler EventHandler) component.ScrollSubscriber {
	return scrollSubscriber{uri: resource, h: h, eh: evHandler}
}

package text

import (
	"context"
	"fmt"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
	termpb "github.com/ernestrc/go-tui/term/rpc"
	textpb "github.com/ernestrc/go-tui/text/rpc"
	"github.com/ernestrc/go-tui/workspace"
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
	eventTypeCommand
)

// Event encapsulates eventual information about a particular editor resource.
type Event struct {
	Type     EventType
	URI      workspace.URI
	Resource Handler

	Start, End term.Coordinates
	From, To   term.Coordinates
	Content    string

	// used internally by server/client
	cmdArgs []string
}

func protoTypeToModel(protoType textpb.EditorEvent_Type) (ev EventType, err error) {
	switch protoType {
	case textpb.EditorEvent_TypeClose:
		ev = EventTypeClose
	case textpb.EditorEvent_TypeFlush:
		ev = EventTypeFlush
	case textpb.EditorEvent_TypeOpen:
		ev = EventTypeOpen
	case textpb.EditorEvent_TypeEdit:
		ev = EventTypeEdit
	case textpb.EditorEvent_TypeScroll:
		ev = EventTypeScroll
	case textpb.EditorEvent_TypeCursor:
		ev = EventTypeCursor
	case textpb.EditorEvent_TypeFocus:
		ev = EventTypeFocus
	case textpb.EditorEvent_TypeUnfocus:
		ev = EventTypeUnfocus
	case textpb.EditorEvent_TypeCommand:
		ev = eventTypeCommand
	default:
		err = fmt.Errorf("failed to convert proto editor event: invalid type: %v",
			protoType)
	}

	return
}

func (e *Event) fromProto(pe *textpb.EditorEvent) (err error) {
	e.Type, err = protoTypeToModel(pe.GetType())
	if err != nil {
		return
	}
	if pe.GetResourceName().GetUri() != "" {
		e.URI, err = textpb.NewURIFromProto(pe.GetResourceName())
		if err != nil {
			return
		}
	}
	if pe.ResourceId != 0 {
		e.Resource = Token{
			Token:    browser.Token{ID: uint64(pe.GetResourceId())},
			resource: e.URI,
		}
	}
	e.Start = pe.GetStart().ToModel()
	e.End = pe.GetEnd().ToModel()
	e.From = pe.GetFrom().ToModel()
	e.To = pe.GetTo().ToModel()
	e.Content = pe.GetContent()
	e.cmdArgs = pe.GetCmdArgs()
	return nil
}

func (e Event) protoType() textpb.EditorEvent_Type {
	switch e.Type {
	case EventTypeClose:
		return textpb.EditorEvent_TypeClose
	case EventTypeFlush:
		return textpb.EditorEvent_TypeFlush
	case EventTypeOpen:
		return textpb.EditorEvent_TypeOpen
	case EventTypeEdit:
		return textpb.EditorEvent_TypeEdit
	case EventTypeScroll:
		return textpb.EditorEvent_TypeScroll
	case EventTypeCursor:
		return textpb.EditorEvent_TypeCursor
	case EventTypeFocus:
		return textpb.EditorEvent_TypeFocus
	case EventTypeUnfocus:
		return textpb.EditorEvent_TypeUnfocus
	case eventTypeCommand:
		return textpb.EditorEvent_TypeCommand
	default:
		panic(fmt.Sprintf("failed to convert editor event to proto: invalid type: %v", e.Type))
	}
}

// expects ev Resource to be a browser.Token
func (e *Event) toProto() textpb.EditorEvent {
	ret := textpb.EditorEvent{}
	ret.Type = e.protoType()

	ret.ResourceName = textpb.NewURI(e.URI)
	if e.Resource != nil {
		ret.ResourceId = uint32(e.Resource.(Token).ID)
	}

	var start, end, from, to termpb.Coordinates
	start.FromModel(e.Start)
	end.FromModel(e.End)
	from.FromModel(e.From)
	to.FromModel(e.To)

	ret.Start = &start
	ret.End = &end
	ret.Content = e.Content
	ret.From = &from
	ret.To = &to
	ret.CmdArgs = e.cmdArgs

	return ret
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

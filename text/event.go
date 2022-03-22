package text

//go:generate mockgen -destination=./event_handler_gomock.go -package text -self_package text -source event.go

import (
	"context"
	"fmt"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
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

	// EventTypeUpdate is dispatched when new content is inserted into an editor buffer.
	// Start, End represent the input to Update whereas
	// From, To represent output coordinates. See cell.Writer.Update for
	// more details.
	EventTypeUpdate

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
	Type         EventType
	ResourceName string
	Resource     Handler

	Start, End term.Coordinates
	From, To   term.Coordinates
	Content    string

	// used internally by server/client
	cmdArgs []string
}

func protoTypeToModel(protoType proto.EditorEvent_Type) (ev EventType, err error) {
	switch protoType {
	case proto.EditorEvent_TypeClose:
		ev = EventTypeClose
	case proto.EditorEvent_TypeFlush:
		ev = EventTypeFlush
	case proto.EditorEvent_TypeOpen:
		ev = EventTypeOpen
	case proto.EditorEvent_TypeUpdate:
		ev = EventTypeUpdate
	case proto.EditorEvent_TypeScroll:
		ev = EventTypeScroll
	case proto.EditorEvent_TypeCursor:
		ev = EventTypeCursor
	case proto.EditorEvent_TypeFocus:
		ev = EventTypeFocus
	case proto.EditorEvent_TypeUnfocus:
		ev = EventTypeUnfocus
	case proto.EditorEvent_TypeCommand:
		ev = eventTypeCommand
	default:
		err = fmt.Errorf("failed to convert proto editor event: invalid type: %v",
			protoType)
	}

	return
}

func (e *Event) fromProto(pe *proto.EditorEvent) (err error) {
	e.Type, err = protoTypeToModel(pe.GetType())
	if err != nil {
		return
	}
	e.ResourceName = pe.GetResourceName()
	if pe.ResourceId != 0 {
		e.Resource = Token{
			Token:    browser.Token{ID: uint64(pe.GetResourceId())},
			resource: e.ResourceName,
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

func (e Event) protoType() proto.EditorEvent_Type {
	switch e.Type {
	case EventTypeClose:
		return proto.EditorEvent_TypeClose
	case EventTypeFlush:
		return proto.EditorEvent_TypeFlush
	case EventTypeOpen:
		return proto.EditorEvent_TypeOpen
	case EventTypeUpdate:
		return proto.EditorEvent_TypeUpdate
	case EventTypeScroll:
		return proto.EditorEvent_TypeScroll
	case EventTypeCursor:
		return proto.EditorEvent_TypeCursor
	case EventTypeFocus:
		return proto.EditorEvent_TypeFocus
	case EventTypeUnfocus:
		return proto.EditorEvent_TypeUnfocus
	case eventTypeCommand:
		return proto.EditorEvent_TypeCommand
	default:
		panic(fmt.Sprintf("failed to convert editor event to proto: invalid type: %v", e.Type))
	}
}

// expects ev Resource to be a browser.Token
func (e *Event) toProto() proto.EditorEvent {
	ret := proto.EditorEvent{}
	ret.Type = e.protoType()

	ret.ResourceName = e.ResourceName
	if e.Resource != nil {
		ret.ResourceId = uint32(e.Resource.(Token).ID)
	}

	var start, end, from, to proto.Coordinates
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
	name string
	h    Handler
	eh   EventHandler

	onWillUpdateStr   string
	onWillUpdateStart term.Coordinates
	onWillUpdateEnd   term.Coordinates
}

func (s *cellSubscriber) OnWillUpdate(start, end term.Coordinates, str string) {
	s.onWillUpdateStr = str
	s.onWillUpdateStart = start
	s.onWillUpdateEnd = end
}

func (s *cellSubscriber) OnDidUpdate(from, to term.Coordinates, old string) {
	s.eh.Handle(context.Background(), Event{
		Type:         EventTypeUpdate,
		Resource:     s.h,
		ResourceName: s.name,
		From:         from,
		To:           to,
		Start:        s.onWillUpdateStart,
		End:          s.onWillUpdateEnd,
		Content:      s.onWillUpdateStr,
	})
}

// CellSubscriber returns a cell.Subscriber which forwards Insert/Delete events to evHandler
func CellSubscriber(name string, h Handler, evHandler EventHandler) cell.Subscriber {
	return &cellSubscriber{name: name, h: h, eh: evHandler}
}

type scrollSubscriber struct {
	name string
	h    Handler
	eh   EventHandler
}

func (s scrollSubscriber) OnSeek(at term.Coordinates) {
	s.eh.Handle(context.Background(), Event{
		Type:         EventTypeScroll,
		Resource:     s.h,
		ResourceName: s.name,
		Start:        at,
	})
}

// ScrollSubscriber returns a component.ScrollSubscriber which forwarsd Scroll events to evHandler
func ScrollSubscriber(name string, h Handler, evHandler EventHandler) component.ScrollSubscriber {
	return scrollSubscriber{name: name, h: h, eh: evHandler}
}

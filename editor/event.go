package editor

//go:generate mockgen -destination=./event_handler_gomock.go -package editor -self_package editor -source event.go

import (
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

	// EventTypeInsert is dispatched when new content is inserted into an editor buffer.
	// Start, End represent the []byte coordinates.
	// From, To represent the raw [][]term.Cell coordinates, which account
	// for tab expansion. Content represents the content that was inserted.
	EventTypeInsert

	// EventTypeDelete is dispatched when content is deleted from an editor buffer.
	// See EventTypeInsert. Content represents the content that was deleted.
	EventTypeDelete

	// EventTypeScroll is dispatched when content is scroll to a new offset.
	// Start represents the scroll offset.
	EventTypeScroll

	// EventTypeFocus is dispatched when an editor handler is on browser.Focus.
	EventTypeFocus

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
	case proto.EditorEvent_TypeDelete:
		ev = EventTypeDelete
	case proto.EditorEvent_TypeInsert:
		ev = EventTypeInsert
	case proto.EditorEvent_TypeScroll:
		ev = EventTypeScroll
	case proto.EditorEvent_TypeFocus:
		ev = EventTypeFocus
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
	e.Resource = browser.Token{ID: uint64(pe.GetResourceId())}
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
	case EventTypeDelete:
		return proto.EditorEvent_TypeDelete
	case EventTypeInsert:
		return proto.EditorEvent_TypeInsert
	case EventTypeScroll:
		return proto.EditorEvent_TypeScroll
	case EventTypeFocus:
		return proto.EditorEvent_TypeFocus
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
	ret.ResourceId = uint32(e.Resource.(browser.Token).ID)

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

	onWillInsert     string
	onWillInsertAt   term.Coordinates
	onWillDeleteFrom term.Coordinates
	onWillDeleteTo   term.Coordinates
}

func (s *cellSubscriber) OnWillInsert(at term.Coordinates, str string) {
	s.onWillInsert = str
	s.onWillInsertAt = at
}

// cell.Writer API does not provide access to the "end" before tab expansion.
func calculateInsertEnd(at term.Coordinates, str string) term.Coordinates {
	var lines int
	var lastLineLen int
	for _, r := range str {
		if r == '\n' {
			lines++
			lastLineLen = 0
			continue
		}
		lastLineLen++
	}
	if lastLineLen > 0 {
		lastLineLen--
	}
	return term.Coordinates{Y: at.Y + lines, X: lastLineLen}
}

func (s *cellSubscriber) OnDidInsert(from, to term.Coordinates) {
	s.eh.Handle(Event{
		Type:         EventTypeInsert,
		Resource:     s.h,
		ResourceName: s.name,
		Start:        s.onWillInsertAt,
		End:          calculateInsertEnd(s.onWillInsertAt, s.onWillInsert),
		From:         from,
		To:           to,
		Content:      s.onWillInsert,
	})
}

func (s *cellSubscriber) OnWillDelete(from, to term.Coordinates) {
	s.onWillDeleteFrom = from
	s.onWillDeleteTo = to
}

func (s *cellSubscriber) OnDidDelete(start, end term.Coordinates, str string) {
	s.eh.Handle(Event{
		Type:         EventTypeDelete,
		Resource:     s.h,
		ResourceName: s.name,
		Start:        start,
		End:          end,
		From:         s.onWillDeleteFrom,
		To:           s.onWillDeleteTo,
		Content:      str,
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
	s.eh.Handle(Event{
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

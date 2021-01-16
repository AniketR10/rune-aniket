package editor

//go:generate mockgen -destination=./event_handler_gomock.go -package editor -self_package editor -source event.go

import (
	"fmt"

	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
)

// EventType is a type of editor event.
type EventType uint8

const (
	// EventTypeOpen is dispatched when an editor is called the Edit method.
	EventTypeOpen EventType = iota

	// EventTypeClose is dispatched when an editor buffer is closed.
	EventTypeClose

	// EventTypeFlush is dispatched when an editor buffer is Flushed.
	EventTypeFlush
)

// Event encapsulates eventual information about a particular editor resource.
type Event struct {
	Type         EventType
	ResourceName string
	Resource     Handler
}

func protoTypeToModel(protoType proto.EditorEvent_Type) (ev EventType, err error) {
	switch protoType {
	case proto.EditorEvent_TypeClose:
		ev = EventTypeClose
	case proto.EditorEvent_TypeFlush:
		ev = EventTypeFlush
	case proto.EditorEvent_TypeOpen:
		ev = EventTypeOpen
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
	e.Resource = handler.Token{ID: pe.GetResourceId()}
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
	default:
		panic(fmt.Sprintf("failed to convert editor event to proto: invalid type: %v", e.Type))
	}
}

// expects ev Resource to be a handler.Token
func (e *Event) toProto() proto.EditorEvent {
	ret := proto.EditorEvent{}
	ret.Type = e.protoType()

	ret.ResourceName = e.ResourceName
	ret.ResourceId = e.Resource.(handler.Token).ID

	return ret
}

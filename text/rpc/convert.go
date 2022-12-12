package rpc

import (
	"fmt"

	textapi "unstable.build/go-tui/api/text"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/text"
)

func protoTypeToModel(protoType EditorEvent_Type) (ev textapi.EventType, err error) {
	switch protoType {
	case EditorEvent_TypeClose:
		ev = textapi.EventTypeClose
	case EditorEvent_TypeFlush:
		ev = textapi.EventTypeFlush
	case EditorEvent_TypeOpen:
		ev = textapi.EventTypeOpen
	case EditorEvent_TypeEdit:
		ev = textapi.EventTypeEdit
	case EditorEvent_TypeScroll:
		ev = textapi.EventTypeScroll
	case EditorEvent_TypeCursor:
		ev = textapi.EventTypeCursor
	case EditorEvent_TypeFocus:
		ev = textapi.EventTypeFocus
	case EditorEvent_TypeUnfocus:
		ev = textapi.EventTypeUnfocus
	case EditorEvent_TypeCommand:
		ev = text.EventTypeCommand
	default:
		err = fmt.Errorf("failed to convert proto editor event: invalid type: %v",
			protoType)
	}

	return
}

func fromProto(e *textapi.Event, pe *EditorEvent) (err error) {
	e.Type, err = protoTypeToModel(pe.GetType())
	if err != nil {
		return
	}
	if pe.GetResourceName().GetUri() != "" {
		e.URI, err = NewURIFromProto(pe.GetResourceName())
		if err != nil {
			return
		}
	}
	if pe.ResourceId != 0 {
		e.Resource = Token{
			ID:       uint64(pe.GetResourceId()),
			resource: e.URI,
		}
	}
	e.Start = pe.GetStart().ToModel()
	e.End = pe.GetEnd().ToModel()
	e.From = pe.GetFrom().ToModel()
	e.To = pe.GetTo().ToModel()
	e.Content = pe.GetContent()
	return nil
}

func protoType(e textapi.Event) EditorEvent_Type {
	switch e.Type {
	case textapi.EventTypeClose:
		return EditorEvent_TypeClose
	case textapi.EventTypeFlush:
		return EditorEvent_TypeFlush
	case textapi.EventTypeOpen:
		return EditorEvent_TypeOpen
	case textapi.EventTypeEdit:
		return EditorEvent_TypeEdit
	case textapi.EventTypeScroll:
		return EditorEvent_TypeScroll
	case textapi.EventTypeCursor:
		return EditorEvent_TypeCursor
	case textapi.EventTypeFocus:
		return EditorEvent_TypeFocus
	case textapi.EventTypeUnfocus:
		return EditorEvent_TypeUnfocus
	case text.EventTypeCommand:
		return EditorEvent_TypeCommand
	default:
		panic(fmt.Sprintf("failed to convert editor event to proto: invalid type: %v", e.Type))
	}
}

// expects ev Resource to be a browser.Token
func toProto(e textapi.Event) EditorEvent {
	var ret EditorEvent
	ret.Type = protoType(e)

	ret.ResourceName = NewURI(e.URI)
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

	return ret
}

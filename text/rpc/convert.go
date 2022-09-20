package rpc

import (
	"fmt"

	"github.com/ernestrc/go-tui/browser"
	termpb "github.com/ernestrc/go-tui/term/rpc"
	"github.com/ernestrc/go-tui/text"
)

func protoTypeToModel(protoType EditorEvent_Type) (ev text.EventType, err error) {
	switch protoType {
	case EditorEvent_TypeClose:
		ev = text.EventTypeClose
	case EditorEvent_TypeFlush:
		ev = text.EventTypeFlush
	case EditorEvent_TypeOpen:
		ev = text.EventTypeOpen
	case EditorEvent_TypeEdit:
		ev = text.EventTypeEdit
	case EditorEvent_TypeScroll:
		ev = text.EventTypeScroll
	case EditorEvent_TypeCursor:
		ev = text.EventTypeCursor
	case EditorEvent_TypeFocus:
		ev = text.EventTypeFocus
	case EditorEvent_TypeUnfocus:
		ev = text.EventTypeUnfocus
	case EditorEvent_TypeCommand:
		ev = text.EventTypeCommand
	default:
		err = fmt.Errorf("failed to convert proto editor event: invalid type: %v",
			protoType)
	}

	return
}

func fromProto(e *text.Event, pe *EditorEvent) (err error) {
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
			Token:    browser.Token{ID: uint64(pe.GetResourceId())},
			resource: e.URI,
		}
	}
	e.Start = pe.GetStart().ToModel()
	e.End = pe.GetEnd().ToModel()
	e.From = pe.GetFrom().ToModel()
	e.To = pe.GetTo().ToModel()
	e.Content = pe.GetContent()
	e.Args = pe.GetCmdArgs()
	return nil
}

func protoType(e text.Event) EditorEvent_Type {
	switch e.Type {
	case text.EventTypeClose:
		return EditorEvent_TypeClose
	case text.EventTypeFlush:
		return EditorEvent_TypeFlush
	case text.EventTypeOpen:
		return EditorEvent_TypeOpen
	case text.EventTypeEdit:
		return EditorEvent_TypeEdit
	case text.EventTypeScroll:
		return EditorEvent_TypeScroll
	case text.EventTypeCursor:
		return EditorEvent_TypeCursor
	case text.EventTypeFocus:
		return EditorEvent_TypeFocus
	case text.EventTypeUnfocus:
		return EditorEvent_TypeUnfocus
	case text.EventTypeCommand:
		return EditorEvent_TypeCommand
	default:
		panic(fmt.Sprintf("failed to convert editor event to proto: invalid type: %v", e.Type))
	}
}

// expects ev Resource to be a browser.Token
func toProto(e text.Event) EditorEvent {
	ret := EditorEvent{}
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
	ret.CmdArgs = e.Args

	return ret
}

// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package textrpc

import (
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi/textrpc"
	"github.com/unstablebuild/rune-go-sdk/term/termrpc"
)

func protoTypeToModel(protoType textrpc.EditorEvent_Type) (ev textapi.EventType, err error) {
	switch protoType {
	case textrpc.EditorEvent_TypeClose:
		ev = textapi.EventTypeClose
	case textrpc.EditorEvent_TypeFlush:
		ev = textapi.EventTypeFlush
	case textrpc.EditorEvent_TypeOpen:
		ev = textapi.EventTypeOpen
	case textrpc.EditorEvent_TypeEdit:
		ev = textapi.EventTypeEdit
	case textrpc.EditorEvent_TypeScroll:
		ev = textapi.EventTypeScroll
	case textrpc.EditorEvent_TypeHidden:
		ev = textapi.EventTypeHidden
	case textrpc.EditorEvent_TypeVisible:
		ev = textapi.EventTypeVisible
	case textrpc.EditorEvent_TypeCursor:
		ev = textapi.EventTypeCursor
	case textrpc.EditorEvent_TypeSelection:
		ev = textapi.EventTypeSelection
	case textrpc.EditorEvent_TypeCreate:
		ev = textapi.EventTypeCreate
	case textrpc.EditorEvent_TypeChange:
		ev = textapi.EventTypeChange
	case textrpc.EditorEvent_TypeFocus:
		ev = textapi.EventTypeFocus
	case textrpc.EditorEvent_TypeUnfocus:
		ev = textapi.EventTypeUnfocus
	case textrpc.EditorEvent_TypeRemove:
		ev = textapi.EventTypeRemove
	case textrpc.EditorEvent_TypeRename:
		ev = textapi.EventTypeRename
	default:
		err = fmt.Errorf("failed to convert proto editor event: invalid type: %v",
			protoType)
	}

	return
}

func fromProto(e *textapi.Event, pe *textrpc.EditorEvent) (err error) {
	e.Type, err = protoTypeToModel(pe.GetType())
	if err != nil {
		return
	}
	if pe.GetResourceName().GetUri() != "" {
		e.URI, err = textrpc.NewURIFromProto(pe.GetResourceName())
		if err != nil {
			return
		}
	}
	if pe.ResourceName != nil {
		uri, err := textrpc.NewURIFromProto(pe.GetResourceName())
		if err != nil {
			return err
		}
		e.Resource = textrpc.Token{
			URI: uri,
		}
	}
	e.Start = pe.GetStart().ToModel()
	e.End = pe.GetEnd().ToModel()
	e.From = pe.GetFrom().ToModel()
	e.To = pe.GetTo().ToModel()
	e.Content = pe.GetContent()
	return nil
}

func protoType(e textapi.Event) textrpc.EditorEvent_Type {
	switch e.Type {
	case textapi.EventTypeClose:
		return textrpc.EditorEvent_TypeClose
	case textapi.EventTypeFlush:
		return textrpc.EditorEvent_TypeFlush
	case textapi.EventTypeOpen:
		return textrpc.EditorEvent_TypeOpen
	case textapi.EventTypeEdit:
		return textrpc.EditorEvent_TypeEdit
	case textapi.EventTypeScroll:
		return textrpc.EditorEvent_TypeScroll
	case textapi.EventTypeHidden:
		return textrpc.EditorEvent_TypeHidden
	case textapi.EventTypeVisible:
		return textrpc.EditorEvent_TypeVisible
	case textapi.EventTypeChange:
		return textrpc.EditorEvent_TypeChange
	case textapi.EventTypeCreate:
		return textrpc.EditorEvent_TypeCreate
	case textapi.EventTypeCursor:
		return textrpc.EditorEvent_TypeCursor
	case textapi.EventTypeSelection:
		return textrpc.EditorEvent_TypeSelection
	case textapi.EventTypeFocus:
		return textrpc.EditorEvent_TypeFocus
	case textapi.EventTypeUnfocus:
		return textrpc.EditorEvent_TypeUnfocus
	case textapi.EventTypeRename:
		return textrpc.EditorEvent_TypeRename
	case textapi.EventTypeRemove:
		return textrpc.EditorEvent_TypeRemove
	default:
		panic(fmt.Sprintf("failed to convert editor event to proto: invalid type: %v", e.Type))
	}
}

// expects ev Resource to be a browser.Token
func toProto(e textapi.Event) textrpc.EditorEvent {
	var ret textrpc.EditorEvent
	ret.Type = protoType(e)

	ret.ResourceName = NewURI(e.URI)

	var start, end, from, to termrpc.Coordinates
	start.FromModel(e.Start)
	end.FromModel(e.End)
	from.FromModel(e.From)
	to.FromModel(e.To)

	ret.Start = &start
	ret.End = &end
	ret.Content = e.Content
	ret.From = &from
	ret.To = &to

	return ret //nolint:govet
}

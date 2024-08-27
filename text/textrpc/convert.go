// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package textrpc

import (
	"fmt"

	textapi "unstable.build/go-tui/api/text"
	termpb "unstable.build/go-tui/term/rpc"
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
	case EditorEvent_TypeSelection:
		ev = textapi.EventTypeSelection
	case EditorEvent_TypeFocus:
		ev = textapi.EventTypeFocus
	case EditorEvent_TypeUnfocus:
		ev = textapi.EventTypeUnfocus
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
	if pe.ResourceName != nil {
		uri, err := NewURIFromProto(pe.GetResourceName())
		if err != nil {
			return err
		}
		e.Resource = Token{
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
	case textapi.EventTypeSelection:
		return EditorEvent_TypeSelection
	case textapi.EventTypeFocus:
		return EditorEvent_TypeFocus
	case textapi.EventTypeUnfocus:
		return EditorEvent_TypeUnfocus
	default:
		panic(fmt.Sprintf("failed to convert editor event to proto: invalid type: %v", e.Type))
	}
}

// expects ev Resource to be a browser.Token
func toProto(e textapi.Event) EditorEvent {
	var ret EditorEvent
	ret.Type = protoType(e)

	ret.ResourceName = NewURI(e.URI)

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

	return ret //nolint:govet
}

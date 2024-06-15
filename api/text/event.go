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

	// EventTypeSelection is dispatched when the user selects a chunk of text.
	// Start, End represent the selection coordinates and Content contains
	// the content selected as a result.
	EventTypeSelection
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

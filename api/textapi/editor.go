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

package textapi

import (
	"context"

	"unstable.build/go-tui/api/browserapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/term"
)

// Handler just wraps a tui.Handler to indicate that this API's handlers might
// not be compatible with other APIs.
type Handler interface {
	browserapi.Handler

	// This is only used to differentiate editor.Handler from the rest
	// of tui.Handler in a browser.Component.
	Resource() workspaceapi.URI
}

// CellEditor is a cell.Editor that can fail.
type CellEditor interface {
	Edit(ctx context.Context, start, end term.Coordinates, str string) (
		from, to term.Coordinates, old string, err error,
	)
}

// CellView wraps a subset of cell.View behaviour with an API that can fail.
type CellView interface {
	RawCells() ([][]term.Cell, error)
}

// LocationPriority defines the order of which location attributes and
// messages are processed.
type LocationPriority uint

const (
	// LocationPriorityInfo defines an informational location.
	LocationPriorityInfo LocationPriority = iota
	// LocationPriorityWarning defines a warning location.
	LocationPriorityWarning
	// LocationPriorityError defines an error location.
	LocationPriorityError
	// LocationPriorityCritical defines an critical location.
	LocationPriorityCritical
)

// CommandRegister abstracts the ability to register new commands.
type CommandRegister interface {
	// RegisterCommand registers command to be dispatched to CommandHandler.
	RegisterCommand(CommandManual, CommandHandler) error
}

// Editor is the interface that wraps an API to manage a text editor.
type Editor interface {
	// SubscribeEvents subscribes EventHandler to events of type EventType.
	SubscribeEvents([]EventType, EventHandler) error

	// Editor returns the editor.Handler that manages the given resource,
	// if there is a resource currently open with the given URI.
	Editor(resource workspaceapi.URI) (Handler, error)

	// SetLocationList sets the Handler's location list for users to
	// navigate the code. See LocationList for more details.
	// In order to remove a location list, SetLocationList must be called
	// with an empty (or nil) LocationList.
	// Locations are removed if underlying buffer is updated. It is the
	// responsibility of the caller to recompute the list of locations
	// and call SetLocationList with the new list of locations after
	// every update. Check cell.Buffer.Subscribe for more details.
	SetLocationList(Handler, LocationPriority, string, LocationList) error

	// Moves cursor to the next location on list with ID.
	MoveToNextLocation(h Handler, ID string) error

	// Moves cursor to the previous location on list with ID.
	MoveToPrevLocation(h Handler, ID string) error

	// Cursor gets the position of Handler's cursor in the underlying
	// content buffer.
	Cursor(Handler) (term.Coordinates, error)

	// SetCursor sets the cursor of Handler to the given Coordinates.
	SetCursor(Handler, term.Coordinates) error

	// CellView returns a CellView which allows to read the editor's internal buffer.
	CellView(Handler) CellView

	// CellEditor returns a CellEditor which allows for direct write access
	// to the editor's internal buffer.
	CellEditor(Handler) CellEditor

	// SetDefaultAttributes sets the default attributes of the given Handler
	// before any LocationList overwrites.
	SetDefaultAttributes(Handler, term.Attributes) error
}

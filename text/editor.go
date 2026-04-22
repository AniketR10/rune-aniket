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

package text

import (
	"context"
	"errors"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
)

// ErrCommandNotRegistered is returned by UnsubscribeCommand and
// UnregisterREPLCommand when the named command is not registered on the
// editor. Callers that want to tolerate the absence of a prior
// registration should compare against this error with errors.Is.
var ErrCommandNotRegistered = errors.New("command not registered")

// Handler just wraps a tui.Handler to indicate that this API's handlers might
// not be compatible with other APIs.
type Handler interface {
	browserapi.Handler
	component.Scrollable

	// This is only used to differentiate editor.Handler from the rest
	// of tui.Handler in a browser.Component.
	Resource() workspaceapi.URI

	// SetWraps defines wheter editor handler should wrap that are longer than
	// available width into the next line or simply truncate them in the view.
	SetWrap(wrap bool)

	// ShowCommandBar hides or shows the editor's command bar.
	ShowCommandBar(show bool)

	// SetCursorAtScroll sets the cursor of this handler at scroll coordinates
	// determined by pos.
	SetCursorAtScroll(pos term.Coordinates) bool

	// CursorAtScroll gets the position of Handler's cursor in the underlying
	// content buffer.
	CursorAtScroll() term.Coordinates

	// SetLocationList sets the Handler's location list for users to
	// navigate the code. See LocationList for more details.
	// In order to remove a location list, SetLocationList must be called
	// with an empty (or nil) LocationList.
	// Locations are removed if underlying buffer is updated. It is the
	// responsibility of the caller to recompute the list of locations
	// and call SetLocationList with the new list of locations after
	// every update. Check cell.Buffer.Subscribe for more details.
	SetLocationList(textapi.LocationPriority, string, LocationList)

	// LocationLists returns a map of location list IDs to their
	// respective locations.
	LocationLists() []LocationSet

	// MoveToNextLocation cursor to the next location on list with ID.
	MoveToNextLocation(ID string) bool

	// MoveToPrevLocation cursor to the previous location on list with ID.
	MoveToPrevLocation(ID string) bool

	// CellView returns a cell.View which allows to read the editor's internal buffer.
	CellView() cell.View

	// CellEditor returns a cell.Editor which allows for direct write access
	// to the editor's internal buffer.
	CellEditor() cell.Editor

	// SetDefaultAttributes sets the default attributes of the given Handler
	// before any LocationList overwrites.
	SetDefaultAttributes(term.Attributes)

	// Dimensions reports the ideal cell width/height this handler
	// would claim to render its buffer without clipping, including
	// any auxiliary chrome (line numbers, folds, git icons, …) the
	// handler draws on top of the buffer. Parents wanting to avoid
	// truncation should size the hosting window to at least these
	// dimensions. Satisfies component.Floating.
	Dimensions() (width, height int)
}

// EventPublisher wraps subscribing and unsubscribing to file events.
type EventPublisher interface {
	// SubscribeEvents subscribes EventHandler to events of type EventType.
	SubscribeEvents([]textapi.EventType, EventHandler) error

	// UnsubscribeEvents unsubscribes the given event handler from
	// all events.
	UnsubscribeEvents(EventHandler) (bool, error)
}

// Editor is the interface that wraps an API to manage a text editor.
type Editor interface {
	EventPublisher
	// Edit opens a file and returns an editor.Handler to edit it or an error
	// if there was an error opening it.
	Edit(
		file workspaceapi.URI, buf *cell.Buffer, readOnly, recovered bool,
	) (Handler, error)

	// Editor returns the editor.Handler with name or returns
	// an error if no editor with name is open via Edit.
	Editor(workspaceapi.URI) (Handler, error)

	// SubscribeCommand registers command to be dispatched to CommandHandler.
	SubscribeCommand(textapi.CommandManual, CommandHandler) error

	// RegisterREPLCommand registers a REPL command to be dispatched to a
	// textapi.REPLHandler.
	RegisterREPLCommand(textapi.CommandManual, textapi.REPLHandler) error

	// UnsubscribeCommand un-registers command. Returns
	// ErrCommandNotRegistered if no such command is registered.
	UnsubscribeCommand(string) error

	// UnregisterREPLCommand un-registers a previously registered REPL
	// command. Returns ErrCommandNotRegistered if no such command is
	// registered.
	UnregisterREPLCommand(string) error
}

// NewREPLHandler returns a REPL handler backed by router.
func NewREPLHandler(comp *Component) textapi.REPLHandler {
	return replHandler{router: comp}
}

// ViewDimensions returns the visual (width, height) needed to render
// the given cell.View without clipping. This is the canonical way for
// leaf text handlers to report their ideal dimensions: the width is
// the widest row's VISUAL width (summing each cell's monospace width
// — cells can hold wide glyphs like Nerd Font icons or CJK, which
// occupy two cells on screen but only one entry in the backing
// slice), and the height is the buffer's row count.
//
// Using View.Columns(y) directly would under-report width for any row
// containing a width-2 glyph, causing a hosting window to be sized
// one cell too narrow and clipping the last character.
func ViewDimensions(v cell.View) (width, height int) {
	height = v.Rows()
	for _, row := range v.RawCells() {
		w := 0
		for _, c := range row {
			if c.Width > 1 {
				w += int(c.Width)
				continue
			}
			w++
		}
		if w > width {
			width = w
		}
	}
	return
}

// replHandler routes REPL commands through a text.Component.
type replHandler struct {
	router *Component
}

// HandleCommand dispatches cmd to a registered REPL handler.
func (h replHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, pw repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	handler, ok := h.router.REPLCommand(cmd.Name)
	if !ok {
		return nil, repl.ErrNotFound
	}
	return handler.HandleCommand(ctx, cmd, pw)
}

// Complete dispatches shell completion to a registered REPL handler.
func (h replHandler) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	handler, ok := h.router.REPLCommand(cmd)
	if !ok {
		return iterator.Empty[string](), nil
	}
	return handler.Complete(ctx, cmd, args)
}

// Help dispatches shell help requests to a registered REPL handler.
func (h replHandler) Help(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return iterator.Empty[component.Responsive](), nil
	}
	handler, ok := h.router.REPLCommand(args[0])
	if !ok {
		return nil, repl.ErrNotFound
	}
	return handler.Help(ctx, args[1:])
}

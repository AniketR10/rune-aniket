// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/text/streamload"
)

var (
	_ Handler              = (*deferHandler)(nil)
	_ component.Scrollable = (*deferHandler)(nil)
)

// deferHandler is the text.Handler wrapper installed on streaming
// tabs in place of a bare *streamload.Handler. Without it, IDE call
// sites that type-assert tab handlers to text.Handler (session
// restore, ex commands, idecursor navigation, syntax jumps, RPC)
// silently fail for the whole async-load window. Mutating calls
// received before Swap are queued and replayed on the real handler;
// calls received after Swap forward directly.
type deferHandler struct {
	*streamload.Handler

	mu      sync.Mutex
	real    Handler
	pending deferPendingOps
}

// cursor / wrap / showCommandBar / defaultAttrs follow last-wins
// semantics; locationLists preserves insertion order so replay
// matches the caller's intent.
type deferPendingOps struct {
	cursor         *term.Coordinates
	wrap           *bool
	showCommandBar *bool
	defaultAttrs   *term.Attributes
	locationLists  []deferPendingLocList
}

type deferPendingLocList struct {
	priority textapi.LocationPriority
	id       string
	list     LocationList
}

func newDeferHandler(sh *streamload.Handler) *deferHandler {
	if sh == nil {
		panic("text: nil *streamload.Handler passed to newDeferHandler")
	}
	return &deferHandler{Handler: sh}
}

// Swap installs real as the destination for subsequent text.Handler
// calls and replays queued mutations on it. Cursor is replayed last
// so it lands on top of any location-list recompute the real handler
// triggers.
func (h *deferHandler) Swap(real Handler) {
	if real == nil {
		panic("text: deferHandler.Swap called with nil Handler")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.real = real
	pending := h.pending
	h.pending = deferPendingOps{}
	if pending.defaultAttrs != nil {
		real.SetDefaultAttributes(*pending.defaultAttrs)
	}
	if pending.wrap != nil {
		real.SetWrap(*pending.wrap)
	}
	if pending.showCommandBar != nil {
		real.ShowCommandBar(*pending.showCommandBar)
	}
	for _, op := range pending.locationLists {
		real.SetLocationList(op.priority, op.id, op.list)
	}
	if pending.cursor != nil {
		real.SetCursorAtScroll(*pending.cursor)
	}
}

func (h *deferHandler) Resource() workspaceapi.URI { return h.Handler.URI() }

func (h *deferHandler) SetWrap(wrap bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.real != nil {
		h.real.SetWrap(wrap)
		return
	}
	h.pending.wrap = &wrap
}

func (h *deferHandler) ShowCommandBar(show bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.real != nil {
		h.real.ShowCommandBar(show)
		return
	}
	h.pending.showCommandBar = &show
}

// SetCursorAtScroll returns true even when queued so callers do not
// fall back to alternative cursor-setting paths during the
// async-load window.
func (h *deferHandler) SetCursorAtScroll(pos term.Coordinates) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.real != nil {
		return h.real.SetCursorAtScroll(pos)
	}
	h.pending.cursor = &pos
	return true
}

func (h *deferHandler) CursorAtScroll() term.Coordinates {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.real != nil {
		return h.real.CursorAtScroll()
	}
	if h.pending.cursor != nil {
		return *h.pending.cursor
	}
	return term.Coordinates{}
}

func (h *deferHandler) SetLocationList(
	pri textapi.LocationPriority, id string, l LocationList,
) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.real != nil {
		h.real.SetLocationList(pri, id, l)
		return
	}
	h.pending.locationLists = append(h.pending.locationLists,
		deferPendingLocList{priority: pri, id: id, list: l})
}

func (h *deferHandler) LocationLists() []LocationSet {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.real != nil {
		return h.real.LocationLists()
	}
	return nil
}

func (h *deferHandler) MoveToNextLocation(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.real != nil {
		return h.real.MoveToNextLocation(id)
	}
	return false
}

func (h *deferHandler) MoveToPrevLocation(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.real != nil {
		return h.real.MoveToPrevLocation(id)
	}
	return false
}

// The streaming buffer is not exposed as the pre-swap CellView
// because coordinates produced against it would not survive the
// swap to the real editor buffer.
var deferEmptyView = cell.NewBuffer().View()

func (h *deferHandler) CellView() cell.View {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.real != nil {
		return h.real.CellView()
	}
	return deferEmptyView
}

func (h *deferHandler) CellEditor() cell.Editor {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.real != nil {
		return h.real.CellEditor()
	}
	return deferNoopCellEditor{}
}

func (h *deferHandler) SetDefaultAttributes(attrs term.Attributes) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.real != nil {
		h.real.SetDefaultAttributes(attrs)
		return
	}
	h.pending.defaultAttrs = &attrs
}

func (h *deferHandler) Dimensions() (width, height int) {
	h.mu.Lock()
	real := h.real
	h.mu.Unlock()
	if real != nil {
		return real.Dimensions()
	}
	return h.Handler.Dimensions()
}

func (h *deferHandler) IsSearchMode() bool {
	h.mu.Lock()
	real := h.real
	h.mu.Unlock()
	if real != nil {
		return real.IsSearchMode()
	}
	return h.Handler.InSearchMode()
}

type deferNoopCellEditor struct{}

func (deferNoopCellEditor) Edit(
	_ context.Context, start, _ term.Coordinates, _ string,
) (from, to term.Coordinates, old string) {
	return start, start, ""
}

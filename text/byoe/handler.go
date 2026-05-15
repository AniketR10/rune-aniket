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

package byoe

import (
	"context"
	"errors"
	"os"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/term/vte"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

// editorHandler wraps a *vte.Handler so that the vte's tui.Handler
// surface (Draw / Resize / Handle / Cursor / Selection / scrolling /
// mouse / clipboard / focus) is inherited by embedding, and adds the
// extra text.Handler methods Rune needs to plug the external editor
// into the rest of the IDE.
type editorHandler struct {
	*vte.Handler

	buf          *cell.Buffer
	resource     workspaceapi.URI
	gotoTemplate gotoTemplate

	cwd              workspace.Workspace
	notifications    browserapi.Notifications
	scheduleNextTick func(func()) bool

	watchID     int
	watchActive bool
	cancelCtx   context.CancelFunc
}

// newHandler builds a wrapper around vteH, hooks the file watcher, and
// returns the handler. The watcher loop is spawned synchronously; the
// caller can stop it via Close.
func newHandler(
	vteH *vte.Handler, buf *cell.Buffer, uri workspaceapi.URI,
	gotoTpl gotoTemplate, cwd workspace.Workspace,
	notifications browserapi.Notifications,
	scheduleNextTick func(func()) bool,
) *editorHandler {
	ctx, cancel := context.WithCancel(context.Background())
	h := &editorHandler{
		Handler:          vteH,
		buf:              buf,
		resource:         uri,
		gotoTemplate:     gotoTpl,
		cwd:              cwd,
		notifications:    notifications,
		scheduleNextTick: scheduleNextTick,
		cancelCtx:        cancel,
	}
	h.startWatcher(ctx)
	return h
}

// startWatcher subscribes to Write/Rename events for the underlying
// file. On each event, the file contents are re-read off the event-loop
// goroutine and the buffer is replaced via scheduleNextTick.
func (h *editorHandler) startWatcher(ctx context.Context) {
	ch := make(chan schemeapi.EventInfo, 8)
	id, err := h.cwd.Watch(h.resource.Path(), ch,
		schemeapi.Write, schemeapi.Rename)
	if err != nil {
		_, _ = h.notifications.Notify(
			browserapi.LevelWarn,
			"byoe: watch %s: %v", h.resource.Path(), err)
		return
	}
	h.watchID = id
	h.watchActive = true
	go debug.CapturePanicReport(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				h.scheduleReload(ev.URI())
			}
		}
	})
}

// scheduleReload runs on the watcher goroutine. It reads the file
// contents (off the UI goroutine), then schedules a single buffer
// replace on the UI goroutine.
func (h *editorHandler) scheduleReload(uri workspaceapi.URI) {
	data, err := readAll(h.cwd, uri.Path())
	if err != nil {
		// External editors typically save via "write to tmp, then
		// rename" so the watcher routinely sees a transient
		// not-exist window between the rename and the next watcher
		// event. Don't spam the user with a warn for what is
		// expected protocol noise; just wait for the next event.
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		_, _ = h.notifications.Notify(
			browserapi.LevelWarn,
			"byoe: reload %s: %v", uri.Path(), err)
		return
	}
	h.scheduleNextTick(func() {
		h.replaceBuffer(data)
	})
}

// replaceBuffer atomically replaces the cell.Buffer contents in a
// single Edit so subscribers see one version bump.
func (h *editorHandler) replaceBuffer(data string) {
	view := h.buf.View()
	rows := view.Rows()
	endY := 0
	endX := 0
	if rows > 0 {
		endY = rows - 1
		endX = view.Columns(endY)
	}
	h.buf.Edit(context.Background(),
		term.Coordinates{Y: 0, X: 0},
		term.Coordinates{Y: endY, X: endX},
		data)
}

// Resource satisfies text.Handler.
func (h *editorHandler) Resource() workspaceapi.URI { return h.resource }

// CursorAtScroll returns zero — v1 byoe has no IPC back from the
// external editor.
func (h *editorHandler) CursorAtScroll() term.Coordinates {
	return term.Coordinates{}
}

// SetCursorAtScroll injects the configured goto sequence into the
// embedded vte. Returns true when the template is non-empty.
func (h *editorHandler) SetCursorAtScroll(pos term.Coordinates) bool {
	if h.gotoTemplate.IsEmpty() {
		return false
	}
	keys := h.gotoTemplate.Render(pos.Y+1, pos.X+1)
	for _, k := range keys {
		_, _ = h.Handler.Handle(keyCombToEvent(k))
	}
	return true
}

// SetWrap is a nop — the external editor manages its own wrapping.
func (h *editorHandler) SetWrap(wrap bool) {}

// ShowCommandBar is a nop.
func (h *editorHandler) ShowCommandBar(show bool) {}

// SetLocationList is a nop.
func (h *editorHandler) SetLocationList(
	textapi.LocationPriority, string, text.LocationList,
) {
}

// LocationLists returns nil.
func (h *editorHandler) LocationLists() []text.LocationSet { return nil }

// MoveToNextLocation is a nop.
func (h *editorHandler) MoveToNextLocation(string) bool { return false }

// MoveToPrevLocation is a nop.
func (h *editorHandler) MoveToPrevLocation(string) bool { return false }

// SetDefaultAttributes is a nop.
func (h *editorHandler) SetDefaultAttributes(term.Attributes) {}

// IsSearchMode returns false.
func (h *editorHandler) IsSearchMode() bool { return false }

// CellView returns the watcher-synced buffer view.
func (h *editorHandler) CellView() cell.View { return h.buf.View() }

// CellEditor returns a no-op cell.Editor: the external editor is the
// only writer to the file and Rune-level edits would fight with it.
func (h *editorHandler) CellEditor() cell.Editor { return nopCellEditor{} }

// Dimensions reports the ideal size needed to render the buffer
// without clipping.
func (h *editorHandler) Dimensions() (int, int) {
	return text.ViewDimensions(h.buf.View())
}

// Close stops the watcher then defers to the embedded vte.Handler.
func (h *editorHandler) Close() error {
	h.cancelCtx()
	if h.watchActive {
		_ = h.cwd.StopWatch(h.watchID)
		h.watchActive = false
	}
	return h.Handler.Close()
}

// nopCellEditor satisfies cell.Editor without mutating anything.
// External-editor mode forbids Rune-level edits.
type nopCellEditor struct{}

func (nopCellEditor) Edit(
	ctx context.Context, start, end term.Coordinates, _ string,
) (from, to term.Coordinates, old string) {
	return start, end, ""
}

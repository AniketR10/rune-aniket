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

package exoeditor

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/term/vte"
	"unstable.build/go-tui/term/vte/vteprobe"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

const gracefulQuitTimeout = 30 * time.Second

type editorHandler struct {
	*vte.Handler

	buf          *cell.Buffer
	resource     workspaceapi.URI
	gotoTemplate gotoTemplate
	quitKeys     []term.KeyComb
	procDone     <-chan error

	cwd              workspace.Workspace
	notifications    browserapi.Notifications
	scheduleNextTick func(func()) bool
	probe            *vteprobe.Cursor
	probeSlab        *vteprobe.Slab
	watchID          int
	watchActive      bool
	cancelCtx        context.CancelFunc
	reloader         Reloader

	overrideHighlights bool
	locations          *text.LocationStore
	lastProbe          atomic.Pointer[vteprobe.Result]

	bufLines atomic.Pointer[[]string]
	bufSub   *bufLineWatcher

	probeMu    sync.Mutex
	probeCells [][]term.Cell
	component  componentSnapshotter
}

type componentSnapshotter interface {
	Snapshot() (vte.Snapshot, error)
	SnapshotInto(dst [][]term.Cell) (vte.Snapshot, error)
}

func newHandler(
	vteH *vte.Handler, buf *cell.Buffer, uri workspaceapi.URI,
	gotoTpl gotoTemplate, cwd workspace.Workspace,
	notifications browserapi.Notifications,
	scheduleNextTick func(func()) bool,
	reloader Reloader,
	overrideHighlights bool,
	quitKeys []term.KeyComb,
	procDone <-chan error,
) *editorHandler {
	if procDone == nil {
		panic("exoeditor.newHandler: procDone is required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	h := &editorHandler{
		Handler:            vteH,
		buf:                buf,
		resource:           uri,
		gotoTemplate:       gotoTpl,
		quitKeys:           quitKeys,
		procDone:           procDone,
		cwd:                cwd,
		notifications:      notifications,
		scheduleNextTick:   scheduleNextTick,
		cancelCtx:          cancel,
		reloader:           reloader,
		probe:              vteprobe.New([]int{8, 4, 2}, 0.6, 8<<20),
		probeSlab:          vteprobe.NewSlab(),
		overrideHighlights: overrideHighlights,
		locations:          text.NewLocationStore(),
	}
	h.component = vteH.Component()
	h.snapshotBufferLines()
	h.bufSub = &bufLineWatcher{h: h}
	buf.Subscribe(h.bufSub)
	h.startWatcher(ctx)
	return h
}

type bufLineWatcher struct{ h *editorHandler }

func (w *bufLineWatcher) OnWillEdit(
	context.Context, term.Coordinates, term.Coordinates, string) {
}

func (w *bufLineWatcher) OnDidEdit(
	context.Context, term.Coordinates, term.Coordinates, string) {
	w.h.snapshotBufferLines()
	w.h.refreshProbe()
}

func (h *editorHandler) startWatcher(ctx context.Context) {
	watchPath := h.resource.Path()
	dirWatch := false
	if _, err := h.cwd.Stat(watchPath); errors.Is(err, os.ErrNotExist) {
		watchPath = workspaceapi.Dir(h.resource).Path()
		dirWatch = true
	}

	ch := make(chan schemeapi.EventInfo, 8)
	id, err := h.cwd.Watch(watchPath, ch,
		schemeapi.Write, schemeapi.Rename,
		schemeapi.Create, schemeapi.Remove)
	if err != nil {
		_, _ = h.notifications.Notify(
			browserapi.LevelWarn,
			"exoeditor: watch %s: %v", watchPath, err)
		return
	}
	h.watchID = id
	h.watchActive = true
	resourceName := h.resource.Name()
	go debug.CapturePanicReport(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				if dirWatch && ev.URI().Name() != resourceName {
					continue
				}
				h.scheduleReload(h.resource)
			}
		}
	})
}

func (h *editorHandler) scheduleReload(uri workspaceapi.URI) {
	h.scheduleNextTick(func() {
		err := h.reloader.Reload(uri)
		if err == nil || errors.Is(err, workspace.ErrFlushInProgress) {
			return
		}
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		_, _ = h.notifications.Notify(
			browserapi.LevelWarn,
			"exoeditor: reload %s: %v", uri.Path(), err)
	})
}

// Resource satisfies text.Handler.
func (h *editorHandler) Resource() workspaceapi.URI { return h.resource }

func (h *editorHandler) CursorAtScroll() term.Coordinates {
	probe := h.lastProbe.Load()
	if probe == nil {
		return term.Coordinates{}
	}
	return probe.CursorAtScroll
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

func (h *editorHandler) SetLocationList(
	pri textapi.LocationPriority, id string, l text.LocationList,
) {
	h.locations.SetLocationList(pri, id, l)
}

// LocationLists returns the currently registered location lists.
func (h *editorHandler) LocationLists() []text.LocationSet {
	return h.locations.LocationLists()
}

func (h *editorHandler) LocationMessageAtCursor() string {
	locs, ok := h.locations.LocationsAtCoordinates(h.CursorAtScroll())
	if !ok {
		return ""
	}
	for _, loc := range locs {
		if loc.Message != "" {
			return loc.Message
		}
	}
	return ""
}

func (h *editorHandler) MoveToNextLocation(id string) bool {
	return h.moveToLocation(id, true)
}

func (h *editorHandler) MoveToPrevLocation(id string) bool {
	return h.moveToLocation(id, false)
}

func (h *editorHandler) moveToLocation(id string, forward bool) bool {
	l, ok := h.locations.LocationList(id)
	if !ok {
		return false
	}
	cursor := h.CursorAtScroll()
	target, ok := pickLocation(l, cursor, forward)
	if !ok {
		return false
	}
	return h.SetCursorAtScroll(target.From)
}

func pickLocation(
	l text.LocationList, cursor term.Coordinates, forward bool,
) (textapi.Location, bool) {
	rewindLocationList(l)
	first, ok := l.Current()
	if !ok {
		return textapi.Location{}, false
	}
	locs := []textapi.Location{first}
	for {
		loc, ok := l.Next()
		if !ok {
			break
		}
		locs = append(locs, loc)
	}
	if forward {
		for _, loc := range locs {
			if isAfter(loc.From, cursor) {
				return loc, true
			}
		}
		return locs[0], true
	}
	for i := len(locs) - 1; i >= 0; i-- {
		if isBefore(locs[i].From, cursor) {
			return locs[i], true
		}
	}
	return locs[len(locs)-1], true
}

func isAfter(pos, cursor term.Coordinates) bool {
	return pos.Y > cursor.Y || (pos.Y == cursor.Y && pos.X > cursor.X)
}

func isBefore(pos, cursor term.Coordinates) bool {
	return pos.Y < cursor.Y || (pos.Y == cursor.Y && pos.X < cursor.X)
}

func rewindLocationList(l text.LocationList) {
	for {
		if _, ok := l.Prev(); !ok {
			return
		}
	}
}

// SetDefaultAttributes is a nop.
func (h *editorHandler) SetDefaultAttributes(term.Attributes) {}

// IsSearchMode returns false.
func (h *editorHandler) IsSearchMode() bool { return false }

// CellView returns the watcher-synced buffer view.
func (h *editorHandler) CellView() cell.View { return h.buf.View() }

func (h *editorHandler) Draw(w term.Writer) {
	probe := h.lastProbe.Load()
	if !h.overrideHighlights {
		h.Handler.Draw(w)
		return
	}
	h.Handler.Draw(ignoreAttrWriter{Writer: w})
	if probe == nil {
		return
	}
	locs := h.locations.SortedLocations()
	drawLocations(w, locs, probe)
}

func (h *editorHandler) refreshProbe() {
	h.probeMu.Lock()
	defer h.probeMu.Unlock()

	snap, err := h.component.SnapshotInto(h.probeCells)
	if err != nil {
		return
	}
	active := snap.Active()
	// Retain the (possibly grown) backing grid for the next refresh.
	h.probeCells = active.Cells
	lines := h.bufferLines()
	res, err := h.probe.Infer(active.Cells, active.Cursor, lines, h.probeSlab)
	if err != nil {
		return
	}
	h.lastProbe.Store(&res)
}

func (h *editorHandler) bufferLines() []string {
	if p := h.bufLines.Load(); p != nil {
		return *p
	}
	return nil
}

func (h *editorHandler) snapshotBufferLines() {
	lines := vteprobe.LinesFromView(h.buf.View())
	h.bufLines.Store(&lines)
}

func (h *editorHandler) CellEditor() cell.Editor { return nopCellEditor{} }

func (h *editorHandler) Dimensions() (int, int) {
	return text.ViewDimensions(h.buf.View())
}

func (h *editorHandler) Close() error {
	h.cancelCtx()
	if h.bufSub != nil {
		h.buf.Unsubscribe(h.bufSub)
		h.bufSub = nil
	}
	if h.watchActive {
		_ = h.cwd.StopWatch(h.watchID)
		h.watchActive = false
	}
	for _, k := range h.quitKeys {
		_, _ = h.Handler.Handle(keyCombToEvent(k))
	}
	go debug.CapturePanicReport(func() {
		select {
		case <-h.procDone:
		case <-time.After(gracefulQuitTimeout):
		}
		h.scheduleNextTick(func() {
			if err := h.Handler.Close(); err != nil {
				_, _ = h.notifications.Notify(
					browserapi.LevelWarn,
					"exoeditor: close pty: %v", err)
			}
		})
	})
	return nil
}

type nopCellEditor struct{}

func (nopCellEditor) Edit(
	ctx context.Context, start, end term.Coordinates, _ string,
) (from, to term.Coordinates, old string) {
	return start, end, ""
}

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
	vteHandler

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
	probeStateMu       sync.RWMutex
	lastProbe          atomic.Pointer[vteprobe.Result]
	lastProbeCells     [][]term.Cell
	pendingGoto        *term.Coordinates

	bufCells        [][]term.Cell
	bufCellsScratch [][]term.Cell
	bufSub          *bufCellWatcher

	probeMu    sync.Mutex
	probeCells [][]term.Cell
	component  componentSnapshotter

	// inferredVersion/inferredBufGen record the component grid version
	// and file-cell generation the last successful Infer ran against, so
	// refreshProbe — called on every draw — returns immediately when
	// neither changed. The component version starts at 1, so the zero
	// value here forces the first refreshProbe to run. This keeps idle
	// editor windows free on the shared redraw that fires for any
	// window's interrupt.
	inferredVersion uint64
	inferredBufGen  uint64
	// bufCellsGen bumps whenever snapshotBufferCells replaces bufCells,
	// i.e. on every file-buffer edit.
	bufCellsGen uint64
}

type vteHandler interface {
	Handle(term.Event) (exit bool, handled bool)
	Resize(width, height int)
	Cursor() (term.Coordinates, term.CursorStyle, bool)
	Selection() (string, bool)
	Draw(term.Writer)
	Close() error
	MaxSeekOffset() int
	SeekOffset() int
	SeekUp() bool
	SeekDown() bool
}

type componentSnapshotter interface {
	Snapshot() (vte.Snapshot, error)
	SnapshotInto(dst [][]term.Cell) (vte.Snapshot, error)
	// DrawSnapshot paints the grid to w and returns a Snapshot of the
	// same grid under one lock, so the overlay can be aligned to exactly
	// what was painted.
	DrawSnapshot(w term.Writer, dst [][]term.Cell) (vte.Snapshot, error)
	Version() uint64
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
		vteHandler:         vteH,
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
	h.snapshotBufferCells()
	h.bufSub = &bufCellWatcher{h: h}
	buf.Subscribe(h.bufSub)
	h.startWatcher(ctx)
	return h
}

type bufCellWatcher struct{ h *editorHandler }

func (w *bufCellWatcher) OnWillEdit(
	context.Context, term.Coordinates, term.Coordinates, string) {
}

func (w *bufCellWatcher) OnDidEdit(
	context.Context, term.Coordinates, term.Coordinates, string) {
	w.h.snapshotBufferCells()
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

func (h *editorHandler) SetCursorAtScroll(pos term.Coordinates) bool {
	if h.gotoTemplate.IsEmpty() {
		return false
	}
	h.probeStateMu.Lock()
	if h.lastProbe.Load() == nil {
		p := pos
		h.pendingGoto = &p
		h.probeStateMu.Unlock()
		return true
	}
	h.probeStateMu.Unlock()
	h.injectGoto(pos)
	return true
}

func (h *editorHandler) injectGoto(pos term.Coordinates) {
	keys := h.gotoTemplate.Render(pos.Y+1, pos.X+1)
	for _, k := range keys {
		_, _ = h.vteHandler.Handle(keyCombToEvent(k))
	}
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
	if !h.overrideHighlights {
		h.vteHandler.Draw(w)
		return
	}
	// Paint and snapshot the grid under one component lock, then align
	// the overlay to that exact snapshot. Going through DrawSnapshot —
	// rather than drawing and snapshotting separately — is what stops the
	// parser goroutine from scrolling the grid between the cells we paint
	// and the probe we overlay, which would land highlights on the wrong
	// rows. The probe itself is only recomputed when the grid version (or
	// the file cells) changed, so an idle window redrawn for some other
	// window's interrupt still pays nothing.
	h.probeMu.Lock()
	snap, err := h.component.DrawSnapshot(ignoreAttrWriter{Writer: w}, h.probeCells)
	var flush *term.Coordinates
	if err == nil {
		flush = h.inferFromSnapshotLocked(snap, snap.Version)
	}
	h.probeMu.Unlock()

	if flush != nil {
		pos := *flush
		h.scheduleNextTick(func() { h.injectGoto(pos) })
	}

	h.probeStateMu.RLock()
	defer h.probeStateMu.RUnlock()
	probe := h.lastProbe.Load()
	if probe == nil {
		return
	}
	locs := h.locations.SortedLocations()
	drawLocations(w, locs, probe)
}

func (h *editorHandler) refreshProbe() {
	h.probeMu.Lock()
	defer h.probeMu.Unlock()

	version := h.component.Version()
	// Cheap change check first: when neither the rendered grid nor the
	// file cells changed since the last inference there is nothing to
	// do. The component version starts at 1, so the zero-valued
	// inferredVersion makes the very first call fall through and probe.
	if version == h.inferredVersion && h.bufCellsGen == h.inferredBufGen {
		return
	}

	snap, err := h.component.SnapshotInto(h.probeCells)
	if err != nil {
		return
	}
	flush := h.inferFromSnapshotLocked(snap, snap.Version)
	if flush != nil {
		pos := *flush
		h.scheduleNextTick(func() { h.injectGoto(pos) })
	}
}

// inferFromSnapshotLocked runs vteprobe against snap, stores the result
// as the current probe, and records version as the inferred revision.
// It is a no-op when version and the file-cell generation match the last
// inference. It returns a pending goto position to flush on the first
// successful probe (so the caller can schedule injectGoto outside the
// lock), or nil. The caller must hold probeMu.
func (h *editorHandler) inferFromSnapshotLocked(
	snap vte.Snapshot, version uint64,
) *term.Coordinates {
	if version == h.inferredVersion && h.bufCellsGen == h.inferredBufGen {
		return nil
	}
	active := snap.Active()
	// Retain the (possibly grown) backing grid for the next refresh.
	h.probeCells = active.Cells
	fileCells := h.bufCells
	res, err := h.probe.Infer(active.Cells, active.Cursor, fileCells, h.probeSlab)
	if err != nil {
		return nil
	}
	h.inferredVersion = version
	h.inferredBufGen = h.bufCellsGen
	h.probeStateMu.Lock()
	firstProbe := h.lastProbe.Load() == nil
	oldProbeCells := h.lastProbeCells
	h.lastProbe.Store(&res)
	h.lastProbeCells = res.FileLines
	if !cellMatricesAlias(oldProbeCells, h.bufCells) {
		h.bufCellsScratch = oldProbeCells
	}
	var flush *term.Coordinates
	if firstProbe {
		flush = h.pendingGoto
		h.pendingGoto = nil
	}
	h.probeStateMu.Unlock()
	return flush
}

func (h *editorHandler) bufferCells() [][]term.Cell {
	h.probeMu.Lock()
	defer h.probeMu.Unlock()
	return h.bufCells
}

func snapshotFileCells(rows [][]term.Cell) [][]term.Cell {
	return snapshotFileCellsInto(nil, rows)
}

func snapshotFileCellsInto(dst [][]term.Cell, rows [][]term.Cell) [][]term.Cell {
	if len(rows) == 0 {
		return nil
	}
	if n := len(rows); len(rows[n-1]) == 0 {
		rows = rows[:n-1]
		if len(rows) == 0 {
			return nil
		}
	}
	return term.CopyCells(dst, rows)
}

func (h *editorHandler) snapshotBufferCells() {
	h.probeMu.Lock()
	defer h.probeMu.Unlock()

	dst := h.bufCellsScratch
	if dst == nil && !cellMatricesAlias(h.bufCells, h.lastProbeCells) {
		dst = h.bufCells
	}
	oldCells := h.bufCells
	h.bufCells = snapshotFileCellsInto(dst, h.buf.View().RawCells())
	h.bufCellsGen++
	h.bufCellsScratch = nil
	if !cellMatricesAlias(oldCells, h.lastProbeCells) && !cellMatricesAlias(oldCells, h.bufCells) {
		h.bufCellsScratch = oldCells
	}
}

func cellMatricesAlias(a, b [][]term.Cell) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	if &a[0] == &b[0] {
		return true
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if len(a[i]) == 0 || len(b[i]) == 0 {
			continue
		}
		if &a[i][0] == &b[i][0] {
			return true
		}
	}
	return false
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
		_, _ = h.vteHandler.Handle(keyCombToEvent(k))
	}
	go debug.CapturePanicReport(func() {
		select {
		case <-h.procDone:
		case <-time.After(gracefulQuitTimeout):
		}
		h.scheduleNextTick(func() {
			if err := h.vteHandler.Close(); err != nil {
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

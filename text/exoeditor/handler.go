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

	// probeMu serializes refreshProbe so the interrupt path (vte reader
	// goroutine) and the reload path (host event loop, via
	// bufLineWatcher) never run Infer concurrently — they share
	// probeSlab, which is not safe for concurrent use.
	probeMu sync.Mutex

	// component is the source of the rendered grid the probe aligns
	// against. It is the embedded vte handler's *vte.Component in
	// production; tests inject a fake snapshotter so the full probe path
	// runs without a pty-backed handler.
	component componentSnapshotter
}

// componentSnapshotter is the slice of *vte.Component refreshProbe needs:
// a consistent snapshot of the rendered grid and cursor.
type componentSnapshotter interface {
	Snapshot() (vte.Snapshot, error)
}

// newHandler builds a wrapper around vteH, hooks the file watcher, and
// returns the handler. The watcher loop is spawned synchronously; the
// caller can stop it via Close.
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
	// Seed the line snapshot from the buffer's current content (on this
	// goroutine, before the vte goroutine starts) and keep it fresh on
	// every subsequent buffer edit. refreshProbe reads the snapshot,
	// never the buffer, so it never races the reload writer.
	h.snapshotBufferLines()
	h.bufSub = &bufLineWatcher{h: h}
	buf.Subscribe(h.bufSub)
	h.startWatcher(ctx)
	return h
}

// bufLineWatcher recomputes the editorHandler's line snapshot whenever
// the mirrored buffer is edited. OnDidEdit fires synchronously on the
// goroutine that performed the edit (the reload runs on the UI tick),
// so reading the buffer here is safe and keeps the snapshot consistent
// with the writer.
type bufLineWatcher struct{ h *editorHandler }

func (w *bufLineWatcher) OnWillEdit(
	context.Context, term.Coordinates, term.Coordinates, string) {
}

func (w *bufLineWatcher) OnDidEdit(
	context.Context, term.Coordinates, term.Coordinates, string) {
	w.h.snapshotBufferLines()
	// A reload changed the mirror, so the cached probe was aligned
	// against stale lines. Re-run it against the now-current snapshot
	// here on the host loop (where the reload's repaint follows), so
	// the overlay is correct without waiting for the next keystroke.
	w.h.refreshProbe()
}

// startWatcher subscribes to Write/Rename events for the underlying
// file. On each event, the file contents are re-read off the event-loop
// goroutine and the buffer is replaced via scheduleNextTick.
//
// A file opened for the first time does not exist on disk yet, and the
// notify backend lstats the watched path at registration time, so
// watching the file directly would fail. In that case watch the parent
// directory instead and filter events down to the resource path; the
// external editor's first save then materializes the file and triggers
// the reload like any other write.
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
	// The notify backend reports the symlink-resolved path (e.g.
	// /private/tmp/a for /tmp/a), which would not match the tab keyed
	// by the original resource URI. Always reload the resource itself,
	// and in directory-watch mode filter sibling events by base name.
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

// scheduleReload runs on the watcher goroutine. The reloader touches
// the open-tab map and the FlusherCloser swap-worker state, both of
// which cooperate with cell.Buffer subscribers that mutate UI-owned
// state — so the call must happen on the host UI goroutine.
// scheduleNextTick is the only hand-off into that loop.
//
// The reload itself is async: Reloader.Reload starts the IDE's
// canonical reload pipeline (the same path :reloadfile uses) and
// returns either nil or a start-failure error. The actual disk I/O,
// buffer reset, and dirty-tab clear happen on the IDE's own awaiter
// goroutine + UI scheduler.
func (h *editorHandler) scheduleReload(uri workspaceapi.URI) {
	h.scheduleNextTick(func() {
		err := h.reloader.Reload(uri)
		if err == nil || errors.Is(err, workspace.ErrFlushInProgress) {
			return
		}
		// External editors typically save via "write to tmp, then
		// rename" so the watcher routinely sees a transient
		// not-exist window between the rename and the next event.
		// Don't spam the user for what is expected protocol noise.
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

// CursorAtScroll returns the file-content cursor inferred by the last
// Draw. Probing happens in Draw rather than in Handle because Handle
// only enqueues bytes into the pty master: the embedded editor's
// response (the screen update we care about) is parsed asynchronously
// by the vte run goroutine and applied to the cell.Buffer through
// ScheduleNextTick. By the time Draw runs the host event loop has
// already drained those scheduled callbacks, so RawCells() reflects
// the post-Handle state. Returns the zero value before the first
// Draw or whenever the most recent Infer failed.
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

// SetLocationList records a location list so the next Draw can
// overlay its attributes on top of the embedded editor's output.
// MoveTo{Next,Prev}Location remain unimplemented because they need
// editor-specific key sequences to drive the embedded cursor, but the
// store is updated so observers querying LocationLists see the same
// state non-exo editors expose.
func (h *editorHandler) SetLocationList(
	pri textapi.LocationPriority, id string, l text.LocationList,
) {
	h.locations.SetLocationList(pri, id, l)
}

// LocationLists returns the currently registered location lists.
func (h *editorHandler) LocationLists() []text.LocationSet {
	return h.locations.LocationLists()
}

// LocationMessageAtCursor returns the first non-empty Message among
// the location lists registered at the embedded editor's current
// cursor coordinates, or the empty string when no such location
// exists. The "first non-empty wins" precedence mirrors
// text/modeless/handler.go:setActiveLocationListMessage so the exo
// message bar surfaces the same string the modal editors would.
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

// MoveToNextLocation drives the embedded editor's cursor to the next
// location on the list named ID, picking the first location strictly
// past CursorAtScroll (or wrapping to the start of the list when the
// cursor is past the last location). Returns false when the list is
// unknown, empty, or no goto template is configured.
func (h *editorHandler) MoveToNextLocation(id string) bool {
	return h.moveToLocation(id, true)
}

// MoveToPrevLocation drives the embedded editor's cursor to the
// location immediately before CursorAtScroll on the list named ID, or
// wraps to the end of the list when no earlier location exists.
// Returns false under the same conditions as MoveToNextLocation.
func (h *editorHandler) MoveToPrevLocation(id string) bool {
	return h.moveToLocation(id, false)
}

// moveToLocation walks the named list to find the location to jump to
// relative to CursorAtScroll, then dispatches the goto sequence via
// SetCursorAtScroll. forward=true walks Next looking for the first
// location strictly past the cursor; forward=false walks Prev looking
// for the first location strictly before it. Both wrap around when no
// candidate satisfies the predicate.
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

// pickLocation walks l from its first entry collecting every
// location, then picks the next one strictly past cursor (when
// forward is true) or the last one strictly before cursor (when
// forward is false). With no candidate, it wraps — returning the
// first location for forward and the last for backward, matching
// text.Cursor.MoveTo{Next,Prev}Location semantics.
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

// Draw forwards to the embedded vte.Handler and, when override is
// enabled, strips the embedded editor's attributes (so Rune fully
// owns highlighting) then overlays the configured location lists on
// top of the rendered grid. The cached probe is refreshed by the
// interrupter wired in editor.Edit, not here: only EventInterrupt
// signals that the embedded editor actually mutated its cell grid,
// and most Draw calls (e.g. focus changes elsewhere in the IDE) do
// not.
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

// refreshProbe re-runs vteprobe against the current cell grid. It is
// called from the vte interrupt path (grid mutations) and from
// bufLineWatcher (buffer reloads); probeMu serializes the two. A failed
// Infer (cursor on chrome, transient redraw mid-clear, confidence below
// threshold, …) keeps the previous result so the overlay does not
// flicker off between successful probes.
func (h *editorHandler) refreshProbe() {
	h.probeMu.Lock()
	defer h.probeMu.Unlock()

	snap, err := h.component.Snapshot()
	if err != nil {
		return
	}
	active := snap.Active()
	lines := h.bufferLines()
	res, err := h.probe.Infer(active.Cells, active.Cursor, lines, h.probeSlab)
	if err != nil {
		return
	}
	h.lastProbe.Store(&res)
}

// bufferLines returns the most recent snapshot of the in-memory buffer
// content split into file lines — the authoritative mirror Rune uses
// everywhere else. The snapshot is recomputed on the buffer-owning
// goroutine by bufLineWatcher whenever the buffer is edited and read
// locklessly here, so refreshProbe never touches the non-thread-safe
// cell.Buffer from the vte goroutine.
func (h *editorHandler) bufferLines() []string {
	if p := h.bufLines.Load(); p != nil {
		return *p
	}
	return nil
}

// snapshotBufferLines recomputes the line snapshot from the buffer. It
// must run on the goroutine that owns h.buf (construction or a buffer
// edit notification), never on the vte goroutine.
func (h *editorHandler) snapshotBufferLines() {
	lines := vteprobe.LinesFromView(h.buf.View())
	h.bufLines.Store(&lines)
}

// CellEditor returns a no-op cell.Editor: the external editor is the
// only writer to the file and Rune-level edits would fight with it.
func (h *editorHandler) CellEditor() cell.Editor { return nopCellEditor{} }

// Dimensions reports the ideal size needed to render the buffer
// without clipping.
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

// nopCellEditor satisfies cell.Editor without mutating anything.
// External-editor mode forbids Rune-level edits.
type nopCellEditor struct{}

func (nopCellEditor) Edit(
	ctx context.Context, start, end term.Coordinates, _ string,
) (from, to term.Coordinates, old string) {
	return start, end, ""
}

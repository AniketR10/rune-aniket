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

// Package streamload provides a read-only browser tab handler that
// streams a file's contents off a walkdir.Reader lazily as the user
// scrolls. It is installed by text.Component while a parallel
// goroutine runs the real workspace.Workspace.Load; when Load
// completes, text.Component swaps this handler out for a real
// editor-backed handler. See text/component.go.
package streamload

import (
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/workspace/walkdir"
)

// Compile-time interface checks.
var (
	_ browserapi.Handler   = (*Handler)(nil)
	_ component.Scrollable = (*Handler)(nil)
)

// Config tunes paging behaviour. Zero values fall back to package
// defaults that work well for typical source files.
type Config struct {
	// InitialPages is the number of pages of content read
	// synchronously inside New so the user sees something
	// immediately. Defaults to 2.
	InitialPages int
	// Overscan is the number of pages of look-ahead the handler keeps
	// loaded ahead of the bottom of the viewport. When the viewport
	// is within Overscan*pageSize rows of the last loaded row,
	// the next Handle call synchronously reads one more page.
	// Defaults to 1.
	Overscan int
	// PageRows is the fallback page size used before the first
	// Resize arrives. Defaults to 64.
	PageRows int

	// Config for the less client
	LessConfig handler.LessConfig
}

func (c Config) withDefaults() Config {
	if c.InitialPages <= 0 {
		c.InitialPages = 2
	}
	if c.Overscan < 0 {
		c.Overscan = 0
	}
	if c.Overscan == 0 {
		c.Overscan = 1
	}
	if c.PageRows <= 0 {
		c.PageRows = 64
	}
	return c
}

// Handler is a read-only browser tab handler that streams a file's
// contents off disk lazily as the user scrolls. It satisfies
// browserapi.Handler and component.Scrollable so it can be installed
// as a browser.Tab's content and host a scrollbar.
//
// Reads happen synchronously inside Handle: when the user scrolls
// near the end of the loaded content, the handler reads another page
// from the underlying File. Single-page reads against any workspace
// scheme (including ssh) are O(ms); blocking the event loop briefly
// is preferable to the cost and complexity of a worker pump.
//
// Handler deliberately does NOT satisfy text.Handler. Tabs whose
// handler is a text.Handler are expected to have had EventTypeOpen
// dispatched at construction time (see text/component.go). The
// streaming handler is transient — text.Component swaps it out for a
// real text.Handler when the parallel workspace.Workspace.Load
// completes — and dispatching EventTypeOpen for it would route LSP /
// extension / syntax-tree consumers through a half-populated buffer.
type Handler struct {
	uri  workspaceapi.URI
	buf  *cell.Buffer
	less handler.Less
	pr   *pageReader
	cfg  Config

	// height is the last Resize height. Stored so Handle can size
	// future page reads to match the viewport.
	height int

	// width is the last Resize width. Stored alongside height so
	// text.Component can resize the swapped-in real handler to
	// match the streaming tab before the next Draw.
	width int

	// loadingMu guards loadingDone. SignalDone may race with the
	// animateTabLoading goroutine reading the channel.
	once        sync.Once
	loadingDone chan struct{}
}

// New opens path on reader and reads the first cfg.InitialPages
// worth of lines into a fresh in-memory buffer. The handler is
// immediately usable as a browser.Tab content.
func New(
	reader walkdir.Reader, uri workspaceapi.URI, cfg Config,
) (*Handler, error) {
	if reader == nil {
		panic("streamload: nil walkdir.Reader")
	}
	cfg = cfg.withDefaults()
	pr, err := newPageReader(reader, uri.Path())
	if err != nil {
		return nil, err
	}
	h := &Handler{
		uri:         uri,
		buf:         cell.NewBuffer(),
		pr:          pr,
		cfg:         cfg,
		height:      cfg.PageRows,
		loadingDone: make(chan struct{}),
	}
	if _, err := h.readMore(cfg.InitialPages * cfg.PageRows); err != nil {
		_ = pr.Close()
		return nil, err
	}
	h.less.InitWithBuffer(h.buf, cfg.LessConfig)
	return h, nil
}

// URI returns the URI of the file backing this handler.
func (h *Handler) URI() workspaceapi.URI { return h.uri }

// Loading returns a channel that is closed when the parent (typically
// text.Component) calls SignalDone, signaling that the streaming tab
// has been swapped out for a real editor handler. Animations driven
// by the tab can use it as their stop signal.
func (h *Handler) Loading() <-chan struct{} { return h.loadingDone }

// Close releases the underlying file. Safe to call multiple times.
// Close also signals done so any animation goroutine watching
// Loading() exits cleanly.
func (h *Handler) Close() error {
	h.once.Do(func() {
		close(h.loadingDone)
		_ = h.pr.Close()
	})
	return nil
}

// Resize satisfies tui.Component.
func (h *Handler) Resize(width, height int) {
	if width > 0 {
		h.width = width
	}
	if height > 0 {
		h.height = height
	}
	h.less.Resize(width, height)
}

// Dimensions returns the most recent width and height passed to
// Resize. Used by text.Component on the swap-in path to size the
// real editor handler to match the streaming tab before its first
// Draw.
func (h *Handler) Dimensions() (width, height int) {
	return h.width, h.height
}

// Draw satisfies tui.Component.
func (h *Handler) Draw(w term.Writer) { h.less.Draw(w) }

// Cursor satisfies tui.Handler. The streaming handler is a
// read-only viewer, so the normal-mode cursor is suppressed —
// otherwise handler.Less.Cursor would overlay a block on the
// bottom row of the viewport. The cursor is only surfaced while
// the / search command bar is open, where it marks the user's
// caret in the search input.
func (h *Handler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	if h.less.Mode() != handler.LessSearchMode {
		return term.Coordinates{}, term.CursorStyleDefault, false
	}
	return h.less.Cursor()
}

// Selection satisfies tui.Handler.
func (h *Handler) Selection() (string, bool) { return h.less.Selection() }

// Handle satisfies tui.Handler. After the inner Less handler
// processes the event, Handle checks whether the viewport has
// scrolled near the bottom of the loaded content and, if so,
// synchronously reads another page from the underlying file.
func (h *Handler) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = h.less.Handle(ev)
	h.maybeReadMore()
	return
}

// SeekUp satisfies component.Scrollable.
func (h *Handler) SeekUp() bool { return h.less.Scroll().SeekUp() }

// SeekDown satisfies component.Scrollable.
func (h *Handler) SeekDown() bool {
	ok := h.less.Scroll().SeekDown()
	h.maybeReadMore()
	return ok
}

// SeekOffset satisfies component.Scrollable.
func (h *Handler) SeekOffset() int { return h.less.Scroll().SeekOffset() }

// MaxSeekOffset satisfies component.Scrollable.
func (h *Handler) MaxSeekOffset() int { return h.less.Scroll().MaxSeekOffset() }

// InSearchMode reports whether the inner less handler is currently
// consuming keystrokes for its `/` search prompt. Used by the
// deferred text.Handler wrapper installed on streaming tabs so
// outer handlers can honour text.Handler.IsSearchMode while the
// streaming load is still in flight.
func (h *Handler) InSearchMode() bool {
	return h.less.Mode() == handler.LessSearchMode
}

// pageSize returns the current effective page height. We use the
// last-known viewport height as the page size so a single readMore
// covers exactly one screen of content; fall back to the configured
// PageRows before the first Resize lands.
func (h *Handler) pageSize() int {
	if h.height > 0 {
		return h.height
	}
	return h.cfg.PageRows
}

// maybeReadMore reads another page if the viewport bottom is within
// Overscan*pageSize rows of the last-loaded row.
func (h *Handler) maybeReadMore() {
	if h.pr == nil || h.pr.atEOF() {
		return
	}
	pageSize := h.pageSize()
	if pageSize <= 0 {
		return
	}
	bottomRow := h.less.Scroll().SeekOffset() + pageSize
	loadedRows := h.buf.View().Rows()
	if bottomRow+h.cfg.Overscan*pageSize >= loadedRows {
		_, _ = h.readMore(pageSize)
	}
}

// readMore reads up to n lines from the underlying file and appends
// them to the buffer. Returns the number of rows added and any error.
func (h *Handler) readMore(n int) (int, error) {
	if h.pr == nil || h.pr.atEOF() || n <= 0 {
		return 0, nil
	}
	before := h.buf.View().Rows()
	chunk, ok := h.pr.readLines(n)
	if !ok {
		return 0, nil
	}
	h.buf.WriteString(chunk)
	return h.buf.View().Rows() - before, nil
}

func (h *Handler) atEOF() bool {
	return h.pr == nil || h.pr.atEOF()
}

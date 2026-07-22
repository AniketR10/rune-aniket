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
	// InitialPages is the number of pages of content pre-read by
	// ReadInitial so the user sees something immediately once
	// InstallInitial runs. Defaults to 2.
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
type Handler struct {
	uri       workspaceapi.URI
	buf       *cell.Buffer
	less      handler.Less
	pr        *pageReader
	cfg       Config
	height    int
	width     int
	primed    bool
	closeOnce sync.Once
}

// New opens path on reader and prepares to read the first
// cfg.InitialPages worth of lines into a fresh in-memory buffer.
func New(
	reader walkdir.Reader, uri workspaceapi.URI, cfg Config,
) (*Handler, error) {
	if reader == nil {
		panic("streamload: nil walkdir.Reader")
	}
	cfg = cfg.withDefaults()
	pr, err := newPageReader(asyncOpenReader{r: reader}, uri.Path())
	if err != nil {
		// Unreachable today — asyncOpenReader.OpenFile never fails
		// synchronously — but kept in the contract so the deferred
		// open can be made synchronous again behind a flag.
		return nil, err
	}
	h := &Handler{
		uri:    uri,
		buf:    cell.NewBuffer(),
		pr:     pr,
		cfg:    cfg,
		height: cfg.PageRows,
	}
	h.less.InitWithBuffer(h.buf, cfg.LessConfig)
	return h, nil
}

func (h *Handler) primeInitial() bool {
	if h.primed {
		return true
	}
	if !h.pr.ready() {
		return false
	}
	h.primed = true
	_, _ = h.readMore(h.cfg.InitialPages * h.cfg.PageRows)
	return true
}

// URI returns the URI of the file backing this handler.
func (h *Handler) URI() workspaceapi.URI { return h.uri }

// Close releases the underlying file. Safe to call multiple times,
func (h *Handler) Close() error {
	h.closeOnce.Do(func() {
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
// Resize.
func (h *Handler) Dimensions() (width, height int) {
	return h.width, h.height
}

// Draw satisfies tui.Component.
func (h *Handler) Draw(w term.Writer) {
	h.primeInitial()
	h.less.Draw(w)
}

// Cursor satisfies tui.Handler.
func (h *Handler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	if h.less.Mode() != handler.LessSearchMode {
		return term.Coordinates{}, term.CursorStyleDefault, false
	}
	return h.less.Cursor()
}

// Selection satisfies tui.Handler.
func (h *Handler) Selection() (string, bool) { return h.less.Selection() }

// Handle satisfies tui.Handler.
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
// consuming keystrokes for its `/` search prompt.
func (h *Handler) InSearchMode() bool {
	return h.less.Mode() == handler.LessSearchMode
}

func (h *Handler) pageSize() int {
	if h.height > 0 {
		return h.height
	}
	return h.cfg.PageRows
}

func (h *Handler) maybeReadMore() {
	if !h.primeInitial() || h.pr.atEOF() {
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

func (h *Handler) readMore(n int) (int, error) {
	if h.pr.atEOF() || n <= 0 {
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
	return h.pr.atEOF()
}

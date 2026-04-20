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

package ide

import (
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	fileexplorercomp "unstable.build/go-tui/component/fileexplorer"
	"unstable.build/go-tui/text"
)

var _ browser.ScrollableFloating = (*fileExplorerHandler)(nil)

type fileExplorerHost interface {
	Prompt(message string, options []string, bindings []term.KeyComb, promptHandler handler.PromptHandler) browser.Window
	SetWindowWidth(win browser.Window, width int) bool
	SetFocus(win browser.Window) (browser.Window, error)
	Focus() (browser.Window, error)
	OpenFile(uri workspaceapi.URI, win browser.Window) error
	SetError(error)
	FrameEnabled() bool
}

type fileExplorerHandler struct {
	browser.ScrollableFloating
	host   fileExplorerHost
	comp   *fileexplorercomp.Component
	buf    *cell.Buffer
	ed     text.Handler
	uri    workspaceapi.URI
	width  int
	height int

	win    browser.Window
	target browser.Window
}

func newFileExplorerHandler(
	host fileExplorerHost,
	comp *fileexplorercomp.Component,
	buf *cell.Buffer,
	ed text.Handler,
	uri workspaceapi.URI,
	target browser.Window,
) (*fileExplorerHandler, error) {
	h := &fileExplorerHandler{
		host:   host,
		comp:   comp,
		buf:    buf,
		ed:     ed,
		uri:    uri,
		target: target,
	}
	h.ScrollableFloating = browser.FuncScrollableFloatingHandler(h, func() error { return nil })
	return h, nil
}

func (h *fileExplorerHandler) SetWindow(win browser.Window) {
	h.win = win
}

func (h *fileExplorerHandler) SetTargetWindow(win browser.Window) {
	h.target = win
}

func (h *fileExplorerHandler) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type == term.EventKey && ev.Mod == 0 && ev.Key == term.KeyEnter {
		if handled = h.enterAtCursor(); handled {
			h.syncWidth()
			return false, true
		}
	}
	exit, handled = h.ed.Handle(ev)
	if !handled {
		return exit, handled
	}
	h.syncWidth()
	if exit {
		h.focusTarget()
		return false, true
	}
	return false, true
}

func (h *fileExplorerHandler) Draw(w term.Writer) {
	h.ed.Draw(w)
}

func (h *fileExplorerHandler) Resize(width, height int) {
	h.width = width
	h.height = height
	h.ed.Resize(width, height)
}

func (h *fileExplorerHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return h.ed.Cursor()
}

func (h *fileExplorerHandler) Selection() (string, bool) {
	return h.ed.Selection()
}

func (h *fileExplorerHandler) Dimensions() (int, int) {
	// The editor reports its own ideal dimensions including any
	// auxiliary chrome (line numbers, folds, git icons) drawn on
	// top of the shared buffer, so the parent window can size
	// itself to fit both the chrome and the full tree without
	// truncation.
	return h.ed.Dimensions()
}

func (h *fileExplorerHandler) SeekUp() bool {
	return h.ed.SeekUp()
}

func (h *fileExplorerHandler) SeekDown() bool {
	return h.ed.SeekDown()
}

func (h *fileExplorerHandler) SeekOffset() int {
	return h.ed.SeekOffset()
}

func (h *fileExplorerHandler) MaxSeekOffset() int {
	return h.ed.MaxSeekOffset()
}

// Close is called when the tiled window hosting the file explorer
// is closed (via :fexplorer toggle). It intentionally does NOT close
// the underlying text editor handler: the editor is cached on the
// ex and re-used when the explorer is re-opened, so that Edit is
// only ever called once per URI. Closing here would unsubscribe the
// vi editor's per-file commands (fold/location/git) and then the
// next open would try to re-subscribe them, failing with
// "command already registered".
func (h *fileExplorerHandler) Close() error { return nil }

// closeEditor closes the underlying editor chain. This MUST be
// called exactly once per explorer lifetime, when the owning ex is
// being torn down.
func (h *fileExplorerHandler) closeEditor() error {
	if h.ed == nil {
		return nil
	}
	err := h.ed.Close()
	h.ed = nil
	return err
}

func (h *fileExplorerHandler) enterAtCursor() bool {
	pos := h.ed.CursorAtScroll()
	prev := pos
	uri, open := h.comp.ExpandNodeAt(pos)
	if open {
		h.open(uri)
		return true
	}
	// Component may have rewritten the buffer; restore the cursor
	// clamped to the new bounds.
	rows := h.ed.CellView().Rows()
	if rows == 0 {
		_ = h.ed.SetCursorAtScroll(term.Coordinates{})
	} else {
		if prev.Y >= rows {
			prev.Y = rows - 1
		}
		cols := h.ed.CellView().Columns(prev.Y)
		if prev.X > cols {
			prev.X = cols
		}
		_ = h.ed.SetCursorAtScroll(prev)
	}
	return true
}

func (h *fileExplorerHandler) flush() error {
	cs := h.comp.DryFlush()
	if cs.HasConflicts() {
		h.host.SetError(conflictsError(cs))
		return nil
	}
	if len(cs.Operations) == 0 {
		return nil
	}
	return h.openApplyPrompt(fileexplorercomp.OrderOperations(cs.Operations))
}

func (h *fileExplorerHandler) forceFlush() error {
	return h.flush()
}

func conflictsError(cs *fileexplorercomp.ChangeSet) error {
	msgs := make([]string, 0, len(cs.Conflicts))
	for _, c := range cs.Conflicts {
		msgs = append(msgs, c.Message)
	}
	return &flushError{msg: strings.Join(msgs, "; ")}
}

type flushError struct{ msg string }

func (e *flushError) Error() string { return "file explorer conflicts: " + e.msg }

func (h *fileExplorerHandler) openApplyPrompt(ops []fileexplorercomp.Operation) error {
	message := h.confirmMessage(ops)
	var promptWin browser.Window
	promptWin = h.host.Prompt(message, []string{yesOpt, noOpt}, yesNoKeyCombs,
		handler.FuncPromptHandler(func(_ int, option string) {
			if promptWin != nil {
				_ = promptWin.Close()
			}
			if option != yesOpt {
				return
			}
			if _, err := h.comp.Flush(); err != nil {
				h.host.SetError(err)
				return
			}
		}, func() error {
			return nil
		}))
	return nil
}

// confirmMessage renders the set of pending operations as a markdown
// prompt message. Operations are grouped and labelled as a markdown
// unordered list so a long change set is easy to scan:
//
//	Apply the following changes?
//	- **CREATE**\t<relative-path>
//	- **MKDIR**\t<relative-path>
//	- **RENAME**\t<old-name> -> <new-name>
//	- **MOVE**\t<old-rel> -> <new-rel>
//	- **DELETE**\t<relative-path>
//
// A RENAME is used only when the source and destination share the
// same parent directory (same URI.Path() dir, different basename).
// A MOVE is used when the parents differ — regardless of whether the
// basename is also different. This matches the user's mental model:
// "rename" never changes where a file lives; "move" always does.
func (h *fileExplorerHandler) confirmMessage(
	ops []fileexplorercomp.Operation,
) string {
	return h.confirmMessageForTest(h.comp.Root(), ops)
}

// confirmMessageForTest is the testable core of confirmMessage; it
// accepts the root URI explicitly so tests can exercise the
// formatting without wiring a component.
func (h *fileExplorerHandler) confirmMessageForTest(
	root workspaceapi.URI,
	ops []fileexplorercomp.Operation,
) string {
	var b strings.Builder
	b.WriteString("Apply the following changes?")
	for _, op := range ops {
		label, body := formatOperation(root, op)
		if label == "" {
			continue
		}
		b.WriteByte('\n')
		b.WriteString("- **")
		b.WriteString(label)
		b.WriteString("**\t")
		b.WriteString(body)
	}
	return b.String()
}

// formatOperation returns the bolded label and the path portion to
// render for a single operation. Rename vs. Move is disambiguated by
// comparing the source and destination parent directories.
func formatOperation(
	root workspaceapi.URI, op fileexplorercomp.Operation,
) (label, body string) {
	rel := func(u workspaceapi.URI) string {
		return relativePath(root, u)
	}
	switch op.Type {
	case fileexplorercomp.OpCreate:
		return "CREATE", rel(op.NewURI)
	case fileexplorercomp.OpMkdir:
		return "MKDIR", rel(op.NewURI)
	case fileexplorercomp.OpDelete:
		return "DELETE", rel(op.URI)
	case fileexplorercomp.OpRename, fileexplorercomp.OpMove:
		if sameParent(op.URI, op.NewURI) {
			return "RENAME",
				op.URI.Name() + " -> " + op.NewURI.Name()
		}
		return "MOVE", rel(op.URI) + " -> " + rel(op.NewURI)
	case fileexplorercomp.OpCopy:
		return "COPY", rel(op.URI) + " -> " + rel(op.NewURI)
	}
	return "", ""
}

// relativePath returns uri expressed relative to root, falling back
// to the absolute path when root is not a prefix of uri.
func relativePath(root, uri workspaceapi.URI) string {
	rp := root.Path()
	up := uri.Path()
	if rp != "" && strings.HasPrefix(up, rp) {
		rel := strings.TrimPrefix(up, rp)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return uri.Name()
		}
		return rel
	}
	return up
}

func sameParent(a, b workspaceapi.URI) bool {
	return workspaceapi.Dir(a).Path() == workspaceapi.Dir(b).Path()
}

func (h *fileExplorerHandler) syncWidth() {
	if h.win == nil || h.win.Closed() {
		return
	}
	width, _ := h.Dimensions()
	if h.host.FrameEnabled() {
		width += 2
	}
	_ = h.host.SetWindowWidth(h.win, width)
}

func (h *fileExplorerHandler) focusTarget() {
	if h.target != nil && !h.target.Closed() && h.target != h.win {
		_, _ = h.host.SetFocus(h.target)
	}
}

func (h *fileExplorerHandler) open(uri workspaceapi.URI) {
	win := h.targetWindow()
	if win == nil {
		return
	}
	if err := h.host.OpenFile(uri, win); err != nil {
		h.host.SetError(err)
	}
}

func (h *fileExplorerHandler) targetWindow() browser.Window {
	if h.target != nil && !h.target.Closed() && h.target != h.win {
		return h.target
	}
	win, _ := h.host.Focus()
	if win != nil && win != h.win && !win.Closed() {
		h.target = win
		return win
	}
	return nil
}

type exFileExplorerHost struct {
	ex *ex
}

func (h exFileExplorerHost) Prompt(
	message string,
	options []string,
	bindings []term.KeyComb,
	promptHandler handler.PromptHandler,
) browser.Window {
	return h.ex.comp.Prompt(message, options, bindings, promptHandler)
}

func (h exFileExplorerHost) SetWindowWidth(win browser.Window, width int) bool {
	return h.ex.comp.SetWindowWidth(win, width)
}

func (h exFileExplorerHost) SetFocus(win browser.Window) (browser.Window, error) {
	return h.ex.comp.SetFocus(win)
}

func (h exFileExplorerHost) Focus() (browser.Window, error) {
	return h.ex.comp.Focus()
}

func (h exFileExplorerHost) OpenFile(uri workspaceapi.URI, win browser.Window) error {
	_, err := h.ex.editFileURI(uri, win, false)
	return err
}

func (h exFileExplorerHost) SetError(err error) {
	h.ex.setError(err)
}

func (h exFileExplorerHost) FrameEnabled() bool {
	return h.ex.config.Frame
}

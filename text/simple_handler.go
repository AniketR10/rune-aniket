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

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/clipboard"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

var _ component.Scrollable = (*simpleEditorHandler)(nil)

type simpleEditorHandler struct {
	buf              *cell.Buffer
	less             handler.Less
	resource         workspaceapi.URI
	cursor           Cursor
	height           int
	mouse            *Mouse
	clipboard        clipboard.Register
	pendingSetCursor *term.Coordinates
}

// NewSimpleHandler returns a modeless, simple-to-use text.Handler.
func NewSimpleHandler(
	clipboard clipboard.Register,
	buf *cell.Buffer, resource workspaceapi.URI,
	wrap, commandBar bool,
	attr, resAttr, barAttr term.Attributes,
	scheduleNextTick func(func()) bool,
) Handler {
	ret := new(simpleEditorHandler)
	ret.Init(clipboard, buf, resource, wrap, commandBar,
		attr, resAttr, barAttr, scheduleNextTick)
	return ret
}

func (h *simpleEditorHandler) Init(
	clipboard clipboard.Register,
	buf *cell.Buffer, resource workspaceapi.URI,
	wrap, commandBar bool,
	attr, resAttr, barAttr term.Attributes,
	scheduleNextTick func(func()) bool,
) {
	h.buf = buf
	h.resource = resource
	h.less.InitWithBuffer(buf, handler.LessConfig{
		Wrap:       wrap,
		NoBar:      !commandBar,
		BarAttr:    barAttr,
		ResAttr:    resAttr,
		Attributes: attr,
	})
	h.cursor.Init(h.less.Scroll(), scheduleNextTick)
	h.mouse = NewMouse(CursorMouseDelegate(&h.cursor))
	h.clipboard = clipboard
}

// Resize satisfies tui.Component
func (h *simpleEditorHandler) Resize(width, height int) {
	h.height = height
	h.less.Resize(width, height)
	if h.pendingSetCursor != nil {
		h.SetCursorAtScroll(*h.pendingSetCursor)
	}
}

// Draw satisfies tui.Component
func (h *simpleEditorHandler) Draw(w term.Writer) {
	locs, _ := h.cursor.LocationsAtCursor()
	for _, loc := range locs {
		h.less.SetMessage("%s", loc.Message)
		return
	}
	h.less.Draw(w)
	DrawLocations(h.cursor.SortedLocations(), h.less.Scroll(), w)
}

func (h *simpleEditorHandler) Handle(ev term.Event) (exit, handled bool) {
	ctx := context.Background()

	// only a user event clears a pending set cursor
	h.pendingSetCursor = nil

	if ev.Type == term.EventMouse {
		return h.mouse.Handle(ev)
	}

	var shift bool
	if ev.Mod&term.ModShift != 0 {
		if _, ok := h.cursor.SelectionMode(); !ok {
			h.cursor.Select()
		}
		ev.Mod = ev.Mod &^ term.ModShift
		shift = true
	}

	cursorAt := h.cursor.CursorAtScroll()
	switch ev.Mod {
	case term.ModAlt:
		switch ev.Key {
		case term.KeyArrowLeft:
			handled = h.cursor.MoveLeftStartWord()
		case term.KeyArrowRight:
			handled = h.cursor.MoveRightStartWord()
		}
	case 0:
		switch ev.Key {
		case term.KeyArrowLeft:
			handled = h.cursor.MoveLeft()
		case term.KeyArrowRight:
			handled = h.cursor.MoveRight()
		case term.KeyArrowUp:
			handled = h.cursor.MoveUp()
		case term.KeyArrowDown:
			handled = h.cursor.MoveDown()
		case term.KeyEnter:
			if _, ok := h.cursor.SelectionMode(); ok {
				h.cursor.DeleteSelection()
				h.cursor.Unselect()
			}
			h.cursor.Insert('\n')
			handled = true
		case term.KeySpace:
			if _, ok := h.cursor.SelectionMode(); ok {
				h.cursor.DeleteSelection()
				h.cursor.Unselect()
			}
			h.cursor.Insert(' ')
			handled = true
		case term.KeyTab:
			if _, ok := h.cursor.SelectionMode(); ok {
				h.cursor.ShiftSelectionRight()
				h.cursor.Unselect()
			} else {
				h.cursor.Insert('\t')
			}
			handled = true
		case term.KeyBackspace:
			if _, ok := h.cursor.SelectionMode(); ok {
				handled = h.cursor.DeleteSelection()
				h.cursor.Unselect()
			} else {
				handled = h.cursor.Backspace()
			}
		default:
			if ev.Ch != 0 {
				if _, ok := h.cursor.SelectionMode(); ok {
					handled = h.cursor.DeleteSelection()
					h.cursor.Unselect()
				} else {
					h.cursor.Insert(ev.Ch)
					handled = true
				}
			}
		}
	case term.ModCtrl:
		switch ev.Ch {
		case 'c':
			_, err := h.cursor.CopySelection(clipboard.DefaultRegisterID, h.clipboard)
			if err != nil {
				log.Errorf("cursor copy selection: %v", err)
			}
			handled = true
		case 'v':
			paste, err := h.clipboard.Paste(clipboard.DefaultRegisterID)
			if err != nil {
				log.Errorf("clipboard paste: %v", err)
			} else {
				str := paste.Text
				h.cursor.Paste(str, StandardSelection, false)
			}
			handled = true
		case 'a':
			handled = h.cursor.MoveStartLine()
		case 'e':
			handled = h.cursor.MoveEndLine()
		case 'z':
			handled = h.cursor.Undo()
		case 'Z':
			handled = h.cursor.ToggleFold(ctx)
		case 'A':
			handled = h.cursor.ToggleAllFolds(ctx)
		case 'r':
			handled = h.cursor.Redo()
		case 'f':
			ev.Key = 0
			ev.Ch = '/'
			_, handled = h.less.Handle(ev)
		}
	case term.ModCtrlAlt:
		switch ev.Ch {
		case 'h':
			handled = h.cursor.HideSelection()
			h.cursor.Unselect()
		case 'v':
			handled = h.cursor.Unhide()
			h.cursor.Unselect()
		}
	}

	// if moved and shift is not pressed
	if cursorAt != h.cursor.CursorAtScroll() && !shift {
		h.cursor.Unselect()
	}
	return
}

// Cursor satisfies tui.Handler
func (h *simpleEditorHandler) Cursor() (
	pos term.Coordinates, style term.CursorStyle, show bool,
) {
	return h.cursor.Coordinates(), term.CursorStyleSteadyBar, true
}

// Selection satisfies tui.Handler.
func (h *simpleEditorHandler) Selection() (string, bool) {
	text := h.cursor.Selection()
	return text, text != ""
}

// Man satisfies tui.Handler
func (h *simpleEditorHandler) Man() tui.Manual {
	return tui.Manual{}
}

// Close satisfies editor.Handler.
func (h *simpleEditorHandler) Close() error {
	return nil
}

// Resource satisfies editor.Handler.
func (h *simpleEditorHandler) Resource() workspaceapi.URI {
	return h.resource
}

// SetWrap satisfies editor.Handler.
func (h *simpleEditorHandler) SetWrap(wrap bool) {
	h.less.Scroll().Wrap = wrap
}
func (t *simpleEditorHandler) ShowCommandBar(show bool) {
	t.less.ShowCommandBar(show)
}

func (h *simpleEditorHandler) SetCursorAtScroll(pos term.Coordinates) bool {
	// setCursor should be robust against resizes, etc.
	// only the first client interaction should clear this position
	h.pendingSetCursor = new(term.Coordinates)
	*h.pendingSetCursor = pos

	_, ok := h.cursor.MoveToScroll(pos)
	return ok
}

func (h *simpleEditorHandler) SeekUp() bool {
	return h.less.Scroll().SeekUp()
}

func (h *simpleEditorHandler) SeekDown() bool {
	return h.less.Scroll().SeekDown()
}

func (h *simpleEditorHandler) SeekOffset() int {
	return h.less.Scroll().SeekOffset()
}

func (h *simpleEditorHandler) MaxSeekOffset() int {
	return h.less.Scroll().MaxSeekOffset()
}

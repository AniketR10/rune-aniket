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
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/clipboard"
)

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
) Handler {
	ret := new(simpleEditorHandler)
	ret.Init(clipboard, buf, resource, wrap, commandBar, attr, resAttr, barAttr)
	return ret
}

func (h *simpleEditorHandler) Init(
	clipboard clipboard.Register,
	buf *cell.Buffer, resource workspaceapi.URI,
	wrap, commandBar bool,
	attr, resAttr, barAttr term.Attributes,
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
	h.cursor.Init(h.less.Scroll())
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
		h.less.SetMessage(loc.Message)
		return
	}
	h.less.Draw(w)
	DrawLocations(h.cursor.SortedLocations(), h.less.Scroll(), w)
}

func (h *simpleEditorHandler) Handle(ev term.Event) (exit, handled bool) {
	// only a user event clears a pending set cursor
	h.pendingSetCursor = nil

	if ev.Type == term.EventMouse {
		return h.mouse.Handle(ev)
	}

	switch ev.Mod {
	case 0:
		switch ev.Key {
		case term.KeyArrowLeft:
			if ev.Mod == term.ModAlt {
				handled = h.cursor.MoveLeftStartWord()
			} else {
				handled = h.cursor.MoveLeft()
			}
		case term.KeyArrowRight:
			if ev.Mod == term.ModAlt {
				handled = h.cursor.MoveRightStartWord()
			} else {
				handled = h.cursor.MoveRight()
			}
		case term.KeyArrowUp:
			handled = h.cursor.MoveUp()
		case term.KeyArrowDown:
			handled = h.cursor.MoveDown()
		case term.KeyEnter:
			h.cursor.Insert('\n')
			handled = true
		case term.KeySpace:
			h.cursor.Insert(' ')
			handled = true
		case term.KeyTab:
			h.cursor.Insert('\t')
			handled = true
		case term.KeyBackspace:
			if h.cursor.Selection() != "" {
				handled = h.cursor.DeleteSelection()
			} else {
				handled = h.cursor.Backspace()
			}
		default:
			if ev.Ch != 0 {
				h.cursor.Insert(ev.Ch)
				handled = true
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
		case 'r':
			handled = h.cursor.Redo()
		case 'f':
			ev.Key = 0
			ev.Ch = '/'
			_, handled = h.less.Handle(ev)
		}
	}
	return
}

// Cursor satisfies tui.Handler
func (h *simpleEditorHandler) Cursor() (
	pos term.Coordinates, style term.CursorStyle, show bool,
) {
	return h.cursor.Coordinates(), term.CursorStyleSteadyBar, true
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

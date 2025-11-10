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
	"errors"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/clipboard"
	"unstable.build/go-tui/term"
)

// DefaultSimpleEditor returns a simple to use Editor implementation.
func DefaultSimpleEditor(clipboard clipboard.Register) Editor {
	searchAttr := term.Attributes{Attrs: tcell.AttrReverse}
	return NewSimpleEditor(clipboard, false, true, false, false,
		term.Attributes{}, searchAttr, term.Attributes{},
		func(fn func()) bool { fn(); return true })
}

// NewSimpleEditor allocates storage for a new Editor and initializes it.
// The scheduleNextTick parameter can be nil if auxBar is false.
func NewSimpleEditor(
	clipboard clipboard.Register,
	wrap, commandBar, auxBar, auxBarFolds bool,
	attr, searchAttr, barAttr term.Attributes,
	scheduleNextTick func(func()) bool,
) Editor {
	if auxBar && scheduleNextTick == nil {
		panic("nil schedule function")
	}
	ret := new(simpleEditor)
	ret.wrap = wrap
	ret.commandBar = commandBar
	ret.auxBar = auxBar
	ret.auxBarFolds = auxBarFolds
	ret.scheduleNextTick = scheduleNextTick
	ret.resAttr = searchAttr
	ret.barAttr = barAttr
	ret.attr = attr
	ret.clipboard = clipboard
	ret.pub.Init()
	return ret
}

type simpleEditor struct {
	pub              Publisher
	wrap             bool
	commandBar       bool
	auxBar           bool
	auxBarFolds      bool
	attr             term.Attributes
	resAttr          term.Attributes
	barAttr          term.Attributes
	clipboard        clipboard.Register
	scheduleNextTick func(func()) bool
}

func (e *simpleEditor) Edit(file workspaceapi.URI, buf *cell.Buffer) (Handler, error) {
	rootIfc := NewSimpleHandler(e.clipboard, buf, file, e.wrap,
		e.commandBar, e.attr, e.resAttr, e.barAttr, e.scheduleNextTick)
	root := rootIfc.(*simpleEditorHandler)
	ret := e.pub.PublishEdit(file, buf, root, &root.cursor)
	if e.auxBar {
		ret = WithAuxBar(ret, buf, root.less.Scroll(),
			e.auxBarFolds, true, true, true, e.scheduleNextTick)
	}
	return ret, nil
}

func (e *simpleEditor) SubscribeCommand(cmd textapi.CommandManual, h CommandHandler) error {
	return errors.New("not supported")
}

func (c *simpleEditor) UnsubscribeCommand(cmd string) error {
	return errors.New("not supported")
}

func (e *simpleEditor) Editor(file workspaceapi.URI) (Handler, error) {
	return nil, errors.New("not supported")
}

func (e *simpleEditor) SubscribeEvents(evs []textapi.EventType, sub EventHandler) error {
	e.pub.SubscribeEvents(evs, sub)
	return nil
}

func (e *simpleEditor) UnsubscribeEvents(sub EventHandler) (bool, error) {
	ok := e.pub.UnsubscribeEvents(sub)
	return ok, nil
}

func (e simpleEditor) SetLocationList(
	h Handler, pri textapi.LocationPriority, ID string, loc LocationList,
) error {
	e.unwrapHandler(h).cursor.SetLocationList(pri, ID, loc)
	return nil
}

func (e *simpleEditor) MoveToNextLocation(h Handler, ID string) error {
	hh := h
	if e.auxBar {
		hh = UnwrapAuxBar(h)
	}
	dispatch := e.pub.RecordCursorChange(hh)
	defer dispatch()

	e.unwrapHandler(h).cursor.MoveToNextLocation(ID)
	return nil
}

func (e *simpleEditor) MoveToPrevLocation(h Handler, ID string) error {
	hh := h
	if e.auxBar {
		hh = UnwrapAuxBar(h)
	}
	dispatch := e.pub.RecordCursorChange(hh)
	defer dispatch()

	e.unwrapHandler(h).cursor.MoveToPrevLocation(ID)
	return nil
}

func (e *simpleEditor) CellView(h Handler) CellView {
	return NewCellView(e.unwrapHandler(h).buf.View())
}

func (e *simpleEditor) CellEditor(h Handler) CellEditor {
	return NewCellEditor(e.unwrapHandler(h).buf.Editor())
}

func (e *simpleEditor) SetDefaultAttributes(h Handler, attr term.Attributes) error {
	e.unwrapHandler(h).less.Scroll().Attributes = attr
	return nil
}

func (e *simpleEditor) SetCursor(h Handler, pos term.Coordinates) error {
	hh := h
	if e.auxBar {
		hh = UnwrapAuxBar(h)
	}
	dispatch := e.pub.RecordCursorChange(hh)
	defer dispatch()

	ok := e.unwrapHandler(h).SetCursorAtScroll(pos)
	if !ok {
		return errors.New("invalid cursor position")
	}
	return nil
}

func (e *simpleEditor) Cursor(h Handler) (term.Coordinates, error) {
	pos := e.unwrapHandler(h).cursor.CursorAtScroll()
	return pos, nil
}

func (e *simpleEditor) unwrapHandler(h Handler) *simpleEditorHandler {
	if !e.auxBar {
		return e.pub.UnwrapHandler(h).(*simpleEditorHandler)
	}
	return e.pub.UnwrapHandler(UnwrapAuxBar(h)).(*simpleEditorHandler)
}

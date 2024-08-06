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
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/clipboard"
)

// DefaultSimpleEditor returns a simple to use Editor implementation.
func DefaultSimpleEditor(clipboard clipboard.Register) Editor {
	searchAttr := term.Attributes{Attrs: tcell.AttrReverse}
	return NewSimpleEditor(clipboard, false, true,
		term.Attributes{}, searchAttr, term.Attributes{})
}

// NewSimpleEditor allocates storage for a new Editor and initializes it.
func NewSimpleEditor(
	clipboard clipboard.Register,
	wrap, commandBar bool,
	attr, searchAttr, barAttr term.Attributes,
) Editor {
	ret := new(simpleEditor)
	ret.wrap = wrap
	ret.commandBar = commandBar
	ret.resAttr = searchAttr
	ret.barAttr = barAttr
	ret.attr = attr
	ret.clipboard = clipboard
	ret.pub.Init()
	return ret
}

type simpleEditor struct {
	pub        Publisher
	wrap       bool
	commandBar bool
	attr       term.Attributes
	resAttr    term.Attributes
	barAttr    term.Attributes
	clipboard  clipboard.Register
}

func (e *simpleEditor) Edit(file workspaceapi.URI, buf *cell.Buffer) (Handler, error) {
	rootIfc := NewSimpleHandler(e.clipboard, buf, file, e.wrap,
		e.commandBar, e.attr, e.resAttr, e.barAttr)
	root := rootIfc.(*simpleEditorHandler)
	return e.pub.PublishEdit(file, buf, root, &root.cursor), nil
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
	e.pub.Handler(h).(*simpleEditorHandler).cursor.SetLocationList(pri, ID, loc)
	return nil
}

func (e *simpleEditor) MoveToNextLocation(h Handler, ID string) error {
	dispatch := e.pub.RecordCursorChange(h)
	defer dispatch()

	e.pub.Handler(h).(*simpleEditorHandler).cursor.MoveToNextLocation(ID)
	return nil
}

func (e *simpleEditor) MoveToPrevLocation(h Handler, ID string) error {
	dispatch := e.pub.RecordCursorChange(h)
	defer dispatch()

	e.pub.Handler(h).(*simpleEditorHandler).cursor.MoveToPrevLocation(ID)
	return nil
}

func (e *simpleEditor) CellView(h Handler) CellView {
	return NewCellView(e.pub.Handler(h).(*simpleEditorHandler).buf.View())
}

func (e *simpleEditor) CellEditor(h Handler) CellEditor {
	return NewCellEditor(e.pub.Handler(h).(*simpleEditorHandler).buf.Editor())
}

func (e *simpleEditor) SetDefaultAttributes(h Handler, attr term.Attributes) error {
	e.pub.Handler(h).(*simpleEditorHandler).less.Scroll().Attributes = attr
	return nil
}

func (e *simpleEditor) SetCursor(h Handler, pos term.Coordinates) error {
	dispatch := e.pub.RecordCursorChange(h)
	defer dispatch()

	ok := e.pub.Handler(h).(*simpleEditorHandler).SetCursorAtScroll(pos)
	if !ok {
		return errors.New("MoveToScroll: invalid cursor position")
	}
	return nil
}

func (e *simpleEditor) Cursor(h Handler) (term.Coordinates, error) {
	pos := e.pub.Handler(h).(*simpleEditorHandler).cursor.CursorAtScroll()
	return pos, nil
}

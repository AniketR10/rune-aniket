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

package vi

import (
	"errors"

	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

type viEditor struct {
	text.Publisher
	opts []Option
}

// Editor returns a Vi text.Editor.
func Editor(opts ...Option) text.Editor {
	ret := &viEditor{opts: opts}
	ret.Publisher.Init()
	return ret
}

func (e *viEditor) Edit(file workspaceapi.URI, buf *cell.Buffer) (text.Handler, error) {
	root := New(buf, file, e.opts...)
	// publisher does not mutate cursor and it should never do so
	cursor := root.cursor
	return e.Publisher.PublishEdit(file, buf, root, cursor), nil
}

// SubscribeCommand is not supported.
func (e *viEditor) SubscribeCommand(cmd textapi.CommandManual, h text.CommandHandler) error {
	return errors.New("not supported")
}

func (c *viEditor) UnsubscribeCommand(cmd string) error {
	return errors.New("not supported")
}

// Editor is not supported
func (e *viEditor) Editor(file workspaceapi.URI) (text.Handler, error) {
	// NOTE: it would be dead code
	return nil, errors.New("not supported")
}

// SubscribeEvents subsribes sub to ev. Note that this Editor is only capable
// of dispatching EventTypeOpen, EventTypeEdit EventType events.
func (e *viEditor) SubscribeEvents(
	evs []textapi.EventType, sub text.EventHandler,
) error {
	e.Publisher.SubscribeEvents(evs, sub)
	return nil
}

func (e *viEditor) UnsubscribeEvents(sub text.EventHandler) (bool, error) {
	ok := e.Publisher.UnsubscribeEvents(sub)
	return ok, nil
}

func (e *viEditor) SetDefaultAttributes(h text.Handler, attrs term.Attributes) error {
	return e.Publisher.Handler(h).(*Vi).SetDefaultAttributes(attrs)
}

func (e viEditor) SetLocationList(
	h text.Handler, pri textapi.LocationPriority, ID string, loc text.LocationList,
) error {
	e.Publisher.Handler(h).(*Vi).SetLocationList(pri, ID, loc)
	return nil
}

func (e *viEditor) MoveToNextLocation(h text.Handler, ID string) error {
	dispatch := e.Publisher.RecordCursorChange(h)
	defer dispatch()

	e.Publisher.Handler(h).(*Vi).MoveToNextLocation(ID)
	return nil
}

func (e *viEditor) MoveToPrevLocation(h text.Handler, ID string) error {
	dispatch := e.Publisher.RecordCursorChange(h)
	defer dispatch()

	e.Publisher.Handler(h).(*Vi).MoveToPrevLocation(ID)
	return nil
}

func (e *viEditor) CellView(h text.Handler) text.CellView {
	return text.NewCellView(e.Publisher.Handler(h).(*Vi).CellView())
}

func (e *viEditor) CellEditor(h text.Handler) text.CellEditor {
	return text.NewCellEditor(e.Publisher.Handler(h).(*Vi).CellEditor())
}

func (e *viEditor) SetCursor(h text.Handler, pos term.Coordinates) error {
	dispatch := e.Publisher.RecordCursorChange(h)
	defer dispatch()

	vi := e.Publisher.Handler(h).(*Vi)
	ok := vi.SetCursorAtScroll(pos)
	if !ok {
		if vi.CursorAtScroll() == pos {
			// already set at position
			return nil
		}
		return errors.New("SetCursor: invalid cursor position")
	}
	return nil
}

func (e *viEditor) Cursor(h text.Handler) (term.Coordinates, error) {
	pos := e.Publisher.Handler(h).(*Vi).CursorAtScroll()
	return pos, nil
}

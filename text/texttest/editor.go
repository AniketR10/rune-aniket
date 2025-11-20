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

//revive:disable:exported
package texttest

import (
	context "context"
	"errors"

	"github.com/unstablebuild/blue/iterator"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/workspaceapi"
	cell "unstable.build/go-tui/cell"
	term "unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

// EditorFromAPIEditor wraps a textapi.Editor to satisfy text.Editor.
type EditorFromAPIEditor struct {
	Ed textapi.Editor
}

func (e EditorFromAPIEditor) CellView(h text.Handler) text.CellView {
	return e.Ed.CellView(h)
}

func (e EditorFromAPIEditor) CellEditor(h text.Handler) text.CellEditor {
	return e.Ed.CellEditor(h)
}
func (e EditorFromAPIEditor) Edit(file workspaceapi.URI, buf *cell.Buffer) (text.Handler, error) {
	return nil, errors.New("Edit is unimplemented on textapi.Editor")
}

func (e EditorFromAPIEditor) SubscribeEvents(t []textapi.EventType, h text.EventHandler) error {
	return e.Ed.SubscribeEvents(t, h)
}

func (e EditorFromAPIEditor) UnsubscribeEvents(h text.EventHandler) (bool, error) {
	return e.Ed.(interface {
		UnsubscribeEvents(text.EventHandler) (bool, error)
	}).UnsubscribeEvents(h)
}

func (e EditorFromAPIEditor) Editor(file workspaceapi.URI) (text.Handler, error) {
	ed, err := e.Ed.Editor(file)
	if err != nil {
		return nil, err
	}
	return HandlerFromAPIHandler{ed}, nil
}

func (e EditorFromAPIEditor) SubscribeCommand(cmd textapi.CommandManual, h text.CommandHandler) error {
	return e.Ed.(interface {
		SubscribeCommand(textapi.CommandManual, textapi.CommandHandler) error
	}).SubscribeCommand(cmd, APICommandHandlerFromCommandHandler{h})
}

func (e EditorFromAPIEditor) UnsubscribeCommand(cmd string) error {
	return e.Ed.(interface {
		UnsubscribeCommand(string) error
	}).UnsubscribeCommand(cmd)
}

func (e EditorFromAPIEditor) SetLocationList(h text.Handler, p textapi.LocationPriority, arg string, l text.LocationList) error {
	return e.Ed.SetLocationList(h, p, arg, l)
}

func (e EditorFromAPIEditor) MoveToNextLocation(h text.Handler, ID string) error {
	return e.Ed.MoveToNextLocation(h, ID)
}

func (e EditorFromAPIEditor) MoveToPrevLocation(h text.Handler, ID string) error {
	return e.Ed.MoveToPrevLocation(h, ID)
}

func (e EditorFromAPIEditor) Cursor(h text.Handler) (term.Coordinates, error) {
	return e.Ed.Cursor(h)
}

func (e EditorFromAPIEditor) SetCursor(h text.Handler, pos term.Coordinates) error {
	return e.Ed.SetCursor(h, pos)
}

func (e EditorFromAPIEditor) SetDefaultAttributes(h text.Handler, attr term.Attributes) error {
	return e.Ed.SetDefaultAttributes(h, attr)
}

// HandlerFromAPIHandler adapts api Handler to text.Handler.
type HandlerFromAPIHandler struct {
	textapi.Handler
}

func (w HandlerFromAPIHandler) SetWrap(wrap bool) {
}

func (w HandlerFromAPIHandler) ShowCommandBar(show bool) {
}

func (w HandlerFromAPIHandler) SetCursorAtScroll(pos term.Coordinates) bool {
	return false
}

// SeekUp satisfies component.Scrollable.
func (t HandlerFromAPIHandler) SeekUp() bool {
	return false
}

// SeekDown satisfies component.Scrollable.
func (t HandlerFromAPIHandler) SeekDown() bool {
	return false
}

// SeekOffset satisfies component.Scrollable.
func (t HandlerFromAPIHandler) SeekOffset() int {
	return 0
}

// MaxSeekOffset satisfies component.Scrollable.
func (t HandlerFromAPIHandler) MaxSeekOffset() int {
	return 0
}

type APICommandHandlerFromCommandHandler struct {
	text.CommandHandler
}

func (c APICommandHandlerFromCommandHandler) Complete(ctx context.Context, name string, args []string) (
	iterator.Iterator[string], error,
) {
	it, _, err := c.CommandHandler.Complete(ctx, textapi.Command{Name: name, Args: args})
	return it, err
}

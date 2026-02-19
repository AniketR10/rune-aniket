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

package ide

import (
	"context"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/text"
)

var _ textapi.Editor = (*editorAdapter)(nil)

func newEditorAdapter(e text.Editor) textapi.Editor {
	return &editorAdapter{e: e}
}

type editorAdapter struct {
	e text.Editor
}

func (a editorAdapter) SubscribeEvents(
	types []textapi.EventType,
	h textapi.EventHandler,
) error {
	return a.e.SubscribeEvents(types, h)
}

func (a editorAdapter) Editor(
	uri workspaceapi.URI,
) (textapi.Handler, error) {
	return a.e.Editor(uri)
}

func (a editorAdapter) SetLocationList(
	h textapi.Handler,
	p textapi.LocationPriority,
	id string,
	l textapi.LocationList,
) error {
	eh, err := asEditorHandler(h)
	if err != nil {
		return err
	}
	eh.SetLocationList(p, id, l)
	return nil
}

func (a editorAdapter) MoveToNextLocation(
	h textapi.Handler, id string,
) error {
	eh, err := asEditorHandler(h)
	if err != nil {
		return err
	}
	if !eh.MoveToNextLocation(id) {
		return fmt.Errorf(
			"no next location for list %q", id,
		)
	}
	return nil
}

func (a editorAdapter) MoveToPrevLocation(
	h textapi.Handler, id string,
) error {
	eh, err := asEditorHandler(h)
	if err != nil {
		return err
	}
	if !eh.MoveToPrevLocation(id) {
		return fmt.Errorf(
			"no previous location for list %q",
			id,
		)
	}
	return nil
}

func (a editorAdapter) Cursor(
	h textapi.Handler,
) (term.Coordinates, error) {
	eh, err := asEditorHandler(h)
	if err != nil {
		return term.Coordinates{}, err
	}
	return eh.CursorAtScroll(), nil
}

func (a editorAdapter) SetCursor(
	h textapi.Handler, pos term.Coordinates,
) error {
	eh, err := asEditorHandler(h)
	if err != nil {
		return err
	}
	if !eh.SetCursorAtScroll(pos) {
		return fmt.Errorf(
			"could not set cursor at (%d, %d)",
			pos.X, pos.Y,
		)
	}
	return nil
}

func (a editorAdapter) CellView(
	h textapi.Handler,
) textapi.CellView {
	eh, ok := h.(text.Handler)
	if !ok {
		return nil
	}
	return adaptCells(eh.CellView())
}

func (a editorAdapter) CellEditor(
	h textapi.Handler,
) textapi.CellEditor {
	eh, ok := h.(text.Handler)
	if !ok {
		return nil
	}
	return adaptEditor(eh.CellEditor())
}

func (a editorAdapter) SetDefaultAttributes(
	h textapi.Handler, attr term.Attributes,
) error {
	eh, err := asEditorHandler(h)
	if err != nil {
		return err
	}
	eh.SetDefaultAttributes(attr)
	return nil
}

func asEditorHandler(
	h textapi.Handler,
) (text.Handler, error) {
	eh, ok := h.(text.Handler)
	if !ok {
		return nil, fmt.Errorf("extraneous text.Handler")
	}
	return eh, nil
}

func adaptEditor(ed cell.Editor) textapi.CellEditor {
	return cellEditor{ed}
}

func adaptCells(view cell.View) textapi.CellView {
	return cellView{view}
}

type cellView struct {
	cell.View
}

type cellEditor struct {
	cell.Editor
}

func (c cellView) RawCells() ([][]term.Cell, error) {
	return c.View.RawCells(), nil
}

func (c cellEditor) Edit(ctx context.Context, start, end term.Coordinates, str string) (
	from, to term.Coordinates, old string, err error,
) {
	from, to, old = c.Editor.Edit(ctx, start, end, str)
	return
}

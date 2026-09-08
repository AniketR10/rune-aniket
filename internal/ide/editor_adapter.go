// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package ide

import (
	"context"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/cell"
	"unstable.build/rune/internal/text"
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

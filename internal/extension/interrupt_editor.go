// Copyright (C) 2017-2026 The Rune Authors
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

package extension

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/textapi/textrpc"
)

// this structure wraps a text.Editor to
// provide interrupt on write requests coming from the wire
type interruptEditor struct {
	textrpc.EditorServer
	interruptDraw func()
}

func interruptEditorServer(srv textrpc.EditorServer, interruptDraw func()) textrpc.EditorServer {
	return &interruptEditor{EditorServer: srv, interruptDraw: interruptDraw}
}

func (e *interruptEditor) Edit(ctx context.Context, req *textrpc.EditRequest) (
	*textrpc.EditResponse, error,
) {
	res, err := e.EditorServer.Edit(ctx, req)
	e.interruptDraw()
	return res, err
}

func (e *interruptEditor) SubscribeEvent(stream textrpc.Editor_SubscribeEventServer) error {
	return e.EditorServer.SubscribeEvent(stream)
}

func (e *interruptEditor) SubscribeCommand(stream textrpc.Editor_SubscribeCommandServer) error {
	return e.EditorServer.SubscribeCommand(stream)

}

func (e *interruptEditor) SetLocationList(ctx context.Context, req *textrpc.SetLocationListRequest) (
	*textrpc.SetLocationListResponse, error,
) {
	res, err := e.EditorServer.SetLocationList(ctx, req)
	e.interruptDraw()
	return res, err

}
func (e *interruptEditor) MoveToNextLocation(ctx context.Context, req *textrpc.MoveToLocationRequest) (
	*textrpc.MoveToLocationResponse, error,
) {
	res, err := e.EditorServer.MoveToNextLocation(ctx, req)
	e.interruptDraw()
	return res, err

}
func (e *interruptEditor) MoveToPrevLocation(ctx context.Context, req *textrpc.MoveToLocationRequest) (
	*textrpc.MoveToLocationResponse, error,
) {
	res, err := e.EditorServer.MoveToPrevLocation(ctx, req)
	e.interruptDraw()
	return res, err

}
func (e *interruptEditor) EditCell(ctx context.Context, req *textrpc.EditCellRequest) (
	*textrpc.EditCellResponse, error,
) {
	res, err := e.EditorServer.EditCell(ctx, req)
	e.interruptDraw()
	return res, err

}

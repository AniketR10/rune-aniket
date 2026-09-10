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

package handlerrpctest

import handlerrpc "github.com/unstablebuild/rune-go-sdk/handler/handlerrpc"

// SetDraw satisfies handlerrpc.StreamMessage.
func (m *TestMessage) SetDraw(r *handlerrpc.DrawStreamResponse) {
	m.Draw = r
	m.Type = handlerrpc.MessageType_Draw
}

// SetHandle satisfies handlerrpc.StreamMessage.
func (m *TestMessage) SetHandle(r *handlerrpc.HandleStreamResponse) {
	m.Handle = r
	m.Type = handlerrpc.MessageType_Handle
}

// SetClose satisfies handlerrpc.StreamMessage.
func (m *TestMessage) SetClose(r *handlerrpc.CloseStreamResponse) {
	m.Close = r
	m.Type = handlerrpc.MessageType_Close
}

// SetCursor satisfies handlerrpc.StreamMessage.
func (m *TestMessage) SetCursor(r *handlerrpc.CursorStreamResponse) {
	m.Cursor = r
	m.Type = handlerrpc.MessageType_Cursor
}

// SetSelection satisfies handlerrpc.StreamMessage.
func (m *TestMessage) SetSelection(r *handlerrpc.SelectionStreamResponse) {
	m.Selection = r
	m.Type = handlerrpc.MessageType_Selection
}

// SetDimensions satisfies handlerrpc.StreamMessage.
func (m *TestMessage) SetDimensions(r *handlerrpc.DimensionsStreamResponse) {
	m.Dimensions = r
	m.Type = handlerrpc.MessageType_Dimensions
}

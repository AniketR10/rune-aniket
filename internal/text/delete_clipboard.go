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

package text

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/cell"
)

type delClip struct {
	clipboard  clipboard.Register
	registerID string
	pub        cell.Publisher
	cur        *Cursor
	mode       SelectMode
}

// WithCopyDelete installs a cell.Subscriber to a cell.Buffer
// which persists all the deleted content to a Clipboard.
func WithCopyDelete(
	registerID string, clipboard clipboard.Register,
	cur *Cursor, buf *cell.Buffer,
) {
	c := new(delClip)
	c.clipboard = clipboard
	c.pub = buf
	c.cur = cur
	c.registerID = registerID
	buf.SubscribeUsage(c)
}

func (c *delClip) OnWillEdit(
	ctx context.Context, start, end term.Coordinates, str string,
) {
	if start == end {
		return
	}

	var ok bool
	c.mode, ok = c.cur.SelectionMode()
	if !ok {
		c.mode = StandardSelection
	}
}

func (c *delClip) OnDidEdit(
	ctx context.Context, from, to term.Coordinates, old string,
) {
	if old != "" {
		_ = c.clipboard.Copy(c.registerID, clipboard.Data{Text: old, Metadata: c.mode})
	}
}

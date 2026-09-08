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

package locationpicker

import (
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

type pickerInner struct {
	handler *Picker
}

func (li *pickerInner) Dimensions() (int, int) {
	h := li.handler
	idealW := h.maxEntryW
	for _, cells := range h.previewCells {
		idealW = max(idealW, len(cells))
	}
	idealW = max(idealW, minPreviewWidth)
	listH := min(len(h.entries), maxListHeight)
	return idealW, listH + previewContextLines + separatorHeight
}

func (li *pickerInner) Resize(w, h int) {
	handler := li.handler
	handler.innerW = w
	handler.innerH = h
	handler.previewH = max(0, min(previewContextLines, h-1-separatorHeight))
	handler.listH = h - handler.previewH - separatorHeight
	handler.listH = max(handler.listH, 1)
	handler.list.Resize(w, handler.listH)
}

func (li *pickerInner) Draw(w term.Writer) {
	h := li.handler
	h.drawPreview(w)
	h.drawSeparator(w)
	vw := &component.VirtualWriter{
		Writer: w,
		Offset: term.Coordinates{Y: h.previewH + separatorHeight},
		Width:  h.innerW,
		Height: h.listH,
	}
	h.list.Draw(vw)
}

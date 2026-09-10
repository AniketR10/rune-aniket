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

package drawrect

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// Batch accumulates filled rectangles and strokes into a single reused
// vertex/index buffer so an entire frame's worth of solid geometry can
// be issued with one DrawTriangles call. Because every quad shares the
// same source image (whiteSubImage), shader, blend and fill rule, and
// per-quad color is baked into the vertices, a single DrawTriangles is
// enough — and ebiten does not have to merge many small commands.
//
// A Batch is not safe for concurrent use; the renderer drives it from
// the single GUI goroutine.
type Batch struct {
	path     Path
	vertices []ebiten.Vertex
	indices  []uint16
}

// Reset clears the batch for a new frame without releasing the backing
// arrays, so steady-state framing does not allocate.
func (b *Batch) Reset() {
	b.vertices = b.vertices[:0]
	b.indices = b.indices[:0]
}

// Empty reports whether the batch has no accumulated geometry.
func (b *Batch) Empty() bool {
	return len(b.indices) == 0
}

// AddRect appends a filled rectangle in the given color to the batch.
func (b *Batch) AddRect(x, y, width, height float32, clr color.RGBA) {
	b.path.Reset()
	b.path.MoveTo(x, y)
	b.path.LineTo(x, y+height)
	b.path.LineTo(x+width, y+height)
	b.path.LineTo(x+width, y)
	start := len(b.vertices)
	b.vertices, b.indices = b.path.AppendVerticesAndIndicesForFilling(b.vertices, b.indices)
	colorize(b.vertices[start:], clr)
}

// AddStroke appends a stroked line segment in the given color to the
// batch.
func (b *Batch) AddStroke(x0, y0, x1, y1, strokeWidth float32, clr color.RGBA) {
	b.path.Reset()
	b.path.MoveTo(x0, y0)
	b.path.LineTo(x1, y1)
	op := &StrokeOptions{Width: strokeWidth}
	start := len(b.vertices)
	b.vertices, b.indices = b.path.AppendVerticesAndIndicesForStroke(b.vertices, b.indices, op)
	colorize(b.vertices[start:], clr)
}

// Flush issues the accumulated geometry as one DrawTriangles call and
// resets the batch. It is a no-op when nothing was accumulated.
func (b *Batch) Flush(dst *ebiten.Image) {
	if len(b.indices) == 0 {
		return
	}
	dst.DrawTriangles(b.vertices, b.indices, whiteSubImage, &defaultDrawTrianglesOptions)
	b.Reset()
}

// colorize bakes clr into the premultiplied per-vertex color channels
// of the given vertices.
func colorize(vs []ebiten.Vertex, clr color.RGBA) {
	r, g, bl, a := clr.RGBA()
	fr := float32(r) / 0xffff
	fg := float32(g) / 0xffff
	fb := float32(bl) / 0xffff
	fa := float32(a) / 0xffff
	for i := range vs {
		vs[i].SrcX = 1
		vs[i].SrcY = 1
		vs[i].ColorR = fr
		vs[i].ColorG = fg
		vs[i].ColorB = fb
		vs[i].ColorA = fa
	}
}

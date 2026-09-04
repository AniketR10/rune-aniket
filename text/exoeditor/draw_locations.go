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

package exoeditor

import (
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/term/vte/vteprobe"
)

// drawLocations overlays loc.Attr on every cell of w covered by a
// location in locs, using probe.Bands/Rows to translate file
// coordinates back to screen coordinates. Locations that map to a
// folded row or to a row outside the content band are skipped.
//
// The projector mirrors text.DrawLocations but reads its layout from
// a vteprobe.Result rather than a component.Scroll because the exo
// editor does not own a Rune scroll: the embedded vte editor does.
func drawLocations(
	w term.Writer, locs []textapi.Location, probe *vteprobe.Result,
) {
	if probe == nil {
		return
	}
	bodyWidth := probe.Bands.GridWidth - probe.Bands.GutterWidth
	if bodyWidth <= 0 {
		return
	}
	for _, loc := range locs {
		drawLocation(w, loc, probe, bodyWidth)
	}
}

// drawLocation paints a single location across every terminal row in
// the content band that holds a slice of the location's [From, To]
// range. Cells outside the band, on folded rows, or to the right of
// the visible body are dropped.
func drawLocation(
	w term.Writer, loc textapi.Location, probe *vteprobe.Result,
	bodyWidth int,
) {
	fromY := loc.From.Y
	toY := loc.To.Y
	if toY < fromY {
		fromY, toY = toY, fromY
	}
	for y := probe.Bands.Top; y <= probe.Bands.Bottom && y < len(probe.Rows); y++ {
		rm := probe.Rows[y]
		if rm.FileLine == 0 || rm.Folded {
			continue
		}
		fileLine := rm.FileLine - 1
		if fileLine < fromY || fileLine > toY {
			continue
		}
		startRaw, endRaw := lineCellRange(loc, fileLine, fromY, toY)
		var lineCells []term.Cell
		if fileLine >= 0 && fileLine < len(probe.FileLines) {
			lineCells = probe.FileLines[fileLine]
		}
		startCol := vteprobe.RawToVisualCol(lineCells, startRaw, probe.Tabstop)
		endCol := vteprobe.RawToVisualCol(lineCells, endRaw, probe.Tabstop)
		segStart := rm.WrapOffset
		segEnd := segStart + bodyWidth
		if startCol < segStart {
			startCol = segStart
		}
		if endCol > segEnd {
			endCol = segEnd
		}
		for col := startCol; col < endCol; col++ {
			screenX := probe.Bands.GutterWidth + (col - rm.WrapOffset)
			if screenX < 0 || screenX >= probe.Bands.GridWidth {
				continue
			}
			w.UnionAttributes(
				term.Coordinates{X: screenX, Y: y}, loc.Attr)
		}
	}
}

// lineCellRange returns [start, end) cell columns covered by loc on
// fileLine, assuming the location spans file lines [fromY, toY]. For
// intermediate lines the range is the whole visible body; for the
// first/last line the range is clipped by loc.From.X / loc.To.X.
//
// Values are in raw rune columns (file coordinates), not visual
// columns: the caller converts them to visual columns once it knows
// which file line each row belongs to.
func lineCellRange(
	loc textapi.Location, fileLine, fromY, toY int,
) (start, end int) {
	const wholeLine = 1 << 30
	switch {
	case fileLine == fromY && fileLine == toY:
		return loc.From.X, loc.To.X
	case fileLine == fromY:
		return loc.From.X, wholeLine
	case fileLine == toY:
		return 0, loc.To.X
	default:
		return 0, wholeLine
	}
}

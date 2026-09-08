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

package component

import (
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
)

// WriteText draws text at (x, y), one grapheme cluster per cell, and returns
// the column following the last cluster written.
//
// Wide clusters are written as a single Width 2 cell and their continuation
// column is left untouched, which is what the renderer expects. A cluster that
// would cross maxX is not drawn, so wide clusters never straddle the boundary.
func WriteText(
	w term.Writer, x, y, maxX int, text string, attr term.Attributes,
) int {
	state := -1
	var cluster string
	var width uint8
	for len(text) > 0 {
		// An ASCII byte whose successor is also ASCII (or end of string) is a
		// complete width-1 cluster: combining marks are never ASCII. This skips
		// the grapheme state machine for the dominant case.
		if text[0] < utf8.RuneSelf &&
			(len(text) == 1 || text[1] < utf8.RuneSelf) {
			if x >= maxX {
				break
			}
			w.SetCell(term.Coordinates{X: x, Y: y},
				term.NewCell(rune(text[0]), 1, attr))
			x++
			text = text[1:]
			state = -1
			continue
		}
		cluster, text, width, state = graphemecluster.StepString(text, state)
		cols := max(1, int(width))
		if x+cols > maxX {
			break
		}
		if len(cluster) == 0 {
			continue
		}
		var cell term.Cell
		if len(cluster) == 1 { // single byte, avoids the []rune alloc
			cell = term.NewCell(rune(cluster[0]), uint8(cols), attr)
		} else {
			runes := []rune(cluster)
			cell = term.NewCell(runes[0], uint8(cols), attr)
			if len(runes) > 1 {
				cell.SetCombining(runes[1:])
			}
		}
		w.SetCell(term.Coordinates{X: x, Y: y}, cell)
		x += cols
	}
	return x
}

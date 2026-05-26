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

package vte

import (
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term/vte/vtescreen"
)

// lastPromptLine this attempts to find the last "shell" prompt line,
// or falls back to returning the last block of content.
//
// cursorY is the row where the shell PTY cursor currently lives.
// When non-negative, lastPromptLine walks UPWARD from cursorY across
// wrapped continuation rows to find the prompt anchor (the topmost
// row whose predecessor is not a full-width wrap), then walks DOWN
// from there including each subsequent row only when its predecessor
// is full-width. This:
//   - excludes shell-drawn artifacts rendered below the real prompt
//     (e.g. orphan rows left over from completion-menu cleanup),
//   - correctly includes wrapped command lines, and
//   - works whether modal mode is entered on the prompt row or on
//     a wrapped continuation row.
//
// When cursorY is negative, lastPromptLine falls back to the legacy
// bottom-up search.
//
//	┌─────────┐
//	│$ OOOOO  │
//	│$ OOOOOOO│
//	│OOOOOOOO │
//	│$ XXXXXX │
//	└─────────┘
func lastPromptLine(view cell.View, width, cursorY int, excludeTailSpaces bool) (from term.Coordinates, to term.Coordinates) {
	cells := view.RawCells()
	if len(cells) == 0 {
		return
	}
	if cursorY >= 0 && cursorY < len(cells) {
		return lastPromptLineFromAnchor(cells, width, cursorY, excludeTailSpaces)
	}
	to.Y = len(cells) - 1
	to.X = len(cells[to.Y])

	if excludeTailSpaces {
	outerSpace:
		for y := len(cells) - 1; y >= 0; y-- {
			for x := len(cells[y]) - 1; x >= 0; x-- {
				ch := cells[y][x].Ch
				if ch != vtescreen.DefaultChar && ch != ' ' {
					to = term.Coordinates{Y: y, X: x + 1}
					break outerSpace
				}
			}
		}
	} else {
	outer:
		for y := len(cells) - 1; y >= 0; y-- {
			for x := len(cells[y]) - 1; x >= 0; x-- {
				if cells[y][x].Ch != vtescreen.DefaultChar {
					to = term.Coordinates{Y: y, X: x + 1}
					break outer
				}
			}
		}
	}

	var last term.Coordinates
	for y := to.Y; y >= 0; y-- {
		x := len(cells[y]) - 1
		if y == to.Y {
			x = to.X - 2
			// be robust against resizes. If there's a gap between
			// the end of the line and width, then consider that a
			// separate prompt line
		} else if x < width-1 {
			from = last
			return
		}
		for ; x >= 0; x-- {
			cell := cells[y][x]
			if cell.Ch == vtescreen.DefaultChar {
				from = last
				return
			}
			last = term.Coordinates{Y: y, X: x}
		}
	}

	from = last
	return
}

// lastPromptLineFromAnchor finds the prompt block by first walking
// UP from cursorY across wrapped continuation rows (rows whose
// predecessor is full-width), then walking DOWN from the anchor
// including each subsequent row only if its predecessor is
// full-width. This ignores rows below the block that are separated
// by a partial-width row (shell artifacts such as completion-menu
// cleanup orphans) while still picking up the real prompt row when
// the cursor sits on a wrapped continuation line.
func lastPromptLineFromAnchor(
	cells [][]term.Cell, width, cursorY int, excludeTailSpaces bool,
) (from, to term.Coordinates) {
	// last meaningful column on a row, taking excludeTailSpaces
	// into account (matches viHandler.lastValidLineColumn).
	lastContentX := func(y int) int {
		if y < 0 || y >= len(cells) {
			return -1
		}
		row := cells[y]
		for x := len(row) - 1; x >= 0; x-- {
			ch := row[x].Ch
			if ch == vtescreen.DefaultChar {
				continue
			}
			if excludeTailSpaces && ch == ' ' {
				continue
			}
			return x
		}
		return -1
	}

	// Walk UP from cursorY across wrap-continuation rows to find
	// the prompt anchor: the topmost row whose predecessor is not
	// full-width.
	anchorY := cursorY
	for anchorY > 0 && lastContentX(anchorY-1) >= width-1 {
		anchorY--
	}

	from = term.Coordinates{Y: anchorY, X: 0}
	to = term.Coordinates{Y: anchorY, X: 0}

	prevFullWidth := true // unused for the anchor row itself
	for y := anchorY; y < len(cells); y++ {
		if y > anchorY && !prevFullWidth {
			break
		}
		lx := lastContentX(y)
		if lx < 0 {
			if y == anchorY {
				// no content on the prompt row; return
				// an empty block at the anchor.
				return
			}
			break
		}
		to = term.Coordinates{Y: y, X: lx + 1}
		prevFullWidth = lx >= width-1
	}
	return
}

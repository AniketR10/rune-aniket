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

package dialoguetui

import (
	"context"
	"strings"
	"unicode"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

// selRange represents a selection coordinate pair.
// start and end are inclusive and sorted (start <= end in file order).
type selRange struct {
	start, end term.Coordinates
	active     bool
}

func (s *selRange) Contains(pos term.Coordinates) bool {
	if !s.active {
		return false
	}
	if pos.Y < s.start.Y || pos.Y > s.end.Y {
		return false
	}
	if s.start.Y == s.end.Y {
		return pos.X >= s.start.X && pos.X <= s.end.X
	}
	if pos.Y == s.start.Y {
		return pos.X >= s.start.X
	}
	if pos.Y == s.end.Y {
		return pos.X <= s.end.X
	}
	return true
}

// cellGrid is a term.Writer that captures Draw output as a flat cell grid.
// It is modeled after term.StringWriter.
type cellGrid struct {
	cells         []term.Cell
	width, height int
	ctx           context.Context
}

var _ term.Writer = (*cellGrid)(nil)

func (g *cellGrid) Resize(w, h int) {
	needed := w * h
	if cap(g.cells) >= needed {
		g.cells = g.cells[:needed]
	} else {
		g.cells = make([]term.Cell, needed)
	}
	g.width, g.height = w, h
	g.Clear()
}

func (g *cellGrid) Clear() {
	clear(g.cells)
}

func (g *cellGrid) outOfBounds(pos term.Coordinates) bool {
	return pos.X < 0 || pos.Y < 0 || pos.X >= g.width || pos.Y >= g.height
}

func (g *cellGrid) SetCell(pos term.Coordinates, cell term.Cell) {
	if g.outOfBounds(pos) {
		return
	}
	g.cells[pos.Y*g.width+pos.X] = cell
}

func (g *cellGrid) UnionAttributes(pos term.Coordinates, attr term.Attributes) {
	if g.outOfBounds(pos) {
		return
	}
	idx := pos.Y*g.width + pos.X
	g.cells[idx].Attributes = term.AttributesUnion(g.cells[idx].Attributes, attr)
}

func (g *cellGrid) Context() context.Context {
	if g.ctx != nil {
		return g.ctx
	}
	return context.Background()
}

// ClearReverse strips AttrReverse from every cell in the rectangle
// starting at (x0, y0) with the given width and height.
func (g *cellGrid) ClearReverse(x0, y0, w, h int) {
	for y := y0; y < y0+h && y < g.height; y++ {
		for x := x0; x < x0+w && x < g.width; x++ {
			idx := y*g.width + x
			g.cells[idx].Attrs &^= tcell.AttrReverse
		}
	}
}

// Dump copies all cells to w. If sel is non-nil and active, cells within the
// selection range have AttrReverse applied via UnionAttributes.
func (g *cellGrid) Dump(w term.Writer, sel *selRange) {
	reverseAttr := term.Attributes{Attrs: tcell.AttrReverse}
	for y := range g.height {
		for x := range g.width {
			pos := term.Coordinates{X: x, Y: y}
			cell := g.cells[y*g.width+x]
			if cell.Ch == 0 {
				cell.Ch = ' '
				cell.Width = 1
			}
			w.SetCell(pos, cell)
			if sel != nil && sel.Contains(pos) {
				w.UnionAttributes(pos, reverseAttr)
			}
		}
	}
}

// TextBetween extracts text from cells between start and end (inclusive).
func (g *cellGrid) TextBetween(start, end term.Coordinates) string {
	start, end = term.CoordinatesSort(start, end)
	var lines []string
	for y := start.Y; y <= end.Y; y++ {
		if y < 0 || y >= g.height {
			continue
		}
		startX := 0
		endX := g.width - 1
		if y == start.Y {
			startX = start.X
		}
		if y == end.Y {
			endX = end.X
		}
		if startX < 0 {
			startX = 0
		}
		if endX >= g.width {
			endX = g.width - 1
		}
		var line strings.Builder
		for x := startX; x <= endX; x++ {
			cell := g.cells[y*g.width+x]
			if cell.Ch == 0 {
				continue
			}
			line.WriteRune(cell.Ch)
			for _, r := range cell.Combining {
				line.WriteRune(r)
			}
		}
		lines = append(lines, strings.TrimRight(line.String(), " \t"))
	}
	// Trim trailing empty lines.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// WordBoundsAt returns the start and end coordinates (inclusive) of the word
// at pos. ok is false if pos is out of bounds, empty, or whitespace.
func (g *cellGrid) WordBoundsAt(pos term.Coordinates) (start, end term.Coordinates, ok bool) {
	if g.outOfBounds(pos) {
		return
	}
	cell := g.cells[pos.Y*g.width+pos.X]
	if cell.Ch == 0 || unicode.IsSpace(cell.Ch) {
		return
	}
	isWord := isWordChar(cell.Ch)

	startX := pos.X
	for startX > 0 {
		prev := g.cells[pos.Y*g.width+startX-1]
		if prev.Ch == 0 || unicode.IsSpace(prev.Ch) || isWordChar(prev.Ch) != isWord {
			break
		}
		startX--
	}
	endX := pos.X
	for endX < g.width-1 {
		next := g.cells[pos.Y*g.width+endX+1]
		if next.Ch == 0 || unicode.IsSpace(next.Ch) || isWordChar(next.Ch) != isWord {
			break
		}
		endX++
	}
	return term.Coordinates{X: startX, Y: pos.Y}, term.Coordinates{X: endX, Y: pos.Y}, true
}

// LineBounds returns the start and end coordinates (inclusive) of the
// non-empty content on row y. ok is false if the row is entirely empty.
func (g *cellGrid) LineBounds(y int) (start, end term.Coordinates, ok bool) {
	if y < 0 || y >= g.height {
		return
	}
	firstX := -1
	lastX := -1
	for x := range g.width {
		if g.cells[y*g.width+x].Ch != 0 {
			if firstX == -1 {
				firstX = x
			}
			lastX = x
		}
	}
	if firstX == -1 {
		return
	}
	return term.Coordinates{X: firstX, Y: y}, term.Coordinates{X: lastX, Y: y}, true
}

func isWordChar(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
}

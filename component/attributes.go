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
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/cell"
)

type attributesAt struct {
	term.Coordinates
	term.Attributes
}

var _ component.WithAttributes = (*AttrSetter)(nil)
var _ component.Floating = (*AttrSetter)(nil)
var _ component.Responsive = (*AttrSetter)(nil)

// AttrSetter wraps another tui.Component to satisfy WithAttributes
// and add the ability to set the Attributes of arbitrary cells.
type AttrSetter struct {
	def    term.Attributes
	attr   []attributesAt
	comp   tui.Component
	width  int
	height int
}

// WithAttrSetter wraps a tui.Component to satisfy WithAttributes.
func WithAttrSetter(comp tui.Component) *AttrSetter {
	attr := make([]attributesAt, 0, 20)
	return &AttrSetter{term.Attributes{}, attr, comp, 0, 0}
}

// Reset resets all the previous calls to SetAttrAt and SetAttr.
func (s *AttrSetter) Reset() {
	s.attr = s.attr[:0]
	s.def = term.Attributes{}
}

// SetAttr sets the default attributes drawn by this component.
// This attributes will be set in each of the final term.Cells drawn.
func (s *AttrSetter) SetAttr(attr term.Attributes) (ret term.Attributes) {
	ret = s.def
	s.def = attr
	return
}

// Dimensions satisfies Floating if underlying tui.Component
// satisfies Floating, or panics if it doesn't.
func (s *AttrSetter) Dimensions() (width, height int) {
	return s.comp.(component.Floating).Dimensions()
}

// Height satisfies Responsive if underlying tui.Component
// satisfies Responsive, or panics if it doesn't.
func (s *AttrSetter) Height(width int) int {
	return s.comp.(component.Responsive).Height(width)
}

// SetAttrAt sets the attributes at the given coordinates.
// If coordinates are out of bounds, this method will NOT panic, but
// the next call to Draw will.
func (s *AttrSetter) SetAttrAt(at term.Coordinates, attr term.Attributes) {
	s.attr = append(s.attr, attributesAt{at, attr})
}

// Content returns the underlying tui.Component of this AttrSetter.
func (s *AttrSetter) Content() tui.Component {
	return s.comp
}

// Resize satisfies tui.Component
func (s *AttrSetter) Resize(width, height int) {
	s.width, s.height = width, height
}

// Draw draws the underlying component along with
// the attributes.
func (s *AttrSetter) Draw(w term.Writer) {
	compWithAttr, is := s.comp.(component.WithAttributes)
	if is {
		compWithAttr.SetAttr(s.def)
	}

	var bw cell.BufferWriter
	bw.Init(w.Context(), s.width, s.height)
	s.comp.Resize(s.width, s.height)
	s.comp.Draw(&bw)
	cells := bw.RawCells()

	if !is {
		for y, row := range cells {
			for x := range row {
				cells[y][x].SetAttributes(term.AttributesUnion(cells[y][x].Attributes(), s.def))
			}
		}
	}

	for _, a := range s.attr {
		if a.Y >= s.height || a.Y < 0 ||
			a.X >= s.width || a.X < 0 {
			continue
		}
		cells[a.Y][a.X].SetAttributes(term.AttributesUnion(
			cells[a.Y][a.X].Attributes(), a.Attributes))
	}

	var buf cell.Buffer
	bw.ToBuffer(&buf)
	var sc Scroll
	sc.InitPerformance(&buf)
	sc.Resize(s.width, s.height)
	sc.Draw(w)
}

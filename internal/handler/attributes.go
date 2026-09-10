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

package handler

import (
	compapi "github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/internal/component"
)

var _ compapi.WithAttributes = AttrSetter{}

// AttrSetter provides an API like component.AttrSetter for a tui.Handler.
type AttrSetter struct {
	comp *component.AttrSetter
	tui.Handler
}

// WithAttrSetter wraps h with a component.AttrSetter.
func WithAttrSetter(h tui.Handler) AttrSetter {
	return AttrSetter{
		comp:    component.WithAttrSetter(h),
		Handler: h,
	}
}

// Resize satisfies tui.Component
func (s AttrSetter) Resize(width, height int) {
	s.comp.Resize(width, height)
}

// Draw satisfies tui.Component
func (s AttrSetter) Draw(w term.Writer) {
	s.comp.Draw(w)
}

// Reset resets all the previous calls to SetAttrAt and SetAttr.
func (s AttrSetter) Reset() {
	s.comp.Reset()
}

// SetAttr sets the default attributes drawn by this component.
// This attributes will be set in each of the final term.Cells drawn.
func (s AttrSetter) SetAttr(attr term.Attributes) term.Attributes {
	return s.comp.SetAttr(attr)
}

// SetAttrAt sets the attributes at the given coordinates.
// If coordinates are out of bounds, this method will NOT panic, but
// the next call to Draw will.
func (s AttrSetter) SetAttrAt(at term.Coordinates, attr term.Attributes) {
	s.comp.SetAttrAt(at, attr)
}

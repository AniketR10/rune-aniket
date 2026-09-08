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

package handler

import (
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

type keyMappingHandler struct {
	tui.Component
	inner    tui.Handler
	mappings map[term.KeyComb]term.KeyComb
}

// WithMapping takes a handler and a set of event mappings to provide
// key and event mapping to override default handler event handler.
func WithMapping(
	inner tui.Handler, mappings map[term.KeyComb]term.KeyComb,
) tui.Handler {
	return keyMappingHandler{inner, inner, mappings}
}

// Handle finds a mapping and overwrites event or delegates the event to
// underlying handler.
func (k keyMappingHandler) Handle(ev term.Event) (bool, bool) {
	if ev.Type != term.EventKey {
		return k.inner.Handle(ev)
	}
	mapped, ok := k.mappings[ev.KeyComb()]
	if ok {
		ev = term.Event{
			Type: term.EventKey,
			Ch:   mapped.Ch,
			Mod:  mapped.Mod,
			Key:  mapped.Key,
			Raw:  ev.Raw,
		}
	}
	return k.inner.Handle(ev)
}

// Cursor delegates call to underlying handler.
func (k keyMappingHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return k.inner.Cursor()
}

// Selection delegates call to underlying handler.
func (k keyMappingHandler) Selection() (string, bool) {
	return k.inner.Selection()
}

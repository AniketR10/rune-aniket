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

package extutil

import (
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/component"
	"unstable.build/rune/internal/text"
)

// TrackedResource holds the state of a resource, replicated
// via ResourceTracker.
type TrackedResource struct {
	cursor *text.Cursor
	uri    workspaceapi.URI

	// Scroll is a mirror of the monitored resource's scroll.
	// Clients MUST NOT use any methods that update
	// the state of this scroll.
	component.Scroll

	// Metadata can be used by users to store data along
	// a TrackedResource.
	Metadata any
}

// URI returns this TrackedResource's resource URI.
func (t *TrackedResource) URI() workspaceapi.URI {
	return t.uri
}

// Cursor returns the cursor coordinates of this TrackedResource.
// Using this requires ResourceTracker to be subscribed
// to EventTypeCursor events.
func (t *TrackedResource) Cursor() term.Coordinates {
	if t.cursor == nil {
		return term.Coordinates{}
	}
	return t.cursor.CursorAtScroll()
}

// WindowCoordinates translates the given content
// position into window coordinates, given this TrackedResource's
// offset (and wraps offset given width, height in wrap mode).
// Using this requires ResourceTracker to be subscribed
// to EventTypeCursor, EventTypeScroll and EventTypeFocus events.
// The second returned value is used to indicate that the given
// position is inside a hidden block (false), or not (true).
func (t *TrackedResource) WindowCoordinates(pos term.Coordinates) (term.Coordinates, bool) {
	if t.Scroll.Width() == 0 && t.Scroll.Wrap {
		return term.CoordinatesDiff(pos, t.Scroll.Offset()), true
	}
	return t.Scroll.ScrollToWindowCoordinates(pos)
}

// ContentCoordinates translates the given window coordinates
// into content coordinates, given this TrackedResource's
// offset (and wraps offset given width, height in wrap mode).
// Using this requires ResourceTracker to be subscribed
// to EventTypeCursor, EventTypeScroll and EventTypeFocus events.
func (t *TrackedResource) ContentCoordinates(pos term.Coordinates) term.Coordinates {
	return t.Scroll.WindowToScrollCoordinates(pos)
}

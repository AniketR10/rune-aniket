package util

import (
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

// TrackedResource holds the state of a resource, replicated
// via ResourceTracker.
type TrackedResource struct {
	cursor text.Cursor
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
	return t.cursor.CursorAtScroll()
}

// WindowCoordinates translates the given content
// position into window coordinates, given this TrackedResource's
// offset (and wraps offset given width, height in wrap mode).
// Using this requires ResourceTracker to be subscribed
// to EventTypeCursor, EventTypeScroll and EventTypeFocus events.
func (t *TrackedResource) WindowCoordinates(pos term.Coordinates) term.Coordinates {
	return t.cursor.WindowCoordinates(pos)
}

// ContentCoordinates translates the given window coordinates
// into content coordinates, given this TrackedResource's
// offset (and wraps offset given width, height in wrap mode).
// Using this requires ResourceTracker to be subscribed
// to EventTypeCursor, EventTypeScroll and EventTypeFocus events.
func (t *TrackedResource) ContentCoordinates(pos term.Coordinates) term.Coordinates {
	return t.cursor.ScrollCoordinates(pos)
}

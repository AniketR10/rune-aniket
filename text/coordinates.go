package text

import (
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// ScrollToWindowCoordinates translates scroll content Coordinates to window Coordinates,
// taking into consideration scroll offsets and wrapped lines.
//
// Deprecated: use component.Scroll.ScrollToWindowCoordinates
func ScrollToWindowCoordinates(scroll *component.Scroll, pos term.Coordinates) term.Coordinates {
	return scroll.ScrollToWindowCoordinates(pos)
}

// WindowToScrollCoordinates translates window Coordinates to scroll content Coordinates,
// taking into consideration scroll offsets and wrapped lines.
//
// Deprecated: use component.Scroll.WindowToScrollCoordinates
func WindowToScrollCoordinates(scroll *component.Scroll, pos term.Coordinates) term.Coordinates {
	return scroll.WindowToScrollCoordinates(pos)
}

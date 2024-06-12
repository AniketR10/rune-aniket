package term

import (
	"context"

	"github.com/unstablebuild/tcell/v3"
)

type (
	InputMode int
	EventType uint8
	Modifier  uint8
	Key       uint16
)

// Attributes represents a cell background and foreground attributes.
type Attributes tcell.Style

// Cell represents a location with content on a terminal screen.
// 'Ch' is a unicode character, 'Fg' and 'Bg' are foreground
// and background attributes respectively. Unicode graphene clusters
// should be processed accordingly and stored into Ch and Combining fields.
type Cell struct {
	Ch rune
	Attributes
	Combining []rune
	// Width is the monospace width
	Width int
}

// Event represents a terminal event. The 'Mod', 'Key' and 'Ch' fields are
// valid if 'Type' is EventKey. The 'Width' and 'Height' fields are valid if
// 'Type' is EventResize. The 'Err' field is valid if 'Type' is EventError.
type Event struct {
	Type     EventType // one of Event* constants
	Mod      Modifier  // one of Mod* constants or 0
	Key      Key       // one of Key* constants, invalid if 'Ch' is not 0
	Ch       rune      // a unicode character
	Width    int       // width of the screen
	Height   int       // height of the screen
	Err      error     // error in case if input failed
	MouseX   int       // x coord of mouse
	MouseY   int       // y coord of mouse
	Raw      []byte
	UserFunc func()
}

// KeyComb returns the KeyComb representation of this Event. If this
// event is not of type EventKey, then this method panics.
func (e Event) KeyComb() KeyComb {
	if e.Type != EventKey {
		panic("called KeyComb on non-key event")
	}
	return KeyComb{
		Key: e.Key,
		Mod: e.Mod,
		Ch:  e.Ch,
	}
}

// Writer abstracts termbox write functionality to decouple components from
// termbox, so they're easier to test.
type Writer interface {
	// Context returns the current context of the Writer.
	// This context can be used by tui.Components in combination
	// with term.Interrupter.Interrupt(context.Context) to disambiguate
	// regular calls to Draw from interrupt-driven calls to Draw.
	Context() context.Context
	// SetCell sets the contents of the given cell location.  If
	// the coordinates are out of range, then the operation is ignored.
	SetCell(Coordinates, Cell)
	// UnionAttributes computes the set union between a and b,
	// that is overrides a set of attributes at the given coordinates
	// that contain all the bit flags set in a, b or both, and uses the color
	// defined in b or if not set, uses the color in a.
	UnionAttributes(Coordinates, Attributes)
}

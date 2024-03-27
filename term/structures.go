package term

import (
	"context"

	"github.com/ernestrc/tcell/v3"
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

// KeyComb represents is a key combination. See event for more details.
type KeyComb struct {
	Mod Modifier
	Key Key
	Ch  rune
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

func (e Event) KeyComb() KeyComb {
	return KeyComb{
		Key: e.Key,
		Mod: e.Mod,
		Ch:  e.Ch,
	}
}

// Writer abstracts termbox write functionality to decouple components from
// termbox, so they're easier to test.
type Writer interface {
	Context() context.Context
	SetCell(Coordinates, Cell)
	UnionAttributes(Coordinates, Attributes)
	Flush() error
	Clear(Attributes) error
	SetCursor(Coordinates)
}

// ContextWriter adds SetContext to a Writer.
type ContextWriter interface {
	Writer
	SetContext(context.Context)
}

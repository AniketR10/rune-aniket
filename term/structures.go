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

package term

import (
	"context"

	"github.com/unstablebuild/tcell/v3"
)

type (
	// InputMode is the keyboard input mode.
	InputMode int
	// EventType is the type of event being dispatched.
	EventType uint8
	// Modifier is a key modifier like <alt> or <ctrl>.
	Modifier uint8
	// Key is a keyboard key.
	Key uint16
)

// Attributes represents a cell background and foreground attributes.
type Attributes tcell.Style

const (
	// AttrVerticalRenderOffset instructs the renderer to render
	// the cell with an offset equal to half the height of the cell.
	AttrVerticalRenderOffset tcell.AttrMask = tcell.AttrInvalid << 1
)

// Cell represents a location with content on a terminal screen.
// 'Ch' is a unicode character, 'Fg' and 'Bg' are foreground
// and background attributes respectively. Unicode graphene clusters
// should be processed accordingly and stored into Ch and Combining fields.
type Cell struct {
	Attributes
	// Ch is the main character held by this cell.
	// If character cannot fit in the storage provided by the
	// builtin 'rune', then Width() returns > 1 and Cell.Combining
	// contains the rest of data.
	Ch rune
	// Combining are the remaining data that does not fin in Ch.
	Combining []rune
	// Width returns the monospace width of this Cell.
	Width uint8
	// Bytes is the number of bytes consumed by this Cell.
	Bytes uint8
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

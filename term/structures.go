package term

import (
	"context"
	"fmt"

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
	// Context returns the current context of the Writer.
	// This context can be used by tui.Components in combination
	// with term.Interrupter.Interrupt(context.Context), to connect
	// requests to via interrupt with actual calls to Draw.
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

func (k KeyComb) String() string {
	if k.Key == 0 && k.Mod == 0 {
		return string(k.Ch)
	}

	if k.Ch != 0 && k.Key != 0 && k.Key != KeySpace {
		// this is invalid but return Ch
		return string(k.Ch)
	}

	if k.Ch != 0 && k.Mod == ModAlt && k.Key != KeySpace {
		return fmt.Sprintf("<m-%c>", k.Ch)
	}

	// non ModAlt +k.Ch are ignored

	switch k {
	case KeyComb{Key: KeyF1}:
		return "<f1>"
	case KeyComb{Key: KeyF2}:
		return "<f2>"
	case KeyComb{Key: KeyF3}:
		return "<f3>"
	case KeyComb{Key: KeyF4}:
		return "<f4>"
	case KeyComb{Key: KeyF5}:
		return "<f5>"
	case KeyComb{Key: KeyF6}:
		return "<f6>"
	case KeyComb{Key: KeyF7}:
		return "<f7>"
	case KeyComb{Key: KeyF8}:
		return "<f8>"
	case KeyComb{Key: KeyF9}:
		return "<f9>"
	case KeyComb{Key: KeyF10}:
		return "<f10>"
	case KeyComb{Key: KeyF11}:
		return "<f11>"
	case KeyComb{Key: KeyF12}:
		return "<f12>"
	case KeyComb{Key: KeyInsert}:
		return "<insert>"
	case KeyComb{Key: KeyDelete}:
		return "<delete>"
	case KeyComb{Key: KeyHome}:
		return "<home>"
	case KeyComb{Key: KeyEnd}:
		return "<end>"
	case KeyComb{Key: KeyPgup}:
		return "<pgup>"
	case KeyComb{Key: KeyPgdn}:
		return "<pgdn>"
	case KeyComb{Key: KeyArrowUp}:
		return "<up>"
	case KeyComb{Key: KeyArrowDown}:
		return "<down>"
	case KeyComb{Key: KeyArrowLeft}:
		return "<left>"
	case KeyComb{Key: KeyArrowRight}:
		return "<right>"
	case KeyComb{Key: MouseLeft}:
		return "<mouse-left>"
	case KeyComb{Key: MouseMiddle}:
		return "<mouse-middle>"
	case KeyComb{Key: MouseRight}:
		return "<mouse-right>"
	case KeyComb{Key: MouseRelease}:
		return "<mouse-release>"
	case KeyComb{Key: MouseWheelUp}:
		return "<mouse-wheel-up>"
	case KeyComb{Key: MouseWheelDown}:
		return "<mouse-wheel-down>"
	case KeyComb{Key: KeyCtrlTilde}:
		return "<c-`>"
	case KeyComb{Key: KeyCtrl2}:
		return "<c-2>"
	case KeyComb{Key: KeyCtrlSpace}:
		return "<c-space>"
	case KeyComb{Key: KeyCtrlA}:
		return "<c-a>"
	case KeyComb{Key: KeyCtrlB}:
		return "<c-b>"
	case KeyComb{Key: KeyCtrlC}:
		return "<c-c>"
	case KeyComb{Key: KeyCtrlD}:
		return "<c-d>"
	case KeyComb{Key: KeyCtrlE}:
		return "<c-e>"
	case KeyComb{Key: KeyCtrlF}:
		return "<c-f>"
	case KeyComb{Key: KeyCtrlG}:
		return "<c-g>"
	case KeyComb{Key: KeyBackspace}:
		return "<c-backspace>"
	case KeyComb{Key: KeyCtrlH}:
		return "<c-h>"
	case KeyComb{Key: KeyTab}:
		return "<c-tab>"
	case KeyComb{Key: KeyCtrlI}:
		return "<c-i>"
	case KeyComb{Key: KeyCtrlJ}:
		return "<c-j>"
	case KeyComb{Key: KeyCtrlK}:
		return "<c-k>"
	case KeyComb{Key: KeyCtrlL}:
		return "<c-l>"
	case KeyComb{Key: KeyEnter}:
		return "<enter>"
	case KeyComb{Key: KeyCtrlM}:
		return "<c-m>"
	case KeyComb{Key: KeyCtrlN}:
		return "<c-n>"
	case KeyComb{Key: KeyCtrlO}:
		return "<c-o>"
	case KeyComb{Key: KeyCtrlP}:
		return "<c-p>"
	case KeyComb{Key: KeyCtrlQ}:
		return "<c-q>"
	case KeyComb{Key: KeyCtrlR}:
		return "<c-r>"
	case KeyComb{Key: KeyCtrlS}:
		return "<c-s>"
	case KeyComb{Key: KeyCtrlT}:
		return "<c-t>"
	case KeyComb{Key: KeyCtrlU}:
		return "<c-u>"
	case KeyComb{Key: KeyCtrlV}:
		return "<c-v>"
	case KeyComb{Key: KeyCtrlW}:
		return "<c-w>"
	case KeyComb{Key: KeyCtrlX}:
		return "<c-x>"
	case KeyComb{Key: KeyCtrlY}:
		return "<c-y>"
	case KeyComb{Key: KeyCtrlZ}:
		return "<c-z>"
	case KeyComb{Key: KeyEsc}:
		return "<esc>"
	case KeyComb{Key: KeyCtrlLsqBracket}:
		return "<c-[>"
	case KeyComb{Key: KeyCtrl3}:
		return "<c-3>"
	case KeyComb{Key: KeyCtrl4}:
		return "<c-4>"
	case KeyComb{Key: KeyCtrlBackslash}:
		return "<c-\\>"
	case KeyComb{Key: KeyCtrl5}:
		return "<c-5>"
	case KeyComb{Key: KeyCtrlRsqBracket}:
		return "<c-]>"
	case KeyComb{Key: KeyCtrl6}:
		return "<c-6>"
	case KeyComb{Key: KeyCtrl7}:
		return "<c-7>"
	case KeyComb{Key: KeyCtrlSlash}:
		return "<c-/>"
	case KeyComb{Key: KeyCtrlUnderscore}:
		return "<c-_>"
	case KeyComb{Key: KeySpace, Ch: ' '}, KeyComb{Key: KeySpace}:
		return "<space>"
	case KeyComb{Key: KeyBackspace2}:
		return "<backspace>"
	case KeyComb{Key: KeyCtrl8}:
		return "<c-8>"
	case KeyComb{Key: KeyF1, Mod: ModAlt}:
		return "<m-f1>"
	case KeyComb{Key: KeyF2, Mod: ModAlt}:
		return "<m-f2>"
	case KeyComb{Key: KeyF3, Mod: ModAlt}:
		return "<m-f3>"
	case KeyComb{Key: KeyF4, Mod: ModAlt}:
		return "<m-f4>"
	case KeyComb{Key: KeyF5, Mod: ModAlt}:
		return "<m-f5>"
	case KeyComb{Key: KeyF6, Mod: ModAlt}:
		return "<m-f6>"
	case KeyComb{Key: KeyF7, Mod: ModAlt}:
		return "<m-f7>"
	case KeyComb{Key: KeyF8, Mod: ModAlt}:
		return "<m-f8>"
	case KeyComb{Key: KeyF9, Mod: ModAlt}:
		return "<m-f9>"
	case KeyComb{Key: KeyF10, Mod: ModAlt}:
		return "<m-f10>"
	case KeyComb{Key: KeyF11, Mod: ModAlt}:
		return "<m-f11>"
	case KeyComb{Key: KeyF12, Mod: ModAlt}:
		return "<m-f12>"
	case KeyComb{Key: KeyInsert, Mod: ModAlt}:
		return "<m-insert>"
	case KeyComb{Key: KeyDelete, Mod: ModAlt}:
		return "<m-delete>"
	case KeyComb{Key: KeyHome, Mod: ModAlt}:
		return "<m-home>"
	case KeyComb{Key: KeyEnd, Mod: ModAlt}:
		return "<m-end>"
	case KeyComb{Key: KeyPgup, Mod: ModAlt}:
		return "<m-pgup>"
	case KeyComb{Key: KeyPgdn, Mod: ModAlt}:
		return "<m-pgdn>"
	case KeyComb{Key: KeyArrowUp, Mod: ModAlt}:
		return "<m-up>"
	case KeyComb{Key: KeyArrowDown, Mod: ModAlt}:
		return "<m-down>"
	case KeyComb{Key: KeyArrowLeft, Mod: ModAlt}:
		return "<m-left>"
	case KeyComb{Key: KeyArrowRight, Mod: ModAlt}:
		return "<m-right>"
	case KeyComb{Key: MouseLeft, Mod: ModAlt}:
		return "<m-mouse-left>"
	case KeyComb{Key: MouseMiddle, Mod: ModAlt}:
		return "<m-mouse-middle>"
	case KeyComb{Key: MouseRight, Mod: ModAlt}:
		return "<m-mouse-right>"
	case KeyComb{Key: MouseRelease, Mod: ModAlt}:
		return "<m-mouse-release>"
	case KeyComb{Key: MouseWheelUp, Mod: ModAlt}:
		return "<m-mouse-wheel-up>"
	case KeyComb{Key: MouseWheelDown, Mod: ModAlt}:
		return "<m-mouse-wheel-down>"
	case KeyComb{Key: KeyCtrlTilde, Mod: ModAlt}:
		return "<m-c-`>"
	case KeyComb{Key: KeyCtrl2, Mod: ModAlt}:
		return "<m-c-2>"
	case KeyComb{Key: KeyCtrlSpace, Mod: ModAlt}:
		return "<m-c-space>"
	case KeyComb{Key: KeyCtrlA, Mod: ModAlt}:
		return "<m-c-a>"
	case KeyComb{Key: KeyCtrlB, Mod: ModAlt}:
		return "<m-c-b>"
	case KeyComb{Key: KeyCtrlC, Mod: ModAlt}:
		return "<m-c-c>"
	case KeyComb{Key: KeyCtrlD, Mod: ModAlt}:
		return "<m-c-d>"
	case KeyComb{Key: KeyCtrlE, Mod: ModAlt}:
		return "<m-c-e>"
	case KeyComb{Key: KeyCtrlF, Mod: ModAlt}:
		return "<m-c-f>"
	case KeyComb{Key: KeyCtrlG, Mod: ModAlt}:
		return "<m-c-g>"
	case KeyComb{Key: KeyBackspace, Mod: ModAlt}:
		return "<m-c-backspace>"
	case KeyComb{Key: KeyCtrlH, Mod: ModAlt}:
		return "<m-c-h>"
	case KeyComb{Key: KeyTab, Mod: ModAlt}:
		return "<m-c-tab>"
	case KeyComb{Key: KeyCtrlI, Mod: ModAlt}:
		return "<m-c-i>"
	case KeyComb{Key: KeyCtrlJ, Mod: ModAlt}:
		return "<m-c-j>"
	case KeyComb{Key: KeyCtrlK, Mod: ModAlt}:
		return "<m-c-k>"
	case KeyComb{Key: KeyCtrlL, Mod: ModAlt}:
		return "<m-c-l>"
	case KeyComb{Key: KeyEnter, Mod: ModAlt}:
		return "<m-enter>"
	case KeyComb{Key: KeyCtrlM, Mod: ModAlt}:
		return "<m-c-m>"
	case KeyComb{Key: KeyCtrlN, Mod: ModAlt}:
		return "<m-c-n>"
	case KeyComb{Key: KeyCtrlO, Mod: ModAlt}:
		return "<m-c-o>"
	case KeyComb{Key: KeyCtrlP, Mod: ModAlt}:
		return "<m-c-p>"
	case KeyComb{Key: KeyCtrlQ, Mod: ModAlt}:
		return "<m-c-q>"
	case KeyComb{Key: KeyCtrlR, Mod: ModAlt}:
		return "<m-c-r>"
	case KeyComb{Key: KeyCtrlS, Mod: ModAlt}:
		return "<m-c-s>"
	case KeyComb{Key: KeyCtrlT, Mod: ModAlt}:
		return "<m-c-t>"
	case KeyComb{Key: KeyCtrlU, Mod: ModAlt}:
		return "<m-c-u>"
	case KeyComb{Key: KeyCtrlV, Mod: ModAlt}:
		return "<m-c-v>"
	case KeyComb{Key: KeyCtrlW, Mod: ModAlt}:
		return "<m-c-w>"
	case KeyComb{Key: KeyCtrlX, Mod: ModAlt}:
		return "<m-c-x>"
	case KeyComb{Key: KeyCtrlY, Mod: ModAlt}:
		return "<m-c-y>"
	case KeyComb{Key: KeyCtrlZ, Mod: ModAlt}:
		return "<m-c-z>"
	case KeyComb{Key: KeyEsc, Mod: ModAlt}:
		return "<m-esc>"
	case KeyComb{Key: KeyCtrlLsqBracket, Mod: ModAlt}:
		return "<m-c-[>"
	case KeyComb{Key: KeyCtrl3, Mod: ModAlt}:
		return "<m-c-3>"
	case KeyComb{Key: KeyCtrl4, Mod: ModAlt}:
		return "<m-c-4>"
	case KeyComb{Key: KeyCtrlBackslash, Mod: ModAlt}:
		return "<m-c-\\>"
	case KeyComb{Key: KeyCtrl5, Mod: ModAlt}:
		return "<m-c-5>"
	case KeyComb{Key: KeyCtrlRsqBracket, Mod: ModAlt}:
		return "<m-c-]>"
	case KeyComb{Key: KeyCtrl6, Mod: ModAlt}:
		return "<m-c-6>"
	case KeyComb{Key: KeyCtrl7, Mod: ModAlt}:
		return "<m-c-7>"
	case KeyComb{Key: KeyCtrlSlash, Mod: ModAlt}:
		return "<m-c-/>"
	case KeyComb{Key: KeyCtrlUnderscore, Mod: ModAlt}:
		return "<m-c-_>"
	case KeyComb{Key: KeySpace, Mod: ModAlt}, KeyComb{Ch: ' ', Key: KeySpace, Mod: ModAlt}:
		return "<m-space>"
	case KeyComb{Key: KeyBackspace2, Mod: ModAlt}:
		return "<m-backspace>"
	case KeyComb{Key: KeyCtrl8, Mod: ModAlt}:
		return "<m-c-8>"
	default:
		return "<INVALID>"
	}
}

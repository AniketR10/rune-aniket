package vte

import (
	"fmt"

	"unstable.build/go-tui/term"
)

func getModifierStr(ev term.Event) string {
	switch ev.Mod {
	case term.ModAlt:
		return ";3"
	default:
		return ""
	}
}

// return an escape sequence for what requires state terminal, otherwise
// the raw bytes coming from the event are already correct
func mapKeyToEscapeSequence(comp *Component, ev term.Event) ([]byte, bool) {
	switch ev.Key {
	case term.KeyArrowUp:
		if comp.IsApplicationCursorKeysMode() {
			return []byte{0x1b, 'O', 'A'}, true
		}
		return []byte(fmt.Sprintf("\x1b[%sA", getModifierStr(ev))), true
	case term.KeyArrowDown:
		if comp.IsApplicationCursorKeysMode() {
			return []byte{0x1b, 'O', 'B'}, true
		}
		return []byte(fmt.Sprintf("\x1b[%sB", getModifierStr(ev))), true
	case term.KeyArrowRight:
		if comp.IsApplicationCursorKeysMode() {
			return []byte{0x1b, 'O', 'C'}, true
		}
		return []byte(fmt.Sprintf("\x1b[%sC", getModifierStr(ev))), true
	case term.KeyArrowLeft:
		if comp.IsApplicationCursorKeysMode() {
			return []byte{0x1b, 'O', 'D'}, true
		}
		return []byte(fmt.Sprintf("\x1b[%sD", getModifierStr(ev))), true
	case term.KeyEnter:
		if comp.IsNewLineMode() {
			return []byte{0x0d, 0x0a}, true
		}
		return []byte{0x0d}, true
	case term.KeyHome:
		if comp.IsApplicationCursorKeysMode() {
			return []byte(fmt.Sprintf("\x1b[1%s~", getModifierStr(ev))), true
		}
		return []byte("\x1b[H"), true
	default:
		return nil, false
	}
}

// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package byoe

import (
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// keyCombToEvent renders a term.KeyComb into the term.Event that
// vte.Handler.Handle expects when byoe forwards a synthetic key into
// the embedded pty. Critically, it populates ev.Raw: vte's
// handleInput falls back to ev.Raw for any key it does not specially
// translate (plain ASCII runes, ESC, Enter, F-keys, arrows, etc.), so
// a KeyComb-only event would silently produce a zero-byte pty write
// and the external editor would never see the keystroke.
//
// The byte sequences must match what a real vt100/xterm-like terminal
// would emit so that whichever TUI editor the user picks (vim,
// neovim, helix, kakoune, emacs -nw, …) interprets them identically
// to a hardware keypress. The implementation mirrors the production
// GUI input table in term/gui/input.go so that byoe-synthesised
// events round-trip through vte and into the external editor exactly
// like real keyboard events.
func keyCombToEvent(k term.KeyComb) term.Event {
	ev := term.Event{
		Type: term.EventKey,
		Mod:  k.Mod,
		Key:  k.Key,
		Ch:   k.Ch,
	}
	ev.Raw = rawForKeyComb(k)
	return ev
}

// rawForKeyComb returns the byte sequence equivalent of k. Returns
// nil for unsupported combinations (the embedded vte will then ignore
// the event, which matches the production behaviour for modifier
// combos a real terminal would not emit).
func rawForKeyComb(k term.KeyComb) []byte {
	if seq, ok := rawForSpecialKey(k); ok {
		return seq
	}
	if k.Ch != 0 {
		return rawForRune(k.Ch, k.Mod)
	}
	return nil
}

// rawForSpecialKey handles every term.Key value vte recognises.
// Returns ok == true for any key we know how to render (including
// keys whose rendering is mod-gated and returns nil for unsupported
// mod combos, matching term/gui/input.go semantics).
func rawForSpecialKey(k term.KeyComb) ([]byte, bool) {
	switch k.Key {
	case term.KeyEsc:
		if k.Mod == 0 {
			return []byte{0x1b}, true
		}
		return nil, true
	case term.KeyEnter:
		switch k.Mod {
		case 0:
			return []byte{0x0d, 0x0a}, true
		case term.ModShift:
			// kitty protocol shift+enter.
			return []byte{0x1b, '[', '1', '3', ';', '2', 'u'}, true
		}
		return nil, true
	case term.KeySpace:
		switch k.Mod {
		case 0, term.ModShift:
			return []byte{' '}, true
		default:
			return []byte{' '}, true
		}
	case term.KeyTab:
		switch k.Mod {
		case 0:
			return []byte{0x09}, true
		case term.ModShift:
			return []byte{0x1b, '[', 'Z'}, true
		}
		return nil, true
	case term.KeyBackspace:
		switch k.Mod {
		case 0:
			return []byte{0x7f}, true
		case term.ModAlt:
			return []byte{0x1b, 0x7f}, true
		}
		return nil, true
	case term.KeyArrowUp:
		return fmt.Appendf(nil, "\x1b[%sA", modifierStr1(k.Mod)), true
	case term.KeyArrowDown:
		return fmt.Appendf(nil, "\x1b[%sB", modifierStr1(k.Mod)), true
	case term.KeyArrowRight:
		return fmt.Appendf(nil, "\x1b[%sC", modifierStr1(k.Mod)), true
	case term.KeyArrowLeft:
		return fmt.Appendf(nil, "\x1b[%sD", modifierStr1(k.Mod)), true
	case term.KeyHome:
		return fmt.Appendf(nil, "\x1b[%sH", modifierStr1(k.Mod)), true
	case term.KeyEnd:
		return fmt.Appendf(nil, "\x1b[%sF", modifierStr1(k.Mod)), true
	case term.KeyPgup:
		return fmt.Appendf(nil, "\x1b[5%s~", modifierStr2(k.Mod)), true
	case term.KeyPgdn:
		return fmt.Appendf(nil, "\x1b[6%s~", modifierStr2(k.Mod)), true
	case term.KeyInsert:
		return fmt.Appendf(nil, "\x1b[2%s~", modifierStr2(k.Mod)), true
	case term.KeyDelete:
		return fmt.Appendf(nil, "\x1b[3%s~", modifierStr2(k.Mod)), true
	case term.KeyF1:
		if k.Mod == 0 {
			return []byte("\x1bOP"), true
		}
		return fmt.Appendf(nil, "\x1b[%sP", modifierStr1(k.Mod)), true
	case term.KeyF2:
		if k.Mod == 0 {
			return []byte("\x1bOQ"), true
		}
		return fmt.Appendf(nil, "\x1b[%sQ", modifierStr1(k.Mod)), true
	case term.KeyF3:
		if k.Mod == 0 {
			return []byte("\x1bOR"), true
		}
		return fmt.Appendf(nil, "\x1b[%sR", modifierStr1(k.Mod)), true
	case term.KeyF4:
		if k.Mod == 0 {
			return []byte("\x1bOS"), true
		}
		return fmt.Appendf(nil, "\x1b[%sS", modifierStr1(k.Mod)), true
	case term.KeyF5:
		return fmt.Appendf(nil, "\x1b[15%s~", modifierStr2(k.Mod)), true
	case term.KeyF6:
		return fmt.Appendf(nil, "\x1b[17%s~", modifierStr2(k.Mod)), true
	case term.KeyF7:
		return fmt.Appendf(nil, "\x1b[18%s~", modifierStr2(k.Mod)), true
	case term.KeyF8:
		return fmt.Appendf(nil, "\x1b[19%s~", modifierStr2(k.Mod)), true
	case term.KeyF9:
		return fmt.Appendf(nil, "\x1b[20%s~", modifierStr2(k.Mod)), true
	case term.KeyF10:
		return fmt.Appendf(nil, "\x1b[21%s~", modifierStr2(k.Mod)), true
	case term.KeyF11:
		return fmt.Appendf(nil, "\x1b[23%s~", modifierStr2(k.Mod)), true
	case term.KeyF12:
		return fmt.Appendf(nil, "\x1b[24%s~", modifierStr2(k.Mod)), true
	}
	return nil, false
}

// rawForRune renders a printable rune under an optional modifier into
// the same bytes a real vt100-like terminal would emit. Mirrors
// term/gui/input.go's getCharEscapeSequence so byoe-synthesised input
// is indistinguishable from a real keypress.
func rawForRune(ch rune, mod term.Modifier) []byte {
	switch mod {
	case 0:
		return []byte(string(ch))
	case term.ModShift:
		// Shift on a printable rune does not change the byte we
		// emit; the rune itself already carries the shifted glyph
		// (e.g. 'A' vs 'a'). vt100 has no Shift-prefix.
		return []byte(string(ch))
	case term.ModAlt:
		return append([]byte{0x1b}, []byte(string(ch))...)
	case term.ModCtrl:
		if seq, ok := ctrlRune(ch); ok {
			return seq
		}
		return nil
	case term.ModCtrlAlt:
		if seq, ok := ctrlRune(ch); ok {
			return append([]byte{0x1b}, seq...)
		}
		return nil
	}
	return nil
}

// ctrlRune returns the C0 byte that vt100-like terminals emit for
// Ctrl+<ch>. Letters fold to 0x01..0x1A; a handful of punctuation
// keys map to additional C0 codes (Ctrl+[, Ctrl+\, Ctrl+], Ctrl+_,
// Ctrl+`). Returns (nil, false) for combinations a terminal would
// not emit.
func ctrlRune(ch rune) ([]byte, bool) {
	switch {
	case ch >= 'a' && ch <= 'z':
		return []byte{byte(ch - 'a' + 1)}, true
	case ch >= 'A' && ch <= 'Z':
		return []byte{byte(ch - 'A' + 1)}, true
	}
	switch ch {
	case '[':
		return []byte{0x1b}, true
	case '\\':
		return []byte{0x1c}, true
	case ']':
		return []byte{0x1d}, true
	case '_':
		return []byte{0x1f}, true
	case '`':
		return []byte{0x60}, true
	}
	return nil, false
}

// modifierStr1 returns the xterm parameter prefix used inside CSI
// escape sequences that already carry a leading parameter (arrows,
// Home, End, F1..F4 modified form). Mirrors term/gui/input.go's
// getModifierStr.
func modifierStr1(mod term.Modifier) string {
	switch mod {
	case 0:
		return ""
	case term.ModShift:
		return "1;2"
	case term.ModAlt:
		return "1;3"
	case term.ModAltShift:
		return "1;4"
	case term.ModCtrl:
		return "1;5"
	case term.ModCtrlShift:
		return "1;6"
	case term.ModCtrlAlt:
		return "1;7"
	case term.ModCtrlShiftAlt:
		return "1;8"
	case term.ModMeta:
		return "1;9"
	case term.ModShiftMeta:
		return "1;10"
	case term.ModAltMeta:
		return "1;11"
	case term.ModAltShiftMeta:
		return "1;12"
	case term.ModCtrlMeta:
		return "1;13"
	case term.ModCtrlShiftMeta:
		return "1;14"
	case term.ModCtrlAltMeta:
		return "1;15"
	}
	return ""
}

// modifierStr2 returns the xterm parameter suffix used inside CSI
// escape sequences whose first parameter is the key code itself
// (PgUp/PgDn/Insert/Delete/F5..F12). Mirrors term/gui/input.go's
// getModifierStr2.
func modifierStr2(mod term.Modifier) string {
	switch mod {
	case 0:
		return ""
	case term.ModShift:
		return ";2"
	case term.ModAlt:
		return ";3"
	case term.ModAltShift:
		return ";4"
	case term.ModCtrl:
		return ";5"
	case term.ModCtrlShift:
		return ";6"
	case term.ModCtrlAlt:
		return ";7"
	case term.ModCtrlShiftAlt:
		return ";8"
	case term.ModMeta:
		return ";9"
	case term.ModShiftMeta:
		return ";10"
	case term.ModAltMeta:
		return ";11"
	case term.ModAltShiftMeta:
		return ";12"
	case term.ModCtrlMeta:
		return ";13"
	case term.ModCtrlShiftMeta:
		return ";14"
	case term.ModCtrlAltMeta:
		return ";15"
	}
	return ""
}

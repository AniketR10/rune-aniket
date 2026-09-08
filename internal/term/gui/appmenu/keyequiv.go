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

package appmenu

import (
	"unicode"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// NSEventModifierFlags bits accepted by NSMenuItem.keyEquivalentModifierMask.
const (
	modShiftMask   = 1 << 17
	modControlMask = 1 << 18
	modOptionMask  = 1 << 19
	modCommandMask = 1 << 20
)

// specialKeyEquivalents maps non-character keys to the character AppKit
// expects as a key equivalent. Function and navigation keys use the
// private-use codepoints declared in NSText.h (NSUpArrowFunctionKey and
// friends).
var specialKeyEquivalents = map[term.Key]string{
	term.KeyF1:         "\uF704",
	term.KeyF2:         "\uF705",
	term.KeyF3:         "\uF706",
	term.KeyF4:         "\uF707",
	term.KeyF5:         "\uF708",
	term.KeyF6:         "\uF709",
	term.KeyF7:         "\uF70A",
	term.KeyF8:         "\uF70B",
	term.KeyF9:         "\uF70C",
	term.KeyF10:        "\uF70D",
	term.KeyF11:        "\uF70E",
	term.KeyF12:        "\uF70F",
	term.KeyInsert:     "\uF727",
	term.KeyDelete:     "\uF728",
	term.KeyHome:       "\uF729",
	term.KeyEnd:        "\uF72B",
	term.KeyPgup:       "\uF72C",
	term.KeyPgdn:       "\uF72D",
	term.KeyArrowUp:    "\uF700",
	term.KeyArrowDown:  "\uF701",
	term.KeyArrowLeft:  "\uF702",
	term.KeyArrowRight: "\uF703",
	term.KeyBackspace:  "\u0008",
	term.KeyTab:        "\u0009",
	term.KeyEnter:      "\u000D",
	term.KeyEsc:        "\u001B",
	term.KeySpace:      " ",
}

// keyEquivalent translates k into the (keyEquivalent, modifierMask) pair
// AppKit uses to render and match a menu accelerator. ok is false when k
// has no representable equivalent, in which case the item is rendered
// without an accelerator.
func keyEquivalent(k term.KeyComb) (equiv string, mods uint, ok bool) {
	mods = modifierMask(k.Mod)

	// KeySpace is the one key that carries both a Key and a Ch.
	if k.Ch != 0 && k.Key != term.KeySpace {
		ch := k.Ch
		// AppKit expects the unshifted character plus an explicit
		// shift bit, matching how term normalizes shifted runes.
		if unicode.IsUpper(ch) {
			ch = unicode.ToLower(ch)
			mods |= modShiftMask
		}
		return string(ch), mods, true
	}

	equiv, ok = specialKeyEquivalents[k.Key]
	if !ok {
		return "", 0, false
	}
	return equiv, mods, true
}

func modifierMask(m term.Modifier) uint {
	var mask uint
	if m&term.ModShift != 0 {
		mask |= modShiftMask
	}
	if m&term.ModCtrl != 0 {
		mask |= modControlMask
	}
	if m&term.ModAlt != 0 {
		mask |= modOptionMask
	}
	if m&term.ModMeta != 0 {
		mask |= modCommandMask
	}
	return mask
}

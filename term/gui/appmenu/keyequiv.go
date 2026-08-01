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

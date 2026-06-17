// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package gui

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/term/gui/font"
)

// abstracts ebiten* input methods
type keysManager interface {
	AppendInputChars([]rune) []rune
	AppendKeyEvents([]ebiten.KeyEvent) []ebiten.KeyEvent
}

type input struct {
	fontManager *font.Manager
	keyEvents   []ebiten.KeyEvent
	chars       []rune
	input       keysManager
	keyMapping  map[ebiten.KeyEvent]ebiten.KeyEvent
	modMapping  map[ebiten.KeyModifier]ebiten.KeyModifier
}

func newInput(fontManager *font.Manager) *input {
	return &input{
		fontManager: fontManager,
		input:       ebitenInputManager{},
	}
}

func (i *input) setKeyMapping(m map[term.KeyComb]term.KeyComb) {
	if len(m) == 0 {
		i.keyMapping = nil
		i.modMapping = nil
		return
	}
	resolved := make(map[ebiten.KeyEvent]ebiten.KeyEvent, len(m))
	mods := make(map[ebiten.KeyModifier]ebiten.KeyModifier)
	for from, to := range m {
		src, ok := combToEvent[from]
		if !ok {
			continue
		}
		dst, ok := combToEvent[to]
		if !ok {
			continue
		}
		resolved[src] = dst
		if srcBit, ok := bareModBit(src); ok {
			if dstBit, ok := bareModBit(dst); ok {
				mods[srcBit] = dstBit
			}
		}
	}
	if len(resolved) == 0 {
		i.keyMapping = nil
		i.modMapping = nil
		return
	}
	i.keyMapping = resolved
	if len(mods) == 0 {
		i.modMapping = nil
	} else {
		i.modMapping = mods
	}
}

// bareModBit reports the modifier bit a pure-modifier key event represents.
func bareModBit(ev ebiten.KeyEvent) (ebiten.KeyModifier, bool) {
	if ev.Mods != 0 {
		return 0, false
	}
	switch ev.Key {
	case ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight:
		return ebiten.KeyModControl, true
	case ebiten.KeyShift, ebiten.KeyShiftLeft, ebiten.KeyShiftRight:
		return ebiten.KeyModShift, true
	case ebiten.KeyAlt, ebiten.KeyAltLeft, ebiten.KeyAltRight:
		return ebiten.KeyModAlt, true
	case ebiten.KeyMeta, ebiten.KeyMetaLeft, ebiten.KeyMetaRight:
		return ebiten.KeyModSuper, true
	}
	return 0, false
}

// remapMods rewrites each modifier bit through the configured bare
// modifier remap (e.g. ctrl->meta) so held modifiers on other keys
// follow the same mapping as a standalone modifier press.
func (i *input) remapMods(mods ebiten.KeyModifier) ebiten.KeyModifier {
	if len(i.modMapping) == 0 || mods == 0 {
		return mods
	}
	var out ebiten.KeyModifier
	for _, bit := range []ebiten.KeyModifier{
		ebiten.KeyModControl, ebiten.KeyModShift,
		ebiten.KeyModAlt, ebiten.KeyModSuper,
	} {
		if mods&bit == 0 {
			continue
		}
		if to, ok := i.modMapping[bit]; ok {
			out |= to
		} else {
			out |= bit
		}
	}
	return out
}

// processEvents collects discrete key events and fallback chars from Ebiten,
// converts them to term.Events, and appends them to dst.
func (i *input) processEvents(dst []term.Event) []term.Event {
	i.keyEvents = i.input.AppendKeyEvents(i.keyEvents[:0])
	i.chars = i.input.AppendInputChars(i.chars[:0])

	// Track whether any key event produced a terminal event, so we
	// know whether to use the fallback chars from AppendInputChars.
	keyEventFired := false

	for _, ke := range i.keyEvents {
		if ke.Action == ebiten.KeyActionRelease {
			continue
		}
		lookupMods := ke.Mods
		switch ke.Key {
		case ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight:
			lookupMods &^= ebiten.KeyModControl
		case ebiten.KeyShift, ebiten.KeyShiftLeft, ebiten.KeyShiftRight:
			lookupMods &^= ebiten.KeyModShift
		case ebiten.KeyAlt, ebiten.KeyAltLeft, ebiten.KeyAltRight:
			lookupMods &^= ebiten.KeyModAlt
		case ebiten.KeyMeta, ebiten.KeyMetaLeft, ebiten.KeyMetaRight:
			lookupMods &^= ebiten.KeyModSuper
		}
		rep, ok := i.keyMapping[ebiten.KeyEvent{Key: ke.Key, Mods: lookupMods}]
		if ok {
			ke.Key, ke.Mods = rep.Key, rep.Mods
		}
		ke.Mods = i.remapMods(ke.Mods)
		if isModifierKey(ke.Key) {
			continue
		}

		mod := ebitenModToTermMod(ke.Mods)

		// Try character-producing key first.
		if base, shift, ok := keyToBaseAndShift(ke.Key); ok {
			keyEventFired = true
			ch, mod := resolveCharKey(base, shift, mod)
			dst = append(dst, term.Event{
				Type: term.EventKey,
				Mod:  mod,
				Ch:   ch,
				Raw:  getCharEscapeSequence(ch, mod),
			})
			continue
		}

		// Non-character key (arrows, F-keys, Enter, etc.)
		if ev, ok := mapEbitenKey(ke.Key, mod); ok {
			keyEventFired = true
			dst = append(dst, ev)
		}
	}

	// Fallback: if no key events produced terminal events this frame,
	// use AppendInputChars for IME / paste / other text input.
	//
	// Limitation: fallback chars carry no modifier information. If a
	// modifier (e.g. Alt) is held while a char arrives here without a
	// corresponding key event, the modifier is lost. In practice this
	// does not happen on desktop/GLFW because the key callback always
	// fires for physical key presses, so modifier+char combos are
	// handled by the key-event path above.
	if !keyEventFired {
		for _, ch := range i.chars {
			dst = append(dst, term.Event{
				Type: term.EventKey,
				Ch:   ch,
				Raw:  getCharEscapeSequence(ch, 0),
			})
		}
	}

	return dst
}

// ebitenModToTermMod converts an Ebiten modifier bitmask to a term.Modifier.
func ebitenModToTermMod(mods ebiten.KeyModifier) term.Modifier {
	var m term.Modifier
	if mods&ebiten.KeyModControl != 0 {
		m |= term.ModCtrl
	}
	if mods&ebiten.KeyModShift != 0 {
		m |= term.ModShift
	}
	if mods&ebiten.KeyModAlt != 0 {
		m |= term.ModAlt
	}
	if mods&ebiten.KeyModSuper != 0 {
		m |= term.ModMeta
	}
	return m
}

// isModifierKey returns true for keys that are pure modifiers.
func isModifierKey(key ebiten.Key) bool {
	switch key {
	case ebiten.KeyMeta, ebiten.KeyMetaLeft, ebiten.KeyMetaRight,
		ebiten.KeyAlt, ebiten.KeyAltLeft, ebiten.KeyAltRight,
		ebiten.KeyShift, ebiten.KeyShiftLeft, ebiten.KeyShiftRight,
		ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight:
		return true
	}
	return false
}

// keyToBaseAndShift returns the base and shift characters for a
// character-producing key. Returns false for non-character keys.
func keyToBaseAndShift(key ebiten.Key) (base, shift rune, ok bool) {
	switch key {
	case ebiten.KeyA:
		return 'a', 'A', true
	case ebiten.KeyB:
		return 'b', 'B', true
	case ebiten.KeyC:
		return 'c', 'C', true
	case ebiten.KeyD:
		return 'd', 'D', true
	case ebiten.KeyE:
		return 'e', 'E', true
	case ebiten.KeyF:
		return 'f', 'F', true
	case ebiten.KeyG:
		return 'g', 'G', true
	case ebiten.KeyH:
		return 'h', 'H', true
	case ebiten.KeyI:
		return 'i', 'I', true
	case ebiten.KeyJ:
		return 'j', 'J', true
	case ebiten.KeyK:
		return 'k', 'K', true
	case ebiten.KeyL:
		return 'l', 'L', true
	case ebiten.KeyM:
		return 'm', 'M', true
	case ebiten.KeyN:
		return 'n', 'N', true
	case ebiten.KeyO:
		return 'o', 'O', true
	case ebiten.KeyP:
		return 'p', 'P', true
	case ebiten.KeyQ:
		return 'q', 'Q', true
	case ebiten.KeyR:
		return 'r', 'R', true
	case ebiten.KeyS:
		return 's', 'S', true
	case ebiten.KeyT:
		return 't', 'T', true
	case ebiten.KeyU:
		return 'u', 'U', true
	case ebiten.KeyV:
		return 'v', 'V', true
	case ebiten.KeyW:
		return 'w', 'W', true
	case ebiten.KeyX:
		return 'x', 'X', true
	case ebiten.KeyY:
		return 'y', 'Y', true
	case ebiten.KeyZ:
		return 'z', 'Z', true
	case ebiten.KeyDigit0:
		return '0', ')', true
	case ebiten.KeyDigit1:
		return '1', '!', true
	case ebiten.KeyDigit2:
		return '2', '@', true
	case ebiten.KeyDigit3:
		return '3', '#', true
	case ebiten.KeyDigit4:
		return '4', '$', true
	case ebiten.KeyDigit5:
		return '5', '%', true
	case ebiten.KeyDigit6:
		return '6', '^', true
	case ebiten.KeyDigit7:
		return '7', '&', true
	case ebiten.KeyDigit8:
		return '8', '*', true
	case ebiten.KeyDigit9:
		return '9', '(', true
	case ebiten.KeyMinus:
		return '-', '_', true
	case ebiten.KeyEqual:
		return '=', '+', true
	case ebiten.KeyComma:
		return ',', '<', true
	case ebiten.KeyPeriod:
		return '.', '>', true
	case ebiten.KeySlash:
		return '/', '?', true
	case ebiten.KeyBackslash:
		return '\\', '|', true
	case ebiten.KeyIntlBackslash:
		return '\\', '|', true
	case ebiten.KeyBracketLeft:
		return '[', '{', true
	case ebiten.KeyBracketRight:
		return ']', '}', true
	case ebiten.KeyBackquote:
		return '`', '~', true
	case ebiten.KeyQuote:
		return '\'', '"', true
	case ebiten.KeySemicolon:
		return ';', ':', true
	case ebiten.KeyNumpadAdd:
		return '+', '+', true
	case ebiten.KeyNumpadDecimal:
		return '.', '.', true
	case ebiten.KeyNumpadDivide:
		return '/', '/', true
	case ebiten.KeyNumpadEqual:
		return '=', '=', true
	case ebiten.KeyNumpadMultiply:
		return '*', '*', true
	case ebiten.KeyNumpadSubtract:
		return '-', '-', true
	case ebiten.KeyNumpad0:
		return '0', '0', true
	case ebiten.KeyNumpad1:
		return '1', '1', true
	case ebiten.KeyNumpad2:
		return '2', '2', true
	case ebiten.KeyNumpad3:
		return '3', '3', true
	case ebiten.KeyNumpad4:
		return '4', '4', true
	case ebiten.KeyNumpad5:
		return '5', '5', true
	case ebiten.KeyNumpad6:
		return '6', '6', true
	case ebiten.KeyNumpad7:
		return '7', '7', true
	case ebiten.KeyNumpad8:
		return '8', '8', true
	case ebiten.KeyNumpad9:
		return '9', '9', true
	default:
		return 0, 0, false
	}
}

// resolveCharKey picks the correct character (base vs shift) and
// strips the Shift modifier from mod when Shift is consumed by
// the character mapping.
func resolveCharKey(base, shift rune, mod term.Modifier) (rune, term.Modifier) {
	switch mod {
	case term.ModShift:
		return shift, 0
	case term.ModCtrlShift:
		return shift, term.ModCtrl
	case term.ModCtrlShiftAlt:
		return shift, term.ModCtrlAlt
	case term.ModCtrlShiftMeta:
		return shift, term.ModCtrlMeta
	case term.ModShiftMeta:
		return shift, term.ModMeta
	case term.ModAltShiftMeta:
		return shift, term.ModAltMeta
	case term.ModAltShift:
		return shift, term.ModAlt
	default:
		return base, mod
	}
}

func mapEbitenKey(key ebiten.Key, mod term.Modifier) (ev term.Event, ok bool) {
	ev, ok = doMapEbitenKey(key, mod)
	if ok {
		ev.Type = term.EventKey
	}
	return
}

func doMapEbitenKey(key ebiten.Key, mod term.Modifier) (ev term.Event, ok bool) {
	switch key {
	case ebiten.KeyA, ebiten.KeyB, ebiten.KeyC, ebiten.KeyD, ebiten.KeyE,
		ebiten.KeyF, ebiten.KeyG, ebiten.KeyH, ebiten.KeyI, ebiten.KeyJ,
		ebiten.KeyK, ebiten.KeyL, ebiten.KeyM, ebiten.KeyN, ebiten.KeyO,
		ebiten.KeyP, ebiten.KeyQ, ebiten.KeyR, ebiten.KeyS, ebiten.KeyT,
		ebiten.KeyU, ebiten.KeyV, ebiten.KeyW, ebiten.KeyX, ebiten.KeyY, ebiten.KeyZ,
		ebiten.KeyMinus, ebiten.KeyNumpad0, ebiten.KeyNumpad1, ebiten.KeyNumpad2,
		ebiten.KeyNumpad3, ebiten.KeyNumpad4, ebiten.KeyNumpad5, ebiten.KeyNumpad6,
		ebiten.KeyNumpad7, ebiten.KeyNumpad8, ebiten.KeyNumpad9, ebiten.KeyNumpadAdd,
		ebiten.KeyNumpadDecimal, ebiten.KeyNumpadDivide, ebiten.KeyNumpadEnter, ebiten.KeyNumpadEqual,
		ebiten.KeyNumpadMultiply, ebiten.KeyNumpadSubtract, ebiten.KeyEqual, ebiten.KeyDigit0,
		ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4,
		ebiten.KeyDigit5, ebiten.KeyDigit6, ebiten.KeyDigit7, ebiten.KeyDigit8,
		ebiten.KeyDigit9, ebiten.KeyComma, ebiten.KeyBracketLeft, ebiten.KeyBracketRight,
		ebiten.KeyBackquote, ebiten.KeyBackslash, ebiten.KeySlash, ebiten.KeyIntlBackslash,
		ebiten.KeyPeriod, ebiten.KeyQuote, ebiten.KeySemicolon:
		// handled via AppendInputChars, which takes into consideration shift, etc.
		return
	case ebiten.KeyAltLeft, ebiten.KeyAltRight, ebiten.KeyAlt,
		ebiten.KeyMetaLeft, ebiten.KeyMetaRight, ebiten.KeyMeta,
		ebiten.KeyControlLeft, ebiten.KeyControlRight, ebiten.KeyControl,
		ebiten.KeyShiftLeft, ebiten.KeyShiftRight, ebiten.KeyShift:
		// specially handled to combine different modifiers
		return
	case ebiten.KeyCapsLock, ebiten.KeyContextMenu, ebiten.KeyF13, ebiten.KeyF14, ebiten.KeyF15,
		ebiten.KeyF16, ebiten.KeyF17, ebiten.KeyF18, ebiten.KeyF19,
		ebiten.KeyF20, ebiten.KeyF21, ebiten.KeyF22, ebiten.KeyF23,
		ebiten.KeyF24, ebiten.KeyNumLock, ebiten.KeyPause, ebiten.KeyPrintScreen,
		ebiten.KeyScrollLock:
		// unhandled
		return
	case ebiten.KeySpace:
		switch mod {
		case 0, term.ModShift:
			return term.Event{
				Key: term.KeySpace,
				Raw: []byte{' '},
			}, true
		default:
			return term.Event{
				Key: term.KeySpace,
				Raw: []byte{' '},
				Mod: mod,
			}, true
		}
	case ebiten.KeyArrowDown:
		return term.Event{
			Mod: mod,
			Key: term.KeyArrowDown,
			Raw: fmt.Appendf(nil, "\x1b[%sB", getModifierStr(mod)),
		}, true
	case ebiten.KeyArrowLeft:
		return term.Event{
			Mod: mod,
			Key: term.KeyArrowLeft,
			Raw: fmt.Appendf(nil, "\x1b[%sD", getModifierStr(mod)),
		}, true
	case ebiten.KeyArrowRight:
		return term.Event{
			Mod: mod,
			Key: term.KeyArrowRight,
			Raw: fmt.Appendf(nil, "\x1b[%sC", getModifierStr(mod)),
		}, true
	case ebiten.KeyArrowUp:
		return term.Event{
			Mod: mod,
			Key: term.KeyArrowUp,
			Raw: fmt.Appendf(nil, "\x1b[%sA", getModifierStr(mod)),
		}, true
	case ebiten.KeyBackspace:
		var raw []byte
		switch mod {
		case term.ModAlt:
			raw = []byte{0x1b, 0x7f}
		case 0:
			// the rest of modifiers do nothing
			raw = []byte{0x7f}
		}
		return term.Event{
			Mod: mod,
			Key: term.KeyBackspace,
			Raw: raw,
		}, true
	case ebiten.KeyDelete:
		return term.Event{
			Mod: mod,
			Key: term.KeyDelete,
			Raw: fmt.Appendf(nil, "\x1b[3%s~", getModifierStr2(mod)),
		}, true
	case ebiten.KeyEnd:
		return term.Event{
			Mod: mod,
			Key: term.KeyEnd,
			Raw: fmt.Appendf(nil, "\x1b[%sF", getModifierStr(mod)),
		}, true
	case ebiten.KeyHome:
		return term.Event{
			Mod: mod,
			Key: term.KeyHome,
			Raw: fmt.Appendf(nil, "\x1b[%sH", getModifierStr(mod)),
		}, true
	case ebiten.KeyInsert:
		return term.Event{
			Mod: mod,
			Key: term.KeyInsert,
			Raw: fmt.Appendf(nil, "\x1b[2%s~", getModifierStr2(mod)),
		}, true
	case ebiten.KeyPageDown:
		return term.Event{
			Mod: mod,
			Key: term.KeyPgdn,
			Raw: fmt.Appendf(nil, "\x1b[6%s~", getModifierStr2(mod)),
		}, true
	case ebiten.KeyPageUp:
		return term.Event{
			Mod: mod,
			Key: term.KeyPgup,
			Raw: fmt.Appendf(nil, "\x1b[5%s~", getModifierStr2(mod)),
		}, true
	case ebiten.KeyEnter:
		var raw []byte
		switch mod {
		case term.ModShift:
			// kitty protocol shift+enter
			raw = []byte{0x1b, 0x5b, 0x31, 0x33, 0x3b, 0x32, 0x75}
		case 0:
			raw = []byte{0x0d, 0x0a}
		}
		return term.Event{
			Mod: mod,
			Key: term.KeyEnter,
			Raw: raw,
		}, true
	case ebiten.KeyEscape:
		var raw []byte
		if mod == 0 {
			raw = []byte{0x1b}
		}
		return term.Event{
			Mod: mod,
			Key: term.KeyEsc,
			Raw: raw,
		}, true
	case ebiten.KeyF1:
		if mod != 0 {
			return term.Event{
				Mod: mod,
				Key: term.KeyF1,
				Raw: fmt.Appendf(nil, "\x1b[%sP", getModifierStr(mod)),
			}, true
		}
		return term.Event{
			Mod: mod,
			Key: term.KeyF1,
			Raw: []byte("\x1bOP"),
		}, true
	case ebiten.KeyF2:
		if mod != 0 {
			return term.Event{
				Mod: mod,
				Key: term.KeyF2,
				Raw: fmt.Appendf(nil, "\x1b[%sQ", getModifierStr(mod)),
			}, true
		}
		return term.Event{
			Mod: mod,
			Key: term.KeyF2,
			Raw: []byte("\x1bOQ"),
		}, true
	case ebiten.KeyF3:
		if mod != 0 {
			return term.Event{
				Mod: mod,
				Key: term.KeyF3,
				Raw: fmt.Appendf(nil, "\x1b[%sR", getModifierStr(mod)),
			}, true
		}
		return term.Event{
			Mod: mod,
			Key: term.KeyF3,
			Raw: []byte("\x1bOR"),
		}, true
	case ebiten.KeyF4:
		if mod != 0 {
			return term.Event{
				Mod: mod,
				Key: term.KeyF4,
				Raw: fmt.Appendf(nil, "\x1b[%sS", getModifierStr(mod)),
			}, true
		}
		return term.Event{
			Mod: mod,
			Key: term.KeyF4,
			Raw: []byte("\x1bOS"),
		}, true
	case ebiten.KeyF5:
		return term.Event{
			Mod: mod,
			Key: term.KeyF5,
			Raw: fmt.Appendf(nil, "\x1b[15%s~", getModifierStr2(mod)),
		}, true
	case ebiten.KeyF6:
		return term.Event{
			Mod: mod,
			Key: term.KeyF6,
			Raw: fmt.Appendf(nil, "\x1b[17%s~", getModifierStr2(mod)),
		}, true
	case ebiten.KeyF7:
		return term.Event{
			Mod: mod,
			Key: term.KeyF7,
			Raw: fmt.Appendf(nil, "\x1b[18%s~", getModifierStr2(mod)),
		}, true
	case ebiten.KeyF8:
		return term.Event{
			Mod: mod,
			Key: term.KeyF8,
			Raw: fmt.Appendf(nil, "\x1b[19%s~", getModifierStr2(mod)),
		}, true
	case ebiten.KeyF9:
		return term.Event{
			Mod: mod,
			Key: term.KeyF9,
			Raw: fmt.Appendf(nil, "\x1b[20%s~", getModifierStr2(mod)),
		}, true
	case ebiten.KeyF10:
		return term.Event{
			Mod: mod,
			Key: term.KeyF10,
			Raw: fmt.Appendf(nil, "\x1b[21%s~", getModifierStr2(mod)),
		}, true
	case ebiten.KeyF11:
		return term.Event{
			Mod: mod,
			Key: term.KeyF11,
			Raw: fmt.Appendf(nil, "\x1b[23%s~", getModifierStr2(mod)),
		}, true
	case ebiten.KeyF12:
		return term.Event{
			Mod: mod,
			Key: term.KeyF12,
			Raw: fmt.Appendf(nil, "\x1b[24%s~", getModifierStr2(mod)),
		}, true
	case ebiten.KeyTab:
		var raw []byte
		switch mod {
		case term.ModShift:
			raw = []byte{0x1b, 0x5b, 0x5a}
		case 0:
			raw = []byte{0x09}
		}
		return term.Event{
			Mod: mod,
			Key: term.KeyTab,
			Raw: raw,
		}, true
	default:
		return
	}
}

// to get this modifiers, run `showkeys -a` on linux
// and try combining modifiers with the F2 key
func getModifierStr(mod term.Modifier) string {
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
	default:
		panic(fmt.Sprintf("unknown modifier: %d", mod))
	}
}

func getModifierStr2(mod term.Modifier) string {
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
	default:
		panic(fmt.Sprintf("unknown modifier: %d", mod))
	}
}

func getCharEscapeSequence(ch rune, mod term.Modifier) []byte {
	switch mod {
	case term.ModCtrl:
		switch ch {
		case 'a', 'A':
			return []byte{0x01}
		case 'b', 'B':
			return []byte{0x02}
		case 'c', 'C':
			return []byte{0x03}
		case 'd', 'D':
			return []byte{0x04}
		case 'e', 'E':
			return []byte{0x05}
		case 'f', 'F':
			return []byte{0x06}
		case 'g', 'G':
			return []byte{0x07}
		case 'h', 'H':
			return []byte{0x08}
		case 'i', 'I':
			return []byte{0x09}
		case 'j', 'J':
			return []byte{0x0a}
		case 'k', 'K':
			return []byte{0x0b}
		case 'l', 'L':
			return []byte{0x0c}
		case 'm', 'M':
			return []byte{0x0d}
		case 'n', 'N':
			return []byte{0x0e}
		case 'o', 'O':
			return []byte{0x0f}
		case 'p', 'P':
			return []byte{0x10}
		case 'q', 'Q':
			return []byte{0x11}
		case 'r', 'R':
			return []byte{0x12}
		case 's', 'S':
			return []byte{0x13}
		case 't', 'T':
			return []byte{0x14}
		case 'u', 'U':
			return []byte{0x15}
		case 'v', 'V':
			return []byte{0x16}
		case 'w', 'W':
			return []byte{0x17}
		case 'x', 'X':
			return []byte{0x18}
		case 'y', 'Y':
			return []byte{0x19}
		case 'z', 'Z':
			return []byte{0x1a}
		case '[':
			return []byte{0x1b}
		case '\\':
			return []byte{0x1c}
		case ']':
			return []byte{0x1d}
		case '_':
			return []byte{0x1f}
		case '`':
			return []byte{0x60}
		default:
			return nil
		}
	case term.ModAlt:
		return append([]byte{0x1b}, string(ch)...)
	case 0:
		return []byte(string(ch))
	default:
		return nil
	}
}

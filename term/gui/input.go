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
package gui

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/gui/font"
)

const (
	defaultKeyPressDelay  = 500 * time.Millisecond
	defaultKeyPressRepeat = 30 * time.Millisecond
)

// abstracts ebiten* input methods
type keysManager interface {
	IsKeyPressed(ebiten.Key) bool
	AppendInputChars([]rune) []rune
	Now() time.Time
}

type input struct {
	fontManager *font.Manager
	chars       []rune
	// ebiten sometimes sends a char on frame N, and a key
	// for the same char on frame N+1; this field helps coalesce to
	// chars when that's the case so we don't miss-fire char keys
	// on frame N+1
	keyChars       []rune
	input          keysManager
	keyPressDelay  time.Duration
	keyPressRepeat time.Duration

	keyState    map[ebiten.Key]press
	charState   map[rune]press
	modState    map[term.Modifier]press
	charCurrent map[rune]struct{}
	modCurrent  map[term.Modifier]struct{}
}

type press struct {
	at        time.Time
	repeating bool
}

func newInput(fontManager *font.Manager) *input {
	ret := new(input)
	ret.keyState = make(map[ebiten.Key]press)
	ret.charState = make(map[rune]press)
	ret.modState = make(map[term.Modifier]press)
	ret.modCurrent = make(map[term.Modifier]struct{})
	ret.charCurrent = make(map[rune]struct{})
	ret.fontManager = fontManager
	ret.input = ebitenInputManager{}
	ret.keyPressDelay = defaultKeyPressDelay
	ret.keyPressRepeat = defaultKeyPressRepeat
	return ret
}

func (i *input) processEvents() (ev term.Event, ok, resize bool) {
	now := i.input.Now()
	defer clearPressedCache[rune](i.charState, i.charCurrent, now)
	defer clearPressedCache[term.Modifier](i.modState, i.modCurrent, now)

	var mod term.Modifier
	mod = i.processModifiers()
	switch mod {
	case term.ModMeta:
		if i.input.IsKeyPressed(ebiten.KeyMinus) {
			err := i.fontManager.DecreaseSize()
			if err != nil {
				i.log(log.ErrorLevel, "decrease font size: %v", err)
			}
			resize = true
			return
		}
		if i.input.IsKeyPressed(ebiten.KeyEqual) {
			err := i.fontManager.IncreaseSize()
			if err != nil {
				i.log(log.ErrorLevel, "increase font size: %v", err)
			}
			resize = true
			return
		}
	}

	// process all first so shouldFire populates the cache in any case
	modEv, modOk := i.handleModifier(i.modCurrent, now, mod)
	keyEv, mod, keyOk := i.handleKeys(now, mod)
	chEv, chOk := i.handleChars(i.charCurrent, now, mod)

	if keyOk {
		ev = keyEv
		ok = true
		return
	}

	if chOk {
		ev = chEv
		ok = true
		return
	}

	if modOk {
		ev = modEv
		ok = true
	}
	return
}

func (i *input) processModifiers() (mod term.Modifier) {
	isMetaPressed := i.input.IsKeyPressed(ebiten.KeyMeta)
	isCtrlPressed := i.input.IsKeyPressed(ebiten.KeyControl)
	isAltPressed := i.input.IsKeyPressed(ebiten.KeyAlt)
	isShiftPressed := i.input.IsKeyPressed(ebiten.KeyShift)

	// handle modifiers separately first, so keys below can be interpreted
	// already taking modifiers into consideration.
	if isCtrlPressed && isShiftPressed && isAltPressed {
		mod = term.ModCtrlShiftAlt
	} else if isCtrlPressed && isShiftPressed && isMetaPressed {
		mod = term.ModCtrlShiftMeta
	} else if isCtrlPressed && isAltPressed && isMetaPressed {
		mod = term.ModCtrlAltMeta
	} else if isAltPressed && isShiftPressed && isMetaPressed {
		mod = term.ModAltShiftMeta
	} else if isCtrlPressed && isShiftPressed {
		mod = term.ModCtrlShift
	} else if isCtrlPressed && isAltPressed {
		mod = term.ModCtrlAlt
	} else if isCtrlPressed && isMetaPressed {
		mod = term.ModCtrlMeta
	} else if isShiftPressed && isMetaPressed {
		mod = term.ModShiftMeta
	} else if isAltPressed && isMetaPressed {
		mod = term.ModAltMeta
	} else if isAltPressed && isShiftPressed {
		mod = term.ModAltShift
	} else if isAltPressed {
		mod = term.ModAlt
	} else if isShiftPressed {
		mod = term.ModShift
	} else if isMetaPressed {
		mod = term.ModMeta
	} else if isCtrlPressed {
		mod = term.ModCtrl
	}
	return
}

func (i *input) handleKeys(now time.Time, mod term.Modifier) (
	ev term.Event, retMod term.Modifier, ok bool,
) {
	retMod = mod
	i.keyChars = i.keyChars[:0]
	// We need to iterate over them all so shouldFireNow can clear
	// state of keys that are no longer pressed.
	for key := ebiten.Key(0); key < ebiten.KeyAlt; key++ {
		switch key {
		case ebiten.KeyMeta, ebiten.KeyMetaLeft, ebiten.KeyMetaRight,
			ebiten.KeyAlt, ebiten.KeyAltLeft, ebiten.KeyAltRight,
			ebiten.KeyShift, ebiten.KeyShiftLeft, ebiten.KeyShiftRight,
			ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight:
		case ebiten.KeyA:
			i.charPressedMap(now, ebiten.KeyA, 'a', 'A', &retMod)
		case ebiten.KeyB:
			i.charPressedMap(now, ebiten.KeyB, 'b', 'B', &retMod)
		case ebiten.KeyC:
			i.charPressedMap(now, ebiten.KeyC, 'c', 'C', &retMod)
		case ebiten.KeyD:
			i.charPressedMap(now, ebiten.KeyD, 'd', 'D', &retMod)
		case ebiten.KeyE:
			i.charPressedMap(now, ebiten.KeyE, 'e', 'E', &retMod)
		case ebiten.KeyF:
			i.charPressedMap(now, ebiten.KeyF, 'f', 'F', &retMod)
		case ebiten.KeyG:
			i.charPressedMap(now, ebiten.KeyG, 'g', 'G', &retMod)
		case ebiten.KeyH:
			i.charPressedMap(now, ebiten.KeyH, 'h', 'H', &retMod)
		case ebiten.KeyI:
			i.charPressedMap(now, ebiten.KeyI, 'i', 'I', &retMod)
		case ebiten.KeyJ:
			i.charPressedMap(now, ebiten.KeyJ, 'j', 'J', &retMod)
		case ebiten.KeyK:
			i.charPressedMap(now, ebiten.KeyK, 'k', 'K', &retMod)
		case ebiten.KeyL:
			i.charPressedMap(now, ebiten.KeyL, 'l', 'L', &retMod)
		case ebiten.KeyM:
			i.charPressedMap(now, ebiten.KeyM, 'm', 'M', &retMod)
		case ebiten.KeyN:
			i.charPressedMap(now, ebiten.KeyN, 'n', 'N', &retMod)
		case ebiten.KeyO:
			i.charPressedMap(now, ebiten.KeyO, 'o', 'O', &retMod)
		case ebiten.KeyP:
			i.charPressedMap(now, ebiten.KeyP, 'p', 'P', &retMod)
		case ebiten.KeyQ:
			i.charPressedMap(now, ebiten.KeyQ, 'q', 'Q', &retMod)
		case ebiten.KeyR:
			i.charPressedMap(now, ebiten.KeyR, 'r', 'R', &retMod)
		case ebiten.KeyS:
			i.charPressedMap(now, ebiten.KeyS, 's', 'S', &retMod)
		case ebiten.KeyT:
			i.charPressedMap(now, ebiten.KeyT, 't', 'T', &retMod)
		case ebiten.KeyU:
			i.charPressedMap(now, ebiten.KeyU, 'u', 'U', &retMod)
		case ebiten.KeyV:
			i.charPressedMap(now, ebiten.KeyV, 'v', 'V', &retMod)
		case ebiten.KeyW:
			i.charPressedMap(now, ebiten.KeyW, 'w', 'W', &retMod)
		case ebiten.KeyX:
			i.charPressedMap(now, ebiten.KeyX, 'x', 'X', &retMod)
		case ebiten.KeyY:
			i.charPressedMap(now, ebiten.KeyY, 'y', 'Y', &retMod)
		case ebiten.KeyZ:
			i.charPressedMap(now, ebiten.KeyZ, 'z', 'Z', &retMod)
		case ebiten.KeyMinus:
			i.charPressedMap(now, ebiten.KeyMinus, '-', '_', &retMod)
		case ebiten.KeyDigit0:
			i.charPressedMap(now, ebiten.KeyDigit0, '0', ')', &retMod)
		case ebiten.KeyDigit1:
			i.charPressedMap(now, ebiten.KeyDigit1, '1', '!', &retMod)
		case ebiten.KeyDigit2:
			i.charPressedMap(now, ebiten.KeyDigit2, '2', '@', &retMod)
		case ebiten.KeyDigit3:
			i.charPressedMap(now, ebiten.KeyDigit3, '3', '#', &retMod)
		case ebiten.KeyDigit4:
			i.charPressedMap(now, ebiten.KeyDigit4, '4', '$', &retMod)
		case ebiten.KeyDigit5:
			i.charPressedMap(now, ebiten.KeyDigit5, '5', '%', &retMod)
		case ebiten.KeyDigit6:
			i.charPressedMap(now, ebiten.KeyDigit6, '6', '^', &retMod)
		case ebiten.KeyDigit7:
			i.charPressedMap(now, ebiten.KeyDigit7, '7', '&', &retMod)
		case ebiten.KeyDigit8:
			i.charPressedMap(now, ebiten.KeyDigit8, '8', '*', &retMod)
		case ebiten.KeyDigit9:
			i.charPressedMap(now, ebiten.KeyDigit9, '9', '(', &retMod)
		case ebiten.KeyEqual:
			i.charPressedMap(now, ebiten.KeyEqual, '=', '+', &retMod)
		case ebiten.KeyNumpadAdd:
			i.charPressed(now, ebiten.KeyNumpadAdd, '+')
		case ebiten.KeyNumpadDecimal:
			i.charPressed(now, ebiten.KeyNumpadDecimal, '.')
		case ebiten.KeyNumpadDivide:
			i.charPressed(now, ebiten.KeyNumpadDivide, '/')
		case ebiten.KeyNumpadEqual:
			i.charPressed(now, ebiten.KeyNumpadEqual, '=')
		case ebiten.KeyNumpadMultiply:
			i.charPressed(now, ebiten.KeyNumpadMultiply, '*')
		case ebiten.KeyNumpadSubtract:
			i.charPressed(now, ebiten.KeyNumpadSubtract, '-')
		case ebiten.KeyNumpad0:
			i.charPressed(now, ebiten.KeyNumpad0, '0')
		case ebiten.KeyNumpad1:
			i.charPressed(now, ebiten.KeyNumpad1, '1')
		case ebiten.KeyNumpad2:
			i.charPressed(now, ebiten.KeyNumpad2, '2')
		case ebiten.KeyNumpad3:
			i.charPressed(now, ebiten.KeyNumpad3, '3')
		case ebiten.KeyNumpad4:
			i.charPressed(now, ebiten.KeyNumpad4, '4')
		case ebiten.KeyNumpad5:
			i.charPressed(now, ebiten.KeyNumpad5, '5')
		case ebiten.KeyNumpad6:
			i.charPressed(now, ebiten.KeyNumpad6, '6')
		case ebiten.KeyNumpad7:
			i.charPressed(now, ebiten.KeyNumpad7, '7')
		case ebiten.KeyNumpad8:
			i.charPressed(now, ebiten.KeyNumpad8, '8')
		case ebiten.KeyNumpad9:
			i.charPressed(now, ebiten.KeyNumpad9, '9')
		case ebiten.KeyComma:
			i.charPressedMap(now, ebiten.KeyComma, ',', '<', &retMod)
		case ebiten.KeyBracketLeft:
			i.charPressedMap(now, ebiten.KeyBracketLeft, '[', '{', &retMod)
		case ebiten.KeyBracketRight:
			i.charPressedMap(now, ebiten.KeyBracketRight, ']', '}', &retMod)
		case ebiten.KeyBackquote:
			i.charPressedMap(now, ebiten.KeyBackquote, '`', '~', &retMod)
		case ebiten.KeyBackslash:
			i.charPressedMap(now, ebiten.KeyBackslash, '\\', '|', &retMod)
		case ebiten.KeySlash:
			i.charPressedMap(now, ebiten.KeySlash, '/', '?', &retMod)
		case ebiten.KeyIntlBackslash:
			i.charPressedMap(now, ebiten.KeyIntlBackslash, '\\', '|', &retMod)
		case ebiten.KeyPeriod:
			i.charPressedMap(now, ebiten.KeyPeriod, '.', '>', &retMod)
		case ebiten.KeyQuote:
			i.charPressedMap(now, ebiten.KeyQuote, '\'', '"', &retMod)
		case ebiten.KeySemicolon:
			i.charPressedMap(now, ebiten.KeySemicolon, ';', ':', &retMod)
		default:
			if i.shouldFireKey(now, key) {
				ev, ok = mapEbitenKey(key, mod)
			}
		}
	}
	return
}

func (i *input) handleChars(curr map[rune]struct{}, now time.Time, mod term.Modifier) (
	ev term.Event, ok bool,
) {
	// compiler optimizes this
	for k := range curr {
		delete(curr, k)
	}
	// key-processed chars take preference over raw chars
	if len(i.keyChars) == 0 {
		i.chars = i.input.AppendInputChars(i.chars[:0])
		// if we are using chars directly, then shift modifier is omitted
		// as they have already been processed. Note that in theory on MacOS the alt
		// modifier is also pre-processed; in practice when a pre-processed alt and char
		// is dispatched, ebiten also dispatches an ebiten.Key so keyChars forces the correct
		// interpretation.
		_, mod = removeShiftModifier(0, 0, mod)
	} else {
		i.chars = append(i.chars, i.keyChars...)
	}
	for _, ch := range i.chars {
		curr[ch] = struct{}{}
		shouldFire := shouldFireKey[rune](
			i.charState, now, i.keyPressDelay, i.keyPressRepeat, ch)
		if shouldFire {
			ev = term.Event{
				Type: term.EventKey,
				Mod:  mod,
				Ch:   ch,
				Raw:  getCharEscapeSequence(ch, mod),
			}
			ok = true
		}
	}
	return
}

func (i *input) charPressed(now time.Time, key ebiten.Key, ch rune) {
	if i.shouldFireKey(now, key) {
		i.keyChars = append(i.keyChars, ch)
	}
}

func (i *input) charPressedMap(
	now time.Time, key ebiten.Key,
	char, shiftChar rune, mod *term.Modifier,
) {
	if i.shouldFireKey(now, key) {
		char, *mod = removeShiftModifier(char, shiftChar, *mod)
		i.keyChars = append(i.keyChars, char)
	}
}

func (i *input) shouldFireKey(now time.Time, key ebiten.Key) bool {
	if !i.input.IsKeyPressed(key) {
		delete(i.keyState, key)
		return false
	}

	return shouldFireKey[ebiten.Key](i.keyState, now,
		i.keyPressDelay, i.keyPressRepeat, key)
}

func (i *input) handleModifier(curr map[term.Modifier]struct{}, now time.Time, mod term.Modifier) (
	ev term.Event, ok bool,
) {
	// we do not dispatch shift on its own due to complications with
	// pre-processed vs non pre-processed characters leading to miss-firing of single
	// shift modifier events
	_, mod = removeShiftModifier(0, 0, mod)
	if mod == 0 {
		return
	}

	// compiler optimizes this
	for k := range curr {
		delete(curr, k)
	}
	curr[mod] = struct{}{}

	shouldFire := shouldFireKey[term.Modifier](i.modState, now,
		i.keyPressDelay, i.keyPressRepeat, mod)
	if shouldFire {
		ok = true
		ev = term.Event{Type: term.EventKey, Mod: mod}
	}
	return
}

func removeShiftModifier(char, shiftChar rune, mod term.Modifier) (rune, term.Modifier) {
	switch mod {
	case term.ModShift:
		return shiftChar, 0
	case term.ModCtrlShift:
		return shiftChar, term.ModCtrl
	case term.ModCtrlShiftAlt:
		return shiftChar, term.ModCtrlAlt
	case term.ModCtrlShiftMeta:
		return shiftChar, term.ModCtrlMeta
	case term.ModShiftMeta:
		return shiftChar, term.ModMeta
	case term.ModAltShiftMeta:
		return shiftChar, term.ModAltMeta
	case term.ModAltShift:
		return shiftChar, term.ModAlt
	default:
		return char, mod
	}
}

func clearPressedCache[T comparable](
	cache map[T]press, curr map[T]struct{}, now time.Time,
) {
	// clear all chars that are not pressed now
	for k := range cache {
		if _, ok := curr[k]; !ok {
			delete(cache, k)
		}
	}
}

func shouldFireKey[T comparable](
	cache map[T]press, now time.Time,
	keyPressDelay, keyPressRepeat time.Duration,
	cacheKey T,
) bool {
	event, ok := cache[cacheKey]
	if !ok {
		cache[cacheKey] = press{at: now}
		return true
	}

	since := now.Sub(event.at)
	if !event.repeating && since > keyPressDelay {
		cache[cacheKey] = press{at: now, repeating: true}
		return true
	} else if event.repeating && since > keyPressRepeat {
		cache[cacheKey] = press{at: now, repeating: true}
		return true
	}

	return false
}

func mapEbitenKey(key ebiten.Key, mod term.Modifier) (ev term.Event, ok bool) {
	ev, ok = doMapEbitenKey(key, mod)
	if ok {
		ev.Mod = mod
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
		return term.Event{
			Key: term.KeySpace,
			Raw: []byte{' '},
		}, true
	case ebiten.KeyArrowDown:
		return term.Event{
			Key: term.KeyArrowDown,
			Raw: []byte(fmt.Sprintf("\x1b[%sB", getModifierStr(mod))),
		}, true
	case ebiten.KeyArrowLeft:
		return term.Event{
			Key: term.KeyArrowLeft,
			Raw: []byte(fmt.Sprintf("\x1b[%sD", getModifierStr(mod))),
		}, true
	case ebiten.KeyArrowRight:
		return term.Event{
			Key: term.KeyArrowRight,
			Raw: []byte(fmt.Sprintf("\x1b[%sC", getModifierStr(mod))),
		}, true
	case ebiten.KeyArrowUp:
		return term.Event{
			Key: term.KeyArrowUp,
			Raw: []byte(fmt.Sprintf("\x1b[%sA", getModifierStr(mod))),
		}, true
	case ebiten.KeyBackspace:
		var raw []byte
		if mod == term.ModAlt {
			raw = []byte{0x1b, 0x7f}
		} else if mod == 0 {
			// the rest of modifiers do nothing
			raw = []byte{0x7f}
		}
		return term.Event{
			Key: term.KeyBackspace,
			Raw: raw,
		}, true
	case ebiten.KeyDelete:
		return term.Event{
			Key: term.KeyDelete,
			Raw: []byte(fmt.Sprintf("\x1b[3%s~", getModifierStr2(mod))),
		}, true
	case ebiten.KeyEnd:
		return term.Event{
			Key: term.KeyEnd,
			Raw: []byte(fmt.Sprintf("\x1b[%sF", getModifierStr(mod))),
		}, true
	case ebiten.KeyHome:
		return term.Event{
			Key: term.KeyHome,
			Raw: []byte(fmt.Sprintf("\x1b[%sH", getModifierStr(mod))),
		}, true
	case ebiten.KeyInsert:
		return term.Event{
			Key: term.KeyInsert,
			Raw: []byte(fmt.Sprintf("\x1b[2%s~", getModifierStr2(mod))),
		}, true
	case ebiten.KeyPageDown:
		return term.Event{
			Key: term.KeyPgdn,
			Raw: []byte(fmt.Sprintf("\x1b[6%s~", getModifierStr2(mod))),
		}, true
	case ebiten.KeyPageUp:
		return term.Event{
			Key: term.KeyPgup,
			Raw: []byte(fmt.Sprintf("\x1b[5%s~", getModifierStr2(mod))),
		}, true
	case ebiten.KeyEnter:
		var raw []byte
		if mod == 0 {
			raw = []byte{0x0d, 0x0a}
		}
		return term.Event{
			Key: term.KeyEnter,
			Raw: raw,
		}, true
	case ebiten.KeyEscape:
		var raw []byte
		if mod == 0 {
			raw = []byte{0x1b}
		}
		return term.Event{
			Key: term.KeyEsc,
			Raw: raw,
		}, true
	case ebiten.KeyF1:
		if mod != 0 {
			return term.Event{
				Key: term.KeyF1,
				Raw: []byte(fmt.Sprintf("\x1b[%sP", getModifierStr(mod))),
			}, true
		}
		return term.Event{
			Key: term.KeyF1,
			Raw: []byte("\x1bOP"),
		}, true
	case ebiten.KeyF2:
		if mod != 0 {
			return term.Event{
				Key: term.KeyF2,
				Raw: []byte(fmt.Sprintf("\x1b[%sQ", getModifierStr(mod))),
			}, true
		}
		return term.Event{
			Key: term.KeyF2,
			Raw: []byte("\x1bOQ"),
		}, true
	case ebiten.KeyF3:
		if mod != 0 {
			return term.Event{
				Key: term.KeyF3,
				Raw: []byte(fmt.Sprintf("\x1b[%sR", getModifierStr(mod))),
			}, true
		}
		return term.Event{
			Key: term.KeyF3,
			Raw: []byte("\x1bOR"),
		}, true
	case ebiten.KeyF4:
		if mod != 0 {
			return term.Event{
				Key: term.KeyF4,
				Raw: []byte(fmt.Sprintf("\x1b[%sS", getModifierStr(mod))),
			}, true
		}
		return term.Event{
			Key: term.KeyF4,
			Raw: []byte("\x1bOS"),
		}, true
	case ebiten.KeyF5:
		return term.Event{
			Key: term.KeyF5,
			Raw: []byte(fmt.Sprintf("\x1b[15%s~", getModifierStr2(mod))),
		}, true
	case ebiten.KeyF6:
		return term.Event{
			Key: term.KeyF6,
			Raw: []byte(fmt.Sprintf("\x1b[17%s~", getModifierStr2(mod))),
		}, true
	case ebiten.KeyF7:
		return term.Event{
			Key: term.KeyF7,
			Raw: []byte(fmt.Sprintf("\x1b[18%s~", getModifierStr2(mod))),
		}, true
	case ebiten.KeyF8:
		return term.Event{
			Key: term.KeyF8,
			Raw: []byte(fmt.Sprintf("\x1b[19%s~", getModifierStr2(mod))),
		}, true
	case ebiten.KeyF9:
		return term.Event{
			Key: term.KeyF9,
			Raw: []byte(fmt.Sprintf("\x1b[20%s~", getModifierStr2(mod))),
		}, true
	case ebiten.KeyF10:
		return term.Event{
			Key: term.KeyF10,
			Raw: []byte(fmt.Sprintf("\x1b[21%s~", getModifierStr2(mod))),
		}, true
	case ebiten.KeyF11:
		return term.Event{
			Key: term.KeyF11,
			Raw: []byte(fmt.Sprintf("\x1b[23%s~", getModifierStr2(mod))),
		}, true
	case ebiten.KeyF12:
		return term.Event{
			Key: term.KeyF12,
			Raw: []byte(fmt.Sprintf("\x1b[24%s~", getModifierStr2(mod))),
		}, true
	case ebiten.KeyTab:
		var raw []byte
		if mod == term.ModShift {
			raw = []byte{0x1b, 0x5b, 0x5a}
		} else if mod == 0 {
			raw = []byte{0x09}
		}
		return term.Event{
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

func (e *input) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "gui").Logf(level, msg, args...)
}

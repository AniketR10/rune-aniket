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
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/term/gui/font"
)

func press(key ebiten.Key, mods ...ebiten.KeyModifier) ebiten.KeyEvent {
	var m ebiten.KeyModifier
	for _, mod := range mods {
		m |= mod
	}
	return ebiten.KeyEvent{Key: key, Action: ebiten.KeyActionPress, Mods: m}
}

func repeat(key ebiten.Key, mods ...ebiten.KeyModifier) ebiten.KeyEvent {
	var m ebiten.KeyModifier
	for _, mod := range mods {
		m |= mod
	}
	return ebiten.KeyEvent{Key: key, Action: ebiten.KeyActionRepeat, Mods: m}
}

func release(key ebiten.Key, mods ...ebiten.KeyModifier) ebiten.KeyEvent {
	var m ebiten.KeyModifier
	for _, mod := range mods {
		m |= mod
	}
	return ebiten.KeyEvent{Key: key, Action: ebiten.KeyActionRelease, Mods: m}
}

func TestInputFireOnce(t *testing.T) {
	suite := []struct {
		description    string
		keyEvents      []ebiten.KeyEvent
		chars          []rune
		expectedEvents []term.Event
	}{
		{
			description:    "dispatches a single non-char key",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyEnter)},
			expectedEvents: []term.Event{{Type: term.EventKey, Key: term.KeyEnter, Raw: []byte{0x0d, 0x0a}}},
		},
		{
			description: "plain printable key alone dispatches nothing (text comes from chars)",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyA)},
		},
		{
			description:    "dispatches a single key char, via char",
			chars:          []rune{'a'},
			expectedEvents: []term.Event{{Type: term.EventKey, Ch: 'a', Raw: []byte("a")}},
		},
		{
			description:    "dispatches a space key",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeySpace)},
			expectedEvents: []term.Event{{Type: term.EventKey, Key: term.KeySpace, Raw: []byte(" ")}},
		},
		{
			description:    "dispatches only one space when OS sends both key event and char",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeySpace)},
			chars:          []rune{' '},
			expectedEvents: []term.Event{{Type: term.EventKey, Key: term.KeySpace, Raw: []byte(" ")}},
		},
		{
			description:    "dispatches a shift+space like a space key",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeySpace, ebiten.KeyModShift)},
			expectedEvents: []term.Event{{Type: term.EventKey, Key: term.KeySpace, Raw: []byte(" ")}},
		},
		{
			description:    "dispatches a meta+space as meta+space key",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeySpace, ebiten.KeyModSuper)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModMeta, Key: term.KeySpace, Raw: []byte(" ")}},
		},
		{
			description:    "dispatches a single key ctrl + char",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModControl)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'a', Raw: []byte{0x01}}},
		},
		{
			description:    "dispatches a single key shift + ctrl + char",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModShift, ebiten.KeyModControl)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'A', Raw: []byte{0x01}}},
		},
		{
			description:    "dispatches a single key meta + char",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModSuper)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModMeta, Ch: 'a'}},
		},
		{
			// NOTE: The fallback char path has no modifier information.
			// If Alt is held but only a char arrives (no key event), the
			// Alt modifier is lost. On desktop/GLFW the key callback
			// always fires so this path is only hit for IME/paste input
			// where modifiers aren't relevant.
			description: "fallback char carries no modifier even if one was held",
			keyEvents:   nil,
			chars:       []rune{'a'},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Ch: 'a', Raw: []byte("a")},
			},
		},
		{
			description:    "dispatches a single key alt + char, via key",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModAlt)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModAlt, Ch: 'a', Raw: []byte{0x1b, 'a'}}},
		},
		{
			description:    "dispatches a single key alt + char, undoes macos special chars",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModAlt)},
			chars:          []rune{'å'},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModAlt, Ch: 'a', Raw: []byte{0x1b, 'a'}}},
		},
		{
			description: "shift + printable key alone dispatches nothing (text comes from chars)",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModShift)},
		},
		{
			description:    "shifted char via char stream dispatches the shifted rune",
			chars:          []rune{'A'},
			expectedEvents: []term.Event{{Type: term.EventKey, Ch: 'A', Raw: []byte("A")}},
		},
		{
			description: "dispatches a single key ctrl + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModControl)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModCtrl,
				Key: term.KeyF1, Raw: []byte{0x1b, 0x5b, 0x31, 0x3b, 0x35, 0x50}}},
		},
		{
			description: "dispatches a single key meta + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModSuper)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModMeta,
				Key: term.KeyF1, Raw: []byte{0x1b, 0x5b, 0x31, 0x3b, 0x39, 0x50}}},
		},
		{
			description:    "dispatches a single key alt + char, via key",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModAlt)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModAlt, Ch: 'a', Raw: []byte{0x1b, 'a'}}},
		},
		{
			description: "unhandled key dispatches no event",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF24)},
		},
		{
			description: "dispatches a single key ctrl + alt + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModControl, ebiten.KeyModAlt)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModCtrlAlt,
				Key: term.KeyF1, Raw: []byte("\x1b[1;7P")}},
		},
		{
			description: "dispatches a single key ctrl + meta + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModControl, ebiten.KeyModSuper)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModCtrlMeta,
				Key: term.KeyF1, Raw: []byte("\x1b[1;13P")}},
		},
		{
			description: "dispatches a single key ctrl + shift + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModControl, ebiten.KeyModShift)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModCtrlShift,
				Key: term.KeyF1, Raw: []byte("\x1b[1;6P")}},
		},
		{
			description: "dispatches a single key ctrl + alt + shift + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModControl, ebiten.KeyModAlt, ebiten.KeyModShift)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModCtrlShiftAlt,
				Key: term.KeyF1, Raw: []byte("\x1b[1;8P")}},
		},
		{
			description: "dispatches a single key ctrl + shift + meta + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModControl, ebiten.KeyModSuper, ebiten.KeyModShift)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModCtrlShiftMeta,
				Key: term.KeyF1, Raw: []byte("\x1b[1;14P")}},
		},
		{
			description: "dispatches a single key ctrl + alt + meta + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModControl, ebiten.KeyModSuper, ebiten.KeyModAlt)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModCtrlAltMeta,
				Key: term.KeyF1, Raw: []byte("\x1b[1;15P")}},
		},
		{
			description: "dispatches a single key ctrl + shift + alt + meta + key",
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyArrowRight, ebiten.KeyModControl,
				ebiten.KeyModShift, ebiten.KeyModAlt, ebiten.KeyModSuper)},
			expectedEvents: []term.Event{{Type: term.EventKey,
				Mod: term.ModCtrlShiftAlt | term.ModMeta,
				Key: term.KeyArrowRight, Raw: []byte("\x1b[1;16C")}},
		},
		{
			description: "dispatches a tilde key with ctrl + shift + alt + meta",
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyDelete, ebiten.KeyModControl,
				ebiten.KeyModShift, ebiten.KeyModAlt, ebiten.KeyModSuper)},
			expectedEvents: []term.Event{{Type: term.EventKey,
				Mod: term.ModCtrlShiftAlt | term.ModMeta,
				Key: term.KeyDelete, Raw: []byte("\x1b[3;16~")}},
		},
		{
			description: "dispatches a single key shift + meta + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModShift, ebiten.KeyModSuper)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModShiftMeta,
				Key: term.KeyF1, Raw: []byte("\x1b[1;10P")}},
		},
		{
			description: "dispatches a single key alt + meta + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModAlt, ebiten.KeyModSuper)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModAltMeta,
				Key: term.KeyF1, Raw: []byte("\x1b[1;11P")}},
		},
		{
			description: "dispatches a single key alt + shift + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModAlt, ebiten.KeyModShift)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModAltShift,
				Key: term.KeyF1, Raw: []byte("\x1b[1;4P")}},
		},
		{
			description: "dispatches a single key alt + shift + meta + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModAlt, ebiten.KeyModShift, ebiten.KeyModSuper)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModAltShiftMeta,
				Key: term.KeyF1, Raw: []byte("\x1b[1;12P")}},
		},
		{
			description: "does not dispatch event with raw for meta + enter",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyEnter, ebiten.KeyModSuper)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Mod: term.ModMeta, Key: term.KeyEnter},
			},
		},
		{
			description: "dispatches modifier ctrl + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModControl)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModCtrl,
				Key: term.KeyF1, Raw: []byte{0x1b, 0x5b, 0x31, 0x3b, 0x35, 0x50}}},
		},
		{
			description: "dispatches modifier meta + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModSuper)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModMeta,
				Key: term.KeyF1, Raw: []byte{0x1b, 0x5b, 0x31, 0x3b, 0x39, 0x50}}},
		},
		{
			description: "dispatches modifier alt + key",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF1, ebiten.KeyModAlt)},
			expectedEvents: []term.Event{{Type: term.EventKey, Mod: term.ModAlt,
				Key: term.KeyF1, Raw: []byte{0x1b, 0x5b, 0x31, 0x3b, 0x33, 0x50}}},
		},
		{
			description: "unhandled key dispatches no event (duplicate)",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyF24)},
		},
		{
			description: "does not dispatch modifier ctrl + alt",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyControl), press(ebiten.KeyAlt)},
		},
		{
			description: "does not dispatch modifier ctrl + meta",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyControl), press(ebiten.KeyMeta)},
		},
		{
			description: "does not dispatch modifier ctrl + shift",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyControl), press(ebiten.KeyShift)},
		},
		{
			description: "does not dispatch modifier ctrl + alt + shift",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyControl), press(ebiten.KeyAlt), press(ebiten.KeyShift)},
		},
		{
			description: "does not dispatch modifier ctrl + shift + meta",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyControl), press(ebiten.KeyMeta), press(ebiten.KeyShift)},
		},
		{
			description: "does not dispatch modifier ctrl + alt + meta",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyControl), press(ebiten.KeyMeta), press(ebiten.KeyAlt)},
		},
		{
			description: "does not dispatch modifier shift + meta",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyShift), press(ebiten.KeyMeta)},
		},
		{
			description: "does not dispatch modifier alt + meta",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyAlt), press(ebiten.KeyMeta)},
		},
		{
			description: "does not dispatch modifier alt + shift",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyAlt), press(ebiten.KeyShift)},
		},
		{
			description: "does not dispatch modifier alt + shift + meta",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyAlt), press(ebiten.KeyShift), press(ebiten.KeyMeta)},
		},
		{
			description: "dispatches non-alt modifier with arrow key with raw set",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyArrowUp, ebiten.KeyModShift, ebiten.KeyModSuper)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Key: term.KeyArrowUp, Mod: term.ModShiftMeta,
					Raw: []byte("\x1b[1;10A")},
			},
		},
		{
			description: "dispatches arrow key with raw set",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyArrowUp)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Key: term.KeyArrowUp,
					Raw: []byte("\x1b[A")},
			},
		},
		{
			description: "dispatches alt arrow key with raw set",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyArrowUp, ebiten.KeyModAlt)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Mod: term.ModAlt, Key: term.KeyArrowUp,
					Raw: []byte("\x1b[1;3A")},
			},
		},
		{
			description: "shift+2 key alone dispatches nothing (@ comes from chars)",
			keyEvents:   []ebiten.KeyEvent{press(ebiten.KeyDigit2, ebiten.KeyModShift)},
		},
		{
			description:    "shifted symbol via char stream dispatches the symbol",
			chars:          []rune{'@'},
			expectedEvents: []term.Event{{Type: term.EventKey, Ch: '@', Raw: []byte("@")}},
		},
		{
			description: "OS repeat of a printable key alone dispatches nothing",
			keyEvents:   []ebiten.KeyEvent{repeat(ebiten.KeyA)},
		},
		{
			description: "does not dispatch releases",
			keyEvents:   []ebiten.KeyEvent{release(ebiten.KeyA)},
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			mock, input := newTestInput(t)
			mock.keyEvents = test.keyEvents
			mock.chars = test.chars

			events := input.processEvents(nil)
			if len(test.expectedEvents) == 0 {
				assert.Empty(t, events)
				return
			}
			require.Equal(t, len(test.expectedEvents), len(events))
			assert.Equal(t, test.expectedEvents, events)
		})
	}
}

// frame represents one frame of input for multi-frame tests.
type frame struct {
	keyEvents      []ebiten.KeyEvent
	chars          []rune
	expectedEvents []term.Event
}

func TestInputMultiFrame(t *testing.T) {
	suite := []struct {
		description string
		frames      []frame
	}{
		{
			description: "multiple unhandled key dispatches no events",
			frames: []frame{
				{keyEvents: []ebiten.KeyEvent{press(ebiten.KeyF24)}},
				{keyEvents: []ebiten.KeyEvent{press(ebiten.KeyF24)}},
			},
		},
		{
			description: "press dispatches once, held key with no repeat produces nothing",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyEnter)},
					expectedEvents: []term.Event{{Key: term.KeyEnter}},
				},
				{
					// Key held, no OS repeat yet → no events
				},
			},
		},
		{
			description: "different key on next frame dispatches new event",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyEnter)},
					expectedEvents: []term.Event{{Key: term.KeyEnter}},
				},
				{
					// Key held, no OS repeat yet
				},
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeySpace)},
					expectedEvents: []term.Event{{Key: term.KeySpace}},
				},
			},
		},
		{
			description: "char key press alone dispatches nothing; text arrives via chars",
			frames: []frame{
				{
					// Printable key event only; the char commit lands next frame.
					keyEvents: []ebiten.KeyEvent{press(ebiten.KeyA)},
				},
				{
					chars:          []rune{'a'},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
			},
		},
		{
			description: "chars dispatch one event per frame in order",
			frames: []frame{
				{chars: []rune{'a'}, expectedEvents: []term.Event{{Ch: 'a'}}},
				{chars: []rune{'b'}, expectedEvents: []term.Event{{Ch: 'b'}}},
				{chars: []rune{'a'}, expectedEvents: []term.Event{{Ch: 'a'}}},
				{chars: []rune{'b'}, expectedEvents: []term.Event{{Ch: 'b'}}},
			},
		},
		{
			description: "ctrl + char dispatches once, held produces nothing",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModControl)},
					expectedEvents: []term.Event{{Mod: term.ModCtrl, Ch: 'a'}},
				},
				{
					// Key held, no OS repeat yet
				},
			},
		},
		{
			description: "alternating modifier-only events produce nothing",
			frames: []frame{
				{keyEvents: []ebiten.KeyEvent{press(ebiten.KeyControl)}},
				{keyEvents: []ebiten.KeyEvent{press(ebiten.KeyMeta)}},
				{keyEvents: []ebiten.KeyEvent{press(ebiten.KeyControl)}},
				{keyEvents: []ebiten.KeyEvent{press(ebiten.KeyMeta)}},
			},
		},
		{
			description: "different ctrl+char dispatches a new event",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModControl)},
					expectedEvents: []term.Event{{Mod: term.ModCtrl, Ch: 'a'}},
				},
				{
					// Key held
				},
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyB, ebiten.KeyModControl)},
					expectedEvents: []term.Event{{Mod: term.ModCtrl, Ch: 'b'}},
				},
			},
		},
		{
			description: "shift + ctrl + char dispatches once, held produces nothing",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModShift, ebiten.KeyModControl)},
					expectedEvents: []term.Event{{Mod: term.ModCtrl, Ch: 'A'}},
				},
				{
					// Key held
				},
			},
		},
		{
			description: "different shift+ctrl+char dispatches a new event",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModShift, ebiten.KeyModControl)},
					expectedEvents: []term.Event{{Mod: term.ModCtrl, Ch: 'A'}},
				},
				{
					// Key held
				},
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyB, ebiten.KeyModShift, ebiten.KeyModControl)},
					expectedEvents: []term.Event{{Mod: term.ModCtrl, Ch: 'B'}},
				},
			},
		},
		{
			description: "alt + char via key on both frames",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModAlt)},
					expectedEvents: []term.Event{{Mod: term.ModAlt, Ch: 'a'}},
				},
				{
					// Key held
				},
			},
		},
		{
			description: "alt + shift + char via key on both frames",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModAlt, ebiten.KeyModShift)},
					expectedEvents: []term.Event{{Mod: term.ModAlt, Ch: 'A'}},
				},
				{
					// Key held
				},
			},
		},
		{
			description: "joining chord keys dispatches only the new chord each frame",
			frames: []frame{
				{
					// Shift only → no event
					keyEvents: []ebiten.KeyEvent{press(ebiten.KeyShift)},
				},
				{
					// Shift still held
				},
				{
					// Shift+A is plain printable text → delivered via chars, not
					// the key path.
					keyEvents: []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModShift)},
				},
				{
					// Ctrl added, B still held with shift+ctrl
					// (no new press event from OS, just modifier change)
				},
				{
					// B held with shift+ctrl, no repeat
				},
				{
					// C pressed with shift+ctrl
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyC, ebiten.KeyModShift, ebiten.KeyModControl)},
					expectedEvents: []term.Event{{Mod: term.ModCtrl, Ch: 'C'}},
				},
				{
					// Modifiers held, no char keys
				},
				{
					// All released
				},
			},
		},
		{
			// Printable keys are not echoed by the key path, so an inline char
			// on the same frame is the only text source and must dispatch once.
			description: "printable key with same-frame char dispatches the char only",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA)},
					chars:          []rune{'a'},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
			},
		},
		{
			// Under IBus the key event fires with no inline char and the commit
			// lands a frame later. With separated streams the key path emits
			// nothing, so the delayed commit is the sole, single dispatch.
			description: "IBus delayed commit dispatches once, key path stays silent",
			frames: []frame{
				{
					keyEvents: []ebiten.KeyEvent{press(ebiten.KeyA)},
				},
				{
					chars:          []rune{'a'}, // delayed IBus commit
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
			},
		},
		{
			// Held key with OS repeats: the key path never emits printable text,
			// so each frame's dispatch count is driven only by the chars stream.
			description: "held printable key repeats emit only via chars",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA)},
					chars:          []rune{'a'},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
				{
					keyEvents:      []ebiten.KeyEvent{repeat(ebiten.KeyA)},
					chars:          []rune{'a'},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
				{
					// Repeat with no accompanying commit → nothing.
					keyEvents: []ebiten.KeyEvent{repeat(ebiten.KeyA)},
				},
				{
					keyEvents: []ebiten.KeyEvent{release(ebiten.KeyA)},
				},
			},
		},
		{
			// Interleaving a second key while the first is held: chars are the
			// only text source and dispatch in stream order, so a lagging commit
			// cannot resurrect once its key is released.
			description: "interleaving a second held key dispatches chars in order",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA)},
					chars:          []rune{'a'},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyB)},
					chars:          []rune{'b'},
					expectedEvents: []term.Event{{Ch: 'b'}},
				},
				{
					keyEvents: []ebiten.KeyEvent{release(ebiten.KeyB)},
				},
				{
					keyEvents: []ebiten.KeyEvent{release(ebiten.KeyA)},
				},
			},
		},
		{
			// A composed character (dead key, CJK) has no key-path echo and is
			// delivered verbatim by the chars stream.
			description: "composed IME char passes through the chars stream",
			frames: []frame{
				{
					keyEvents: []ebiten.KeyEvent{press(ebiten.KeyA)},
				},
				{
					chars:          []rune{'é'},
					expectedEvents: []term.Event{{Ch: 'é'}},
				},
			},
		},
		{
			// Alt+printable is a chord on the key path; macOS also delivers the
			// Option-composed rune via chars, which must be dropped so the same
			// physical key is not doubled.
			description: "macOS Alt chord drops the composed char on the same frame",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModAlt)},
					chars:          []rune{'å'},
					expectedEvents: []term.Event{{Mod: term.ModAlt, Ch: 'a'}},
				},
			},
		},
		{
			description: "Ctrl+Alt chord drops the matching char on the same frame",
			frames: []frame{
				{
					keyEvents: []ebiten.KeyEvent{
						press(ebiten.KeyPeriod, ebiten.KeyModControl, ebiten.KeyModAlt),
					},
					chars:          []rune{'.'},
					expectedEvents: []term.Event{{Mod: term.ModCtrlAlt, Ch: '.'}},
				},
			},
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			mock, input := newTestInput(t)
			for fi, f := range test.frames {
				mock.keyEvents = f.keyEvents
				mock.chars = f.chars

				events := input.processEvents(nil)
				if len(f.expectedEvents) == 0 {
					assert.Empty(t, events, "frame %d", fi)
				} else {
					require.Equal(t, len(f.expectedEvents), len(events), "frame %d", fi)
					for ei, expected := range f.expectedEvents {
						actual := events[ei]
						// Strip Raw and Type for multi-frame tests;
						// those are covered in TestInputFireOnce.
						actual.Raw = nil
						actual.Type = 0
						assert.Equal(t, expected, actual, "frame %d event %d", fi, ei)
					}
				}
			}
		})
	}
}

func newTestInput(t *testing.T) (*mockInputManager, *input) {
	mock := &mockInputManager{}
	f, err := font.NewManager(1, 1)
	require.NoError(t, err)
	f.SetFontByFamilyName("")
	f.SetDeviceScale(1)
	ret := newInput(f)
	ret.input = mock
	return mock, ret
}

type mockInputManager struct {
	keyEvents []ebiten.KeyEvent
	chars     []rune
}

func (m *mockInputManager) AppendKeyEvents(buf []ebiten.KeyEvent) []ebiten.KeyEvent {
	return append(buf, m.keyEvents...)
}

func (m *mockInputManager) AppendInputChars(buf []rune) []rune {
	return append(buf, m.chars...)
}

func TestInputKeyMapping(t *testing.T) {
	suite := []struct {
		description    string
		mapping        map[term.KeyComb]term.KeyComb
		keyEvents      []ebiten.KeyEvent
		chars          []rune
		expectedEvents []term.Event
	}{
		{
			description: "remaps CapsLock to Esc",
			mapping: map[term.KeyComb]term.KeyComb{
				{Key: term.KeyCapsLock}: {Key: term.KeyEsc},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyCapsLock)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Key: term.KeyEsc, Raw: []byte{0x1b}},
			},
		},
		{
			description: "unmapped CapsLock dispatches no event",
			mapping: map[term.KeyComb]term.KeyComb{
				{Key: term.KeyNumLock}: {Key: term.KeyEsc},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyCapsLock)},
		},
		{
			description:    "CapsLock with no mapping table dispatches no event",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyCapsLock)},
			expectedEvents: nil,
		},
		{
			description: "remaps NumLock to a char target",
			mapping: map[term.KeyComb]term.KeyComb{
				{Key: term.KeyNumLock}: {Ch: 'a'},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyNumLock)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Ch: 'a', Raw: []byte("a")},
			},
		},
		{
			description: "remaps ContextMenu (menu) to Esc",
			mapping: map[term.KeyComb]term.KeyComb{
				{Key: term.KeyMenu}: {Key: term.KeyEsc},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyContextMenu)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Key: term.KeyEsc, Raw: []byte{0x1b}},
			},
		},
		{
			description: "remaps a char source to a named key",
			mapping: map[term.KeyComb]term.KeyComb{
				{Ch: 'a'}: {Key: term.KeyEsc},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyA)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Key: term.KeyEsc, Raw: []byte{0x1b}},
			},
		},
		{
			description: "remaps a named key source to a char",
			mapping: map[term.KeyComb]term.KeyComb{
				{Key: term.KeyEsc}: {Ch: 'a'},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyEscape)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Ch: 'a', Raw: []byte("a")},
			},
		},
		{
			// A key remapped to a printable target has no char-stream echo, so
			// it must be emitted from the key path even though plain printable
			// keys are otherwise skipped. An unrelated char on the same frame
			// still flows through independently.
			description: "remapped key-to-char emits alongside an independent char",
			mapping: map[term.KeyComb]term.KeyComb{
				{Key: term.KeyNumLock}: {Ch: 'a'},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyNumLock)},
			chars:     []rune{'b'},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Ch: 'a', Raw: []byte("a")},
				{Type: term.EventKey, Ch: 'b', Raw: []byte("b")},
			},
		},
		{
			description: "passes through keys absent from a non-empty table",
			mapping: map[term.KeyComb]term.KeyComb{
				{Key: term.KeyCapsLock}: {Key: term.KeyEsc},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyEnter)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Key: term.KeyEnter, Raw: []byte{0x0d, 0x0a}},
			},
		},
		{
			description: "remap matches modifier in source",
			mapping: map[term.KeyComb]term.KeyComb{
				{Key: term.KeyArrowLeft, Mod: term.ModCtrl}: {Key: term.KeyHome},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyArrowLeft, ebiten.KeyModControl)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Key: term.KeyHome, Raw: []byte("\x1b[H")},
			},
		},
		{
			description: "remaps ctrl-a to meta-a",
			mapping: map[term.KeyComb]term.KeyComb{
				{Mod: term.ModCtrl, Ch: 'a'}: {Mod: term.ModMeta, Ch: 'a'},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModControl)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Mod: term.ModMeta, Ch: 'a'},
			},
		},
		{
			description: "ctrl-b passes through when only ctrl-a is remapped",
			mapping: map[term.KeyComb]term.KeyComb{
				{Mod: term.ModCtrl, Ch: 'a'}: {Mod: term.ModMeta, Ch: 'a'},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyB, ebiten.KeyModControl)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'b', Raw: []byte{0x02}},
			},
		},
		{
			description: "ctrl-a to meta-a leaves other modifier+char combos untouched",
			mapping: map[term.KeyComb]term.KeyComb{
				{Mod: term.ModCtrl, Ch: 'a'}: {Mod: term.ModMeta, Ch: 'a'},
			},
			keyEvents: []ebiten.KeyEvent{
				press(ebiten.KeyB, ebiten.KeyModControl),
				press(ebiten.KeyB, ebiten.KeyModSuper),
				press(ebiten.KeyB, ebiten.KeyModAlt),
			},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'b', Raw: getCharEscapeSequence('b', term.ModCtrl)},
				{Type: term.EventKey, Mod: term.ModMeta, Ch: 'b', Raw: getCharEscapeSequence('b', term.ModMeta)},
				{Type: term.EventKey, Mod: term.ModAlt, Ch: 'b', Raw: getCharEscapeSequence('b', term.ModAlt)},
			},
		},
		{
			description: "ctrl-a to alt-a with ctrl to meta remaps ctrl-a to alt-a and ctrl-b to meta-b",
			mapping: map[term.KeyComb]term.KeyComb{
				{Mod: term.ModCtrl, Ch: 'a'}: {Mod: term.ModAlt, Ch: 'a'},
				{Mod: term.ModCtrl}:          {Mod: term.ModMeta},
			},
			keyEvents: []ebiten.KeyEvent{
				press(ebiten.KeyA, ebiten.KeyModControl),
				press(ebiten.KeyB, ebiten.KeyModControl),
			},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Mod: term.ModAlt, Ch: 'a', Raw: getCharEscapeSequence('a', term.ModAlt)},
				{Type: term.EventKey, Mod: term.ModMeta, Ch: 'b', Raw: getCharEscapeSequence('b', term.ModMeta)},
			},
		},
		{
			description: "remaps bare ctrl to meta and emits nothing",
			mapping: map[term.KeyComb]term.KeyComb{
				{Mod: term.ModCtrl}: {Mod: term.ModMeta},
			},
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyControl)},
			expectedEvents: nil,
		},
		{
			description: "remaps bare ctrl to meta with self-bit set and emits nothing",
			mapping: map[term.KeyComb]term.KeyComb{
				{Mod: term.ModCtrl}: {Mod: term.ModMeta},
			},
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyControl, ebiten.KeyModControl)},
			expectedEvents: nil,
		},
		{
			description: "remaps bare ctrl to esc",
			mapping: map[term.KeyComb]term.KeyComb{
				{Mod: term.ModCtrl}: {Key: term.KeyEsc},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyControl, ebiten.KeyModControl)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Key: term.KeyEsc, Raw: []byte{0x1b}},
			},
		},
		{
			description: "remaps shift-ctrl-a to shift-meta-a",
			mapping: map[term.KeyComb]term.KeyComb{
				{Mod: term.ModCtrl, Ch: 'A'}: {Mod: term.ModMeta, Ch: 'A'},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModControl, ebiten.KeyModShift)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Mod: term.ModMeta, Ch: 'A', Raw: getCharEscapeSequence('A', term.ModMeta)},
			},
		},
		{
			description: "shift-ctrl-b passes through when only shift-ctrl-a is remapped",
			mapping: map[term.KeyComb]term.KeyComb{
				{Mod: term.ModCtrl, Ch: 'A'}: {Mod: term.ModMeta, Ch: 'A'},
			},
			keyEvents: []ebiten.KeyEvent{press(ebiten.KeyB, ebiten.KeyModControl, ebiten.KeyModShift)},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'B', Raw: getCharEscapeSequence('B', term.ModCtrl)},
			},
		},
		{
			description: "shift-ctrl-a to shift-meta-a leaves other shifted modifier+char combos untouched",
			mapping: map[term.KeyComb]term.KeyComb{
				{Mod: term.ModCtrl, Ch: 'A'}: {Mod: term.ModMeta, Ch: 'A'},
			},
			keyEvents: []ebiten.KeyEvent{
				press(ebiten.KeyB, ebiten.KeyModControl, ebiten.KeyModShift),
				press(ebiten.KeyB, ebiten.KeyModSuper, ebiten.KeyModShift),
				press(ebiten.KeyB, ebiten.KeyModAlt, ebiten.KeyModShift),
			},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'B', Raw: getCharEscapeSequence('B', term.ModCtrl)},
				{Type: term.EventKey, Mod: term.ModMeta, Ch: 'B', Raw: getCharEscapeSequence('B', term.ModMeta)},
				{Type: term.EventKey, Mod: term.ModAlt, Ch: 'B', Raw: getCharEscapeSequence('B', term.ModAlt)},
			},
		},
		{
			description: "shift-ctrl-a to shift-alt-a with ctrl to meta remaps shift-ctrl-a to shift-alt-a and shift-ctrl-b to shift-meta-b",
			mapping: map[term.KeyComb]term.KeyComb{
				{Mod: term.ModCtrl, Ch: 'A'}: {Mod: term.ModAlt, Ch: 'A'},
				{Mod: term.ModCtrl}:          {Mod: term.ModMeta},
			},
			keyEvents: []ebiten.KeyEvent{
				press(ebiten.KeyA, ebiten.KeyModControl, ebiten.KeyModShift),
				press(ebiten.KeyB, ebiten.KeyModControl, ebiten.KeyModShift),
			},
			expectedEvents: []term.Event{
				{Type: term.EventKey, Mod: term.ModAlt, Ch: 'A', Raw: getCharEscapeSequence('A', term.ModAlt)},
				{Type: term.EventKey, Mod: term.ModMeta, Ch: 'B', Raw: getCharEscapeSequence('B', term.ModMeta)},
			},
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			mock, input := newTestInput(t)
			input.setKeyMapping(test.mapping)
			mock.keyEvents = test.keyEvents
			mock.chars = test.chars

			events := input.processEvents(nil)
			if len(test.expectedEvents) == 0 {
				assert.Empty(t, events)
				return
			}
			require.Equal(t, len(test.expectedEvents), len(events))
			assert.Equal(t, test.expectedEvents, events)
		})
	}
}

// TestInputForwardsEmojiSequenceRunes asserts that the character stream is
// forwarded rune for rune, including the zero-width joiner that composes a
// family emoji. The emoji picker delivers a ZWJ sequence through
// AppendInputChars; the runtime must emit a key event for every rune so the
// buffer can coalesce them into one grapheme cluster. Dropping the joiner
// here (as the upstream ebiten IsPrint filter used to) would split the emoji.
func TestInputForwardsEmojiSequenceRunes(t *testing.T) {
	mock, input := newTestInput(t)
	family := []rune{'\U0001F468', '\u200d', '\U0001F469', '\u200d', '\U0001F467'}
	mock.chars = family

	events := input.processEvents(nil)

	require.Len(t, events, len(family))
	for i, r := range family {
		assert.Equal(t, term.Event{
			Type: term.EventKey,
			Ch:   r,
			Raw:  getCharEscapeSequence(r, 0),
		}, events[i], "rune %d (%#U) must be forwarded as a key event", i, r)
	}
}

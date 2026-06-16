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
			description:    "dispatches a single key char, via key",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA)},
			expectedEvents: []term.Event{{Type: term.EventKey, Ch: 'a', Raw: []byte("a")}},
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
			description:    "omits shift modifier for shift + char, via key",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModShift)},
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
			description:    "shift+2 dispatches @",
			keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyDigit2, ebiten.KeyModShift)},
			expectedEvents: []term.Event{{Type: term.EventKey, Ch: '@', Raw: []byte("@")}},
		},
		{
			description:    "OS repeat dispatches event",
			keyEvents:      []ebiten.KeyEvent{repeat(ebiten.KeyA)},
			expectedEvents: []term.Event{{Type: term.EventKey, Ch: 'a', Raw: []byte("a")}},
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
			description: "char key press dispatches once, held key with no repeat produces nothing",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA)},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
				{
					// Key held, no OS repeat yet
				},
			},
		},
		{
			description: "different char key dispatches new event",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA)},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
				{
					// Key held, no OS repeat yet
				},
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyB)},
					expectedEvents: []term.Event{{Ch: 'b'}},
				},
			},
		},
		{
			description: "different fallback char dispatches new event each frame",
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
			description: "dispatches only new keys when joining keys together",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA)},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
				{
					// Key A still held, no OS repeat
				},
				{
					// Key A still held, no OS repeat
				},
				{
					// A still held, B pressed in same frame
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyB)},
					expectedEvents: []term.Event{{Ch: 'b'}},
				},
				{
					// Both held, no OS repeat
				},
				{
					// A released, B still held
				},
				{
					// B still held, no OS repeat
				},
				{
					// B still held, C pressed
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyC)},
					expectedEvents: []term.Event{{Ch: 'c'}},
				},
				{
					// C held, no OS repeat
				},
				{
					// C held, no OS repeat
				},
			},
		},
		{
			description: "dispatches only new keys when joining keys with modifiers together",
			frames: []frame{
				{
					// Shift only → no event
					keyEvents: []ebiten.KeyEvent{press(ebiten.KeyShift)},
				},
				{
					// Shift still held
				},
				{
					// A pressed with shift
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModShift)},
					expectedEvents: []term.Event{{Ch: 'A'}},
				},
				{
					// A held, B pressed with shift
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyB, ebiten.KeyModShift)},
					expectedEvents: []term.Event{{Ch: 'B'}},
				},
				{
					// A released, B held with shift, no repeat
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
			description: "fast typed second char dispatches even when first key still held",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyS)},
					expectedEvents: []term.Event{{Ch: 's'}},
				},
				{
					// S still held, T pressed — both arrive as discrete events
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyT)},
					expectedEvents: []term.Event{{Ch: 't'}},
				},
				{
					// Both released
				},
			},
		},
		{
			description: "multiple keys in same frame all dispatch",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA), press(ebiten.KeyB)},
					expectedEvents: []term.Event{{Ch: 'a'}, {Ch: 'b'}},
				},
			},
		},
		{
			description: "OS repeat generates events on each frame",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA)},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
				{
					// No events: key held but OS hasn't sent repeat yet
				},
				{
					keyEvents:      []ebiten.KeyEvent{repeat(ebiten.KeyA)},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
				{
					keyEvents:      []ebiten.KeyEvent{repeat(ebiten.KeyA)},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
			},
		},
		{
			description: "release between presses does not produce terminal event",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA)},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
				{
					keyEvents: []ebiten.KeyEvent{release(ebiten.KeyA)},
				},
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA)},
					expectedEvents: []term.Event{{Ch: 'a'}},
				},
			},
		},
		{
			description: "key event suppresses fallback chars in same frame",
			frames: []frame{
				{
					keyEvents:      []ebiten.KeyEvent{press(ebiten.KeyA, ebiten.KeyModAlt)},
					chars:          []rune{'å'}, // macOS composed char ignored
					expectedEvents: []term.Event{{Mod: term.ModAlt, Ch: 'a'}},
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

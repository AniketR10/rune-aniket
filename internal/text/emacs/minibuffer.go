// Copyright (C) 2017-2026 The Rune Authors
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

package emacs

import "github.com/unstablebuild/rune-go-sdk/term"

// minibuffer is a small Emacs-style echo-area prompt. It reads a line of text
// from the user at the bottom of the editor while the handler routes every
// keystroke to it. It is deliberately minimal: no cursor movement, kill-ring
// or history. It backs go-to-line and the query-replace search/replacement
// prompts.
//
// The handler owns display: the minibuffer only exposes prompt() so the
// handler can render "<label><input>" through the less message bar and keeps
// the buffer cursor untouched, matching how Emacs leaves point in place while
// reading from the echo area.
type minibuffer struct {
	active bool
	label  string
	input  []rune
	// onSubmit fires when the user presses Enter; onCancel fires on C-g or
	// Esc. Exactly one runs per activation, after which the minibuffer is
	// inactive.
	onSubmit func(string)
	onCancel func()
}

func (m *minibuffer) start(label string, onSubmit func(string), onCancel func()) {
	m.active = true
	m.label = label
	m.input = m.input[:0]
	m.onSubmit = onSubmit
	m.onCancel = onCancel
}

func (m *minibuffer) reset() {
	m.active = false
	m.label = ""
	m.input = m.input[:0]
	m.onSubmit = nil
	m.onCancel = nil
}

// prompt returns the text to display in the echo area.
func (m *minibuffer) prompt() string {
	return m.label + string(m.input)
}

// handle consumes a key event while the minibuffer is active. It always
// reports handled=true so the key cannot leak to the buffer. submitted and
// canceled report which terminal transition (if any) occurred so the handler
// can run follow-up work after the callback.
func (m *minibuffer) handle(ev term.Event) (submitted, canceled bool) {
	if ev.Type != term.EventKey {
		return
	}
	if ev.Mod == term.ModCtrl && ev.Ch == 'g' {
		cb := m.onCancel
		m.reset()
		if cb != nil {
			cb()
		}
		return false, true
	}
	switch ev.Key {
	case term.KeyEsc:
		cb := m.onCancel
		m.reset()
		if cb != nil {
			cb()
		}
		return false, true
	case term.KeyEnter:
		text := string(m.input)
		cb := m.onSubmit
		m.reset()
		if cb != nil {
			cb(text)
		}
		return true, false
	case term.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return
	case term.KeySpace:
		m.input = append(m.input, ' ')
		return
	default:
		if ev.Mod == 0 && ev.Ch != 0 {
			m.input = append(m.input, ev.Ch)
		}
		return
	}
}

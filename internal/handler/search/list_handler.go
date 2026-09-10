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

package search

import (
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

// Handler wraps a List to satisfy tui.Handler.
// It handles enter key by calling fn with the element in focus,
// if there's an element in focus at all.
// It handles esc key by exiting and handles arrow keys up/down
// by scrolling up and down the list.
func Handler(l *List, ed tui.Handler, fn func(string)) tui.Handler {
	ret := simpleHandler{List: l, fn: fn, ed: ed}
	return ret
}

type simpleHandler struct {
	*List
	ed tui.Handler
	fn func(string)
}

func (s simpleHandler) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return
	}

	switch ev.Mod {
	case 0:
		switch ev.Key {
		case term.KeyEnter, term.KeyTab:
			s.Cancel()
			s.Wait()
			item, ok := s.Focus()
			if ok {
				handled = true
				exit = true
				s.fn(string(item.data))
			}
		case term.KeyEsc:
			handled = true
			exit = true
		case term.KeyArrowDown:
			handled = s.FocusDown()
		case term.KeyArrowUp:
			handled = s.FocusUp()
		default:
			_, handled = s.ed.Handle(ev)
		}
	case term.ModCtrl:
		switch ev.Ch {
		case 'c':
			s.Cancel()
			handled = true
		case 'j', 'n':
			handled = s.FocusDown()
		case 'k', 'p':
			handled = s.FocusUp()
		}
	}

	return
}

func (s simpleHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	c, style, ok := s.ed.Cursor()
	if s.cfg.bottomSearchBar {
		c.Y += (s.List.height - s.List.inputHeight())
	}
	return c, style, ok
}

func (s simpleHandler) Selection() (string, bool) {
	match, ok := s.Focus()
	if !ok {
		return "", false
	}
	return string(match.Data()), true
}

func (s simpleHandler) Resize(width, height int) {
	s.List.Resize(width, height)

	inputHeight := s.InputHeight()
	s.ed.Resize(width, inputHeight)
}

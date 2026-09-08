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

package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestTabsOnClick(t *testing.T) {
	t.Run("dispatches click event out of any tabs as -1 index", func(t *testing.T) {
		h := NewTabs()
		var called int
		h.OnClick = func(tabIdx int) bool {
			called++
			assert.Equal(t, -1, tabIdx)
			return true
		}

		_, handled := h.Handle(term.Event{
			Type:   term.EventMouse,
			Key:    term.MouseLeft,
			MouseX: 100000,
		})
		assert.True(t, handled)

		assert.Equal(t, 1, called)
	})

	t.Run("dispatches click event on tab with tab index", func(t *testing.T) {
		h := NewTabs()
		var called int
		h.OnClick = func(tabIdx int) bool {
			called++
			assert.Equal(t, 1, tabIdx)
			return true
		}

		assert.Equal(t, 0, h.Add(0, "Burning"))
		assert.Equal(t, 1, h.Add(0, "Man"))
		assert.Equal(t, 2, h.Add('x', "2024"))

		_, handled := h.Handle(term.Event{
			Type:   term.EventMouse,
			Key:    term.MouseLeft,
			MouseX: 10,
		})
		assert.True(t, handled)

		assert.Equal(t, 1, called)
	})

	t.Run("mouse drag dispatches OnClick only once", func(t *testing.T) {
		h := NewTabs()
		var called int
		h.OnClick = func(tabIdx int) bool {
			called++
			return true
		}

		_, handled := h.Handle(term.Event{Type: term.EventMouse, Key: term.MouseLeft})
		assert.True(t, handled)

		handled, _ = h.Handle(term.Event{Type: term.EventMouse, Key: term.MouseLeft})
		assert.False(t, handled)

		handled, _ = h.Handle(term.Event{Type: term.EventMouse, Key: term.MouseLeft})
		assert.False(t, handled)

		handled, _ = h.Handle(term.Event{Type: term.EventMouse, Key: term.MouseLeft})
		assert.False(t, handled)

		assert.Equal(t, 1, called)
	})

	t.Run("MouseLeft, MouseRelease, MouseLeft dispatches OnClick twice", func(t *testing.T) {
		h := NewTabs()
		var called int
		h.OnClick = func(tabIdx int) bool {
			called++
			return true
		}

		_, handled := h.Handle(term.Event{Type: term.EventMouse, Key: term.MouseLeft})
		assert.True(t, handled)

		_, handled = h.Handle(term.Event{Type: term.EventMouse, Key: term.MouseLeft})
		assert.False(t, handled)

		_, handled = h.Handle(term.Event{Type: term.EventMouse, Key: term.MouseRelease})
		assert.False(t, handled)

		_, handled = h.Handle(term.Event{Type: term.EventMouse, Key: term.MouseLeft})
		assert.True(t, handled)

		assert.Equal(t, 2, called)
	})
}

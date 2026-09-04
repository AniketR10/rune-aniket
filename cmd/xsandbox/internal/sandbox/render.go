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

package sandbox

import (
	"strings"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// defaultRenderWidth and defaultRenderHeight size the headless browser
// component at startup. The size is fixed so window installs resize
// their new child synchronously, flushing the install-response
// handshake back to the extension; render() can override it per call.
const (
	defaultRenderWidth  = 80
	defaultRenderHeight = 24
)

// render resizes the headless browser component to width x height and
// draws it into an in-memory cell grid, returning the grid as a plain
// string (one line per row, trailing blanks trimmed). It composites
// every installed handler exactly as a real terminal would.
func (h *browserHost) render(width, height int) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.width, h.height = width, height
	w := term.NewStringWriter(width, height)
	h.comp.Resize(width, height)
	h.comp.Draw(w)
	if cur, _, show := h.comp.Cursor(); show {
		w.SetCursor(cur)
	}
	_ = w.Flush()
	return trimGrid(w.String())
}

// sendKeys parses a handlertest-style key sequence and delivers each
// resulting key event to the component's focused handler, returning
// whether the last event was handled and whether it requested exit.
func (h *browserHost) sendKeys(sequence string) (handled, quit bool, err error) {
	keys, err := term.ParseKeys(sequence)
	if err != nil {
		return false, false, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, key := range keys {
		ev := term.Event{Ch: key.Ch, Mod: key.Mod, Key: key.Key, Type: term.EventKey}
		quit, handled = h.comp.Handle(ev)
	}
	return handled, quit, nil
}

// trimGrid removes trailing whitespace from each row and trailing
// blank rows so rendered output is easy to assert against.
func trimGrid(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

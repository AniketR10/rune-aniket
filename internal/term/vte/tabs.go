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

package vte

const initialTabstops int = 8

type tabstops struct {
	tabs []bool
}

func (t *tabstops) clearAll() {
	for i := range t.tabs {
		t.tabs[i] = false
	}
}

func (t *tabstops) resize(columns int) {
	if columns < len(t.tabs) {
		t.tabs = t.tabs[:columns]
		return
	}

	i := len(t.tabs)
	for i < columns {
		t.tabs = append(t.tabs, i%initialTabstops == 0)
		i++
	}
}

func (t *tabstops) get(i int) bool {
	return t.tabs[i]
}

func (t *tabstops) set(i int, value bool) {
	t.tabs[i] = value
}

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

// Package iter exercises qualified-type and selector resolution.
package iter

type Iterator interface {
	Next() (int, bool)
}

func Reduce(init int, _ Iterator) int { return init }

// OnlyDefined is exported but never referenced; resolvable only via
// the definitions phase.
func OnlyDefined() int { return 42 }

// MaxItems exercises that package-level constants are captured as
// local.definition.var by tree-sitter.
const MaxItems = 128

var DefaultIterator Iterator

func privateOnlyDefined() int { return 0 }

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

// Package iter is split across multiple files to exercise
// same-package, multi-file resolution.
package iter

// Aggregate is unreferenced outside its declaration; resolves only
// via the definitions phase.
func Aggregate(_ Iterator) int { return 0 }

func IsEmpty(_ Iterator) bool { return true }

// New is intentionally homonymous with mylib.New to exercise
// cross-package isolation.
func New() Iterator { return nil }

// Map exercises type-parameter syntax in a func declaration.
func Map[T any](_ Iterator, _ func(int) T) []T { return nil }

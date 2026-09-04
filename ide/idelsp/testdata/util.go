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

package main

// Multiply multiplies two integers.
func Multiply(a, b int) int {
	return a * b
}

// Max returns the maximum of two integers.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Counter tracks a count with a value receiver method.
type Counter struct {
	N int
}

// Count returns the current count (value receiver).
func (c Counter) Count() int {
	return c.N
}

// Increment increases the counter (pointer receiver).
func (c *Counter) Increment() {
	c.N++
}

// Reset sets the counter to zero (pointer receiver).
func (c *Counter) Reset() {
	c.N = 0
}

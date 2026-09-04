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

// Package greeter provides a deterministic local symbol surface for the
// Go REPL end-to-end completion tests.
package greeter

// Prefix is a greeting prefix used by the REPL completion tests.
const Prefix = "Hello, "

// Greet returns a greeting for name.
func Greet(name string) string {
	return Prefix + name + "!"
}

// Shout returns an emphatic greeting for name.
func Shout(name string) string {
	return Prefix + name + "!!!"
}

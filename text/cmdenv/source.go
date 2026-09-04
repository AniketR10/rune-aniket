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

// Package cmdenv resolves the named variables that Rune expands inside
// command alias bodies and dispatched argv at dispatch time.
package cmdenv

import (
	"os"
)

// Source resolves variable names to values for command-time expansion.
// Returning ok=false indicates the variable is unknown to this source;
// callers chain to other lookups (typically os.Getenv). A nil Source
// is equivalent to one that returns ok=false for every name.
type Source func(name string) (value string, ok bool)

// Lookup returns the function expected by mvdan.cc/sh/v3/shell
// (Fields / Expand) that resolves variables through src first and
// falls back to os.Getenv. A nil src behaves like os.Getenv alone.
func Lookup(src Source) func(string) string {
	if src == nil {
		return os.Getenv
	}
	return func(name string) string {
		if v, ok := src(name); ok {
			return v
		}
		return os.Getenv(name)
	}
}

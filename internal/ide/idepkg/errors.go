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

package idepkg

import "errors"

var (
	// ErrVersionInUse is returned when trying to perform an operation on a package
	// version that's in use.
	ErrVersionInUse = errors.New("package version is in use")

	// ErrNotInstalled is returned when a version of a package is not installed.
	ErrNotInstalled = errors.New("package version is not installed")
)

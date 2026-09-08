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

package shader

import (
	"github.com/unstablebuild/rune-go-sdk/term"
)

// Shader abstracts the ability to apply shader-like effects
// to other components.
type Shader interface {
	// Shade applies the shader's transformation
	// to the given component turning it into an animated component,
	// or returns false if this Shader's animation is done.
	Shade(frame, total int, in [][]term.Cell)
}

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

package glslshader

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwizzling(t *testing.T) {
	t.Run("swizzling", func(t *testing.T) {
		v := vec4(1.0, 2.0, 3.0, 4.0)
		assert.Equal(t, v.wzyx(), vec4(4.0, 3.0, 2.0, 1.0))
		assert.Equal(t, v.yy(), vec2(2.0, 2.0))
	})
}

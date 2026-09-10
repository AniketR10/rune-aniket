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

package vtescreen

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsScreenContext(t *testing.T) {
	t.Run("returns false if context is not screen", func(t *testing.T) {
		assert.False(t, IsScreenContext(context.Background()))
	})
	t.Run("returns true if context is screen", func(t *testing.T) {
		ctx := NewContext(context.Background())
		assert.True(t, IsScreenContext(ctx))
	})
	t.Run("returns true if context double set screen", func(t *testing.T) {
		ctx := NewContext(NewContext(context.Background()))
		assert.True(t, IsScreenContext(ctx))
	})
	t.Run("returns true if is derivative", func(t *testing.T) {
		ctx, cancel := context.WithCancel(NewContext(context.Background()))
		cancel()
		assert.True(t, IsScreenContext(ctx))
	})
}

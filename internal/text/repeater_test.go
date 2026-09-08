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

package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/rune/internal/cell"
	"unstable.build/rune/internal/component"
)

func TestRepeater(t *testing.T) {
	buf := cell.NewBuffer()
	scroll := component.NewScroll(buf)
	scroll.Resize(10, 10)
	cursor := NewCursor(scroll, nil)
	cursor.RightInclusiveSemantics = true
	repeater := NewRepeater(cursor, buf)

	cursor.InsertString("helloworld")
	assert.True(t, repeater.Repeat())
	assert.Equal(t, "helloworldhelloworld", buf.String())
	repeater.Clear()

	cursor.Insert('X')
	cursor.MoveUp()
	cursor.MoveStartLine()
	assert.True(t, repeater.Repeat())
	assert.Equal(t, "XhelloworldhelloworldX", buf.String())
	assert.True(t, repeater.Repeat())
	assert.Equal(t, "XXhelloworldhelloworldX", buf.String())
	repeater.Clear()

	// all inserts are accumulated into the repeater
	cursor.InsertString("00")
	cursor.InsertString("11")
	cursor.InsertString("22")
	assert.True(t, repeater.Repeat())
	assert.Equal(t, "XX001122001122helloworldhelloworldX", buf.String())
	repeater.Clear()

	cursor.MoveStartLine()
	cursor.Select()
	cursor.MoveRight()
	cursor.DeleteSelection()
	assert.True(t, repeater.Repeat())
	assert.Equal(t, "1122001122helloworldhelloworldX", buf.String())
	assert.True(t, cursor.MoveEndLine())
	cursor.cursor.X++
	require.False(t, repeater.Repeat())
	// nop because end is out of bounds
	assert.Equal(t, "1122001122helloworldhelloworldX", buf.String())

	cursor.MoveLeft()
	cursor.MoveLeft()
	cursor.MoveLeft()
	require.True(t, repeater.Repeat())
	assert.Equal(t, "1122001122helloworldhelloworl", buf.String())
}

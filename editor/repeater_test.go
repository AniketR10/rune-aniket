package editor

import (
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepeater(t *testing.T) {
	buf := cell.NewBuffer()
	scroll := component.NewScroll()
	scroll.InitWithBuffer(buf)
	scroll.Resize(10, 10)
	cursor := NewCursor(scroll)
	repeater := NewRepeater(cursor, buf)

	cursor.InsertString("helloworld")
	assert.True(t, repeater.Repeat())
	assert.Equal(t, "helloworldhelloworld", buf.String())

	cursor.Insert('X')
	cursor.MoveUp()
	cursor.MoveStartLine()
	assert.True(t, repeater.Repeat())
	assert.Equal(t, "XhelloworldhelloworldX", buf.String())

	cursor.MoveStartLine()
	cursor.Select()
	cursor.MoveRight()
	cursor.DeleteSelection()
	assert.True(t, repeater.Repeat())
	assert.Equal(t, "loworldhelloworldX", buf.String())
	cursor.MoveEndLine()
	cursor.MoveRight()
	assert.False(t, repeater.Repeat())
	// nop because end is out of bounds
	assert.Equal(t, "loworldhelloworldX", buf.String())

	cursor.MoveLeft()
	cursor.MoveLeft()
	require.True(t, repeater.Repeat())
	assert.Equal(t, "loworldhelloworl", buf.String())
}

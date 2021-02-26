package editor

import (
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteClipboard(t *testing.T) {
	buf := cell.NewBuffer()
	clip := NewEphemeralClipboard()
	var scroll component.Scroll
	scroll.InitWithBuffer(buf)
	c := NewCursor(&scroll)
	WithCopyDelete(clip, c, buf)

	content := "my whatever"
	buf.InsertString(term.Coordinates{}, content)
	c.SelectLine()
	c.DeleteSelection()

	data, err := clip.Get()
	require.NoError(t, err)
	assert.Equal(t, Paste{Data: content, Metadata: LineSelection}, data)

	// test that undo inserts do not get copied to clipboard
	buf.Undo()
	buf.Undo()

	data, err = clip.Get()
	require.NoError(t, err)
	assert.Equal(t, Paste{Data: content, Metadata: LineSelection}, data)
}

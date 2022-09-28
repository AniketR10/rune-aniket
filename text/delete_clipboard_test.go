package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

func TestDeleteClipboard(t *testing.T) {
	buf := cell.NewBuffer()
	clip := NewInMemoryClipboard()
	var scroll component.Scroll
	scroll.Init(buf)
	c := NewCursor(&scroll)
	WithCopyDelete("", clip, c, buf)

	content := "my whatever"
	buf.InsertString(term.Coordinates{}, content)
	c.SelectLine()
	c.DeleteSelection()

	data, err := clip.Paste("")
	require.NoError(t, err)
	assert.Equal(t, ClipboardData{Text: content, Metadata: LineSelection}, data)

	// test that undo inserts do not get copied to clipboard
	buf.Undo()
	buf.Undo()

	data, err = clip.Paste("")
	require.NoError(t, err)
	assert.Equal(t, ClipboardData{Text: content, Metadata: LineSelection}, data)
}

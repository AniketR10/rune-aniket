package editor

import (
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteClipboard(t *testing.T) {
	buf := cell.NewBuffer()
	clip := NewEphemeralClipboard()
	WithCopyDelete(clip, buf)

	content := "my whatever"
	buf.InsertString(term.Coordinates{}, content)
	buf.DeleteRow(0)

	data, err := clip.Get()
	require.NoError(t, err)
	assert.Equal(t, Paste{Data: content}, data)
}

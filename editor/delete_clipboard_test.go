package editor

import (
	"testing"

	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/term"
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

	str, err := clip.Get()
	require.NoError(t, err)
	assert.Equal(t, str, content)
}

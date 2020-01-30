package editor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGetSet(t *testing.T, clip Clipboard, str string) {
	assert.NoError(t, clip.Set(str))

	actual, err := clip.Get()
	require.NoError(t, err)
	assert.Equal(t, str, actual)
}

func testClipboard(t *testing.T, clip Clipboard) {
	t.Run("Sets a small value to the clipboard", func(t *testing.T) {
		str := "test1234"
		testGetSet(t, clip, str)
	})

	t.Run("Sets a value to the clipboard with newlines, tabs and carriage returns", func(t *testing.T) {
		str := "a\nb\nc\nd\t\n\r\n"
		testGetSet(t, clip, str)
	})

	t.Run("Sets a large value to the clipboard", func(t *testing.T) {
		str := []rune{}
		for i := 0; i < 10000; i++ {
			str = append(str, '\x00')
			str = append(str, 'a')
			str = append(str, '\n')
		}
		testGetSet(t, clip, string(str))
	})

	t.Run("once value is set, it can be retrieved multiple times", func(t *testing.T) {
		str := "test1234"
		testGetSet(t, clip, str)

		for i := 0; i < 100; i++ {
			actual, err := clip.Get()
			require.NoError(t, err)
			assert.Equal(t, str, actual)
		}
	})
}

func TestEphemeralClipboard(t *testing.T) {
	c := NewEphemeralClipboard()
	testClipboard(t, c)
}

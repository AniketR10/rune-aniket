package editor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGetSet(t *testing.T, clip Clipboard, data Paste) {
	assert.NoError(t, clip.Set(data))

	actual, err := clip.Get()
	require.NoError(t, err)
	assert.Equal(t, data, actual)
}

func testClipboard(t *testing.T, clip Clipboard) {
	t.Run("Sets a small value to the clipboard", func(t *testing.T) {
		str := "test1234"
		testGetSet(t, clip, Paste{Data: str})
	})

	t.Run("Sets a value with metadata to the clipboard", func(t *testing.T) {
		str := "test1234"
		testGetSet(t, clip, Paste{Data: str, Metadata: 1234})
	})

	t.Run("Sets a value to the clipboard with newlines, tabs and carriage returns", func(t *testing.T) {
		str := "a\nb\nc\nd\t\n\r\n"
		testGetSet(t, clip, Paste{Data: str})
	})

	t.Run("Sets a large value to the clipboard", func(t *testing.T) {
		str := []rune{}
		for i := 0; i < 10000; i++ {
			str = append(str, '\x00')
			str = append(str, 'a')
			str = append(str, '\n')
		}
		testGetSet(t, clip, Paste{Data: string(str)})
	})

	t.Run("once value is set, it can be retrieved multiple times", func(t *testing.T) {
		str := "test1234"
		testGetSet(t, clip, Paste{Data: str})

		for i := 0; i < 100; i++ {
			actual, err := clip.Get()
			require.NoError(t, err)
			assert.Equal(t, Paste{Data: str}, actual)
		}
	})
}

func TestEphemeralClipboard(t *testing.T) {
	c := NewEphemeralClipboard()
	testClipboard(t, c)
}

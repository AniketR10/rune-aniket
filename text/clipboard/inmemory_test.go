package clipboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const registerID = "DancingWithWolves"

func testGetCopy(t *testing.T, clip Register, data Data) {
	assert.NoError(t, clip.Copy(registerID, data))

	actual, err := clip.Paste(registerID)
	require.NoError(t, err)
	assert.Equal(t, data, actual)
}

func testRegister(t *testing.T, clip Register) {
	t.Run("Sets a small value to the clipboard", func(t *testing.T) {
		str := "test1234"
		testGetCopy(t, clip, Data{Text: str})
	})

	t.Run("Sets a value with metadata to the clipboard", func(t *testing.T) {
		str := "test1234"
		testGetCopy(t, clip, Data{Text: str, Metadata: 1234})
	})

	t.Run("Sets a value to the clipboard with newlines, tabs and carriage returns", func(t *testing.T) {
		str := "a\nb\nc\nd\t\n\r\n"
		testGetCopy(t, clip, Data{Text: str})
	})

	t.Run("Sets a large value to the clipboard", func(t *testing.T) {
		str := []rune{}
		for i := 0; i < 10000; i++ {
			str = append(str, '\x00')
			str = append(str, 'a')
			str = append(str, '\n')
		}
		testGetCopy(t, clip, Data{Text: string(str)})
	})

	t.Run("once value is set, it can be retrieved multiple times", func(t *testing.T) {
		str := "test1234"
		testGetCopy(t, clip, Data{Text: str})

		for i := 0; i < 100; i++ {
			actual, err := clip.Paste(registerID)
			require.NoError(t, err)
			assert.Equal(t, Data{Text: str}, actual)
		}
	})
}

func TestEphemeralRegister(t *testing.T) {
	c := NewInMemory()
	testRegister(t, c)
}

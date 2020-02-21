package cell

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFlush(t *testing.T) {
	width, height := 5, 6
	writer := NewBufferWriter(width, height)

	c := 'E'
	for i := width - 1; i >= 0; i-- {
		for j := height - 1; j >= 0; j-- {
			if i > j-1 {
				writer.SetCell(term.Coordinates{X: j, Y: i}, term.Cell{Ch: c})
			}
		}
		c--
	}

	// should be fine to wtry to write out of bounds
	writer.SetCell(term.Coordinates{X: width + 1, Y: height + 1}, term.Cell{Ch: '='})

	require.NoError(t, writer.Flush())

	expected := "A\nBB\nCCC\nDDDD\nEEEEE"
	assert.Equal(t, expected, writer.String())
}

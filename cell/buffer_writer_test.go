package cell

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/term"
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

	expected := "A\nBB\nCCC\nDDDD\nEEEEE\n"
	assert.Equal(t, expected, CellsToString(writer.RawCells()))
}

func benchBufferWriter(b *testing.B, n int) {
	width, height := n, n
	for i := 0; i < b.N; i++ {
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
	}
}

func BenchmarkBufferWriter10(b *testing.B) {
	benchBufferWriter(b, 10)
}
func BenchmarkBufferWriter100(b *testing.B) {
	benchBufferWriter(b, 100)
}
func BenchmarkBufferWriter1000(b *testing.B) {
	benchBufferWriter(b, 1000)
}

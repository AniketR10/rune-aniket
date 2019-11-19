package cell

import (
	"strings"
	"testing"

	"github.com/ernestrc/fractal/term"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUninitializedNotPanic(t *testing.T) {

	t.Run("GetAttr", func(t *testing.T) {
		var b Buffer
		b.GetAttr(term.Coordinates{X: 10, Y: 0})
	})

	t.Run("ConflateRow", func(t *testing.T) {
		var b Buffer
		_ = b.ConflateRow(10)
	})

	t.Run("InsertRowAt", func(t *testing.T) {
		var b Buffer
		b.InsertRowAt(134)
	})

	t.Run("InsertAt", func(t *testing.T) {
		var b Buffer
		_ = b.InsertAt(term.Coordinates{X: 10, Y: 10}, 'f')
	})

	t.Run("RawCells", func(t *testing.T) {
		var b Buffer
		assert.NotNil(t, b.RawCells())
	})

	t.Run("ResetAttr", func(t *testing.T) {
		var b Buffer
		b.ResetAttr()
	})

	t.Run("Columns", func(t *testing.T) {
		var b Buffer
		_, _ = b.Columns(10)
	})

	t.Run("Rows", func(t *testing.T) {
		var b Buffer
		_ = b.Rows()
	})

	t.Run("SetAttr", func(t *testing.T) {
		var b Buffer
		b.SetAttr(term.Coordinates{X: 1, Y: 10}, term.Attributes{})
	})

	t.Run("String", func(t *testing.T) {
		var b Buffer
		_ = b.String()
	})

	t.Run("DeleteCell", func(t *testing.T) {
		var b Buffer
		b.DeleteCell(term.Coordinates{X: 10, Y: 10})
	})

	t.Run("TruncateFrom", func(t *testing.T) {
		var b Buffer
		b.TruncateFrom(term.Coordinates{X: 0, Y: 1})
	})

	t.Run("DeleteRow", func(t *testing.T) {
		var b Buffer
		_ = b.DeleteRow(10)
	})

	t.Run("TruncateRowFrom", func(t *testing.T) {
		var b Buffer
		b.TruncateRowFrom(term.Coordinates{X: 10, Y: 0})
	})

	t.Run("WriteRune", func(t *testing.T) {
		var b Buffer
		_ = b.WriteRune('h')
	})

	t.Run("WriteString", func(t *testing.T) {
		var b Buffer
		_ = b.WriteString("hfjlkw")
	})

	t.Run("ReadFrom", func(t *testing.T) {
		var b Buffer
		_, _ = b.ReadFrom(strings.NewReader("hfjlkw"))
	})
}

const str = "hello\n\tworld\n"

func assertBufferContent(t *testing.T, buf *Buffer) {
	assert.Equal(t, 3, buf.Rows())

	cols, ok := buf.Columns(0)
	assert.Equal(t, 5, cols)
	assert.True(t, ok)

	cols, ok = buf.Columns(1)
	assert.Equal(t, 9, cols)
	assert.True(t, ok)

	assert.Equal(t, str, buf.String())
}

func TestBufferWriteString(t *testing.T) {
	var buf Buffer
	buf.WriteString(str)
	assertBufferContent(t, &buf)
}

func TestBufferReadFrom(t *testing.T) {
	var buf Buffer
	n, err := buf.ReadFrom(strings.NewReader(str))
	require.NoError(t, err)
	assert.Equal(t, len(str), int(n))
	assertBufferContent(t, &buf)
}

func TestBufferWriteRune(t *testing.T) {
	var buf Buffer

	for _, c := range str {
		buf.WriteRune(c)
	}

	assertBufferContent(t, &buf)
}

func TestBufferInsertAt(t *testing.T) {
	var buf Buffer
	var next term.Coordinates

	next = buf.InsertAt(next, 'h')
	next = buf.InsertAt(next, 'e')
	next = buf.InsertAt(next, 'l')
	next = buf.InsertAt(next, 'l')
	next = buf.InsertAt(next, 'o')
	next = buf.InsertAt(next, '\n')
	next = buf.InsertAt(next, 'w')
	next = buf.InsertAt(next, 'o')
	next = buf.InsertAt(next, 'r')
	next = buf.InsertAt(next, 'l')
	buf.InsertAt(next, 'd')
	buf.InsertAt(term.Coordinates{X: 4, Y: 0}, '\n')

	str := "hell\no\nworld"
	assert.Equal(t, 3, buf.Rows())

	cols, ok := buf.Columns(0)
	assert.True(t, ok)
	assert.Equal(t, 4, cols)

	cols, ok = buf.Columns(1)
	assert.True(t, ok)
	assert.Equal(t, 1, cols)

	cols, ok = buf.Columns(2)
	assert.True(t, ok)
	assert.Equal(t, 5, cols)

	assert.Equal(t, str, buf.String())
}

func TestBufferDeleteRow(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.DeleteRow(0)
	assert.Equal(t, "world", buf.String())

	buf.DeleteRow(0)
	assert.Equal(t, "", buf.String())
}

func TestBufferTruncateRowFrom1(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateRowFrom(term.Coordinates{X: 2, Y: 0})

	assert.Equal(t, "he\nworld", buf.String())
}

func TestBufferTruncateRowFrom2(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateRowFrom(term.Coordinates{X: 2, Y: 1})

	assert.Equal(t, "hello\nwo", buf.String())
}

func TestBufferTruncateFrom1(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateFrom(term.Coordinates{X: 4, Y: 0})

	assert.Equal(t, "hell", buf.String())
}

func TestBufferTruncateFrom2(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateFrom(term.Coordinates{X: 0, Y: 1})

	assert.Equal(t, "hello\n", buf.String())
}

func TestBufferTruncateCellAt(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.DeleteCell(term.Coordinates{X: 0, Y: 1})
	buf.DeleteCell(term.Coordinates{X: 1, Y: 1})
	buf.ConflateRow(0)
	buf.DeleteCell(term.Coordinates{X: 7, Y: 0})

	assert.Equal(t, "hellool", buf.String())
}

func TestBufferDeleteCellAtTab(t *testing.T) {
	const tabspaces = 4
	var buf Buffer
	buf.Init(tabspaces)

	str := "!\t\t!\t"
	buf.WriteString(str)

	start := buf.DeleteCell(term.Coordinates{X: 3, Y: 0})
	assert.Equal(t, term.Coordinates{X: 1}, start)
	assert.Equal(t, "!\t!\t", buf.String())

	start = buf.DeleteCell(term.Coordinates{X: 2, Y: 0})
	assert.Equal(t, term.Coordinates{X: 1}, start)
	assert.Equal(t, "!!\t", buf.String())

	start = buf.DeleteCell(term.Coordinates{X: 2, Y: 0})
	assert.Equal(t, term.Coordinates{X: 2}, start)
	assert.Equal(t, "!!", buf.String())
}

type selectCase struct {
	from     term.Coordinates
	to       term.Coordinates
	expected [][]term.Cell
}

func toString(cells [][]term.Cell) string {
	runes := make([]rune, 0)
	for _, r := range cells {
		for _, c := range r {
			runes = append(runes, c.Ch)
		}
	}
	return string(runes)
}

func assertCellProperties(t *testing.T, cell term.Cell, attr term.Attributes) {
	assert.Equal(t, attr.Fg, cell.Fg)
	assert.Equal(t, attr.Bg, cell.Bg)
}

func TestSetAttr(t *testing.T) {
	var buf Buffer
	str := "0123"
	buf.WriteString(str)

	t.Run("sets attribute to cell at position if exists", func(t *testing.T) {
		pos := term.Coordinates{X: 0, Y: 0}
		attr := term.Attributes{Fg: term.AttrBold, Bg: term.AttrReverse}
		assert.True(t, buf.SetAttr(pos, attr))
		c := buf.cells.cells[pos.Y][pos.X]
		assertCellProperties(t, c, attr)
	})
	t.Run("returns ok=false if cell at position does not exist", func(t *testing.T) {
		coords := []term.Coordinates{
			term.Coordinates{X: 10, Y: 10},
			term.Coordinates{X: 0, Y: 10},
			term.Coordinates{X: 4, Y: 0},
		}
		for _, pos := range coords {
			attr := term.Attributes{Fg: term.AttrBold, Bg: term.AttrReverse}
			assert.False(t, buf.SetAttr(pos, attr))
		}
	})
}

var benchmarkFortune = `
				Love in your heart wasn't put there to stay.
				Love isn't love 'til you give it away.
				-- Oscar Hammerstein 中国
`

func newBenchmarkBuffer(fortunes int) (*Buffer, string) {
	buffer := Buffer{}
	payload := ""
	for i := 0; i < fortunes; i++ {
		payload = payload + benchmarkFortune
	}
	return &buffer, payload
}

func benchmarkBufferWrite(b *testing.B, fortunes int) {
	buffer, payload := newBenchmarkBuffer(fortunes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer.Reset()
		_ = buffer.WriteString(payload)
	}
}

func BenchmarkBufferWrite10(b *testing.B) {
	benchmarkBufferWrite(b, 10)
}
func BenchmarkBufferWrite100(b *testing.B) {
	benchmarkBufferWrite(b, 100)
}
func BenchmarkBufferWrite1000(b *testing.B) {
	benchmarkBufferWrite(b, 1000)
}
func BenchmarkBufferWrite10000(b *testing.B) {
	benchmarkBufferWrite(b, 10000)
}

// func BenchmarkBufferWrite100MB(b *testing.B) {
// 	benchmarkBufferWrite(b, 1000000)
// }

func benchmarkBufferReadFrom(b *testing.B, fortunes int) {
	buffer, payload := newBenchmarkBuffer(fortunes)
	reader := strings.NewReader(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer.Reset()
		reader.Reset(payload)
		_, _ = buffer.ReadFrom(reader)
	}
}

func BenchmarkBufferReadFrom10(b *testing.B) {
	benchmarkBufferReadFrom(b, 10)
}
func BenchmarkBufferReadFrom100(b *testing.B) {
	benchmarkBufferReadFrom(b, 100)
}
func BenchmarkBufferReadFrom1000(b *testing.B) {
	benchmarkBufferReadFrom(b, 1000)
}
func BenchmarkBufferReadFrom10000(b *testing.B) {
	benchmarkBufferReadFrom(b, 10000)
}

// func BenchmarkBufferReadFrom100MB(b *testing.B) {
// 	benchmarkBufferReadFrom(b, 1000000)
// }

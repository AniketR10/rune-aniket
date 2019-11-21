package cell

import (
	"strings"
	"testing"

	"github.com/ernestrc/fractal/term"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const str = "hello\n\tworld\n"
const longStr = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it away.
		-- Oscar Hammerstein 中国`

func assertBufferContent(t *testing.T, buf *Buffer) {
	assert.Equal(t, 3, buf.Rows())

	cols := buf.Columns(0)
	assert.Equal(t, 5, cols)

	cols = buf.Columns(1)
	assert.Equal(t, 9, cols)

	assert.Equal(t, str, buf.String())
}

func TestBufferWriteString(t *testing.T) {
	buf := NewBuffer()
	buf.WriteString(str)
	assertBufferContent(t, buf)
}

func TestBufferReadFrom(t *testing.T) {
	buf := NewBuffer()
	n, err := buf.ReadFrom(strings.NewReader(str))
	require.NoError(t, err)
	assert.Equal(t, len(str), int(n))
	assertBufferContent(t, buf)
}

func TestBufferWriteRune(t *testing.T) {
	buf := NewBuffer()

	for _, c := range str {
		buf.WriteRune(c)
	}

	assertBufferContent(t, buf)
}

func TestBufferInsertAt(t *testing.T) {
	buf := NewBuffer()
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

	cols := buf.Columns(0)
	assert.Equal(t, 4, cols)

	cols = buf.Columns(1)
	assert.Equal(t, 1, cols)

	cols = buf.Columns(2)
	assert.Equal(t, 5, cols)

	assert.Equal(t, str, buf.String())
}

func TestBufferDeleteRow(t *testing.T) {
	buf := NewBuffer()
	str := "hello\nworld"
	buf.WriteString(str)

	buf.DeleteRow(0)
	assert.Equal(t, "world", buf.String())

	buf.DeleteRow(0)
	assert.Equal(t, "", buf.String())
}

func TestBufferTruncateRowFrom1(t *testing.T) {
	buf := NewBuffer()
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateRowFrom(term.Coordinates{X: 2, Y: 0})

	assert.Equal(t, "he\nworld", buf.String())
}

func TestBufferTruncateRowFrom2(t *testing.T) {
	buf := NewBuffer()
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateRowFrom(term.Coordinates{X: 2, Y: 1})

	assert.Equal(t, "hello\nwo", buf.String())
}

func TestBufferTruncateCellAt(t *testing.T) {
	buf := NewBuffer()
	str := "hello\nworld"
	buf.WriteString(str)

	buf.DeleteCell(term.Coordinates{X: 0, Y: 1})
	buf.DeleteCell(term.Coordinates{X: 1, Y: 1})
	buf.ConflateRow(0)
	buf.DeleteCell(term.Coordinates{X: 7, Y: 0})

	assert.Equal(t, "hellool", buf.String())
}

func TestBufferDeleteCellAtTab(t *testing.T) {
	buf := NewBuffer()

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
	buf := NewBuffer()
	str := "0123"
	buf.WriteString(str)

	t.Run("sets attribute to cell at position if exists", func(t *testing.T) {
		pos := term.Coordinates{X: 0, Y: 0}
		attr := term.Attributes{Fg: term.AttrBold, Bg: term.AttrReverse}
		assert.True(t, buf.SetAttr(pos, attr))
		_, c := buf.Cell(pos)
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

func TestBufferTruncateFrom(t *testing.T) {

	tsuite := []struct {
		contents string
		input    term.Coordinates
		ok       bool
		expected string
	}{
		{"hello\nworld", term.Coordinates{X: 4, Y: 0}, true, "hell"},
		{"hello\nworld", term.Coordinates{X: 0, Y: 1}, true, "hello\n"},
		{longStr, term.Coordinates{X: 6, Y: 0}, true, "Love i"},
	}

	for _, tcase := range tsuite {
		buf := NewBuffer()
		buf.WriteString(tcase.contents)
		ok := buf.TruncateFrom(tcase.input)
		if tcase.ok {
			assert.True(t, ok)
			assert.Equal(t, tcase.expected, buf.String())
		} else {
			assert.False(t, ok)
		}
	}
}

var benchmarkFortune = `
				Love in your heart wasn't put there to stay.
				Love isn't love 'til you give it away.
				-- Oscar Hammerstein 中国
`

func newBenchmarkBuffer(fortunes int) (*Buffer, string) {
	buffer := NewBuffer()
	payload := ""
	for i := 0; i < fortunes; i++ {
		payload = payload + benchmarkFortune
	}
	return buffer, payload
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

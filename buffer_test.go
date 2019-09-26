package fractal

import (
	"strings"
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/stretchr/testify/assert"
)

func TestUninitializedNotPanic(t *testing.T) {

	t.Run("GetCellAt", func(t *testing.T) {
		var b Buffer
		b.GetCellAt(Coordinates{X: 10, Y: 0})
	})

	t.Run("GetAttr", func(t *testing.T) {
		var b Buffer
		b.GetAttr(Coordinates{X: 10, Y: 0})
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
		_ = b.InsertAt(Coordinates{X: 10, Y: 10}, 'f')
	})

	t.Run("RawCells", func(t *testing.T) {
		var b Buffer
		assert.NotNil(t, b.RawCells())
	})

	t.Run("ResetAttr", func(t *testing.T) {
		var b Buffer
		b.ResetAttr()
	})

	t.Run("RowLastIdx", func(t *testing.T) {
		var b Buffer
		_, _ = b.RowLastIdx(10)
	})

	t.Run("RowLen", func(t *testing.T) {
		var b Buffer
		_, _ = b.RowLen(10)
	})

	t.Run("Rows", func(t *testing.T) {
		var b Buffer
		_ = b.Rows()
	})

	t.Run("SetAttr", func(t *testing.T) {
		var b Buffer
		b.SetAttr(Coordinates{X: 1, Y: 10}, 0, 0)
	})

	t.Run("String", func(t *testing.T) {
		var b Buffer
		_ = b.String()
	})

	t.Run("TruncateCellAt", func(t *testing.T) {
		var b Buffer
		_, _ = b.TruncateCellAt(Coordinates{X: 10, Y: 10})
	})

	t.Run("TruncateFrom", func(t *testing.T) {
		var b Buffer
		b.TruncateFrom(Coordinates{X: 0, Y: 1})
	})

	t.Run("TruncateRowAt", func(t *testing.T) {
		var b Buffer
		_ = b.TruncateRowAt(10)
	})

	t.Run("TruncateRowFrom", func(t *testing.T) {
		var b Buffer
		b.TruncateRowFrom(Coordinates{X: 10, Y: 0})
	})

	t.Run("WriteRune", func(t *testing.T) {
		var b Buffer
		_ = b.WriteRune('h')
	})

	t.Run("WriteAt", func(t *testing.T) {
		var b Buffer
		b.WriteAt(Coordinates{X: 10, Y: 0}, 'h')
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

func assertBufferContent(t *testing.T, buf *Buffer, str string) {
	if l := buf.Rows(); l != 3 {
		t.Errorf("Lines() is not correct: %d", l)
	}

	if l, _ := buf.RowLen(0); l != 5 {
		t.Errorf("RowLen(0) is not correct: %d", l)
	}

	if l, _ := buf.RowLen(1); l != 9 {
		t.Errorf("RowLen(1) is not correct: %d", l)
	}

	if s := buf.String(); s != str {
		t.Errorf("expected '%+v' found '%+v'", []byte(str), []byte(s))
	}
}

func TestBufferWriteString(t *testing.T) {
	var buf Buffer
	str := "hello\n\tworld\n"
	buf.tabspaces = 4
	buf.WriteString(str)
	assertBufferContent(t, &buf, str)
}

func TestBufferReadFrom(t *testing.T) {
	var buf Buffer
	str := "hello\n\tworld\n"
	buf.tabspaces = 4
	n, err := buf.ReadFrom(strings.NewReader(str))
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(str)) {
		t.Errorf("expected n to be %d but found %d", len(str), n)
	}
	assertBufferContent(t, &buf, str)
}

func TestBufferInsertAt(t *testing.T) {
	var buf Buffer
	var next Coordinates

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
	buf.InsertAt(Coordinates{X: 4, Y: 0}, '\n')

	str := "hell\no\nworld"
	if l := buf.Rows(); l != 3 {
		t.Errorf("Rows() is not correct: %d", l)
	}

	if l, _ := buf.RowLen(0); l != 4 {
		t.Errorf("RowLen(0) is not correct: %+v", buf.cells[0])
	}

	if l, _ := buf.RowLen(1); l != 1 {
		t.Errorf("RowLen(1) is not correct: %+v", buf.cells[1])
	}

	if l, _ := buf.RowLen(2); l != 5 {
		t.Errorf("RowLen(2) is not correct: %+v", buf.cells[2])
	}

	if s := buf.String(); s != str {
		t.Errorf("expected '%q' found '%q'", str, s)
	}
}

func TestBufferWriteAt(t *testing.T) {
	var buf Buffer
	buf.WriteString("hello")

	buf.WriteAt(Coordinates{X: 5, Y: 0}, 'w')
	// overwrite
	buf.WriteAt(Coordinates{X: 5, Y: 0}, '\n')
	buf.WriteAt(Coordinates{X: 0, Y: 1}, 'w')
	buf.WriteAt(Coordinates{X: 1, Y: 1}, 'o')
	buf.WriteAt(Coordinates{X: 2, Y: 1}, 'r')
	buf.WriteAt(Coordinates{X: 3, Y: 1}, 'l')
	buf.WriteAt(Coordinates{X: 4, Y: 1}, 'd')

	if expected, l := 2, buf.Rows(); l != expected {
		t.Errorf("Rows() is not correct: is %d, should be %d", l, expected)
	}

	if l, _ := buf.RowLen(0); l != 5 {
		t.Errorf("RowLen(0) is not correct: is %d, should be 5", l)
	}

	if l, _ := buf.RowLen(1); l != 5 {
		t.Errorf("RowLen(1) is not correct: is %d, should be 5", l)
	}

	str := "hello\nworld"
	if s := buf.String(); s != str {
		t.Errorf("expected '%+v' found '%+v'", []byte(str), []byte(s))
	}
}

func TestBufferTruncateRowAt(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateRowAt(0)

	if s, expected := buf.String(), "world"; s != expected {
		t.Errorf("expected '%q' found '%q'", expected, s)
	}
}

func TestBufferTruncateRowFrom1(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateRowFrom(Coordinates{X: 2, Y: 0})

	if s, expected := buf.String(), "he\nworld"; s != expected {
		t.Errorf("expected '%q' found '%q'", expected, s)
	}
}

func TestBufferTruncateRowFrom2(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateRowFrom(Coordinates{X: 2, Y: 1})

	if s, expected := buf.String(), "hello\nwo"; s != expected {
		t.Errorf("expected '%q' found '%q'", expected, s)
	}
}

func TestBufferTruncateFrom1(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateFrom(Coordinates{X: 4, Y: 0})

	if s := buf.String(); s != "hell" {
		t.Errorf("expected '%+v' found '%+v'", []byte("hell"), []byte(s))
	}
}

func TestBufferTruncateFrom2(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateFrom(Coordinates{X: 0, Y: 1})

	if s, expected := buf.String(), "hello\n"; s != expected {
		t.Errorf("expected '%q' found '%q'", expected, s)
	}
}

func TestBufferTruncateCellAt(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateCellAt(Coordinates{X: 0, Y: 1})
	buf.TruncateCellAt(Coordinates{X: 1, Y: 1})
	buf.ConflateRow(0)
	buf.TruncateCellAt(Coordinates{X: 7, Y: 0})

	if s, expected := buf.String(), "hellool"; s != expected {
		t.Errorf("expected '%q' found '%q'", expected, s)
	}
}

func assertCellCh(t *testing.T, cell termbox.Cell, r rune) {
	if cell.Ch != r {
		t.Errorf("expected cell content to be %c but was %c", r, cell.Ch)
	}
}

func TestBufferTruncateCellAtTab(t *testing.T) {
	const tabspaces = 4
	var buf Buffer
	buf.Init(tabspaces)

	str := "!\t\t!\t"
	buf.WriteString(str)

	cell, n := buf.TruncateCellAt(Coordinates{X: tabspaces - 1, Y: 0})
	if n != tabspaces {
		t.Errorf("TruncateCellAt returned %d instead of %d", n, tabspaces)
	}
	assertCellCh(t, cell, '\t')
	cell, n = buf.TruncateCellAt(Coordinates{X: 2, Y: 0})
	if n != tabspaces {
		t.Errorf("TruncateCellAt returned %d instead of %d", n, tabspaces)
	}
	assertCellCh(t, cell, '\t')
	if s, expected := buf.String(), "!!\t"; s != expected {
		t.Errorf("expected '%q' found '%q'", expected, s)
	}
}

func TestBufferWrite(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"

	for _, c := range str {
		buf.WriteRune(c)
	}

	if l := buf.Rows(); l != 2 {
		t.Errorf("Rows() is not correct: is %d, should be 2", l)
	}

	if l, _ := buf.RowLen(0); l != 5 {
		t.Errorf("RowLen(0) is not correct: is %d, should be 5", l)
	}

	if l, _ := buf.RowLen(1); l != 5 {
		t.Errorf("RowLen(1) is not correct: is %d, should be 5", l)
	}

	if s := buf.String(); s != str {
		t.Errorf("expected '%q' found '%q'", str, s)
	}
}

type selectCase struct {
	from     Coordinates
	to       Coordinates
	expected [][]termbox.Cell
}

func toString(cells [][]termbox.Cell) string {
	runes := make([]rune, 0)
	for _, r := range cells {
		for _, c := range r {
			runes = append(runes, c.Ch)
		}
	}
	return string(runes)
}

func assertCellProperties(t *testing.T, cell termbox.Cell, fg, bg termbox.Attribute) {
	if cell.Fg != fg {
		t.Errorf("cell FG was not %d", fg)
	}
	if cell.Bg != bg {
		t.Errorf("cell BG was not %d", fg)
	}
}

func TestGetCellAt(t *testing.T) {
	t.Run("handles tabs and tab padding", func(t *testing.T) {
		var buf Buffer
		buf.WriteString("\t\tlol")

		for i := buf.tabspaces; i < buf.tabspaces+buf.tabspaces; i++ {
			pos, cell, ok := buf.GetCellAt(Coordinates{X: i, Y: 0})
			if !ok {
				t.Error("GetCellAt returned unexpected ok")
			}
			if cell.Ch != '\t' {
				t.Errorf("expected tab but found '%c'", cell.Ch)
			}
			expected := Coordinates{X: buf.tabspaces * 2, Y: 0}
			if pos != expected {
				t.Errorf("did not return actual tab position: %+v", pos)
			}
		}
	})
	t.Run("returns false if position is outside of bounds", func(t *testing.T) {
		var buf Buffer
		buf.WriteString("\n\t")
		for _, pos := range []Coordinates{Coordinates{X: 1}, Coordinates{Y: 1, X: 4}} {
			_, _, ok := buf.GetCellAt(pos)
			if ok {
				t.Error("GetCellAt returned unexpected ok")
			}
		}
	})
}

func TestSetAttr(t *testing.T) {
	var buf Buffer
	str := "0123"
	buf.WriteString(str)

	t.Run("sets attribute to cell at position if exists", func(t *testing.T) {
		pos := Coordinates{X: 0, Y: 0}
		if !buf.SetAttr(pos, termbox.AttrBold, termbox.AttrReverse) {
			t.Error("expected SetAttr to return true")
		}
		_, c, _ := buf.GetCellAt(pos)
		assertCellProperties(t, c, termbox.AttrBold, termbox.AttrReverse)
	})
	t.Run("returns ok=false if cell at position does not exist", func(t *testing.T) {
		for _, pos := range []Coordinates{Coordinates{X: 10, Y: 10}, Coordinates{X: 0, Y: 10}, Coordinates{X: 4, Y: 0}} {
			if buf.SetAttr(pos, termbox.AttrBold, termbox.AttrReverse) {
				t.Error("expected SetAttr to return false")
			}
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

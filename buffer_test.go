package fractal

import (
	"reflect"
	"strings"
	"github.com/nsf/termbox-go"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUninitializedNotPanic(t *testing.T) {

	t.Run("ConflateRow", func(t *testing.T) {
		var b Buffer
		_ = b.ConflateRow(10)
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
		_, _, _ = b.TruncateCellAt(Coordinates{X: 10, Y: 10})
	})

	t.Run("TruncateFrom", func(t *testing.T) {
		var b Buffer
		b.TruncateFrom(Coordinates{X: 0, Y: 1})
	})

	t.Run("TruncateLastRow", func(t *testing.T) {
		var b Buffer
		b.TruncateLastRow()
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

	t.Run("Select", func(t *testing.T) {
		var b Buffer
		_ = b.Select(Coordinates{X: 0, Y: 0}, Coordinates{X: 1, Y: 1})
	})

	t.Run("SelectLine", func(t *testing.T) {
		var b Buffer
		_ = b.SelectLine(Coordinates{X: 0, Y: 0}, Coordinates{X: 1, Y: 1})
	})

	t.Run("SelectBlock", func(t *testing.T) {
		var b Buffer
		_ = b.SelectBlock(Coordinates{X: 0, Y: 0}, Coordinates{X: 1, Y: 1})
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

func TestBufferTruncateLastRow(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteString(str)

	buf.TruncateLastRow()

	if l := buf.Rows(); l != 1 {
		t.Errorf("Rows() is not correct: is %d, should be 1", l)
	}

	if s, expected := buf.String(), "hello"; s != expected {
		t.Errorf("expected '%q' found '%q'", expected, s)
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
		t.Errorf("found '%q'", s)
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

func TestBufferSelect(t *testing.T) {
	var buf Buffer
	str := "hello\n\tworld\n\nitsme"
	buf.WriteString(str)
	buf.tabspaces = 4

	testCases := []selectCase{
		{
			from: Coordinates{},
			to:   Coordinates{X: 1, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}},
			},
		},
		{
			from: Coordinates{X: 0, Y: 1},
			to:   Coordinates{X: 8, Y: 1},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
			},
		},
		{
			from: Coordinates{X: 2, Y: 0},
			to:   Coordinates{X: 4, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
			},
		},
		{
			from: Coordinates{X: 2, Y: 0},
			to:   Coordinates{X: 7, Y: 1},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}},
			},
		},
		{
			from: Coordinates{X: 7, Y: 1},
			to:   Coordinates{X: 2, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}},
			},
		},
		{
			from: Coordinates{X: 4, Y: 0},
			to:   Coordinates{X: 4, Y: 3},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				[]termbox.Cell{},
				[]termbox.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: Coordinates{X: 0, Y: 2},
			to:   Coordinates{X: 4, Y: 3},
			expected: [][]termbox.Cell{
				[]termbox.Cell{},
				[]termbox.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: Coordinates{X: 0, Y: 2},
			to:   Coordinates{X: 5, Y: 3},
			expected: [][]termbox.Cell{
				[]termbox.Cell{},
				[]termbox.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: Coordinates{X: 0, Y: 2},
			to:   Coordinates{X: 4, Y: 4},
			expected: [][]termbox.Cell{
				[]termbox.Cell{},
				[]termbox.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
	}

	for _, tcase := range testCases {
		selection := buf.Select(tcase.from, tcase.to)
		if !reflect.DeepEqual(selection, tcase.expected) {
			t.Errorf("expected %q found %q", toString(tcase.expected), toString(selection))
		}
	}
}

func TestBufferSelectLine(t *testing.T) {
	var buf Buffer
	str := "hello\n\tworld\n\nitsme"
	buf.WriteString(str)
	buf.tabspaces = 4

	testCases := []selectCase{
		{
			from: Coordinates{},
			to:   Coordinates{X: 1, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
			},
		},
		{
			from: Coordinates{X: 2, Y: 0},
			to:   Coordinates{X: 4, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
			},
		},
		{
			from: Coordinates{X: 2, Y: 0},
			to:   Coordinates{X: 7, Y: 1},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
			},
		},
		{
			from: Coordinates{X: 0, Y: 2},
			to:   Coordinates{X: 2, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				[]termbox.Cell{},
			},
		},
		{
			to:   Coordinates{X: 2, Y: 0},
			from: Coordinates{X: 10, Y: 2},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				[]termbox.Cell{},
			},
		},
		{
			to:   Coordinates{X: 2, Y: 0},
			from: Coordinates{X: 0, Y: 10},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				[]termbox.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
	}

	for _, tcase := range testCases {
		selection := buf.SelectLine(tcase.from, tcase.to)
		if toString(selection) != toString(tcase.expected) {
			t.Errorf("expected %q found %q", toString(tcase.expected), toString(selection))
		}
	}
}

func TestBufferSelectBlock(t *testing.T) {
	var buf Buffer
	str := "hello\n\tworld\n\nitsme"
	buf.WriteString(str)
	buf.tabspaces = 4

	testCases := []selectCase{
		{
			from: Coordinates{},
			to:   Coordinates{X: 1, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}},
			},
		},
		{
			from: Coordinates{X: 0, Y: 0},
			to:   Coordinates{X: 3, Y: 1},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}},
			},
		},
		{
			from: Coordinates{X: 3, Y: 1},
			to:   Coordinates{X: 0, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}},
			},
		},
		{
			from: Coordinates{X: 3, Y: 3},
			to:   Coordinates{X: 0, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}},
				[]termbox.Cell{},
				[]termbox.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}},
			},
		},
	}

	for _, tcase := range testCases {
		selection := buf.SelectBlock(tcase.from, tcase.to)
		if !reflect.DeepEqual(selection, tcase.expected) {
			t.Errorf("expected %q found %q", toString(tcase.expected), toString(selection))
		}
	}
}

var bufferFortune = `
				Love in your heart wasn't put there to stay.
				Love isn't love 'til you give it away.
				-- Oscar Hammerstein 中国
`

func newBenchmarkBuffer(fortunes int) (*Buffer, string) {
	buffer := Buffer{}
	payload := ""
	for i := 0; i < fortunes; i++ {
		payload = payload + bufferFortune
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

package fractal

import (
	"reflect"
	"termbox"
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

	t.Run("Write", func(t *testing.T) {
		var b Buffer
		_ = b.Write('h')
	})

	t.Run("WriteAt", func(t *testing.T) {
		var b Buffer
		b.WriteAt(Coordinates{X: 10, Y: 0}, 'h')
	})

	t.Run("WriteStr", func(t *testing.T) {
		var b Buffer
		_ = b.WriteStr("hfjlkw")
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

func TestBufferWriteStr(t *testing.T) {
	var buf Buffer
	str := "hello\n\tworld"
	buf.WriteStr(str)
	buf.tabspaces = 4

	if l := buf.Rows(); l != 2 {
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
	buf.WriteStr("hello")

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
	buf.WriteStr(str)

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
	buf.WriteStr(str)

	buf.TruncateRowAt(0)

	if s, expected := buf.String(), "world"; s != expected {
		t.Errorf("expected '%q' found '%q'", expected, s)
	}
}

func TestBufferTruncateRowFrom1(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.TruncateRowFrom(Coordinates{X: 2, Y: 0})

	if s, expected := buf.String(), "he\nworld"; s != expected {
		t.Errorf("expected '%q' found '%q'", expected, s)
	}
}

func TestBufferTruncateRowFrom2(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.TruncateRowFrom(Coordinates{X: 2, Y: 1})

	if s, expected := buf.String(), "hello\nwo"; s != expected {
		t.Errorf("expected '%q' found '%q'", expected, s)
	}
}

func TestBufferTruncateFrom1(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.TruncateFrom(Coordinates{X: 4, Y: 0})

	if s := buf.String(); s != "hell" {
		t.Errorf("expected '%+v' found '%+v'", []byte("hell"), []byte(s))
	}
}

func TestBufferTruncateFrom2(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.TruncateFrom(Coordinates{X: 0, Y: 1})

	if s, expected := buf.String(), "hello\n"; s != expected {
		t.Errorf("expected '%q' found '%q'", expected, s)
	}
}

func TestBufferTruncateCellAt(t *testing.T) {
	var buf Buffer
	str := "hello\nworld"
	buf.WriteStr(str)

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
		buf.Write(c)
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
	buf.WriteStr(str)
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
	buf.WriteStr(str)
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
	buf.WriteStr(str)
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

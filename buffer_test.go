package fractal

import (
	"testing"
)

func TestBufferWrite(t *testing.T) {
	var buf CellBuf
	str := "hello\nworld"
	buf.WriteStr(str)

	if buf.Len() != len(str) {
		t.Errorf("length is not correct: is %d, should be %d", buf.Len(), len(str))
	}

	if l := buf.Rows(); l != 2 {
		t.Errorf("Lines() is not correct: is %d, should be 2", l)
	}

	if l := buf.RowLen(0); l != 6 {
		t.Errorf("RowLen(0) is not correct: is %d, should be 6", l)
	}

	if l := buf.RowLen(1); l != 5 {
		t.Errorf("RowLen(1) is not correct: is %d, should be 5", l)
	}

	if s := buf.String(); s != str {
		t.Errorf("expected '%+v' found '%+v'", []byte(str), []byte(s))
	}
}

// TODO test InsertAt with \n modifying the rest of cells coordinates

func TestBufferWriteAt(t *testing.T) {
	var buf CellBuf
	str := "hello\nworld"
	buf.WriteStr("hello")

	buf.WriteAt(Coordinates{X: 5, Y: 0}, 'w')
	// overwrite
	buf.WriteAt(Coordinates{X: 5, Y: 0}, '\n')
	buf.WriteAt(Coordinates{X: 0, Y: 1}, 'w')
	buf.WriteAt(Coordinates{X: 1, Y: 1}, 'o')
	buf.WriteAt(Coordinates{X: 2, Y: 1}, 'r')
	buf.WriteAt(Coordinates{X: 3, Y: 1}, 'l')
	buf.WriteAt(Coordinates{X: 4, Y: 1}, 'd')

	if buf.Len() != len(str) {
		t.Errorf("length is not correct: is %d, should be %d", buf.Len(), len(str))
	}

	if l := buf.Rows(); l != 2 {
		t.Errorf("Rows() is not correct: is %d, should be 2", l)
	}

	if l := buf.RowLen(0); l != 6 {
		t.Errorf("RowLen(0) is not correct: is %d, should be 6", l)
	}

	if l := buf.RowLen(1); l != 5 {
		t.Errorf("RowLen(1) is not correct: is %d, should be 5", l)
	}

	if s := buf.String(); s != str {
		t.Errorf("expected '%+v' found '%+v'", []byte(str), []byte(s))
	}
}

func TestBufferTruncate1(t *testing.T) {
	var buf CellBuf
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.Truncate(7)

	if s := buf.String(); s != "hell" {
		t.Errorf("expected '%+v' found '%+v'", []byte("hell"), []byte(s))
	}
}

func TestBufferTruncate2(t *testing.T) {
	var buf CellBuf
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.Truncate(6)

	if s := buf.String(); s != "hello" {
		t.Errorf("expected '%+v' found '%+v'", []byte("hello"), []byte(s))
	}
}

func TestBufferTruncateLastRow(t *testing.T) {
	var buf CellBuf
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.TruncateLastRow()

	if l := buf.Rows(); l != 1 {
		t.Errorf("Rows() is not correct: is %d, should be 1", l)
	}

	if s := buf.String(); s != "hello" {
		t.Errorf("expected '%+v' found '%+v'", []byte("hello"), []byte(s))
	}
}

func TestBufferTruncateRowAt(t *testing.T) {
	var buf CellBuf
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.TruncateRowAt(Coordinates{X: 0, Y: 0})

	if s := buf.String(); s != "world" {
		t.Errorf("expected '%+v' found '%+v'", []byte("world"), []byte(s))
	}
}

func TestBufferTruncateRowFrom1(t *testing.T) {
	var buf CellBuf
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.TruncateRowFrom(Coordinates{X: 2, Y: 0})

	if s := buf.String(); s != "he\nworld" {
		t.Errorf("expected '%+v' found '%+v'", []byte("he\nworld"), []byte(s))
	}
}

func TestBufferTruncateRowFrom2(t *testing.T) {
	var buf CellBuf
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.TruncateRowFrom(Coordinates{X: 2, Y: 1})

	if s := buf.String(); s != "hello\nwo" {
		t.Errorf("expected '%+v' found '%+v'", []byte("hello\nwo"), []byte(s))
	}
}

func TestBufferTruncateFrom1(t *testing.T) {
	var buf CellBuf
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.TruncateFrom(Coordinates{X: 4, Y: 0})

	if s := buf.String(); s != "hell" {
		t.Errorf("expected '%+v' found '%+v'", []byte("hell"), []byte(s))
	}
}

func TestBufferTruncateFrom2(t *testing.T) {
	var buf CellBuf
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.TruncateFrom(Coordinates{X: 0, Y: 1})

	if s := buf.String(); s != "hello\n" {
		t.Errorf("expected '%+v' found '%+v'", []byte("hello\n"), []byte(s))
	}
}

func TestBufferTruncateCellAt(t *testing.T) {
	var buf CellBuf
	str := "hello\nworld"
	buf.WriteStr(str)

	buf.TruncateCellAt(Coordinates{X: 0, Y: 1})
	buf.TruncateCellAt(Coordinates{X: 1, Y: 1})
	buf.TruncateCellAt(Coordinates{X: 5, Y: 0})
	buf.TruncateCellAt(Coordinates{X: 7, Y: 0})

	if s := buf.String(); s != "hellool" {
		t.Errorf("expected '%+v' found '%+v'", []byte("hellool"), []byte(s))
		t.Errorf("found '%s'", s)
	}
}

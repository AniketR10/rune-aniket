package cell

import (
	"io"
	"log"
	"reflect"

	"github.com/ernestrc/fractal/term"
)

type logger struct {
	out   *log.Logger
	r     Reader
	rType string
	w     Writer
	wType string
}

// WithLogger returns a ReadWriter that logs all operations on r/w to out.
func WithLogger(r Reader, w Writer, out io.Writer) ReadWriter {
	return &logger{
		out:   log.New(out, "", log.LstdFlags),
		r:     r,
		rType: reflect.TypeOf(r).Elem().String(),
		w:     w,
		wType: reflect.TypeOf(w).Elem().String(),
	}
}

func (l *logger) NextWrite() (next term.Coordinates) {
	next = l.w.NextWrite()
	l.out.Printf("(%p: %s).NextWrite(): next=%+v\n",
		l.w, l.wType, next)
	return
}

func (l *logger) Insert(at term.Coordinates, str string) (
	from, to term.Coordinates,
) {
	from, to = l.w.Insert(at, str)
	l.out.Printf("(%p: %s).Insert(at=%+v): from=%+v, to=%+v\n",
		l.w, l.wType, at, from, to)
	return
}

func (l *logger) Delete(from, to term.Coordinates) (
	start, end term.Coordinates, str string,
) {
	start, end, str = l.w.Delete(from, to)
	l.out.Printf("(%p: %s).Delete(from=%+v, to=%+v): start=%+v, end=%+v, str=%s\n",
		l.w, l.wType, from, to, start, end, str)
	return
}

func (l *logger) Reset() {
	l.w.Reset()
	l.out.Printf("(%p: %s).Reset()\n",
		l.w, l.wType)
}

func (l *logger) Rows() (rows int) {
	rows = l.r.Rows()
	l.out.Printf("(%p: %s).Rows(): rows=%d\n",
		l.r, l.rType, rows)
	return
}

func (l *logger) Columns(row int) (cols int) {
	cols = l.r.Columns(row)
	l.out.Printf("(%p: %s).Columns(row=%d): cols=%d\n",
		l.r, l.rType, row, cols)
	return
}

func (l *logger) Cell(p term.Coordinates) (
	actual term.Coordinates, c term.Cell,
) {
	actual, c = l.r.Cell(p)
	l.out.Printf("(%p: %s).Cell(p=%+v): actual=%+v, c=%+v\n",
		l.r, l.rType, p, actual, c)
	return
}

func (l *logger) RawCells() (cells [][]term.Cell) {
	cells = l.r.RawCells()
	l.out.Printf("(%p: %s).RawCells(): (rows=%d)\n",
		l.r, l.rType, len(cells))
	return
}

func (l *logger) String() (str string) {
	str = l.r.String()
	l.out.Printf("(%p: %s).String(): str=%.10s (len=%d)\n",
		l.r, l.rType, str, len(str))
	return
}

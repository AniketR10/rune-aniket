package cell

import (
	"io"
	"log"
	"reflect"

	"github.com/ernestrc/fractal/term"
)

type logger struct {
	out   *log.Logger
	r     reader
	rType string
	w     writer
	wType string
}

func newLogger(r reader, w writer, out io.Writer) (reader, writer) {
	lg := &logger{
		out:   log.New(out, "", log.LstdFlags),
		r:     r,
		rType: reflect.TypeOf(r).Elem().String(),
		w:     w,
		wType: reflect.TypeOf(w).Elem().String(),
	}
	return lg, lg
}

func (l *logger) insert(at term.Coordinates, str string) (
	from, to term.Coordinates,
) {
	from, to = l.w.insert(at, str)
	l.out.Printf("(%p: %s).insert(at=%+v): from=%+v, to=%+v\n",
		l.w, l.wType, at, from, to)
	return
}

func (l *logger) delete(from, to term.Coordinates) (
	start, end term.Coordinates, str string,
) {
	start, end, str = l.w.delete(from, to)
	l.out.Printf("(%p: %s).delete(from=%+v, to=%+v): start=%+v, end=%+v, str=%s\n",
		l.w, l.wType, from, to, start, end, str)
	return
}

func (l *logger) reset() {
	l.w.reset()
	l.out.Printf("(%p: %s).Reset()\n",
		l.w, l.wType)
}

func (l *logger) rows() (rows int) {
	rows = l.r.rows()
	l.out.Printf("(%p: %s).rows(): rows=%d\n",
		l.r, l.rType, rows)
	return
}

func (l *logger) columns(row int) (cols int) {
	cols = l.r.columns(row)
	l.out.Printf("(%p: %s).columns(row=%d): cols=%d\n",
		l.r, l.rType, row, cols)
	return
}

func (l *logger) cell(p term.Coordinates) (
	c term.Cell, ok bool,
) {
	c, ok = l.r.cell(p)
	l.out.Printf("(%p: %s).cell(p=%+v): c=%+v, ok=%+v\n",
		l.r, l.rType, p, c, ok)
	return
}

func (l *logger) rawCells() (cells [][]term.Cell) {
	cells = l.r.rawCells()
	l.out.Printf("(%p: %s).rawCells(): (rows=%d)\n",
		l.r, l.rType, len(cells))
	return
}

func (l *logger) String() (str string) {
	str = l.r.String()
	l.out.Printf("(%p: %s).String(): str=%.10s (len=%d)\n",
		l.r, l.rType, str, len(str))
	return
}

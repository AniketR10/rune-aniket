package cell

import (
	"fmt"
	"reflect"

	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/fractal/term"
)

type logger struct {
	out   *log.Logger
	r     reader
	rType string
	w     writer
	wType string
}

func newLogger(r reader, w writer, out *log.Logger) (reader, writer) {
	lg := &logger{
		out:   out,
		r:     r,
		rType: reflect.TypeOf(r).Elem().String(),
		w:     w,
		wType: reflect.TypeOf(w).Elem().String(),
	}
	return lg, lg
}

func (l *logger) rFields(method string) log.Fields {
	return log.Fields{
		"address": fmt.Sprintf("%p", l.r),
		"type":    l.rType,
		"method":  method,
	}
}

func (l *logger) wFields(method string) log.Fields {
	return log.Fields{
		"address": fmt.Sprintf("%p", l.w),
		"type":    l.wType,
		"method":  method,
	}
}

func (l *logger) insert(at term.Coordinates, str string) (
	from, to term.Coordinates,
) {
	fields := l.wFields("insert")
	fields["at"] = at
	fields["string"] = fmt.Sprintf("%.10s", str)
	fields["length"] = len(str)

	from, to = l.w.insert(at, str)

	fields["from"] = from
	fields["to"] = to

	l.out.WithFields(fields).Trace()
	return
}

func (l *logger) delete(from, to term.Coordinates) (
	start, end term.Coordinates, str string,
) {
	fields := l.wFields("delete")
	fields["from"] = from
	fields["to"] = to

	start, end, str = l.w.delete(from, to)

	fields["string"] = fmt.Sprintf("%.10s", str)
	fields["length"] = len(str)
	fields["start"] = start
	fields["end"] = end

	l.out.WithFields(fields).Trace()

	return
}

func (l *logger) reset() {
	fields := l.wFields("reset")

	l.w.reset()

	l.out.WithFields(fields).Trace()
}

func (l *logger) rows() (rows int) {
	fields := l.rFields("rows")

	rows = l.r.rows()
	fields["rows"] = rows

	l.out.WithFields(fields).Trace()

	return
}

func (l *logger) columns(row int) (cols int) {
	fields := l.rFields("columns")
	fields["row"] = row

	cols = l.r.columns(row)

	fields["cols"] = cols

	l.out.WithFields(fields).Trace()

	return
}

func (l *logger) cell(p term.Coordinates) (
	c term.Cell, ok bool,
) {
	fields := l.rFields("cell")
	fields["position"] = p

	c, ok = l.r.cell(p)

	fields["cell"] = c
	fields["ok"] = ok

	l.out.WithFields(fields).Trace()

	return
}

func (l *logger) rawCells() (cells [][]term.Cell) {
	fields := l.rFields("rawCells")

	cells = l.r.rawCells()

	fields["rows"] = len(cells)

	l.out.WithFields(fields).Trace()

	return
}

func (l *logger) String() (str string) {
	fields := l.rFields("String")

	str = l.r.String()

	fields["string"] = fmt.Sprintf("%.10s", str)
	fields["length"] = len(str)

	l.out.WithFields(fields).Trace()

	return
}

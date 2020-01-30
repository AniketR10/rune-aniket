package cell

import (
	"fmt"
	"reflect"

	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/fractal/term"
)

type logger struct {
	out   *log.Logger
	r     Reader
	rType string
	w     Writer
	wType string
}

func newLogger(r Reader, w Writer, out *log.Logger) *logger {
	lg := &logger{
		out:   out,
		r:     r,
		rType: reflect.TypeOf(r).Elem().String(),
		w:     w,
		wType: reflect.TypeOf(w).Elem().String(),
	}
	return lg
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

func (l *logger) Insert(at term.Coordinates, str string) (
	from, to term.Coordinates,
) {
	fields := l.wFields("insert")
	fields["at"] = at
	fields["string"] = fmt.Sprintf("%.10s", str)
	fields["length"] = len(str)

	from, to = l.w.Insert(at, str)

	fields["from"] = from
	fields["to"] = to

	l.out.WithFields(fields).Trace()
	return
}

func (l *logger) Delete(from, to term.Coordinates) (
	start, end term.Coordinates, str string,
) {
	fields := l.wFields("delete")
	fields["from"] = from
	fields["to"] = to

	start, end, str = l.w.Delete(from, to)

	fields["string"] = fmt.Sprintf("%.10s", str)
	fields["length"] = len(str)
	fields["start"] = start
	fields["end"] = end

	l.out.WithFields(fields).Trace()

	return
}

func (l *logger) Rows() (rows int) {
	fields := l.rFields("rows")

	rows = l.r.Rows()
	fields["rows"] = rows

	l.out.WithFields(fields).Trace()

	return
}

func (l *logger) Columns(row int) (cols int) {
	fields := l.rFields("columns")
	fields["row"] = row

	cols = l.r.Columns(row)

	fields["cols"] = cols

	l.out.WithFields(fields).Trace()

	return
}

func (l *logger) Cell(p term.Coordinates) (
	c term.Cell, ok bool,
) {
	fields := l.rFields("cell")
	fields["position"] = p

	c, ok = l.r.Cell(p)

	fields["cell"] = c
	fields["ok"] = ok

	l.out.WithFields(fields).Trace()

	return
}

func (l *logger) RawCells() (cells [][]term.Cell) {
	fields := l.rFields("rawCells")

	cells = l.r.RawCells()

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

package cell

import (
	"context"
	"fmt"
	"reflect"

	log "github.com/sirupsen/logrus"

	"unstable.build/go-tui/term"
)

type logger struct {
	out   *log.Logger
	r     View
	rType string
	w     Editor
	wType string
}

func newLogger(r View, w Editor, out *log.Logger) *logger {
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

func (l *logger) Edit(ctx context.Context, start, end term.Coordinates, str string) (
	from, to term.Coordinates, old string,
) {
	fields := l.wFields("update")
	fields["start"] = start
	fields["end"] = end
	fields["string"] = fmt.Sprintf("%s", str)
	fields["length"] = len(str)

	from, to, old = l.w.Edit(ctx, start, end, str)

	fields["from"] = from
	fields["to"] = to
	fields["old"] = old

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

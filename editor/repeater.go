package editor

import (
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

// Repeater is a helper structure to enable repeating the last
// insert or delete in a buffer. See Repeat for more details.
type Repeater struct {
	buf    *cell.Buffer
	cursor *Cursor

	i         bool
	insertStr string

	d          bool
	repeating  bool
	deleteFrom term.Coordinates
	deleteTo   term.Coordinates
}

// NewRepeater allocates storage for a Repeater and initializes it.
func NewRepeater(cursor *Cursor, buf *cell.Buffer) *Repeater {
	r := new(Repeater)
	r.Init(cursor, buf)
	return r
}

// Init initializes this Repeater with buf by subscribing it to it.
func (r *Repeater) Init(cursor *Cursor, buf *cell.Buffer) {
	r.buf = buf
	r.cursor = cursor
	buf.SubscribeUsage(r)
}

// Repeat repeats the last call to the buffer's Writer Insert or Delete.
func (r *Repeater) Repeat() (ok bool) {
	if r.i {
		r.cursor.InsertString(r.insertStr)
		ok = true
		return
	}
	if r.d {
		cursor := r.cursor.CursorAtScroll()
		from, to := cell.SortFromTo(r.deleteFrom, r.deleteTo)
		diff := cell.CoordinatesDiff(to, from)

		from = cursor
		to = cell.CoordinatesSum(from, diff)

		// avoid consuming OnWillDelete
		r.repeating = true
		_, _, str := r.buf.Delete(from, to)
		r.repeating = false

		ok = str != ""
		return
	}
	return
}

// OnWillInsert satisfies cell.Subscriber.
func (r *Repeater) OnWillInsert(at term.Coordinates, str string) {
	r.d = false
	r.i = true
	r.insertStr = str
}

// OnDidInsert satisfies cell.Subscriber.
func (r *Repeater) OnDidInsert(from, to term.Coordinates) {
}

// OnWillDelete satisfies cell.Subscriber.
func (r *Repeater) OnWillDelete(from, to term.Coordinates) {
	if r.repeating {
		return
	}
	r.i = false
	r.d = true
	r.deleteFrom = from
	r.deleteTo = to
}

// OnDidDelete satisfies cell.Subscriber.
func (r *Repeater) OnDidDelete(start, end term.Coordinates, str string) {
}

package cell

import (
	"github.com/ernestrc/fractal/term"
)

// Undoer adds Undo and Redo methods to a otherwise, irreversible cell.Writer.
// It satifies the cell.Writer interface and it should be used as a replacement.
type Undoer struct {
	w            Writer
	undoTimeline []op
	redoTimeline []op
}

type op struct {
	do   func()
	undo func()
}

// NewUndoer returns an instance of Undoer which provides undo and redo operations
// for the given Writer.
func NewUndoer(w Writer) *Undoer {
	return &Undoer{
		w:            w,
		undoTimeline: make([]op, 0),
		redoTimeline: make([]op, 0),
	}
}

func popLastOp(timeline []op) ([]op, op, bool) {
	lastCmd := len(timeline) - 1
	if lastCmd < 0 {
		return nil, op{}, false
	}
	op := timeline[lastCmd]
	return timeline[:lastCmd], op, true
}

// Redo reverses the previously reversed update to the underlying buffer.
func (u *Undoer) Redo() bool {
	redoTimeline, op, ok := popLastOp(u.redoTimeline)
	if !ok {
		return ok
	}
	u.redoTimeline = redoTimeline
	op.do()
	u.pushUndo(op)
	return ok
}

// TODO Redo/Undo should return start of update

// Undo reverses the last update to the underlying buffer.
// Redo can be used to reverse the undo.
func (u *Undoer) Undo() bool {
	undoTimeline, op, ok := popLastOp(u.undoTimeline)
	if !ok {
		return ok
	}
	u.undoTimeline = undoTimeline
	op.undo()
	u.pushRedo(op)
	return ok
}

var i int

func (u *Undoer) pushUndo(cmd op) {
	u.undoTimeline = append(u.undoTimeline, cmd)
}
func (u *Undoer) pushRedo(cmd op) {
	u.redoTimeline = append(u.redoTimeline, cmd)
}

func (u *Undoer) resetRedoTimeline() {
	u.redoTimeline = u.redoTimeline[:0]
}

// Insert captures underlying writer Insert so it can be undone. See cell.Writer.Insert
func (u *Undoer) Insert(at term.Coordinates, str string) (from, to term.Coordinates) {
	op := op{
		do: func() {
			from, to = u.w.Insert(at, str)
		},
		undo: func() {
			u.w.Delete(from, to)
		},
	}

	op.do()
	u.pushUndo(op)
	u.resetRedoTimeline()
	return
}

// Delete captures underlying writer Delete so it can be undone. See cell.Writer.Delete
func (u *Undoer) Delete(from, to term.Coordinates) (start, end term.Coordinates, str string) {
	op := op{
		do: func() {
			start, end, str = u.w.Delete(from, to)
		},
		undo: func() {
			u.w.Insert(start, str)
		},
	}

	op.do()
	u.pushUndo(op)
	u.resetRedoTimeline()
	return
}

// Reset resets the undo/redo timelines and the underlying cell.Writer.
func (u *Undoer) Reset() {
	u.resetRedoTimeline()
	u.undoTimeline = u.undoTimeline[:0]
	u.w.Reset()
}

// NextWrite delegates call to underlying cell.Writer.
func (u *Undoer) NextWrite() term.Coordinates {
	return u.w.NextWrite()
}

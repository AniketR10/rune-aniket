package cell

import (
	"github.com/ernestrc/fractal/term"
)

// TODO redo/undo should return start of update

// undoer adds undo and redo methods to a otherwise, irreversible cell.writer.
// It satifies the cell.writer interface and it should be used as a replacement.
type undoer struct {
	w            Writer
	undoTimeline []op
	redoTimeline []op
}

type op struct {
	at   term.Coordinates
	do   func()
	undo func()
}

// Newundoer returns new instance of undoer to undo/redo operations of w.
func newUndoer(w Writer) *undoer {
	u := new(undoer)
	u.init(w)
	return u
}

// Init initializes this undoer to undo/redo operations of w.
func (u *undoer) init(w Writer) {
	u.w = w
	u.undoTimeline = make([]op, 0)
	u.redoTimeline = make([]op, 0)
}

func popLastOp(timeline []op) ([]op, op, bool) {
	lastCmd := len(timeline) - 1
	if lastCmd < 0 {
		return nil, op{}, false
	}
	op := timeline[lastCmd]
	return timeline[:lastCmd], op, true
}

func (u *undoer) redo() (bool, term.Coordinates) {
	redoTimeline, op, ok := popLastOp(u.redoTimeline)
	if !ok {
		return false, term.Coordinates{}
	}
	u.redoTimeline = redoTimeline
	op.do()
	u.pushUndo(op)
	return ok, op.at
}

func (u *undoer) undo() (bool, term.Coordinates) {
	undoTimeline, op, ok := popLastOp(u.undoTimeline)
	if !ok {
		return false, term.Coordinates{}
	}
	u.undoTimeline = undoTimeline
	op.undo()
	u.pushRedo(op)
	return ok, op.at
}

var i int

func (u *undoer) pushUndo(cmd op) {
	u.undoTimeline = append(u.undoTimeline, cmd)
}
func (u *undoer) pushRedo(cmd op) {
	u.redoTimeline = append(u.redoTimeline, cmd)
}

func (u *undoer) resetRedoTimeline() {
	u.redoTimeline = u.redoTimeline[:0]
}

// insert captures underlying writer insert so it can be undone. See cell.writer.insert
func (u *undoer) Insert(at term.Coordinates, str string) (from, to term.Coordinates) {
	op := op{
		at: at,
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

// delete captures underlying writer delete so it can be undone. See cell.writer.delete
func (u *undoer) Delete(from, to term.Coordinates) (start, end term.Coordinates, str string) {
	op := op{
		at: from,
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

// Reset resets the undo/redo timelines and the underlying cell.writer.
func (u *undoer) Reset() {
	u.resetRedoTimeline()
	u.undoTimeline = u.undoTimeline[:0]
	u.w.Reset()
}

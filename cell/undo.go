package cell

import (
	"github.com/ernestrc/fractal/term"
)

// TODO redo/undo should return start of update

// undoer adds undo and redo methods to a otherwise, irreversible cell.writer.
// It satifies the cell.writer interface and it should be used as a replacement.
type undoer struct {
	w            writer
	undoTimeline []op
	redoTimeline []op
}

type op struct {
	do   func()
	undo func()
}

// Newundoer returns new instance of undoer to undo/redo operations of w.
func newUndoer(w writer) *undoer {
	u := new(undoer)
	u.init(w)
	return u
}

// Init initializes this undoer to undo/redo operations of w.
func (u *undoer) init(w writer) {
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

func (u *undoer) redo() bool {
	redoTimeline, op, ok := popLastOp(u.redoTimeline)
	if !ok {
		return ok
	}
	u.redoTimeline = redoTimeline
	op.do()
	u.pushUndo(op)
	return ok
}

func (u *undoer) undo() bool {
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
func (u *undoer) insert(at term.Coordinates, str string) (from, to term.Coordinates) {
	op := op{
		do: func() {
			from, to = u.w.insert(at, str)
		},
		undo: func() {
			u.w.delete(from, to)
		},
	}

	op.do()
	u.pushUndo(op)
	u.resetRedoTimeline()
	return
}

// delete captures underlying writer delete so it can be undone. See cell.writer.delete
func (u *undoer) delete(from, to term.Coordinates) (start, end term.Coordinates, str string) {
	op := op{
		do: func() {
			start, end, str = u.w.delete(from, to)
		},
		undo: func() {
			u.w.insert(start, str)
		},
	}

	op.do()
	u.pushUndo(op)
	u.resetRedoTimeline()
	return
}

// Reset resets the undo/redo timelines and the underlying cell.writer.
func (u *undoer) reset() {
	u.resetRedoTimeline()
	u.undoTimeline = u.undoTimeline[:0]
	u.w.reset()
}

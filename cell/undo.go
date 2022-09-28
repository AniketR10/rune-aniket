package cell

import (
	"unstable.build/go-tui/term"
)

// undoer adds undo and redo methods to a otherwise, irreversible cell.writer.
// It satifies the cell.writer interface and it should be used as a replacement.
type undoer struct {
	version      int
	w            Editor
	undoTimeline []op
	redoTimeline []op
}

type op struct {
	at   term.Coordinates
	do   func()
	undo func()
}

// Newundoer returns new instance of undoer to undo/redo operations of w.
func newUndoer(w Editor) *undoer {
	u := new(undoer)
	u.init(w)
	return u
}

// Init initializes this undoer to undo/redo operations of w.
func (u *undoer) init(w Editor) {
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
	u.version++
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
	u.version--
	u.undoTimeline = undoTimeline
	op.undo()
	u.pushRedo(op)
	return ok, op.at
}

func (u *undoer) pushUndo(cmd op) {
	u.undoTimeline = append(u.undoTimeline, cmd)
}
func (u *undoer) pushRedo(cmd op) {
	u.redoTimeline = append(u.redoTimeline, cmd)
}

func (u *undoer) resetRedoTimeline() {
	u.redoTimeline = u.redoTimeline[:0]
}

// update captures underlying writer update so it can be undone. See cell.writer.Edit
func (u *undoer) Edit(start, end term.Coordinates, str string) (
	from, to term.Coordinates, old string,
) {
	op := op{
		at: start,
		do: func() {
			from, to, old = u.w.Edit(start, end, str)
		},
		undo: func() {
			u.w.Edit(from, to, old)
		},
	}

	op.do()
	u.version++
	u.pushUndo(op)
	u.resetRedoTimeline()
	return
}

func (u *undoer) reset() {
	u.resetRedoTimeline()
	u.undoTimeline = u.undoTimeline[:0]
	u.version = 0
}

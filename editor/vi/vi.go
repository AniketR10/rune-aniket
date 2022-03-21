package vi

import (
	"fmt"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

var _ tui.Handler = (*Vi)(nil)

type snapshot struct {
	content string
	cursor  term.Coordinates
}

func (s snapshot) String() string {
	return fmt.Sprintf("Snapshot{%q:%v}", s.content, s.cursor)
}

// Vi implements a basic vi-like text editor which satisfies tui.Handler
type Vi struct {
	name      string
	handler   viHandler
	buf       *cell.Buffer
	cursor    *editor.Cursor
	logger    *log.Logger
	messenger editor.Messenger
	less      *handler.Less

	repeating     bool
	currUpdated   bool
	evUpdated     bool
	currSnapshot  snapshot
	currUpdates   []term.Event
	repeatUpdates []term.Event

	resetting    bool
	undoTimeline []snapshot
	redoTimeline []snapshot
}

type viSubscriber Vi

// New allocates storage for a new Vi handler, initializes it and returns it.
func New(buf *cell.Buffer, name string, opts ...Option) *Vi {
	vi := new(Vi)
	vi.Init(buf, name, opts...)
	return vi
}

// Init initialies this vi handle with a new Buffer.
func (vi *Vi) Init(buf *cell.Buffer, name string, opts ...Option) {
	vi.name = name

	viHandler := new(viHandlerImpl)
	viHandler.init(buf, opts...)
	vi.handler = viHandler
	vi.buf = buf
	vi.logger = viHandler.config.logger
	vi.messenger = viHandler.config.messenger
	vi.less = &viHandler.less
	vi.cursor = &viHandler.cursor

	vi.repeatUpdates = make([]term.Event, 0)
	vi.currUpdates = make([]term.Event, 0)
	vi.undoTimeline = make([]snapshot, 0)
	vi.redoTimeline = make([]snapshot, 0)

	vi.buf.Subscribe((*viSubscriber)(vi))
	vi.snapshotContent()
}

// Cursor satisfies tui.Handler
func (vi *Vi) Cursor() (term.Coordinates, bool) {
	return vi.handler.Cursor()
}

// Draw satisfies tui.Component
func (vi *Vi) Draw(w term.Writer) {
	vi.handler.Draw(w)
}

func isUpdateMode(mode viMode) bool {
	switch mode {
	case normalMode, gMode, yankMode, searchMode,
		visualMode, visualLineMode, visualBlockMode:
		return false
	case insertMode, deleteMode, replaceMode, replaceOneMode:
		return true
	default:
		panic(fmt.Sprintf("unknown vi mode: %v", mode))
	}
}

func isSelectMode(mode viMode) bool {
	return mode == visualMode || mode == visualLineMode || mode == visualBlockMode
}

func (vi *Vi) resetUpdates() {
	vi.currUpdates = vi.currUpdates[:0]
	vi.currUpdated = false
}

func (vi *Vi) appendLastUpdate(ev term.Event) {
	vi.currUpdates = append(vi.currUpdates, ev)
}

func (vi *Vi) copyRepeat() {
	if !vi.currUpdated {
		return
	}
	vi.repeatUpdates = vi.repeatUpdates[:0]
	vi.repeatUpdates = append(vi.repeatUpdates, vi.currUpdates...)
}

func (vi *Vi) pushNewSnapshot() {
	vi.pushUndo(vi.currSnapshot)
}

func (vi *viSubscriber) OnWillUpdate(from, to term.Coordinates, str string) {
	if !vi.currUpdated && !vi.resetting {
		pubVi := (*Vi)(vi)
		pubVi.currSnapshot.cursor = from
		pubVi.pushNewSnapshot()
		pubVi.resetRedoTimeline()
	}
}

func (vi *viSubscriber) OnDidUpdate(start, end term.Coordinates, old string) {
	if !vi.resetting && !vi.repeating {
		vi.evUpdated = true
		vi.currUpdated = true
	}
}

// Handle satisfies tui.Handler
func (vi *Vi) Handle(ev term.Event) (quit, handled bool) {
	switch vi.handler.mode() {
	case normalMode:
		switch ev.Type {
		case term.EventKey:
			switch ev.Ch {
			case '.':
				handled = vi.repeat()
				return
			case 'u':
				handled = vi.undo()
				return
			}
			switch ev.Key {
			case term.KeyCtrlR:
				handled = vi.redo()
				return
			}
		}
	}

	vi.evUpdated = false
	prevMode := vi.handler.mode()
	quit, handled = vi.handler.Handle(ev)
	nextMode := vi.handler.mode()

	if prevMode == nextMode {
		if !isUpdateMode(prevMode) && vi.evUpdated {
			vi.appendLastUpdate(ev)
			vi.copyRepeat()
			vi.snapshotContent()
			vi.resetUpdates()
		} else if isSelectMode(prevMode) || isUpdateMode(prevMode) {
			vi.appendLastUpdate(ev)
		}
		return
	}

	if !isUpdateMode(prevMode) && isUpdateMode(nextMode) {
		vi.appendLastUpdate(ev)
	} else if isUpdateMode(prevMode) && !isUpdateMode(nextMode) {
		vi.appendLastUpdate(ev)
		vi.copyRepeat()
		vi.snapshotContent()
		vi.resetUpdates()
	} else if isUpdateMode(prevMode) && isUpdateMode(nextMode) {
		vi.appendLastUpdate(ev)
	} else {
		vi.appendLastUpdate(ev)
		if vi.currUpdated || vi.repeating {
			vi.copyRepeat()
			vi.snapshotContent()
			vi.resetUpdates()
		} else if !isSelectMode(nextMode) {
			// reset always if going back to normal
			vi.resetUpdates()
		}
	}
	return quit, handled
}

// Man satisfies tui.Handler
func (vi *Vi) Man() tui.Manual {
	return vi.handler.Man()
}

// Resize satisfies tui.Component
func (vi *Vi) Resize(width, height int) {
	vi.handler.Resize(width, height)
}

// SetMessage uses vi's configured Messenger to set msg with args.
func (vi *Vi) SetMessage(msg string, args ...interface{}) {
	if vi.logger != nil {
		vi.logger.Debugf(msg, args...)
	}

	if vi.messenger != nil {
		vi.messenger.SetMessage(msg, args...)
		return
	}
	vi.less.SetMessage(msg, args...)
}

// MoveToNextLocation moves the cursor to the next location
// in the location list identified by ID.
func (vi *Vi) MoveToNextLocation(ID string) {
	vi.handler.moveToNextLocation(ID)
}

// MoveToPrevLocation moves the cursor to the previous location
// in the location list identified by ID.
func (vi *Vi) MoveToPrevLocation(ID string) {
	vi.handler.moveToPrevLocation(ID)
}

// SetLocationList sets a location list of this handler. See Cursor.SetLocationList
func (vi *Vi) SetLocationList(ID string, l editor.LocationList) {
	vi.handler.setLocationList(ID, l)
}

// SetCursorAtScroll sets the cursor of this Vi handler at content pos.
func (vi *Vi) SetCursorAtScroll(pos term.Coordinates) bool {
	return vi.handler.setCursorAtScroll(pos)
}

// CursorAtScroll sets the cursor of this Vi handler at content pos.
func (vi *Vi) CursorAtScroll() term.Coordinates {
	return vi.handler.cursorAtScroll()
}

// SubscribeScroll subscribe sub to scroll events.
func (vi *Vi) SubscribeScroll(sub component.ScrollSubscriber) {
	vi.handler.subscribeScroll(sub)
}

// Reader returns the underlying cell.Reader.
func (vi *Vi) Reader() cell.Reader {
	return vi.buf.Reader()
}

// Writer returns the underlying cell.Writer.
func (vi *Vi) Writer() cell.Writer {
	return vi.buf.Writer()
}

// Name satisfies editor.Handler.
func (vi *Vi) Name() string {
	return vi.name
}

// Close satisfies editor.Handler.
func (vi *Vi) Close() error {
	return nil
}

func (vi *Vi) repeat() (handled bool) {
	vi.repeating = true
	for _, ev := range vi.repeatUpdates {
		handled = true
		vi.Handle(ev)
	}
	vi.repeating = false
	return
}

func popSnapshot(timeline []snapshot) ([]snapshot, snapshot, bool) {
	lastCmd := len(timeline) - 1
	if lastCmd < 0 {
		return timeline, snapshot{}, false
	}
	snap := timeline[lastCmd]
	return timeline[:lastCmd], snap, true
}

func (vi *Vi) redo() bool {
	redoTimeline, snapshot, ok := popSnapshot(vi.redoTimeline)
	if !ok {
		return false
	}
	vi.redoTimeline = redoTimeline

	current := vi.currSnapshot
	vi.resetToSnapshot(snapshot)
	vi.pushUndo(current)
	return ok
}

func (vi *Vi) undo() bool {
	undoTimeline, snapshot, ok := popSnapshot(vi.undoTimeline)
	if !ok {
		return false
	}
	vi.undoTimeline = undoTimeline

	current := vi.currSnapshot
	vi.resetToSnapshot(snapshot)
	vi.pushRedo(current)
	return ok
}

func (vi *Vi) resetToSnapshot(s snapshot) {
	from, to := term.Coordinates{}, term.Coordinates{Y: vi.buf.Rows()}
	vi.resetting = true
	vi.buf.Update(from, to, s.content)
	vi.handler.setCursorAtScroll(s.cursor)
	vi.currSnapshot = s
	vi.resetting = false
}

func (vi *Vi) snapshotContent() {
	vi.currSnapshot.content = vi.buf.String()
}
func (vi *Vi) pushUndo(content snapshot) {
	// TODO pop last op if exceed mem limit
	vi.undoTimeline = append(vi.undoTimeline, content)
}

func (vi *Vi) pushRedo(content snapshot) {
	vi.redoTimeline = append(vi.redoTimeline, content)
}

func (vi *Vi) resetRedoTimeline() {
	vi.redoTimeline = vi.redoTimeline[:0]
}

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

// Vi implements a basic vi-like text editor which satisfies tui.Handler
type Vi struct {
	name      string
	handler   viHandler
	buf       *cell.Buffer
	cursor    *editor.Cursor
	logger    *log.Logger
	messenger editor.Messenger
	less      *handler.Less

	currUpdated   bool
	evUpdated     bool
	currUpdates   []term.Event
	repeatUpdates []term.Event
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

	vi.buf.Subscribe((*viSubscriber)(vi))
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
	case normalMode, gMode, yankMode, visualMode, visualLineMode, visualBlockMode:
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

func isUndoEvent(mode viMode, ev term.Event) bool {
	return mode == normalMode && (ev.Ch == 'u' || ev.Key == term.KeyCtrlR)
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
	vi.resetUpdates()
}

func (vi *viSubscriber) OnWillUpdate(from, to term.Coordinates, str string) {
}

func (vi *viSubscriber) OnDidUpdate(start, end term.Coordinates, old string) {
	vi.evUpdated = true
	vi.currUpdated = true
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
			}
		}
	}

	vi.evUpdated = false
	prevMode := vi.handler.mode()
	quit, handled = vi.handler.Handle(ev)
	nextMode := vi.handler.mode()

	if prevMode == nextMode {
		if prevMode != normalMode {
			vi.appendLastUpdate(ev)
		}
		if !isUpdateMode(prevMode) && vi.evUpdated && !isUndoEvent(prevMode, ev) {
			vi.appendLastUpdate(ev)
			vi.copyRepeat()
		}
		return
	}

	if !isUpdateMode(prevMode) && isUpdateMode(nextMode) {
		if !isSelectMode(prevMode) {
			vi.resetUpdates()
		}
		vi.appendLastUpdate(ev)
	} else if isUpdateMode(prevMode) && !isUpdateMode(nextMode) {
		vi.appendLastUpdate(ev)
		vi.copyRepeat()
	} else if isUpdateMode(prevMode) && isUpdateMode(nextMode) {
		vi.appendLastUpdate(ev)
	} else {
		vi.appendLastUpdate(ev)
		if vi.evUpdated {
			vi.copyRepeat()
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
	for _, ev := range vi.repeatUpdates {
		handled = true
		vi.handler.Handle(ev)
	}
	return
}

package vi

import (
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
// and tui.Component.
type Vi struct {
	name      string
	handler   viHandler
	buf       *cell.Buffer
	cursor    *editor.Cursor
	logger    *log.Logger
	messenger editor.Messenger
	less      *handler.Less

	currUpdates []term.Event
	lastUpdates []term.Event
}

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
}

func (vi *Vi) Cursor() (term.Coordinates, bool) {
	return vi.handler.Cursor()
}

func (vi *Vi) Draw(w term.Writer) {
	vi.handler.Draw(w)
}

func (vi *Vi) Handle(ev term.Event) (quit, handled bool) {
	return vi.handler.Handle(ev)
}

func (vi *Vi) Man() tui.Manual {
	return vi.handler.Man()
}

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

func (vi *Vi) SubscribeScroll(sub component.ScrollSubscriber) {
	vi.handler.subscribeScroll(sub)
}

func (vi *Vi) Buffer() *cell.Buffer {
	return vi.buf
}

// Name satisfies editor.Handler.
func (vi *Vi) Name() string {
	return vi.name
}

// Close satisfies editor.Handler.
func (vi *Vi) Close() error {
	return nil
}

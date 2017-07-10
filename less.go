package less

import (
	"fmt"
	"math"

	"github.com/ernestrc/less/buffer"
	"github.com/ernestrc/less/config"
	"github.com/ernestrc/less/window"
	termbox "github.com/nsf/termbox-go"
)

type mode uint8

const (
	normalMode mode = iota
	searchMode
)

type handle struct {
	mode       mode
	cmdWindow  *window.Window
	cmdBuf     *buffer.Buffer
	cmdChan    chan []byte
	msgWindow  *window.Window
	msgBuf     *buffer.Buffer
	msgChan    chan []byte
	contWindow *window.Window
	contBuf    *buffer.Buffer
	contChan   chan []byte
	evChan     chan Event
	delEOF     bool
	xcursor    int
	ycursor    int
	height     int
	width      int
	config     *config.Config
}

// EventType represents a less event
type EventType uint8

const (
	// EOF is dispatched when user has reached end of buffer
	EOF EventType = iota
	// Search is dispatched when user has performed a text search
	// `Data` field in `Event` struct will be set to the search text
	Search
	// Error is dispatched when there was an error
	// `Err` field in `Event` struct will be set to the error that triggered event
	Error
	// Exit is dispatched when user wants to exit
	Exit
)

// Event type represents a less event.
type Event struct {
	Type EventType
	Data []byte
	Err  error
}

var (
	h *handle
)

func sendEvent(ev Event) {
	h.evChan <- ev
}

func sendError(err error) {
	sendEvent(Event{Type: Error, Data: nil, Err: err})
}

func redraw() error {
	var err error
	if err = termbox.Clear(h.config.FG, h.config.BG); err != nil {
		return err
	}

	if err = h.contWindow.Draw(); err != nil {
		return err
	}

	if !h.contWindow.CanMoveDown() && !h.delEOF {
		h.delEOF = true
		sendEvent(Event{Type: EOF})
	}

	if err = h.cmdWindow.Draw(); err != nil {
		return err
	}

	if err = h.msgWindow.Draw(); err != nil {
		return err

	}

	termbox.SetCursor(h.xcursor, h.ycursor)
	if err = termbox.Flush(); err != nil {
		return err
	}

	return nil
}

func resetCursor() {
	h.xcursor = 1
	h.ycursor = h.height - 1
}

func setNormalMode() {
	resetCursor()
	h.cmdBuf.Reset()
	if _, err := h.cmdBuf.WriteRune(':'); err != nil {
		sendError(err)
	}
	h.mode = normalMode
}

func setSearchMode() {
	h.cmdBuf.Reset()
	if _, err := h.cmdBuf.WriteRune('/'); err != nil {
		sendError(err)
	}
	h.mode = searchMode
}

func searchHandleEvent(ev termbox.Event) (exit bool, err error) {
	switch ev.Key {

	case termbox.KeyBackspace:
		fallthrough
	case termbox.KeyBackspace2:
		h.xcursor--
		h.cmdBuf.Truncate(h.cmdBuf.Len() - 1)

	case termbox.KeyEnter:
		data := h.cmdBuf.Bytes()[1:]
		h.contBuf.Search(data, h.contWindow.Cells(), h.config.FG,
			h.config.BG, h.config.ResFG, h.config.ResBG)
		setNormalMode()
		moveNextResult()
		sendEvent(Event{Type: Search, Data: data})

	case termbox.KeyEsc:
		setNormalMode()

	default:
		h.xcursor++
		if _, err = h.cmdBuf.WriteRune(ev.Ch); err != nil {
			return
		}
	}

	return false, nil
}

func movePrevResult() {
	c, ok := h.contBuf.PrevResult()

	if !ok {
		setMessage([]byte("pattern not found"))
		return
	}

	h.contWindow.MoveVertical(c.Y)
}

func moveNextResult() {
	c, ok := h.contBuf.NextResult()

	if !ok {
		setMessage([]byte("pattern not found"))
		return
	}

	// go to result line
	h.contWindow.MoveVertical(c.Y)
}

func normalHandleEvent(ev termbox.Event) (exit bool, err error) {
	switch ev.Type {
	case termbox.EventResize:
		resetCursor()
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyEsc:
			return true, nil
		default:
			switch ev.Ch {
			case 'q':
				return true, nil
			case 'N':
				movePrevResult()
			case 'n':
				moveNextResult()
			case '0':
				h.contWindow.MoveStartLine()
			case '$':
				h.contWindow.MoveEndLine()
			case 'g':
				h.contWindow.MoveStartFile()
			case 'G':
				h.contWindow.MoveEndFile()
			case 'j':
				h.contWindow.MoveDown()
			case 'k':
				h.contWindow.MoveUp()
			case 'h':
				h.contWindow.MoveLeft()
			case 'l':
				h.contWindow.MoveRight()
			case '/':
				setSearchMode()
			}
		}
	}

	return false, nil
}

func setMessage(data []byte) {
	h.msgBuf.Reset()
	if _, err := h.msgBuf.Write(data); err != nil {
		sendError(err)
	}

	if err := update(); err != nil {
		sendError(err)
		return
	}
}

func setContent(data []byte) {
	h.contBuf.Reset()
	h.delEOF = false

	if _, err := h.contBuf.Write(data); err != nil {
		sendError(err)
		return
	}

	if err := update(); err != nil {
		sendError(err)
		return
	}

	return
}

// scan buffer to calcuate rows and columns and bounds
func update() error {
	var err error

	contentHeight := h.height - h.config.CmdBarHeight
	msgWindowWidth := int(float32(h.width) * float32(h.config.Msgwidth) / 100)
	msgWidth := int(math.Min(float64(msgWindowWidth), float64(h.msgBuf.Len())))
	cmdBarWidth := h.width - msgWidth

	if err = h.contWindow.Resize(h.width, contentHeight, 0, 0); err != nil {
		return err
	}

	if err = h.cmdWindow.Resize(cmdBarWidth,
		h.config.CmdBarHeight, 0, contentHeight); err != nil {
		return err
	}

	if err = h.msgWindow.Resize(msgWidth,
		h.config.CmdBarHeight, cmdBarWidth, contentHeight); err != nil {
		return err
	}

	return nil
}

func run() {
	var err error
	var exit bool

	if err = termbox.Init(); err != nil {
		sendError(err)
		return
	}

	h.width, h.height = termbox.Size()

	setNormalMode()

	if err = update(); err != nil {
		sendError(err)
		return
	}

	termChan := make(chan termbox.Event)

	go func() {
		for {
			termChan <- termbox.PollEvent()
		}
	}()

receive:
	for !exit && err == nil {
		if err = redraw(); err != nil {
			break receive
		}

		select {
		case data := <-h.contChan:
			setContent(data)
		case data := <-h.msgChan:
			setMessage(data)
		case ev := <-termChan:
			switch ev.Type {
			case termbox.EventError:
				err = ev.Err
				break receive
			case termbox.EventResize:
				h.width, h.height = ev.Width, ev.Height
				if err = update(); err != nil {
					break receive
				}
				fallthrough
			case termbox.EventKey:
				switch h.mode {
				case normalMode:
					exit, err = normalHandleEvent(ev)
				case searchMode:
					exit, err = searchHandleEvent(ev)
				}
			case termbox.EventMouse:
			case termbox.EventInterrupt:
			case termbox.EventRaw:
			case termbox.EventNone:
			}
		}
	}

	if err != nil {
		sendError(err)
	} else if exit {
		sendEvent(Event{Type: Exit})
	}
}

// Init initializes the library and takes control of stdout.
// This function should be called before any other functions.
func Init(cfg *config.Config, content []byte) error {
	h = new(handle)

	if cfg == nil {
		h.config = config.New()
	} else {
		h.config = cfg
	}

	h.cmdBuf = buffer.New()
	h.cmdWindow = window.New(h.cmdBuf, 1, true)
	h.cmdChan = make(chan []byte)

	h.msgBuf = buffer.New()
	h.msgWindow = window.New(h.msgBuf, 1, true)
	h.msgChan = make(chan []byte)

	h.contBuf = buffer.New()
	h.contWindow = window.New(h.contBuf, h.config.Tabspaces, h.config.Wrap)
	h.contChan = make(chan []byte)

	h.evChan = make(chan Event)

	if content != nil {
		if _, err := h.contBuf.Write(content); err != nil {
			return err
		}
	}

	go run()

	return nil
}

// PollEvent will block until there is an event dispatched
func PollEvent() Event {
	return <-h.evChan
}

// Message will draw a message on the bottom right corner
func Message(text string, args ...interface{}) {
	h.msgChan <- []byte(fmt.Sprintf(text, args...))
}

// Content will change the content of the buffer to `text`
func Content(text string) {
	h.contChan <- []byte(text)
}

// Close all resources
func Close() {
	termbox.Close()
}

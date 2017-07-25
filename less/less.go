package less

import (
	"fmt"
	"math"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
	"github.com/ernestrc/fractal/writer"
	termbox "github.com/nsf/termbox-go"
)

type mode uint8

const (
	normalMode mode = iota
	searchMode
)

type handle struct {
	cmdWindow  *component.Window
	cmdBuf     fractal.Buffer
	cmdChan    chan []byte
	msgWindow  *component.Window
	msgBuf     fractal.Buffer
	msgChan    chan []byte
	contWindow *component.Window
	contBuf    fractal.Buffer
	contChan   chan []byte
	mode       mode
	termChan   chan termbox.Event
	evChan     chan Event
	delEOF     bool
	xcursor    int
	ycursor    int
	height     int
	width      int
	config     *Config
	search     string
	pending    []Event
	cells      map[int]fractal.Cell // color information used for printing to component
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
	h handle
	w writer.TermboxWriter
)

func sendEvent(ev Event) {
	h.pending = append(h.pending, ev)
}

func sendError(err error) {
	sendEvent(Event{Type: Error, Data: nil, Err: err})
}

func redraw() error {
	var err error
	if err = w.Clear(h.config.FG, h.config.BG); err != nil {
		return err
	}

	if err = h.contWindow.Draw(&w); err != nil {
		return err
	}

	if !h.contWindow.CanSeekDown() && !h.delEOF {
		h.delEOF = true
		sendEvent(Event{Type: EOF})
	}

	if err = h.cmdWindow.Draw(&w); err != nil {
		return err
	}

	if err = h.msgWindow.Draw(&w); err != nil {
		return err

	}

	termbox.SetCursor(h.xcursor, h.ycursor)
	if err = w.Flush(); err != nil {
		return err
	}

	return nil
}

func resetCursor() {
	h.xcursor = 1
	h.ycursor = h.height - 1
}

func setNormalMode() error {
	resetCursor()
	h.cmdBuf.Reset()
	if _, err := h.cmdBuf.WriteRune(':'); err != nil {
		return err
	}
	h.mode = normalMode

	return nil
}

func setSearchMode() error {
	h.cmdBuf.Reset()
	if _, err := h.cmdBuf.WriteRune('/'); err != nil {
		return err
	}
	h.mode = searchMode

	return nil
}

func searchHandleEvent(ev termbox.Event) (exit bool, err error) {
	switch ev.Key {

	case termbox.KeyBackspace:
		fallthrough
	case termbox.KeyBackspace2:
		h.xcursor--
		h.cmdBuf.Truncate(h.cmdBuf.Len() - 1)

	case termbox.KeyEnter:
		str := h.cmdBuf.String()
		bytes := []byte(str)[1:]
		h.search = string(bytes)
		h.contWindow.Search(h.search)
		if err = setNormalMode(); err != nil {
			return
		}
		h.contWindow.SeekNextResult()
		sendEvent(Event{Type: Search, Data: bytes})

	case termbox.KeyEsc:
		if err = setNormalMode(); err != nil {
			return
		}

	default:
		h.xcursor++
		if _, err = h.cmdBuf.WriteRune(ev.Ch); err != nil {
			return
		}
	}

	return
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
				h.contWindow.SeekPrevResult()
			case 'n':
				h.contWindow.SeekNextResult()
			case '0':
				h.contWindow.SeekStartLine()
			case '$':
				h.contWindow.SeekEndLine()
			case 'g':
				h.contWindow.SeekStartFile()
			case 'G':
				h.contWindow.SeekEndFile()
			case 'j':
				h.contWindow.SeekDown()
			case 'k':
				h.contWindow.SeekUp()
			case 'h':
				h.contWindow.SeekLeft()
			case 'l':
				h.contWindow.SeekRight()
			case '/':
				err = setSearchMode()
			}
		}
	}

	return
}

func setMessage(data []byte) error {
	h.msgBuf.Reset()
	if _, err := h.msgBuf.Write(data); err != nil {
		return err
	}

	if err := update(); err != nil {
		return err
	}

	return nil
}

func setContent(data []byte) error {
	h.contBuf.Reset()
	h.delEOF = false

	if _, err := h.contBuf.Write(data); err != nil {
		return err
	}

	if len(h.search) != 0 {
		h.contWindow.Search(h.search)
	}

	return nil
}

// scan buffer to calcuate rows and columns and bounds
func update() error {
	var err error

	contentHeight := h.height - h.config.CmdBarHeight
	msgWindowWidth := int(float32(h.width) * float32(h.config.Msgwidth) / 100)
	msgWidth := int(math.Min(float64(msgWindowWidth), float64(h.msgBuf.Len())))
	cmdBarWidth := h.width - msgWidth

	if err = h.contWindow.Resize(h.width, contentHeight); err != nil {
		return err
	}

	h.cmdWindow.Move(0, contentHeight)
	if err = h.cmdWindow.Resize(cmdBarWidth, h.config.CmdBarHeight); err != nil {
		return err
	}

	h.msgWindow.Move(cmdBarWidth, contentHeight)
	if err = h.msgWindow.Resize(msgWidth, h.config.CmdBarHeight); err != nil {
		return err
	}

	return nil
}

func run() {
	var err error
	var exit bool

	go func() {
		for {
			h.termChan <- termbox.PollEvent()
		}
	}()

	for n := 0; !exit && err == nil || n != 0; n = len(h.pending) {
		if err = redraw(); err != nil {
			break
		}

		// dispatch outgoing events first if possible
		if n != 0 {
			select {
			case h.evChan <- h.pending[0]:
				h.pending = h.pending[1:]
				continue
			default:
			}
		}

		// wait for data on user and term channels
		select {
		case data := <-h.contChan:
			err = setContent(data)
		case data := <-h.msgChan:
			err = setMessage(data)
		case ev := <-h.termChan:
			switch ev.Type {
			case termbox.EventError:
				err = ev.Err
			case termbox.EventResize:
				h.width, h.height = ev.Width, ev.Height
				if err = update(); err != nil {
					break
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

		if err != nil {
			sendError(err)
			// TODO close channels
			err = nil
		} else if exit {
			sendEvent(Event{Type: Exit})
			// TODO close channels
		}
	}
}

func setupWindow(w *component.Window) {
	w.ResultsFG = h.config.ResFG
	w.ResultsBG = h.config.ResBG
	w.Tabspaces, w.Wrap = h.config.Tabspaces, h.config.Wrap
}

// Init initializes the library and takes control of stdout.
// This function should be called before any other functions.
func Init(cfg *Config, content string) error {
	var err error

	if cfg == nil {
		h.config = DefaultConfig()
	} else {
		h.config = cfg
	}
	h.cmdWindow = component.NewWindow(&h.cmdBuf, h.width, h.height)
	h.msgWindow = component.NewWindow(&h.msgBuf, h.width, h.height)
	h.contWindow = component.NewWindow(&h.contBuf, h.width, h.height)

	setupWindow(h.cmdWindow)
	setupWindow(h.msgWindow)
	setupWindow(h.contWindow)

	h.cmdChan = make(chan []byte)
	h.msgChan = make(chan []byte)
	h.contChan = make(chan []byte)
	h.evChan = make(chan Event)

	if content != "" {
		if _, err = h.contBuf.Write([]byte(content)); err != nil {
			return err
		}
	}

	if err = termbox.Init(); err != nil {
		return err
	}

	h.width, h.height = termbox.Size()

	if err = setNormalMode(); err != nil {
		return err
	}

	if err = update(); err != nil {
		return err
	}

	h.termChan = make(chan termbox.Event)
	h.cells = make(map[int]fractal.Cell)

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

// Width returns the terminal width
func Width() int {
	return h.width
}

// Height returns the terminal height
func Height() int {
	return h.height
}

// Close all resources
func Close() {
	termbox.Close()
}

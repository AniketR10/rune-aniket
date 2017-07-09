package less

import (
	"bytes"
	"container/list"
	"fmt"
	"io"
	"math"

	termbox "github.com/nsf/termbox-go"
)

type mode uint8

const (
	normalMode mode = iota
	searchMode
)

type handle struct {
	mode       mode
	cmdWindow  *Window
	cmdBuf     *Buffer
	msgWindow  *Window
	msgBuf     *Buffer
	contBuf    *Buffer
	contWindow *Window
	evChan     chan Event
	delEOF     bool
	xcursor    int
	ycursor    int
	height     int
	width      int
	config     *Config
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

func queueEvent(ev Event) {
	h.evChan <- ev
}

func sendError(err error) {
	queueEvent(Event{Type: Error, Data: nil, Err: err})
}

func redraw() error {
	var err error
	if err = termbox.Clear(h.config.Bg, h.config.Bg); err != nil {
		return err
	}

	contentHeight := h.height - h.config.CmdBarHeight
	msgWindowWidth := int(float32(h.width) * float32(h.config.Msgwidth) / 100)
	msgwidth := int(math.Min(float64(msgWindowWidth), float64(h.msgBuf.Len())))
	cmdBarWidth := h.width - msgwidth

	contentView := bytes.NewBuffer(h.content.contentBuf.Bytes())
	if _, _, err = draw(contentView, h.content.cells, h.content.xoffset, h.content.yoffset, 0, 0, h.width, contentHeight); err != nil {
		if err != io.EOF {
			return err
		}

		if !h.delEOF {
			h.delEOF = true
			queueEvent(Event{Type: EOF})
		}
	}

	cmdView := bytes.NewBuffer(h.cmdBuf.Bytes())
	if _, _, err = draw(cmdView, nil, 0, 0, 0, contentHeight, cmdBarWidth, 1); err != nil && err != io.EOF {
		return err
	}

	if _, _, err = draw(h.msgBuf, nil, 0, 0, cmdBarWidth, contentHeight, msgwidth, 1); err != nil && err != io.EOF {
		return err
	}

	termbox.SetCursor(h.xcursor, h.ycursor)
	termbox.Flush()

	return nil
}

func resetCursor() {
	h.xcursor = 1
	h.ycursor = h.height - 1
}

func setNormalMode() {
	resetCursor()
	h.cmdBuf.Reset()
	h.cmdBuf.WriteRune(':')
	h.mode = normalMode
}

func setSearchMode() {
	h.cmdBuf.Reset()
	h.cmdBuf.WriteRune('/')
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
		search(data)
		setNormalMode()
		moveNextResult()
		queueEvent(Event{Type: Search, Data: data})

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
	if h.result == nil {
		h.result = h.reslist.Back()
	} else {
		h.result = h.result.Prev()
	}

	if h.result == nil {
		setMessage([]byte("pattern not found"))
		return
	}
	setResultOffsets()
}

func moveNextResult() {
	if h.result == nil {
		h.result = h.reslist.Front()
	} else {
		h.result = h.result.Next()
	}

	if h.result == nil {
		setMessage([]byte("pattern not found"))
		return
	}
	setResultOffsets()
}

func setResultOffsets() {
	c := h.result.Value.(cell)
	// go to result line
	h.yoffset = c.y
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
			case '0':
				h.xoffset = 0
			case 'N':
				movePrevResult()
			case 'n':
				moveNextResult()
			case '$':
				h.xoffset = h.xmaxoffset
			case 'g':
				h.yoffset = 0
			case 'G':
				h.yoffset = h.ymaxoffset
			case 'j':
				h.yoffset++
			case 'k':
				h.yoffset--
			case 'h':
				h.xoffset--
			case 'l':
				h.xoffset++
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
}

func setContent(data []byte) {
	h.contentBuf.Reset()
	h.delEOF = false

	if _, err := h.contentBuf.Write(data); err != nil {
		sendError(err)
		return
	}

	// scan buffer to calcuate rows and columns and bounds
	if err := calculateBounds(); err != nil {
		sendError(err)
		return
	}

	return
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

	if err = calculateBounds(); err != nil {
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
	for !exit {
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
				calculateBounds()
				fallthrough
			case termbox.EventKey:
				switch h.mode {
				case normalMode:
					if exit, err = normalHandleEvent(ev); err != nil {
						break receive
					}
				case searchMode:
					if exit, err = searchHandleEvent(ev); err != nil {
						break receive
					}
				}
			case termbox.EventMouse:
			case termbox.EventInterrupt:
			case termbox.EventRaw:
			case termbox.EventNone:
			}
		}

		normalizeOffsets()
	}

	if err != nil {
		sendError(err)
	} else if exit {
		queueEvent(Event{Type: Exit})
	}
}

// Init initializes the library and takes control of stdout.
// This function should be called before any other functions.
func Init(config *Config, content *bytes.Buffer) error {
	h = new(handle)
	h.cmdBuf = new(bytes.Buffer)
	h.msgBuf = new(bytes.Buffer)
	h.reslist = new(list.List)
	h.cells = make(map[int]cell)
	h.evChan = make(chan Event)
	h.msgChan = make(chan []byte)
	h.contChan = make(chan []byte)

	if config == nil {
		h.config = &defaultConfig
	} else {
		h.config = config
	}

	if content != nil {
		h.contentBuf = content
	} else {
		h.contentBuf = new(bytes.Buffer)
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

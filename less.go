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

type cell struct {
	i  int
	x  int
	y  int
	fg termbox.Attribute
	bg termbox.Attribute
}

type handle struct {
	mode       mode
	contentBuf *bytes.Buffer
	cmdBuf     *bytes.Buffer
	msgBuf     *bytes.Buffer
	reslist    *list.List    // search result list
	result     *list.Element // current focused result
	search     []byte
	evBuf      *list.List
	evChan     chan Event
	contChan   chan []byte
	msgChan    chan []byte
	delEOF     bool
	cells      map[int]cell
	xcursor    int
	ycursor    int
	xoffset    int
	yoffset    int
	xmaxoffset int
	ymaxoffset int
	height     int
	width      int
	columns    int
	rows       int
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

// x/yoffset is the offset from the content
// x/ystart is the offset in the cell grid
// x/ywindow is the x and y max cells to write
func draw(data *bytes.Buffer, xoffset, yoffset, xstart, ystart, xwindow, ywindow int) (x, y int, err error) {
	var c rune
	x = xstart
	y = ystart

	currxoffset := xoffset

	// draw until we've filled all available cells
	for i := 0; y-ystart < ywindow; i++ {
		if c, _, err = data.ReadRune(); err != nil {
			break
		}

		// wrap or skip content
		if x-xstart == xwindow {
			if h.config.wrap {
				y++
				x = xstart
			} else {
				if c == '\n' {
					y++
					x = xstart
					currxoffset = xoffset
				}
				continue
			}
		}

		switch c {
		case '\n':
			if yoffset > 0 {
				yoffset--
			} else {
				y++
				x = xstart
				currxoffset = xoffset
			}
		case '\t':
			if yoffset != 0 {
				continue
			}
			if currxoffset <= 0 {
				x += h.config.tabspaces
			} else {
				currxoffset -= h.config.tabspaces
			}
		default:
			if yoffset != 0 {
				continue
			}
			if currxoffset <= 0 {
				setCell(x, y, i, c)
				x++
			} else {
				currxoffset--
			}
		}
	}

	return x, y, err
}

func setCell(x, y, i int, r rune) {
	set := h.cells[i]
	termbox.SetCell(x, y, r, set.fg, set.bg)
}

func redraw() error {
	var err error
	if err = termbox.Clear(h.config.bg, h.config.bg); err != nil {
		return err
	}

	contentHeight := h.height - h.config.cmdBarHeight
	msgWindowWidth := int(float32(h.width) * float32(h.config.msgwidth) / 100)
	msgwidth := int(math.Min(float64(msgWindowWidth), float64(h.msgBuf.Len())))
	cmdBarWidth := h.width - msgwidth

	contentView := bytes.NewBuffer(h.contentBuf.Bytes())
	if _, _, err = draw(contentView, h.xoffset, h.yoffset, 0, 0, h.width, contentHeight); err != nil {
		if err != io.EOF {
			return err
		}

		if !h.delEOF {
			h.delEOF = true
			queueEvent(Event{Type: EOF})
		}
	}

	cmdView := bytes.NewBuffer(h.cmdBuf.Bytes())
	if _, _, err = draw(cmdView, 0, 0, 0, contentHeight, cmdBarWidth, 1); err != nil && err != io.EOF {
		return err
	}

	if _, _, err = draw(h.msgBuf, 0, 0, cmdBarWidth, contentHeight, msgwidth, 1); err != nil && err != io.EOF {
		return err
	}

	termbox.SetCursor(h.xcursor, h.ycursor)
	termbox.Flush()

	return nil
}

func calculateBounds() error {
	var err error
	var c rune
	var currX int
	h.columns, h.rows = 0, 0
	view := bytes.NewBuffer(h.contentBuf.Bytes())
	for i := 0; ; i++ {
		if c, _, err = view.ReadRune(); err != nil {
			break
		}

		switch c {
		case '\n':
			h.rows++
			if currX > h.columns {
				h.columns = currX
			}
			currX = 0
		case '\t':
			currX += h.config.tabspaces
		default:
			currX++
		}

		// collect x, y coordinates
		prev := h.cells[i]
		h.cells[i] = cell{fg: prev.fg, bg: prev.bg, x: currX, y: h.rows, i: i}
	}

	if h.rows <= h.height {
		h.ymaxoffset = 0
	} else {
		h.ymaxoffset = h.rows - h.height + h.config.cmdBarHeight
	}

	if h.columns >= h.width {
		h.xmaxoffset = h.columns - h.width
	} else {
		h.xmaxoffset = 0
	}

	if err != io.EOF {
		return err
	}

	return nil
}

func normalizeOffsets() {
	if h.xoffset < 0 {
		h.xoffset = 0
	} else if h.xoffset > h.xmaxoffset {
		h.xoffset = h.xmaxoffset
	}
	if h.yoffset < 0 {
		h.yoffset = 0
	} else if h.yoffset > h.ymaxoffset {
		h.yoffset = h.ymaxoffset
	}
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

func search(data []byte) {
	// reset result cells bg/fg
	for el := h.reslist.Front(); el != nil; el = el.Next() {
		c := el.Value.(cell)
		for i, slen := c.i, c.i+len(h.search); i < slen; i++ {
			h.cells[i] = cell{
				bg: h.config.bg,
				fg: h.config.fg,
				x:  h.cells[i].x,
				y:  h.cells[i].y,
				i:  i,
			}
		}
	}

	h.reslist = h.reslist.Init()
	h.result = nil
	h.search = data

	tlen := len(data)
	if tlen == 0 {
		return
	}

	view := h.contentBuf.Bytes()

	a := 0
	var i int
	for {
		if i = bytes.Index(view, data); i == -1 {
			break
		}

		// use anchor to translate index to original slice
		a += i

		for j, last := a, a+tlen; j < last; j++ {
			h.cells[j] = cell{
				fg: h.config.resfg,
				bg: h.config.resbg,
				// use previous cells map to get x,y coordinates
				x: h.cells[j].x,
				y: h.cells[j].y,
				i: j,
			}
		}

		// mark first cell as result index
		h.reslist.PushBack(h.cells[a])

		view = view[i+tlen:]

		// set next anchor
		a += tlen
	}

	return
}

func searchHandleEvent(ev termbox.Event) (exit bool, err error) {
	switch ev.Key {

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

		//send:
		// for next := h.evBuf.Front(); next != nil; next = next.Next() {
		// 	select {
		// 	case h.evChan <- next.Value.(Event):
		// 		h.evBuf.Remove(next)
		// 	default:
		// 		break send
		// 	}
		// }
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
	h.evBuf = new(list.List)

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

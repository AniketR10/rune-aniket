package less

import (
	"bytes"
	"fmt"
	"io"

	termbox "github.com/nsf/termbox-go"
)

type Config struct {
	tabspaces int
	fg        termbox.Attribute
	bg        termbox.Attribute
	wrap      bool
	debug     bool
	// TODO keyMap *KeyMap
}

var defaultConfig = Config{
	tabspaces: 4,
	fg:        termbox.ColorDefault,
	bg:        termbox.ColorDefault,
	wrap:      false,
	debug:     false,
}

type mode uint8

const (
	NormalMode mode = iota
	SearchMode
	ResultsMode
)

type Handle struct {
	mode       mode
	contentBuf *bytes.Buffer
	cmdBuf     *bytes.Buffer
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
	handler    Handler
	config     *Config
}

type Event uint8

const (
	EOF Event = iota
	Search
)

type Handler interface {
	OnEvent(*Event) error
	io.WriterTo
}

func New(handler Handler, config *Config) *Handle {
	h := new(Handle)
	h.handler = handler
	h.contentBuf = new(bytes.Buffer)
	h.cmdBuf = new(bytes.Buffer)

	if config == nil {
		h.config = &defaultConfig
	} else {
		h.config = config
	}

	return h
}

// x/yoffset is the offset from the content
// x/ystart is the offset in the cell grid
// x/ywindow is the x and y max cells to write
func (h *Handle) draw(data *bytes.Buffer, xoffset, yoffset, xstart, ystart, xwindow, ywindow int) (x, y int, err error) {
	var c rune
	x = xstart
	y = ystart

	currxoffset := xoffset

	// draw until we've filled all available cells
	for y-ystart < ywindow {
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
				h.setCell(x, y, c)
				x++
			} else {
				currxoffset--
			}
		}
	}

	if err != io.EOF {
		return x, y, err
	}

	return x, y, nil
}

func (h *Handle) setCell(x, y int, r rune) {
	termbox.SetCell(x, y, r, h.config.fg, h.config.bg)
}

func (h *Handle) redraw() error {
	var err error
	if err = termbox.Clear(h.config.bg, h.config.bg); err != nil {
		return err
	}

	contentView := bytes.NewBuffer(h.contentBuf.Bytes())
	if _, _, err = h.draw(contentView, h.xoffset, h.yoffset, 0, 0, h.width, h.height-1); err != nil {
		return err
	}

	cmdView := bytes.NewBuffer(h.cmdBuf.Bytes())
	if _, _, err = h.draw(cmdView, 0, 0, 0, h.height-1, h.width, 1); err != nil {
		return err
	}

	termbox.SetCursor(h.xcursor, h.ycursor)
	termbox.Flush()

	return nil
}

func (h *Handle) calculate() error {
	var err error
	var c rune
	var currX int
	h.columns, h.rows = 0, 0
	view := bytes.NewBuffer(h.contentBuf.Bytes())
	for {
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
	}

	if h.rows <= h.height {
		h.ymaxoffset = 0
	} else {
		h.ymaxoffset = h.rows - h.height
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

func (h *Handle) pull() error {
	var err error
	if _, err = h.handler.WriteTo(h.contentBuf); err != nil {
		return err
	}

	// scan buffer to calculate rows and columns and bounds
	if err = h.calculate(); err != nil {
		return err
	}

	return nil
}

func (h *Handle) normalizeOffsets() {
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

func (h *Handle) setNormalMode() {
	h.xcursor = 1
	h.ycursor = h.height - 1
	h.cmdBuf.Reset()
	h.cmdBuf.WriteRune(':')
	h.mode = NormalMode
}

func (h *Handle) setSearchMode() {
	h.cmdBuf.Reset()
	h.cmdBuf.WriteRune('/')
	h.mode = SearchMode
}

// func (h *Handle) setResultsMode() {
// 	h.cmdBuf.Reset()
// 	h.cmdBuf.WriteRune('occurrences')
// 	h.mode = ResultsMode
// }

// TODO jump from result to result
func (h *Handle) resultsHandleEvent(ev termbox.Event) bool {
	return false
}

func (h *Handle) searchHandleEvent(ev termbox.Event) bool {
	switch ev.Key {
	case termbox.KeyEnter:
		// TODO search occurrences
		// TODO queue search event
		// h.setResultsMode()
		h.setNormalMode()
	case termbox.KeyEsc:
		h.setNormalMode()
	default:
		h.cmdBuf.WriteRune(ev.Ch)
	}

	return false
}

func (h *Handle) normalHandleEvent(ev termbox.Event) bool {
	switch ev.Type {
	case termbox.EventResize:
		// force set cursor
		h.setNormalMode()
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyEsc:
			return true
		default:
			switch ev.Ch {
			case 'q':
				return true
			case '0':
				h.xoffset = 0
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
				h.setSearchMode()
			}
		}
	}

	return false
}

func (h *Handle) Run() error {
	var err error
	if err = termbox.Init(); err != nil {
		return err
	}

	defer termbox.Close()

	h.width, h.height = termbox.Size()

	h.setNormalMode()

	if err = h.pull(); err != nil {
		return err
	}

	// main loop
	for exit := false; !exit; {
		if err = h.redraw(); err != nil {
			return err
		}

		ev := termbox.PollEvent()
		switch ev.Type {
		case termbox.EventError:
			return ev.Err
		case termbox.EventResize:
			h.width, h.height = ev.Width, ev.Height
			h.calculate()
			fallthrough
		case termbox.EventKey:
			switch h.mode {
			case NormalMode:
				exit = h.normalHandleEvent(ev)
			case SearchMode:
				exit = h.searchHandleEvent(ev)
			case ResultsMode:
				exit = h.resultsHandleEvent(ev)
			}
		case termbox.EventMouse:
		case termbox.EventInterrupt:
		case termbox.EventRaw:
		case termbox.EventNone:
		}
		h.normalizeOffsets()
		if h.config.debug {
			h.cmdBuf.Reset()
			h.cmdBuf.Write([]byte(fmt.Sprintf("%+v", h)))
		}
	}
	return nil
}

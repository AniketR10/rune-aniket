package less

import (
	"bytes"
	"container/list"
	"fmt"
	"io"
	"math"
	"strings"

	termbox "github.com/nsf/termbox-go"
)

const bugMessage = "this is most likely a bug in the library. Please file bug at https://github.com/ernestrc/less/issues"

type Config struct {
	tabspaces int
	fg        termbox.Attribute
	bg        termbox.Attribute
	msgwidth  int8 // 0 - 100
	wrap      bool
	debug     bool
	resfg     termbox.Attribute
	resbg     termbox.Attribute
	// TODO keyMap *KeyMap
}

var defaultConfig = Config{
	tabspaces: 4,
	fg:        termbox.ColorDefault,
	bg:        termbox.ColorDefault,
	msgwidth:  70,
	wrap:      false,
	debug:     false,
	resfg:     termbox.AttrReverse,
	resbg:     termbox.ColorDefault,
}

type mode uint8

const (
	NormalMode mode = iota
	SearchMode
)

type cell struct {
	x  int
	y  int
	fg termbox.Attribute
	bg termbox.Attribute
}

type Handle struct {
	mode       mode
	contentBuf *bytes.Buffer
	cmdBuf     *bytes.Buffer
	msgBuf     *bytes.Buffer
	reslist    *list.List
	result     *list.Element
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
	handler    Handler
	config     *Config
}

type Handler interface {
	OnSearch(h *Handle, text string) error
	io.WriterTo
}

func New(handler Handler, config *Config) *Handle {
	h := new(Handle)
	h.handler = handler
	h.contentBuf = new(bytes.Buffer)
	h.cmdBuf = new(bytes.Buffer)
	h.msgBuf = new(bytes.Buffer)
	h.reslist = new(list.List)
	h.cells = make(map[int]cell)

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
				h.setCell(x, y, i, c)
				x++
			} else {
				currxoffset--
			}
		}
	}

	if err == io.EOF {
		// try to pull more content from handler
		if err = h.pull(); err != nil {
			return x, y, err
		}
		return x, y, nil
	}

	return x, y, err
}

func (h *Handle) setCell(x, y, i int, r rune) {
	set := h.cells[i]
	termbox.SetCell(x, y, r, set.fg, set.bg)
}

func (h *Handle) redraw() error {
	var err error
	if err = termbox.Clear(h.config.bg, h.config.bg); err != nil {
		return err
	}

	contentHeight := h.height - 1
	msgWindowWidth := int(float32(h.width) * float32(h.config.msgwidth) / 100)
	msgwidth := int(math.Min(float64(msgWindowWidth), float64(h.msgBuf.Len())))
	cmdBarWidth := h.width - msgwidth

	contentView := bytes.NewBuffer(h.contentBuf.Bytes())
	if _, _, err = h.draw(contentView, h.xoffset, h.yoffset, 0, 0, h.width, contentHeight); err != nil {
		return err
	}

	cmdView := bytes.NewBuffer(h.cmdBuf.Bytes())
	if _, _, err = h.draw(cmdView, 0, 0, 0, contentHeight, cmdBarWidth, 1); err != nil {
		return err
	}

	if _, _, err = h.draw(h.msgBuf, 0, 0, cmdBarWidth, contentHeight, msgwidth, 1); err != nil {
		return err
	}

	termbox.SetCursor(h.xcursor, h.ycursor)
	termbox.Flush()

	return nil
}

func (h *Handle) calculateBounds() error {
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
		h.cells[i] = cell{fg: prev.fg, bg: prev.bg, x: currX, y: h.rows}
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

	// scan buffer to calcuate rows and columns and bounds
	if err = h.calculateBounds(); err != nil {
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

func (h *Handle) resetCursor() {
	h.xcursor = 1
	h.ycursor = h.height - 1
}

func (h *Handle) setNormalMode() {
	h.resetCursor()
	h.cmdBuf.Reset()
	h.cmdBuf.WriteRune(':')
	h.mode = NormalMode
}

func (h *Handle) setSearchMode() {
	h.cmdBuf.Reset()
	h.cmdBuf.WriteRune('/')
	h.mode = SearchMode
}

// TODO add results to list so we can navigate them
func (h *Handle) searchText(text string) (cells map[int]cell, reslist *list.List, err error) {
	reslist = h.reslist.Init()
	cells = map[int]cell{}

	if text == "" {
		return
	}

	view := string(h.contentBuf.Bytes())
	tlen := len(text)

	a := 0
	var i int
	for {
		if i = strings.Index(view, text); i == -1 {
			break
		}

		// use anchor to translate index to original slice
		a += i

		for j, last := a, a+tlen; j < last; j++ {
			cells[j] = cell{
				fg: h.config.resfg,
				bg: h.config.resbg,
				// use previous cells map to get x,y coordinates
				x: h.cells[j].x,
				y: h.cells[j].y,
			}
		}

		// mark first cell as result index
		reslist.PushBack(cells[a])

		view = view[i+tlen:]

		// set next anchor
		a += tlen
	}

	return
}

func (h *Handle) searchHandleEvent(ev termbox.Event) (exit bool, err error) {
	switch ev.Key {
	case termbox.KeyEnter:
		text := string(h.cmdBuf.Bytes()[1:])

		if err = h.handler.OnSearch(h, text); err != nil {
			return true, err
		}
		if h.cells, h.reslist, err = h.searchText(text); err != nil {
			return true, err
		}
		h.setNormalMode()
		h.moveNextResult()
	case termbox.KeyEsc:
		h.setNormalMode()
	default:
		h.xcursor++
		h.cmdBuf.WriteRune(ev.Ch)
	}

	return false, nil
}

func (h *Handle) movePrevResult() {
	if h.result == nil {
		h.result = h.reslist.Back()
	} else {
		h.result = h.result.Prev()
	}

	if h.result == nil {
		h.Message("pattern not found")
		return
	}
	h.setResultOffsets()
}

func (h *Handle) moveNextResult() {
	if h.result == nil {
		h.result = h.reslist.Front()
	} else {
		h.result = h.result.Next()
	}

	if h.result == nil {
		h.Message("pattern not found")
		return
	}
	h.setResultOffsets()
}

func (h *Handle) setResultOffsets() {
	c := h.result.Value.(cell)
	// go to result line
	h.yoffset = c.y
}

func (h *Handle) normalHandleEvent(ev termbox.Event) (exit bool, err error) {
	switch ev.Type {
	case termbox.EventResize:
		h.resetCursor()
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
				h.movePrevResult()
			case 'n':
				h.moveNextResult()
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

	return false, nil
}

func (h *Handle) Message(text string, args ...interface{}) {
	h.msgBuf.Reset()
	h.msgBuf.Write([]byte(fmt.Sprintf(text, args...)))
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
			h.calculateBounds()
			fallthrough
		case termbox.EventKey:
			switch h.mode {
			case NormalMode:
				if exit, err = h.normalHandleEvent(ev); err != nil {
					return err
				}
			case SearchMode:
				if exit, err = h.searchHandleEvent(ev); err != nil {
					return err
				}
			}
		case termbox.EventMouse:
		case termbox.EventInterrupt:
		case termbox.EventRaw:
		case termbox.EventNone:
		}
		h.normalizeOffsets()
		if h.config.debug {
			h.msgBuf.Reset()
			h.msgBuf.Write([]byte(fmt.Sprintf("%+v", h)))
		}
	}
	return nil
}

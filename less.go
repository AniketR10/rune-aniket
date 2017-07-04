package less

import (
	"bytes"
	"io"

	termbox "github.com/nsf/termbox-go"
)

type Config struct {
	tabspaces int
	fg        termbox.Attribute
	bg        termbox.Attribute
	wrap      bool
	// TODO keyMap *KeyMap
}

var defaultConfig = Config{
	tabspaces: 4,
	fg:        termbox.ColorDefault,
	bg:        termbox.ColorDefault,
	wrap:      false,
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
	h.setNormalMode()

	if config == nil {
		h.config = &defaultConfig
	} else {
		h.config = config
	}

	return h
}

func (h *Handle) draw(buf *bytes.Buffer, xi, yi, width, height int) (x, y int, err error) {
	var c rune
	x = xi
	y = yi

	// draw until we've reached window height
	for y-yi < height {
		if c, _, err = buf.ReadRune(); err != nil {
			break
		}

		// wrap
		if x-xi == width {
			if h.config.wrap {
				y++
				x = 0
			} else if c == '\n' {
				y++
				x = 0
			} else {
				continue
			}
		}

		switch c {
		case '\n':
			y++
			x = 0
		case '\t':
			h.setCell(x, y, c)
			x += h.config.tabspaces
		default:
			h.setCell(x, y, c)
			x++
		}
	}

	if err != io.EOF {
		return x, y, err
	}

	return x, y, nil
}

func seekBuffer(buf *bytes.Buffer, yoffset int) error {
	var err error
	var c rune
	for yoffset > 0 {
		if c, _, err = buf.ReadRune(); err != nil {
			break
		}

		switch c {
		case '\n':
			yoffset--
		}
	}

	if err != io.EOF {
		return err
	}

	return nil
}

func (h *Handle) setCell(x, y int, r rune) {
	termbox.SetCell(x, y, r, h.config.fg, h.config.bg)
}

func (h *Handle) redraw() error {
	var err error
	if err = termbox.Clear(h.config.bg, h.config.bg); err != nil {
		return err
	}

	var y int
	contentView := bytes.NewBuffer(h.contentBuf.Bytes())
	// FIXME xoffset is not used
	// maybe this should be done inside draw in the form of some sort of skipping
	if h.yoffset > 0 {
		seekBuffer(contentView, h.yoffset)
	}
	if _, y, err = h.draw(contentView, 0, 0, h.width, h.height-1); err != nil {
		return err
	}

	cmdView := bytes.NewBuffer(h.cmdBuf.Bytes())
	if _, y, err = h.draw(cmdView, 0, y, h.width, 1); err != nil {
		return err
	}

	termbox.SetCursor(1, y)
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

	h.ymaxoffset = h.rows - h.height
	h.xmaxoffset = h.columns - h.width

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
	switch ev.Key {
	case termbox.KeyEsc:
		return true
	default:
		switch ev.Ch {
		case 'q':
			return true
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

	return false
}

func (h *Handle) Run() error {
	var err error
	if err = termbox.Init(); err != nil {
		return err
	}

	defer termbox.Close()

	h.width, h.height = termbox.Size()

	if err = h.pull(); err != nil {
		return err
	}

	for shouldBreak := false; !shouldBreak; {
		if err = h.redraw(); err != nil {
			return err
		}

		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventResize:
			h.width, h.height = ev.Width, ev.Height
			h.calculate()
		case termbox.EventKey:
			switch h.mode {
			case NormalMode:
				shouldBreak = h.normalHandleEvent(ev)
			case SearchMode:
				shouldBreak = h.searchHandleEvent(ev)
			case ResultsMode:
				shouldBreak = h.resultsHandleEvent(ev)
			}
		case termbox.EventError:
			return ev.Err
		}

		h.normalizeOffsets()
	}
	return nil
}

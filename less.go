package less

import (
	"bytes"
	"fmt"
	"io"

	termbox "github.com/nsf/termbox-go"
)

type Config struct {
	tabspaces int
	bg        termbox.Attribute
	// TODO keyMap *KeyMap
}

type Handle struct {
	buf     *bytes.Buffer
	xOffset int
	yOffset int
	handler Handler
	config  *Config
}

// TODO consider removing and using io interfaces
type Handler interface {
	OnReachedEnd() error
	io.WriterTo
}

func New(handler Handler, config *Config) *Handle {
	h := new(Handle)
	h.handler = handler
	h.buf = new(bytes.Buffer)

	if config == nil {
		config = &Config{tabspaces: 4, bg: termbox.ColorDefault}
	}
	h.config = config

	return h
}

func (h *Handle) setCell(x, y int, r rune) {
	termbox.SetCell(x, y, r, h.config.bg, h.config.bg)
}

func (h *Handle) redraw() error {
	var err error
	if err = termbox.Clear(h.config.bg, h.config.bg); err != nil {
		return err
	}

	view := bytes.NewBuffer(h.buf.Bytes())

	height, width := termbox.Size()
	termbox.SetCursor(0, width)

	height--

	// TODO should seek to offsets
	var c rune
	x := h.xOffset
	y := h.yOffset
	for {
		if c, _, err = view.ReadRune(); err != nil {
			break
		}
		// print command line
		// if height == y {
		// 	for _, r := range "" {
		// 		h.setCell(x, y, rune(r))
		// 	}
		// 	break
		// }
		switch c {
		case '\n':
			y++
			x = h.xOffset
			continue
		case '\t':
			h.setCell(x, y, c)
			x += h.config.tabspaces
		default:
			h.setCell(x, y, c)
			x++
		}
	}

	termbox.Flush()

	if err != io.EOF {
		return err
	}
	return nil
}

func (h *Handle) Run() error {
	var err error
	var data []byte
	if err = termbox.Init(); err != nil {
		return err
	}

	defer termbox.Close()

	if _, err = h.handler.WriteTo(h.buf); err != nil {
		return err
	}

	if _, err = h.buf.Write(data); err != nil {
		return err
	}

user:
	for {
		if err = h.redraw(); err != nil {
			return err
		}

		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			case termbox.KeyEsc:
				break user
			case termbox.KeyArrowLeft:
				h.xOffset++
			case termbox.KeyArrowRight:
				h.xOffset--
			case termbox.KeyArrowUp:
				h.yOffset++
			case termbox.KeyArrowDown:
				h.yOffset--
			default:
				switch ev.Ch {
				case 'q':
					break user
				// TODO case 'G':
				// 	h.yOffset = len(h.buf)
				case 'j':
					h.yOffset--
				case 'k':
					h.yOffset++
				case 'h':
					h.xOffset++
				case 'l':
					h.xOffset--
				case '/':
					fmt.Println("search")
				}
			}
		case termbox.EventError:
			return ev.Err
		}
	}
	return nil
}

package main

import (
	"strconv"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
)

type colorPaletteHandler struct {
	width, height int
	grid          tui.Component
}

func (h *colorPaletteHandler) Resize(width, height int) {
	if h.grid != nil {
		h.grid.Resize(width, height)
	}
	h.width = width
	h.height = height
}

func makeColorGrid() tui.Component {
	ret := make([][]tui.Component, 16)
	var nameNum int
	for y := 0; y < 16; y++ {
		ret[y] = make([]tui.Component, 16)
		for x := 0; x < 16; x++ {
			nameNum++
			attr := term.Attribute(nameNum)
			var name string
			switch attr {
			case term.ColorDefault:
				name = "ColorDefault"
			case term.ColorBlack:
				name = "ColorBlack"
			case term.ColorRed:
				name = "ColorRed"
			case term.ColorGreen:
				name = "ColorGreen"
			case term.ColorYellow:
				name = "ColorYellow"
			case term.ColorBlue:
				name = "ColorBlue"
			case term.ColorMagenta:
				name = "ColorMagenta"
			case term.ColorCyan:
				name = "ColorCyan"
			case term.ColorWhite:
				name = "ColorWhite"
			default:
				name = strconv.Itoa(nameNum)
			}
			ret[y][x] = component.StringBackgroundAttr(name,
				term.Attributes{Bg: attr}, 0, term.Attributes{Bg: attr})
		}
	}
	return component.Grid(ret)
}

func (h *colorPaletteHandler) Draw(w term.Writer) {
	if h.grid == nil {
		h.grid = makeColorGrid()
		h.grid.Resize(h.width, h.height)
	}
	h.grid.Draw(w)
}

func (h *colorPaletteHandler) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return
	}
	exit = ev.Key == term.KeyEsc
	return
}

func (h *colorPaletteHandler) Cursor() (pos term.Coordinates, show bool) {
	return
}

func (h *colorPaletteHandler) Man() tui.Manual {
	return tui.Manual{}
}

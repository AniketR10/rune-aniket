package less

import (
	"bytes"
	"container/list"

	termbox "github.com/nsf/termbox-go"
)

type cell struct {
	i  int // index of the rune this cell represents in the content buffer
	x  int
	y  int
	fg termbox.Attribute
	bg termbox.Attribute
}

type Buffer struct {
	next       *Buffer
	config     *Config
	contentBuf *bytes.Buffer
	contChan   chan []byte
	reslist    *list.List    // search result list
	result     *list.Element // current focused result
	search     []byte        // search term
	cells      map[int]cell  // color information used for printing to window
	columns    int
	rows       int
}

func (h *Buffer) Search(data []byte) {
	// reset result cells bg/fg
	for el := h.reslist.Front(); el != nil; el = el.Next() {
		c := el.Value.(cell)
		for i, slen := c.i, c.i+len(h.search); i < slen; i++ {
			h.cells[i] = cell{
				bg: h.config.Bg,
				fg: h.config.Fg,
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
				fg: h.config.Resfg,
				bg: h.config.Resbg,
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

package viewer

import (
	"bytes"
	"container/list"

	"github.com/ernestrc/fractal"
)

type Buffer struct {
	// TODO should intercept writes
	bytes.Buffer
	reslist      *list.List    // search result list
	result       *list.Element // current focused result
	searchText   []byte        // search term
	resfg, resbg fractal.Attribute
}

func NewBuffer(resfg, resbg fractal.Attribute) *Buffer {
	b := new(Buffer)
	b.reslist = new(list.List)
	b.resfg, b.resbg = resfg, resbg
	return b
}

func (b *Buffer) Reset() {
	b.reslist = b.reslist.Init()
	b.result = nil
	b.searchText = nil
	b.Buffer.Reset()
}

func (b *Buffer) prevResult() (int, bool) {
	if b.result == nil {
		b.result = b.reslist.Back()
	} else {
		b.result = b.result.Prev()
	}

	if b.result == nil {
		return 0, false
	}

	return b.result.Value.(int), true
}

func (b *Buffer) nextResult() (int, bool) {
	if b.result == nil {
		b.result = b.reslist.Front()
	} else {
		b.result = b.result.Next()
	}

	if b.result == nil {
		return 0, false
	}

	return b.result.Value.(int), true
}

func (b *Buffer) searchResults() *list.List {
	return b.reslist
}

func (b *Buffer) search(text []byte, cellbuf map[int]fractal.Cell) {
	b.reslist = b.reslist.Init()
	b.result = nil
	b.searchText = text

	tlen := len(text)
	if tlen == 0 {
		return
	}

	view := b.Bytes()

	var a, i int
	for {
		if i = bytes.Index(view, text); i == -1 {
			break
		}

		// use anchor to translate index to original slice
		a += i

		for j, last := a, a+tlen; j < last; j++ {
			cellbuf[j] = fractal.Cell{
				Fg: b.resfg,
				Bg: b.resbg,
				Ch: cellbuf[j].Ch,
				X:  cellbuf[j].X,
				Y:  cellbuf[j].Y,
			}
		}

		// mark first fractal.Cell as result index
		b.reslist.PushBack(a)

		view = view[i+tlen:]

		// set next anchor
		a += tlen
	}

	return
}

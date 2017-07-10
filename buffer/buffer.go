package buffer

import (
	"bytes"
	"container/list"

	termbox "github.com/nsf/termbox-go"
)

type Cell struct {
	Idx int // index of the rune this Cell represents in the content buffer
	X   int
	Y   int
	FG  termbox.Attribute
	BG  termbox.Attribute
}

type Buffer struct {
	// TODO next    *Buffer
	data    *bytes.Buffer
	reslist *list.List    // search result list
	result  *list.Element // current focused result
	search  []byte        // search term
}

func New() *Buffer {
	b := new(Buffer)
	b.data = new(bytes.Buffer)
	b.reslist = new(list.List)

	return b
}

func (b *Buffer) Bytes() []byte {
	return b.data.Bytes()
}

func (b *Buffer) Reset() {
	// FIXME should reset other properties
	// b.reslist = b.reslist.Init()
	// b.result = nil
	// b.search = nil
	b.data.Reset()
}

func (b *Buffer) Write(p []byte) (n int, err error) {
	return b.data.Write(p)
}

func (b *Buffer) ReadRune() (r rune, size int, err error) {
	return b.data.ReadRune()
}

func (b *Buffer) Len() int {
	return b.data.Len()
}

func (b *Buffer) Truncate(n int) {
	b.data.Truncate(n)
}

func (b *Buffer) WriteRune(r rune) (int, error) {
	return b.data.WriteRune(r)
}

func (b *Buffer) PrevResult() (c Cell, ok bool) {
	if b.result == nil {
		b.result = b.reslist.Back()
	} else {
		b.result = b.result.Prev()
	}

	if b.result == nil {
		return
	}

	return b.result.Value.(Cell), true
}

func (b *Buffer) NextResult() (c Cell, ok bool) {
	if b.result == nil {
		b.result = b.reslist.Front()
	} else {
		b.result = b.result.Next()
	}

	if b.result == nil {
		return
	}

	return b.result.Value.(Cell), true
}

// TODO should not know anything about palette
func (b *Buffer) Search(text []byte, palette map[int]Cell,
	fg, bg, resfg, resbg termbox.Attribute) {
	// reset result Cells bg/fg
	for el := b.reslist.Front(); el != nil; el = el.Next() {
		c := el.Value.(Cell)
		for i, slen := c.Idx, c.Idx+len(b.search); i < slen; i++ {
			palette[i] = Cell{
				BG:  bg,
				FG:  fg,
				X:   palette[i].X,
				Y:   palette[i].Y,
				Idx: i,
			}
		}
	}

	b.reslist = b.reslist.Init()
	b.result = nil
	b.search = text

	tlen := len(text)
	if tlen == 0 {
		return
	}

	view := b.data.Bytes()

	a := 0
	var i int
	for {
		if i = bytes.Index(view, text); i == -1 {
			break
		}

		// use anchor to translate index to original slice
		a += i

		for j, last := a, a+tlen; j < last; j++ {
			palette[j] = Cell{
				FG:  resfg,
				BG:  resbg,
				X:   palette[j].X,
				Y:   palette[j].Y,
				Idx: j,
			}
		}

		// mark first Cell as result index
		b.reslist.PushBack(palette[a])

		view = view[i+tlen:]

		// set next anchor
		a += tlen
	}

	return
}

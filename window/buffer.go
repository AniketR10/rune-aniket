package window

import (
	"bytes"
	"container/list"

	"github.com/ernestrc/fractal"
)

// Buffer is a write-only wrapper around bytes.Buffer that
// keeps track of updates to the underlying buffer so
// window and other components know when to re-scan
type Buffer struct {
	buffer       bytes.Buffer
	reslist      *list.List    // search result list
	result       *list.Element // current focused result
	searchText   []byte        // search term
	resfg, resbg fractal.Attribute
	scanned      bool
}

func NewBuffer(resfg, resbg fractal.Attribute) *Buffer {
	b := new(Buffer)
	b.reslist = new(list.List)
	b.resfg, b.resbg = resfg, resbg
	return b
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

func (b *Buffer) search(text []byte, cellbuf []fractal.Cell) {
	b.reslist = b.reslist.Init()
	b.result = nil
	b.searchText = text

	tlen := len(text)
	if tlen == 0 {
		return
	}

	view := b.buffer.Bytes()

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
				Coordinates: fractal.Coordinates{
					X: cellbuf[j].X,
					Y: cellbuf[j].Y,
				},
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

func (b *Buffer) bytes() []byte {
	return b.buffer.Bytes()
}

func (b *Buffer) markScanned() {
	b.scanned = true
}

func (b *Buffer) markUnscanned() {
	b.scanned = false
}

func (b *Buffer) Scanned() bool {
	return b.scanned
}

func (b *Buffer) Reset() {
	b.markUnscanned()
	b.reslist = b.reslist.Init()
	b.result = nil
	b.searchText = nil
	b.buffer.Reset()
}

func (b *Buffer) Len() int {
	return b.buffer.Len()
}

func (b *Buffer) Cap() int {
	return b.buffer.Cap()
}

func (b *Buffer) Grow(n int) {
	b.buffer.Grow(n)
}
func (b *Buffer) String() string {
	return b.buffer.String()
}
func (b *Buffer) Truncate(n int) {
	b.markUnscanned()
	b.buffer.Truncate(n)
}

func (b *Buffer) Write(p []byte) (n int, err error) {
	b.markUnscanned()
	return b.buffer.Write(p)
}
func (b *Buffer) WriteRune(r rune) (n int, err error) {
	b.markUnscanned()
	return b.buffer.WriteRune(r)
}

func (b *Buffer) WriteByte(c byte) error {
	b.markUnscanned()
	return b.buffer.WriteByte(c)
}
func (b *Buffer) WriteString(s string) (n int, err error) {
	b.markUnscanned()
	return b.buffer.WriteString(s)
}

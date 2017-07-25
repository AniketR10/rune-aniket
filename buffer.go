package fractal

import (
	"bytes"
	"container/list"
)

// Buffer is a mutable write, immutable read wrapper of bytes.Buffer which:
//
// - keeps track of updates to the underlying buffer so
//	 window and other components know when to re-scan
//
// - provides a cell-aware search API
//
// The zero value for Buffer is an empty buffer ready to use.
type Buffer struct {
	buffer     bytes.Buffer
	reslist    list.List     // search result list
	result     *list.Element // current focused result
	searchText []byte        // search term
	scanned    bool
}

func (b *Buffer) Reset() {
	b.buffer.Reset()
	b.reslist.Init()
	b.result = nil
	b.searchText = nil
	b.MarkUnscanned()
}

func (b *Buffer) SearchText() []byte {
	return b.searchText
}

func (b *Buffer) PrevResult() (int, bool) {
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

func (b *Buffer) NextResult() (int, bool) {
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

func (b *Buffer) SearchResults() *list.List {
	return &b.reslist
}

func (b *Buffer) Search(text []byte, cellbuf []Cell, resfg, resbg Attribute) int {
	b.reslist.Init()
	b.result = nil
	b.searchText = text

	tlen := len(text)
	if tlen == 0 {
		return 0
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
			cellbuf[j] = Cell{
				Fg: resfg,
				Bg: resbg,
				Ch: cellbuf[j].Ch,
				Coordinates: Coordinates{
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

	return b.reslist.Len()
}

func (b *Buffer) Bytes() []byte {
	return b.buffer.Bytes()
}

func (b *Buffer) MarkScanned() {
	b.scanned = true
}

func (b *Buffer) MarkUnscanned() {
	b.scanned = false
}

func (b *Buffer) Scanned() bool {
	return b.scanned
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
	b.MarkUnscanned()
	b.buffer.Truncate(n)
}

func (b *Buffer) Write(p []byte) (n int, err error) {
	b.MarkUnscanned()
	return b.buffer.Write(p)
}
func (b *Buffer) WriteRune(r rune) (n int, err error) {
	b.MarkUnscanned()
	return b.buffer.WriteRune(r)
}

func (b *Buffer) WriteByte(c byte) error {
	b.MarkUnscanned()
	return b.buffer.WriteByte(c)
}
func (b *Buffer) WriteString(s string) (n int, err error) {
	b.MarkUnscanned()
	return b.buffer.WriteString(s)
}

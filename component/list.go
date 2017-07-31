package component

import (
	"container/list"

	"github.com/ernestrc/fractal"
)

const resfg, resbg = fractal.AttrReverse, fractal.AttrReverse

type List struct {
	elementHeight int
	width, height int
	pos           fractal.Coordinates
	offset        int
	list.List     // list of fractal.Component
}

func NewList(elementHeight, width, height, x, y int) (l *List) {
	l = new(List)
	l.Init(elementHeight, width, height, x, y)
	return l
}

func (l *List) Reset() {
	l.List.Init()
}

func (l *List) Init(elementHeight, width, height, x, y int) {
	if elementHeight <= 0 {
		panic("element height cannot be smaller than or equal to 0")
	}
	l.elementHeight = elementHeight
	l.width, l.height, l.pos.X, l.pos.Y = width, height, x, y
	l.List.Init()
}

func (l *List) SetElementHeight(height int) (err error) {
	l.elementHeight = height
	return l.Resize(l.width, l.height)
}

func (l *List) ElementHeight() int {
	return l.elementHeight
}

func (l *List) Offset() int {
	return l.offset
}

func (l *List) CanSeekUp() bool {
	return l.offset > 0
}

func (l *List) CanSeekDown() bool {
	return l.offset < l.Len()-l.height/l.elementHeight
}

func (l *List) SeekUp() {
	if l.CanSeekUp() {
		l.offset--
	}
}

func (l *List) SeekDown() {
	if l.CanSeekDown() {
		l.offset++
	}
}

func (l *List) SeekEnd() {
	for l.CanSeekDown() {
		l.offset++
	}
}

func (l *List) SeekStart() {
	l.offset = 0
}

func (l *List) Resize(width, height int) (err error) {
	l.width, l.height = width, height
	var comp fractal.Component
	for i, el := 0, l.Front(); el != nil; el, i = el.Next(), i+1 {
		comp = el.Value.(fractal.Component)
		if err = comp.Resize(l.width, l.elementHeight); err != nil {
			return
		}
		ypos := l.pos.Y + (i-l.offset)*l.elementHeight
		if err = comp.Move(l.pos.X, ypos); err != nil {
			return
		}
	}
	return
}

func (l *List) Move(x, y int) (err error) {
	l.pos.X, l.pos.Y = x, y

	return l.Resize(l.width, l.height)
}

func (l *List) Draw(w fractal.Writer) (err error) {
	// API exposes internal list so we need
	// to make sure that the elements are properly position and sized
	// before drawing
	if err = l.Resize(l.width, l.height); err != nil {
		return
	}

	lastVisible := l.height/l.elementHeight + l.offset

	for i, el := 0, l.Front(); i < lastVisible && el != nil; i, el = i+1, el.Next() {
		if i < l.offset {
			continue
		}
		if err = el.Value.(fractal.Component).Draw(w); err != nil {
			return
		}
	}

	return
}

func (l *List) Height() int {
	return l.height
}

func (l *List) Width() int {
	return l.width
}

func (l *List) Position() (int, int) {
	return l.pos.X, l.pos.Y
}

func (l *List) PushBackList(other *List) {
	if l == other {
		panic("other list cannot be self: components can't be deep cloned")
	}
	l.List.PushBackList(&other.List)
}

func (l *List) PushFrontList(other *List) {
	if l == other {
		panic("other list cannot be self: components can't be deep cloned")
	}
	l.List.PushFrontList(&other.List)
}

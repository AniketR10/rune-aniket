package component

import (
	"unsafe"

	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

type floatingNode struct {
	at                  term.Coordinates
	fwidth, fheight     int // calculated upon Resize, considering trimming
	maxWidth, maxHeight int // wm size
	content             Virtual
	wm                  *WindowManager
}

func newFloatingNode(
	wm *WindowManager, content Floating,
	at term.Coordinates,
	maxWidth, maxHeight int,
) *floatingNode {
	if at.Y < 0 || at.X < 0 {
		panic("invalid floating window coordinates")
	}
	ret := new(floatingNode)
	ret.at = at
	ret.maxWidth = maxWidth
	ret.maxHeight = maxHeight
	ret.wm = wm
	ret.SetContentResize(content, true)
	return ret
}

func (w *floatingNode) ID() uint64 {
	return uint64(uintptr(unsafe.Pointer(w)))
}

func (w *floatingNode) Width() int {
	return w.fwidth
}

func (w *floatingNode) Height() int {
	return w.fheight
}

func (w *floatingNode) Content() tui.Component {
	return w.content.C
}

func (w *floatingNode) Draw(wr term.Writer) {
	w.content.Draw(wr)
}

func (w *floatingNode) SetContentResize(c tui.Component, resize bool) (
	prev tui.Component,
) {
	prev = w.content.C
	w.content.C = c
	if resize {
		w.resize()
	}
	return
}

func (w *floatingNode) Size() int {
	return 1
}

func (w *floatingNode) Close() {
	if w.wm == nil {
		return
	}

	wm := w.wm
	w.wm = nil
	wm.closeFloatingWindow(w)
}

func (w *floatingNode) Position() term.Coordinates {
	return w.at
}

func (w *floatingNode) resize() {
	pos := w.Position()
	if pos.Y >= w.maxHeight || pos.X >= w.maxWidth {
		return
	}

	w.fwidth, w.fheight = w.content.C.(Floating).Dimensions()
	if pos.Y+w.fheight >= w.maxHeight {
		w.fheight = w.maxHeight - pos.Y
	}
	if pos.X+w.fwidth >= w.maxWidth {
		w.fwidth = w.maxWidth - pos.X
	}
	w.content.Move(w.at)
	w.content.Resize(w.fwidth, w.fheight)
}

func (w *floatingNode) SetMaxSize(width, height int) {
	w.maxWidth = width
	w.maxHeight = height
	w.resize()
}

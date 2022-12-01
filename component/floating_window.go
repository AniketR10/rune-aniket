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
	dimwidth, dimheight int // content desired Dimensions size
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
	if f, ok := w.content.C.(prevNodeFloating); ok {
		return f.Component
	}
	return w.content.C
}

func (w *floatingNode) Draw(wr term.Writer) {
	dimwidth, dimheight := w.dimwidth, w.dimheight
	w.updateDimensions()
	if dimwidth != w.dimwidth || dimheight != w.dimheight {
		w.resize()
	}
	w.content.Draw(wr)
}

func (w *floatingNode) SetContentResize(c tui.Component, resize bool) (
	prev tui.Component,
) {
	var ok bool

	if w.wm.config.Frame {
		_, ok = c.(*Frame).Content().(Floating)
	} else {
		_, ok = c.(Floating)
	}

	if !ok {
		// Window.SetContent could pass a non Floating component
		// if that's the case, then set it to a static floating element which
		// uses the last Floating's desired dimensions
		c = prevNodeFloating{Component: c, width: w.dimwidth, height: w.dimheight}
	}

	prev = w.content.C
	if f, ok := prev.(prevNodeFloating); ok {
		prev = f.Component // unwrap
	}
	w.content.C = c
	if resize {
		w.updateDimensions()
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

func (w *floatingNode) updateDimensions() {
	w.dimwidth, w.dimheight = w.content.C.(Floating).Dimensions()
}

func (w *floatingNode) resize() {
	pos := w.Position()
	if pos.Y >= w.maxHeight || pos.X >= w.maxWidth {
		return
	}

	w.fwidth, w.fheight = w.dimwidth, w.dimheight
	if pos.Y+w.fheight >= w.maxHeight {
		w.fheight = w.maxHeight - pos.Y
	}
	if pos.X+w.fwidth >= w.maxWidth {
		w.fwidth = w.maxWidth - pos.X
	}
	w.content.Move(w.at)
	w.content.Resize(w.fwidth, w.fheight)
}

func (w *floatingNode) Resize(width, height int) {
	panic("called resize on a floating node")
}

func (w *floatingNode) SetMaxSize(width, height int) {
	w.maxWidth = width
	w.maxHeight = height
	w.updateDimensions()
	w.resize()
}

// Floating used to indicate that it should be unwrapped in calls to Content
// or as a return of SetContentResize
type prevNodeFloating struct {
	tui.Component
	width, height int
}

func (p prevNodeFloating) Dimensions() (width, height int) {
	return p.width, p.height
}

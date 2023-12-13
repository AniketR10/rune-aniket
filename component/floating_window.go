package component

import (
	"unsafe"

	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

type floatingNode struct {
	desiredOffset               term.Coordinates // desired offset
	alignment                   Alignment        // desired alignment
	maxWidth, maxHeight         int              // window space size
	desiredWidth, desiredHeight int              // content desired Dimensions size

	realWidth, realHeight int              // calculated upon Resize, considering trimming
	realOffset            term.Coordinates // calculated offset with alignment

	wm      *WindowManager
	content Virtual
}

func newFloatingNode(
	wm *WindowManager, content Floating,
	cfg FloatingConfig,
	maxWidth, maxHeight int,
) *floatingNode {
	if cfg.Offset.Y < 0 || cfg.Offset.X < 0 {
		panic("invalid floating window coordinates")
	}
	ret := new(floatingNode)
	ret.desiredOffset = cfg.Offset
	ret.alignment = cfg.Alignment
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
	return w.realWidth
}

func (w *floatingNode) Height() int {
	return w.realHeight
}

func (w *floatingNode) Content() tui.Component {
	if f, ok := w.content.C.(prevNodeFloating); ok {
		return f.Component
	}
	return w.content.C
}

func (w *floatingNode) Draw(wr term.Writer) {
	desiredWidth, desiredHeight := w.desiredWidth, w.desiredHeight
	w.updateDesiredDimensions()
	if desiredWidth != w.desiredWidth || desiredHeight != w.desiredHeight {
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
		c = prevNodeFloating{Component: c, width: w.desiredWidth, height: w.desiredHeight}
	}

	prev = w.content.C
	if f, ok := prev.(prevNodeFloating); ok {
		prev = f.Component // unwrap
	}
	w.content.C = c
	if resize {
		w.updateDesiredDimensions()
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
	return w.realOffset
}

func (w *floatingNode) updateDesiredDimensions() {
	w.desiredWidth, w.desiredHeight = w.content.C.(Floating).Dimensions()
}

func (w *floatingNode) resize() {
	verticalDiff := w.maxHeight - w.desiredHeight
	horizontalDiff := w.maxWidth - w.desiredWidth

	if verticalDiff < 0 {
		verticalDiff = 0
	}
	if horizontalDiff < 0 {
		horizontalDiff = 0
	}

	var offset term.Coordinates
	if w.alignment&SpanAlignmentVerticallyCentered != 0 {
		offset.Y = verticalDiff / 2
	} else if w.alignment&SpanAlignmentBottom != 0 {
		offset.Y = verticalDiff
		offset.Y -= w.desiredOffset.Y
	} else {
		offset.Y += w.desiredOffset.Y
	}

	if w.alignment&SpanAlignmentHorizontallyCentered != 0 {
		offset.X = horizontalDiff / 2
	} else if w.alignment&SpanAlignmentRight != 0 {
		offset.X = horizontalDiff
		offset.X -= w.desiredOffset.X
	} else {
		offset.X += w.desiredOffset.X
	}

	w.realOffset = offset
	if w.realOffset.X < 0 {
		w.realOffset.X = 0
	}
	if w.realOffset.Y < 0 {
		w.realOffset.Y = 0
	}

	w.realWidth, w.realHeight = w.desiredWidth, w.desiredHeight
	if w.realOffset.Y+w.realHeight >= w.maxHeight {
		w.realHeight = w.maxHeight - w.realOffset.Y
	}
	if w.realOffset.X+w.realWidth >= w.maxWidth {
		w.realWidth = w.maxWidth - w.realOffset.X
	}

	if w.realWidth < 0 {
		w.realWidth = 0
	}

	if w.realHeight < 0 {
		w.realHeight = 0
	}

	if w.realOffset.Y >= w.maxHeight || w.realOffset.X >= w.maxWidth {
		return
	}
	w.content.Resize(w.realWidth, w.realHeight)
	w.content.Move(w.realOffset)
}

func (w *floatingNode) Resize(width, height int) {
	panic("called resize on a floating node")
}

func (w *floatingNode) SetMaxSize(width, height int) {
	w.maxWidth = width
	w.maxHeight = height
	w.updateDesiredDimensions()
	w.resize()
}

func (t *floatingNode) Closed() bool {
	return t.wm == nil
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

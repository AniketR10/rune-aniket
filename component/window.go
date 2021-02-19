package component

import (
	"errors"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

var errCalledZeroValuedWin = "called method on zero-valued Window"

type windowNode interface {
	ID() uint64
	Width() int
	Height() int
	Content() tui.Component
	SetContent(tui.Component) tui.Component
	Size() int
	Close()
	Position() term.Coordinates
}

// Window represents a tiled window in a WindowManager.
type Window struct {
	wm   *WindowManager
	node windowNode
}

// Position returns the position of this Window, or false
// if this Window is a zero-valued Window.
func (w Window) Position() (pos term.Coordinates, ok bool) {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	ok = true
	pos = w.node.Position()
	return
}

// Width returns the width of this Window.
func (w Window) Width() int {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	return w.node.Width()
}

// Height returns the width of this Window.
func (w Window) Height() int {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	return w.node.Height()
}

// Content returns the content of this Window, or false
// if this Window is a zero-valued Window.
func (w Window) Content() (c tui.Component) {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}

	if w.wm.config.Frame {
		c = w.node.Content().(*Frame).Content().(tui.Component)
	} else {
		c = w.node.Content().(tui.Component)
	}

	return
}

// SetContent sets the content of this Window to content and
// returns the previous content.
func (w Window) SetContent(content tui.Component) (
	prev tui.Component,
) {
	prev = w.Content()
	if w.wm.config.Frame {
		content = w.wm.withFrame(content)
	}
	w.node.SetContent(content)
	return
}

// SetFrameAttr sets a Window's FrameCharSet default attributes.
// Any Window's frame attributes can be reset by calling SetDefaultAttr
// which sets the default attributes for all windows.
func (w Window) SetFrameAttr(attr term.Attributes) bool {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	if !w.wm.config.Frame {
		return false
	}
	w.node.Content().(*Frame).Attributes = attr
	return true
}

// SetFrameCharSet sets a Window's Frame attributes. This can be reset by calling
// SetDefaultFrameCharSet which sets the default attributes for all windows.
func (w Window) SetFrameCharSet(b FrameCharSet) bool {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	if !w.wm.config.Frame {
		return false
	}
	w.node.Content().(*Frame).FrameCharSet = b
	return true
}

// FrameAttr return this Window's default FrameCharSet attributes or false
// if this Window belongs to a WindowManager configured to not use borders.
func (w Window) FrameAttr() (attr term.Attributes, ok bool) {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	if !w.wm.config.Frame {
		return
	}
	ok = true
	attr = w.node.Content().(*Frame).Attributes
	return
}

// FrameCharSet return this Window's configured FrameCharSet or false
// if this Window belongs to a WindowManager configured to not use borders.
func (w Window) FrameCharSet() (b FrameCharSet, ok bool) {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	if !w.wm.config.Frame {
		return
	}
	ok = true
	b = w.node.Content().(*Frame).FrameCharSet
	return
}

// Size returns the total number of win under this Window.
func (w Window) Size() int {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	return w.node.Size()
}

// TileDown returns the window in the bottom of t or false if t is the
// bottom-most window in the WindowManager.
func (w Window) TileDown() (ret Window, ok bool) {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	t, ok := w.node.(*TileNode)
	if !ok {
		pos := w.node.Position()
		pos.Y += w.node.Height()
		pos.Y++
		return w.wm.WindowAt(pos)
	}
	node := t.TileDown()
	if node == nil {
		ok = false
		return
	}
	return w.wm.nodeToWindow(node), true
}

// TileLeft returns the window left-adjacent to t or false if t is the
// left-most window in the WindowManager.
func (w Window) TileLeft() (ret Window, op bool) {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	t, ok := w.node.(*TileNode)
	if !ok {
		pos := w.node.Position()
		pos.X--
		return w.wm.WindowAt(pos)
	}
	node := t.TileLeft()
	if node == nil {
		ok = false
		return

	}
	return w.wm.nodeToWindow(node), true
}

// TileRight returns the window right-adjacent to t or false if t is the
// right-most window in the tree.
func (w Window) TileRight() (ret Window, ok bool) {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	t, ok := w.node.(*TileNode)
	if !ok {
		pos := w.node.Position()
		pos.X += w.node.Width()
		pos.X++
		return w.wm.WindowAt(pos)
	}
	node := t.TileRight()
	if node == nil {
		ok = false
		return

	}
	return w.wm.nodeToWindow(node), true
}

// TileUp returns the window on top of t or false if t is the
// top-most window in the tree.
func (w Window) TileUp() (ret Window, ok bool) {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	t, ok := w.node.(*TileNode)
	if !ok {
		pos := w.node.Position()
		pos.Y--
		return w.wm.WindowAt(pos)
	}
	node := t.TileUp()
	if node == nil {
		ok = false
		return

	}
	return w.wm.nodeToWindow(node), true
}

// ID returns a unique identifier for this window.
func (w Window) ID() uint64 {
	return w.node.ID()
}

// Close removes this Window from the WindowManager
// It returns an error if window is last window in the WindowManager.
func (w Window) Close() error {
	// zero-valued Window
	if w.wm == nil {
		return nil
	}

	if w.wm.Size() == 1 {
		return errors.New("trying to close last node")
	}

	w.node.Close()
	return nil
}

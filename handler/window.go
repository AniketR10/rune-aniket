package handler

import (
	"errors"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/component"
)

var errCalledZeroValuedWin = "called method on zero-valued Window"

// Window represents a tiled window in a WindowManager.
type Window struct {
	component.Window
	wm *WindowManager
}

// Content returns the content of t.
func (w Window) Content() tui.Handler {
	if w.wm == nil {
		panic(errCalledZeroValuedWin)
	}
	c := w.Window.Content()
	return c.(tui.Handler)
}

// SetContent sets the content of the window to h.
func (w Window) SetContent(h tui.Handler) (
	prev tui.Handler,
) {
	comp := w.Window.Content()
	prev = comp.(tui.Handler)
	w.Window.SetContent(h)
	return
}

// Size returns the total number of win under this Window.
func (w Window) Size() (size int) {
	return w.Window.Size()
}

// TileDown returns the tile in the bottom of t or false if t is the
// bottom-most tile in the tree.
func (w Window) TileDown() (Window, bool) {
	win, ok := w.Window.TileDown()
	return w.wm.newNode(win), ok
}

// TileLeft returns the tile left-adjacent to t or false if t is the
// left-most tile in the tree.
func (w Window) TileLeft() (Window, bool) {
	win, ok := w.Window.TileLeft()
	return w.wm.newNode(win), ok
}

// TileRight returns the tile right-adjacent to t or false if t is the
// right-most tile in the tree.
func (w Window) TileRight() (Window, bool) {
	win, ok := w.Window.TileRight()
	return w.wm.newNode(win), ok
}

// TileUp returns the tile on top of t or false if t is the
// top-most tile in the tree.
func (w Window) TileUp() (Window, bool) {
	win, ok := w.Window.TileUp()
	return w.wm.newNode(win), ok
}

// Close removes this window from the tree.
// It returns an error if window is last window on the WindowManager.
func (w Window) Close() error {
	if w.wm == nil {
		return nil
	}

	ok := true
	if w.wm.focus == w {
		ok = w.wm.ShiftFocus()
	}
	if !ok {
		return errors.New("trying to close last window")
	}

	w.Window.Close()

	return nil
}

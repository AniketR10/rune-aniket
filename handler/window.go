package handler

import (
	"errors"

	"unstable.build/go-tui"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
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

// Frame returns this window's frame component and true or nil and
// false if this window belongs to a window manager configured
// without frames.
func (w Window) Frame() (*component.Frame, bool) {
	return w.Window.Frame()
}

// Focus returns true if window is in focus.
func (w Window) Focus() bool {
	return w.wm.focus == w
}

func (w Window) setContentResize(h tui.Handler, resize bool) (
	prev tui.Handler,
) {
	comp := w.Window.Content()
	prev = comp.(tui.Handler)
	w.Window.SetContentResize(h, resize)
	// SetContent creates a new frame if necessary
	// make sure that the frame created is set with
	// the focus attr if this Window is in focus
	if w.wm.focus.ID() == w.ID() {
		w.wm.setFocusAttr(w)
	}
	return prev
}

// SetContent sets the content of the window to h.
func (w Window) SetContent(h tui.Handler) (
	prev tui.Handler,
) {
	return w.setContentResize(h, true)
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

// Position returns this Window's position offset from the window
// manager's relative position.
func (w Window) Position() term.Coordinates {
	return w.Window.Position()
}

// Width returns the width of this window.
func (w Window) Width() int {
	return w.Window.Width()
}

// Height returns the width of this window.
func (w Window) Height() int {
	return w.Window.Height()
}

// Close removes this window from the tree.
// It returns an error if window is last window on the WindowManager.
func (w Window) Close() error {
	if w.wm == nil {
		return nil
	}

	if w.wm.SizeTiles() == 1 && !w.IsFloating() {
		return errors.New("trying to close last window")
	}

	if w.wm.prevFocus == w {
		w.wm.prevFocus = Window{}
	}

	if w.wm.focus == w {
		w.wm.ShiftFocus()
		w.wm.prevFocus = Window{}
	}

	err := w.Window.Close()
	if err != nil {
		return err
	}

	// make sure that focus attrs are "reset" if wm size is 1
	w.wm.setFocusAttr(w.wm.Focus())

	return nil
}

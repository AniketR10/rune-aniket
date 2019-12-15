package handler

import (
	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
	"github.com/ernestrc/fractal/term"
)

// WindowManager implements Handler as a tiled window manager.
type WindowManager struct {
	component.TileTree
	focus      *component.TileNode
	border     bool
	borderAttr term.Attributes
	focusAttr  term.Attributes
}

// NewWindowManager allocates storage for a new WindowManager and initializes it with the
// given handler. If border is true, it will draw a border around every tile.
func NewWindowManager(handler fractal.Handler, border bool) (wm *WindowManager) {
	wm = new(WindowManager)
	wm.Init(handler, border)
	return
}

// Init initializes this WindowManager with the given handler. If border is true, it will draw
// a border around every tile.
func (wm *WindowManager) Init(handler fractal.Handler, border bool) {
	if border {
		wm.borderAttr.Fg = term.ColorDefault
		wm.borderAttr.Bg = term.ColorDefault
		wm.focusAttr.Fg = term.ColorRed
		wm.focusAttr.Bg = term.ColorDefault
		handler = NewFrame(handler, wm.borderAttr)
	}
	tile := wm.TileTree.Init(handler)
	wm.focus = tile
	wm.border = border
	return
}

// SetAttr sets the default and focus window border attributes.
func (wm *WindowManager) SetAttr(standard, focus term.Attributes) {
	wm.borderAttr = standard
	wm.focusAttr = focus
}

// Handle : Handler
func (wm *WindowManager) Handle(ev term.Event) (exit bool) {
	if ev.Type == term.EventKey && ev.Mod == term.ModAlt {
		switch ev.Ch {
		case 'q':
			exit = true
		case 'k':
			wm.FocusUp()
		case 'j':
			wm.FocusDown()
		case 'h':
			wm.FocusLeft()
		case 'l':
			wm.FocusRight()
		}
		return
	}

	if ev.Type == term.EventMouse {
		mousePos := term.Coordinates{X: ev.MouseX, Y: ev.MouseY}
		childAtMouse := wm.TileTree.TileAt(mousePos)
		if wm.Focus() != childAtMouse {
			if ev.Key == term.MouseLeft {
				wm.SetFocus(childAtMouse)
			}
			return
		}
		offset := wm.TileTree.TilePosition(childAtMouse)
		ev.MouseX -= offset.X
		ev.MouseY -= offset.Y
	}

	hexit := wm.getFocusHandler().Handle(ev)

	// if handler in focus wants to exit, close the window,
	// or signal exit to upstream handler if it was last window
	if hexit {
		if exit = wm.Size() == 1; exit {
			return
		}
		curr := wm.focus
		if !wm.FocusLeft() && !wm.FocusUp() && !wm.FocusRight() {
			wm.FocusDown()
		}
		curr.Close()
	}

	return
}

// SplitVertical creates a new vertical split over the tile currently in focus.
func (wm *WindowManager) SplitVertical(h fractal.Handler) *component.TileNode {
	if wm.border {
		h = NewFrame(h, wm.borderAttr)
	}
	return wm.TileTree.SplitVertical(wm.focus, h)
}

// SplitHorizontal creates a new horizontal split over the tile currently in focus.
func (wm *WindowManager) SplitHorizontal(h fractal.Handler) *component.TileNode {
	if wm.border {
		h = NewFrame(h, wm.borderAttr)
	}
	return wm.TileTree.SplitHorizontal(wm.focus, h)

}

func (wm *WindowManager) switchFocus(tile *component.TileNode) bool {
	if tile == nil {
		return false
	}

	wm.SetFocus(tile)
	return true
}

// FocusLeft switches the focus to the tile on the left side of the tile in focus
// If the tile in focus is the left-most tile in this window manager, then this method does nothing.
func (wm *WindowManager) FocusLeft() bool {
	return wm.switchFocus(wm.focus.TileLeft())
}

// FocusRight switches the focus to the tile on the right side of the tile in focus
// If the tile in focus is the right-most tile in this window manager, then this method does nothing.
func (wm *WindowManager) FocusRight() bool {
	return wm.switchFocus(wm.focus.TileRight())
}

// FocusUp switches the focus to the tile above the tile in focus
// If the tile in focus is the up-most tile in this window manager, then this method does nothing.
func (wm *WindowManager) FocusUp() bool {
	return wm.switchFocus(wm.focus.TileUp())
}

// FocusDown switches the focus to the tile beneath the tile in focus
// If the tile in focus is the down-most tile in this window manager, then this method does nothing.
func (wm *WindowManager) FocusDown() bool {
	return wm.switchFocus(wm.focus.TileDown())
}

func (wm *WindowManager) getFocusHandler() fractal.Handler {
	return wm.focus.Content().(fractal.Handler)
}

// Focus returns the tile currently in focus.
func (wm *WindowManager) Focus() *component.TileNode {
	return wm.focus
}

// SetFocus sets the passed tile in focus. It returns the previous tile in focus.
// The behaviour is undefined if the given tile is not part of this WindowManager.
func (wm *WindowManager) SetFocus(tile *component.TileNode) (
	prev *component.TileNode,
) {
	if wm.border {
		wm.focus.Content().(*Frame).SetAttr(wm.borderAttr)
		tile.Content().(*Frame).SetAttr(wm.focusAttr)
	}
	prev = wm.focus
	wm.focus = tile
	return
}

// Cursor returns the cursor coordinates of the tile in focus.
func (wm *WindowManager) Cursor() (term.Coordinates, bool) {
	offset := wm.TileTree.TilePosition(wm.focus)
	cursor, show := wm.focus.Content().(fractal.Handler).Cursor()
	return term.Coordinates{X: offset.X + cursor.X, Y: offset.Y + cursor.Y}, show
}

// Man : Handler
func (wm *WindowManager) Man() fractal.Manual {
	return fractal.Manual{
		Summary: "WindowManager implements a tiled window manager.",
		Keys: fractal.KeyMap{
			term.Event{Mod: term.ModAlt, Ch: 'q'}: {
				ID:          "Exit",
				Description: "Exit handler.",
			},
			term.Event{Mod: term.ModAlt, Ch: 'j'}: {
				ID:          "FocusDown",
				Description: "Switch focus to tile below tile in focus.",
			},
			term.Event{Mod: term.ModAlt, Ch: 'k'}: {
				ID:          "FocusUp",
				Description: "Switch focus to tile above tile in focus.",
			},
			term.Event{Mod: term.ModAlt, Ch: 'h'}: {
				ID:          "FocusLeft",
				Description: "Switch focus to tile on the left of tile in focus.",
			},
			term.Event{Mod: term.ModAlt, Ch: 'l'}: {
				ID:          "FocusRight",
				Description: "Switch focus to tile on the right of tile in focus.",
			},
		},
	}
}

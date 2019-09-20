package fractal

import (
	"github.com/nsf/termbox-go"
)

// WindowManager implements Handler as a tiled window manager.
type WindowManager struct {
	TileTree
	focus  *TileNode
	border bool
}

// NewWindowManager allocates storage for a new WindowManager and initializes it with the
// given handler. If border is true, it will draw a border around every tile.
func NewWindowManager(handler Handler, border bool) (wm *WindowManager) {
	wm = new(WindowManager)
	wm.Init(handler, border)
	return
}

// Init initializes this WindowManager with the given handler. If border is true, it will draw
// a border around every tile.
func (wm *WindowManager) Init(handler Handler, border bool) {
	if border {
		handler = NewFrameProxy(handler, foreground, background)
	}
	tile := wm.TileTree.Init(handler)
	wm.focus = tile
	wm.border = border
	return
}

// Handle : Handler
func (wm *WindowManager) Handle(ev termbox.Event) (exit bool) {
	if ev.Type == termbox.EventKey && ev.Mod == termbox.ModAlt {
		switch ev.Ch {
		case 'q':
			exit = true
			return
		case 'k':
			wm.FocusUp()
			return
		case 'j':
			wm.FocusDown()
			return
		case 'h':
			wm.FocusLeft()
			return
		case 'l':
			wm.FocusRight()
			return
		}
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
func (wm *WindowManager) SplitVertical(h Handler) *TileNode {
	if wm.border {
		h = NewFrameProxy(h, foreground, background)
	}
	return wm.TileTree.SplitVertical(wm.focus, h)
}

// SplitHorizontal creates a new horizontal split over the tile currently in focus.
func (wm *WindowManager) SplitHorizontal(h Handler) *TileNode {
	if wm.border {
		h = NewFrameProxy(h, foreground, background)
	}
	return wm.TileTree.SplitHorizontal(wm.focus, h)

}

func (wm *WindowManager) switchFocus(tile *TileNode) bool {
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

func (wm *WindowManager) getFocusHandler() Handler {
	return wm.focus.Content().(Handler)
}

// Focus returns the tile currently in focus.
func (wm *WindowManager) Focus() *TileNode {
	return wm.focus
}

// SetFocus sets the passed tile in focus. It returns the previous tile in focus.
// The behaviour is undefined if the given tile is not part of this WindowManager.
func (wm *WindowManager) SetFocus(tile *TileNode) (prev *TileNode) {
	if wm.border {
		wm.focus.Content().(*FrameProxy).SetAttr(foreground, background)
		tile.Content().(*FrameProxy).SetAttr(highlightfg, highlightbg)
	}
	prev = wm.focus
	wm.focus = tile
	return
}

// GetCursor returns the cursor coordinates of the tile in focus.
func (wm *WindowManager) GetCursor() Coordinates {
	offset := wm.TileTree.TilePosition(wm.focus)
	cursor := wm.focus.Content().(Handler).GetCursor()
	return Coordinates{X: offset.X + cursor.X, Y: offset.Y + cursor.Y}
}

// Man : Handler
func (wm *WindowManager) Man() Manual {
	return Manual{
		Summary: "WindowManager implements a tiled window manager.",
		Keys: KeyMap{
			termbox.Event{Mod: termbox.ModAlt, Ch: 'q'}: {
				ID:          "Exit",
				Description: "Exit handler.",
			},
			termbox.Event{Mod: termbox.ModAlt, Ch: 'j'}: {
				ID:          "FocusDown",
				Description: "Switch focus to tile below tile in focus.",
			},
			termbox.Event{Mod: termbox.ModAlt, Ch: 'k'}: {
				ID:          "FocusUp",
				Description: "Switch focus to tile above tile in focus.",
			},
			termbox.Event{Mod: termbox.ModAlt, Ch: 'h'}: {
				ID:          "FocusLeft",
				Description: "Switch focus to tile on the left of tile in focus.",
			},
			termbox.Event{Mod: termbox.ModAlt, Ch: 'l'}: {
				ID:          "FocusRight",
				Description: "Switch focus to tile on the right of tile in focus.",
			},
		},
	}
}

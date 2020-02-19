package handler

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
)

// WindowManager implements Handler as a tiled window manager.
type WindowManager struct {
	tree       component.TileTree
	focus      *component.TileNode
	border     bool
	borderAttr term.Attributes
	focusAttr  term.Attributes
}

// NewWindowManager allocates storage for a new WindowManager and initializes it with the
// given handler. If border is true, it will draw a border around every tile.
func NewWindowManager(handler tui.Handler, border bool) (wm *WindowManager) {
	wm = new(WindowManager)
	wm.Init(handler, border)
	return
}

func (wm *WindowManager) withFrame(handler tui.Handler) tui.Handler {
	f := NewFrame(handler)
	f.SetAttr(wm.borderAttr)
	return f
}

// Init initializes this WindowManager with the given handler. If border is true, it will draw
// a border around every tile.
func (wm *WindowManager) Init(handler tui.Handler, border bool) {
	if border {
		wm.borderAttr.Fg = term.ColorDefault
		wm.borderAttr.Bg = term.ColorDefault
		wm.focusAttr.Fg = term.ColorRed
		wm.focusAttr.Bg = term.ColorDefault
		handler = wm.withFrame(handler)
	}
	tile := wm.tree.Init(handler)
	wm.border = border
	wm.focus = tile
	wm.SetFocus(tile)
	return
}

// SetAttr sets the default and focus window border attributes. Note that
// this has no effect if WindowManager was initialized with border == false.
func (wm *WindowManager) SetAttr(standard, focus term.Attributes) {
	if !wm.border {
		return
	}

	wm.borderAttr = standard
	wm.focusAttr = focus

	wm.tree.Iterate(func(node *component.TileNode) {
		node.Content().(*Frame).SetAttr(wm.borderAttr)
	})

	wm.focus.Content().(*Frame).SetAttr(wm.focusAttr)
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
		childAtMouse := wm.tree.TileAt(mousePos)
		if wm.Focus() != childAtMouse {
			if ev.Key == term.MouseLeft {
				wm.SetFocus(childAtMouse)
			}
			return
		}
		offset := wm.tree.TilePosition(childAtMouse)
		ev.MouseX -= offset.X
		ev.MouseY -= offset.Y
	}

	hexit := wm.FocusContent().Handle(ev)

	// if handler in focus wants to exit, close the window,
	// or signal exit to upstream handler if it was last window
	if hexit {
		if exit = wm.tree.Size() == 1; exit {
			return
		}
		curr := wm.focus
		wm.ShiftFocus()
		curr.Close()
	}

	return
}

// SplitVertical creates a new vertical split over the tile currently in focus.
func (wm *WindowManager) SplitVertical(h tui.Handler) *component.TileNode {
	if wm.border {
		h = wm.withFrame(h)
	}
	return wm.tree.SplitVertical(wm.focus, h)
}

// SplitHorizontal creates a new horizontal split over the tile currently in focus.
func (wm *WindowManager) SplitHorizontal(h tui.Handler) *component.TileNode {
	if wm.border {
		h = wm.withFrame(h)
	}
	return wm.tree.SplitHorizontal(wm.focus, h)

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

// FocusContent returns the current focus content.
func (wm *WindowManager) FocusContent() tui.Handler {
	if wm.border {
		return wm.focus.Content().(*Frame).Content().(tui.Handler)
	}
	return wm.focus.Content().(tui.Handler)
}

// Focus returns the tile currently in focus.
func (wm *WindowManager) Focus() *component.TileNode {
	return wm.focus
}

// ShiftFocus attempts to shift to focus to another tile. It returns
// false if focus did not shift to another tile because there aren't any tiles left.
func (wm *WindowManager) ShiftFocus() (ok bool) {
	ok = wm.FocusLeft()
	if ok {
		return
	}
	ok = wm.FocusUp()
	if ok {
		return
	}
	ok = wm.FocusRight()
	if ok {
		return
	}
	ok = wm.FocusDown()
	return
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

// SetFocusContent sets the content of the tile in focus to h.
func (wm *WindowManager) SetFocusContent(h tui.Handler) (
	prev tui.Handler,
) {
	prev = wm.FocusContent()
	if wm.border {
		h = wm.withFrame(h)
		h.(*Frame).SetAttr(wm.focusAttr)
	}
	wm.focus.SetContent(h)
	return
}

// Cursor returns the cursor coordinates of the tile in focus.
func (wm *WindowManager) Cursor() (term.Coordinates, bool) {
	offset := wm.tree.TilePosition(wm.focus)
	cursor, show := wm.focus.Content().(tui.Handler).Cursor()
	return term.Coordinates{X: offset.X + cursor.X, Y: offset.Y + cursor.Y}, show
}

// Man : Handler
func (wm *WindowManager) Man() tui.Manual {
	return tui.Manual{
		Summary: "WindowManager implements a tiled window manager.",
		Keys: tui.KeyMap{
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

// Draw : tui.Component
func (wm *WindowManager) Draw(w tui.Writer) {
	wm.tree.Draw(w)
}

// Resize : tui.Component
func (wm *WindowManager) Resize(width, height int) {
	wm.tree.Resize(width, height)
}

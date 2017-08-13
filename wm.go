package fractal

import (
	"termbox"
)

type WindowManager struct {
	TileNode
	focus  *Tile
	border bool
}

func NewWindowManager(handler Handler, border bool) (wm *WindowManager, tile *Tile) {
	wm = new(WindowManager)
	tile = wm.TileNode.Init(handler)
	wm.focus = tile
	return
}

func (wm *WindowManager) Handle(ev termbox.Event) (exit bool, err error) {
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

	hexit, err := wm.getFocusHandler().Handle(ev)
	if err != nil {
		return
	}

	// if handler on focus wants to exit, close the window,
	// or signal exit to upstream handler if it was last window
	if hexit {
		if exit = wm.Len() == 1; exit {
			return
		}
		curr := wm.focus
		if !wm.FocusLeft() && !wm.FocusUp() && !wm.FocusRight() {
			wm.FocusDown()
		}
		if err = curr.Close(); err != nil {
			return
		}
	}

	return
}

func (wm *WindowManager) SplitVertical(n Handler) (*Tile, error) {
	return wm.TileNode.SplitVertical(wm.focus, n)
}

func (wm *WindowManager) SplitHorizontal(n Handler) (*Tile, error) {
	return wm.TileNode.SplitHorizontal(wm.focus, n)

}

func (wm *WindowManager) switchFocus(tile *Tile) bool {
	if tile == nil {
		return false
	}

	wm.focus = tile
	return true
}

func (wm *WindowManager) FocusLeft() bool {
	return wm.switchFocus(wm.focus.TileLeft())
}

func (wm *WindowManager) FocusRight() bool {
	return wm.switchFocus(wm.focus.TileRight())
}

func (wm *WindowManager) FocusUp() bool {
	return wm.switchFocus(wm.focus.TileUp())
}

func (wm *WindowManager) FocusDown() bool {
	return wm.switchFocus(wm.focus.TileDown())
}

func (wm *WindowManager) getFocusHandler() Handler {
	return wm.focus.Content().(Handler)
}

func (wm *WindowManager) Focus() *Tile {
	return wm.focus
}

func (wm *WindowManager) SetFocus(tile *Tile) {
	wm.focus = tile
}

func (wm *WindowManager) GetCursor() Coordinates {
	return wm.Focus().Content().(Handler).GetCursor()
}

func (wm *WindowManager) Man() string {
	const manual = `
Alt-Q: exit window manager
Alt-L: move focus to next split
Alt-H: move focus to prev split
Alt-K: move focus to split on top
Alt-J: move focus to split on bottom
`
	return manual + wm.getFocusHandler().Man()
}

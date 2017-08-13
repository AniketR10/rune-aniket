package fractal

import (
	"termbox"
)

// TODO add border + padding configuration
type WindowManager struct {
	TileNode
	focus *Tile
	exit  bool
}

func NewWindowManager(handler Handler) (wm *WindowManager, tile *Tile) {
	wm = new(WindowManager)
	tile = wm.TileNode.Init(handler)
	wm.focus = tile
	return
}

func (wm *WindowManager) Handle(ev termbox.Event) (exit bool, err error) {
	if ev.Type == termbox.EventKey && ev.Mod == termbox.ModAlt {
		switch ev.Ch {
		case 'q':
			return true, nil
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

	if exit, err = wm.getFocusHandler().Handle(ev); err != nil {
		return
	}

	if exit {
		if wm.TileNode.Len() == 1 {
			return true, nil
		}
		curr := wm.focus
		if !wm.FocusLeft() {
			wm.FocusUp()
		}
		if err = curr.Close(); err != nil {
			return
		}
	}

	return false, nil
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

func (wm *WindowManager) IsActive() bool {
	return !wm.exit
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

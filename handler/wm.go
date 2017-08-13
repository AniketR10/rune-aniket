package handler

import (
	"termbox"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
)

type WindowManager struct {
	component.TileNode
	focus *component.Tile
	exit  bool
}

func NewWindowManager(handler fractal.Handler, width, height, x, y int) (wm *WindowManager, tile *component.Tile, err error) {
	wm = new(WindowManager)
	tile, err = wm.TileNode.Init(width, height, x, y, handler)
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

func (wm *WindowManager) SplitVertical(n fractal.Handler) (*component.Tile, error) {
	return wm.TileNode.SplitVertical(wm.focus, n)
}

func (wm *WindowManager) SplitHorizontal(n fractal.Handler) (*component.Tile, error) {
	return wm.TileNode.SplitHorizontal(wm.focus, n)

}

func (wm *WindowManager) switchFocus(tile *component.Tile) bool {
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

func (wm *WindowManager) getFocusHandler() fractal.Handler {
	return wm.focus.Content().(fractal.Handler)
}

func (wm *WindowManager) Focus() *component.Tile {
	return wm.focus
}

func (wm *WindowManager) SetFocus(tile *component.Tile) {
	wm.focus = tile
}

func (wm *WindowManager) GetCursor() fractal.Coordinates {
	return wm.Focus().Content().(fractal.Handler).GetCursor()
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

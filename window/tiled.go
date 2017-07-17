package window

import (
	"fmt"
	"log"
	"math"

	"github.com/ernestrc/fractal/buffer"
	"github.com/ernestrc/fractal/config"
)

type TiledWindow struct {
	Window         // underlying window
	node   *winode // node this tile window belongs to in the tree
}

func newWindow(cfg *config.Config) (t *TiledWindow) {
	if cfg == nil {
		panic("configuration cannot be nil")
	}

	t = new(TiledWindow)
	t.cells = make(map[int]buffer.Cell)
	t.config = cfg

	return
}

type WindowManager struct {
	root   *TiledWindow
	focus  *TiledWindow
	config *config.Config
	tree   *winode
	height int
	width  int
	xerror float64
	yerror float64
}

func NewManager(cfg *config.Config, width, height int) (m *WindowManager, err error) {
	m = new(WindowManager)
	m.root = newWindow(cfg)
	m.focus = m.root
	m.config = cfg
	m.width = width
	m.height = height
	m.tree = newWinode(m.root)

	if err = m.root.Resize(width, height); err != nil {
		return
	}

	return
}

func (m *WindowManager) GetFocus() *TiledWindow {
	return m.focus
}

func (m *WindowManager) Draw() error {
	return m.tree.Draw()
}

func (m *WindowManager) Root() *TiledWindow {
	return m.root
}

func (m *WindowManager) split(win *TiledWindow) (newWin *TiledWindow) {
	leaf := win.node
	leaf.window = nil

	newWin = newWindow(m.config)
	newLeafLeft := newWinode(newWin)
	newLeafRight := newWinode(win)

	leaf.lsplit = newLeafLeft
	leaf.rsplit = newLeafRight

	return
}

func getSplitError(w *TiledWindow) error {
	return fmt.Errorf("cannot split window: %+v", w)
}

func (m *WindowManager) Close(win *TiledWindow) error {
	panic("TODO: not implemented")
}

func (m *WindowManager) SplitVertical(win *TiledWindow) (newWin *TiledWindow, err error) {
	if win == nil {
		win = m.root
	}
	newWin = m.split(win)

	height := win.Height()
	width := win.Width()

	newWidth := float64(width) / 2

	if err = win.Resize(int(math.Ceil(newWidth)), height); err != nil {
		return
	}

	if err = newWin.Resize(int(math.Floor(newWidth)), height); err != nil {
		return
	}

	newWin.Move(win.xstart+win.width, win.ystart)

	return
}

func (m *WindowManager) SplitHorizontal(win *TiledWindow) (newWin *TiledWindow, err error) {
	if win == nil {
		win = m.root
	}
	newWin = m.split(win)

	height := win.Height()
	width := win.Width()

	newHeight := float64(height) / 2

	if err = win.Resize(width, int(math.Ceil(newHeight))); err != nil {
		return
	}

	if err = newWin.Resize(width, int(math.Floor(newHeight))); err != nil {
		return
	}

	newWin.Move(win.xstart, win.ystart+win.height)

	return
}

func (m *WindowManager) Resize(width, height int) (err error) {
	xfactor := float64(width) / float64(m.width)
	yfactor := float64(height) / float64(m.height)

	if m.xerror, m.yerror, err = m.tree.ResizeMove(xfactor, yfactor, m.xerror, m.yerror); err != nil {
		return err
	} else {
		log.Println(m.xerror, m.yerror)
	}

	return nil
}

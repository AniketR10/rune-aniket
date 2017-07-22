package window

import "github.com/ernestrc/fractal"

type WindowManager struct {
	root          *tnode
	focus         *TiledWindow
	width, height int
	/* x, y   int */
}

func NewManager(width, height int, content fractal.Window) (m *WindowManager, err error) {
	m = new(WindowManager)
	twin := newTiledWindow(content)
	m.root = newNode(vertical, twin, 0, 0, width, height)
	twin.node = m.root
	m.focus = twin
	m.width = width
	m.height = height

	if err = m.root.Resize(width, height); err != nil {
		return
	}

	return
}

func (m *WindowManager) GetFocus() *TiledWindow {
	return m.focus
}

func (m *WindowManager) Draw(w fractal.Writer) error {
	return m.root.Draw(w)
}

func (m *WindowManager) Close(win *TiledWindow) error {
	panic("TODO: not implemented")
}

// note that the returned TiledWindow should be initialied by the caller
func (m *WindowManager) SplitVertical(tw *TiledWindow, content fractal.Window) (newtw *TiledWindow, err error) {
	return m.split(tw, vertical, content)
}

func (m *WindowManager) SplitHorizontal(tw *TiledWindow, content fractal.Window) (newtw *TiledWindow, err error) {
	return m.split(tw, horizontal, content)
}

func (m *WindowManager) split(tw *TiledWindow, direction splitdir, content fractal.Window) (newtw *TiledWindow, err error) {
	if tw == nil {
		panic("trying to split a nil tile")
	}

	node := tw.node
	newtw = newTiledWindow(content)

	if node.direction == direction {
		newtw.node = node
		node.tiles = append(node.tiles, newtw)
		err = node.Resize(node.width, node.height)
		return
	}

	x, y := tw.Position()
	// substitute window we are splitting over for a node
	// which will contain the current window and a new one
	nnode := newNode(direction, tw, x, y, tw.Width(), tw.Height())
	nnode.tiles = append(nnode.tiles, newtw)
	newtw.node = nnode

	for i, t := range node.tiles {
		if t == tw {
			node.tiles[i] = nnode
			goto exit
		}
	}

	panic("corrupt node: tile does not belong to this node")

exit:
	tw.node = nnode
	err = nnode.Resize(nnode.width, nnode.height)

	return
}

func (m *WindowManager) Resize(width, height int) (err error) {
	m.width = width
	m.height = height

	if err = m.root.Resize(width, height); err != nil {
		return err
	}

	return
}

package component

import "github.com/ernestrc/fractal"

type splitdir uint8

const (
	vertical splitdir = iota
	horizontal
)

type tnode struct {
	fractal.Coordinates
	width     int
	height    int
	tiles     []fractal.Component
	direction splitdir
}

func newNode(direction splitdir, w *TiledWindow, x, y, width, height int) (t *tnode) {
	t = new(tnode)
	t.X, t.Y, t.width, t.height = x, y, width, height
	t.tiles = []fractal.Component{w}
	t.direction = direction
	return
}

type TiledWindow struct {
	content fractal.Component
	node    *tnode
}

func newTiledWindow(content fractal.Component) (t *TiledWindow) {
	t = new(TiledWindow)
	t.content = content
	return
}

func (t *tnode) Resize(width, height int) (err error) {
	t.height = height
	t.width = width

	if t.direction == vertical {
		width /= len(t.tiles)
	} else {
		height /= len(t.tiles)
	}

	for i, ti := range t.tiles {
		if err = ti.Resize(width, height); err != nil {
			return
		}
		if t.direction == vertical {
			if err = ti.Move(t.X+i*width, t.Y); err != nil {
				return
			}
			continue
		}

		if err = ti.Move(t.X, t.Y+i*height); err != nil {
			return
		}
	}
	return
}

func (t *tnode) Move(x, y int) error {
	t.X = x
	t.Y = y
	return t.Resize(t.width, t.height)
}

func (t *tnode) Draw(w fractal.Writer) (err error) {
	for _, ti := range t.tiles {
		if err = ti.Draw(w); err != nil {
			return
		}
	}

	return
}

func (t *tnode) Height() int {
	return t.height
}

func (t *tnode) Width() int {
	return t.width
}

func (t *tnode) Position() (int, int) {
	return t.X, t.Y
}

func (t *TiledWindow) Resize(width, height int) (err error) {
	return t.content.Resize(width, height)
}

func (t *TiledWindow) Move(x, y int) error {
	return t.content.Move(x, y)
}

func (t *TiledWindow) Draw(w fractal.Writer) (err error) {
	return t.content.Draw(w)
}

func (t *TiledWindow) Height() int {
	return t.content.Height()
}

func (t *TiledWindow) Width() int {
	return t.content.Width()
}

func (t *TiledWindow) Position() (int, int) {
	return t.content.Position()
}

// TODO conform to fractal.Component
type WindowManager struct {
	root          *tnode
	width, height int
	/* x, y   int */
}

func New(width, height int, content fractal.Component) (m *WindowManager, root *TiledWindow, err error) {
	m = new(WindowManager)
	root = newTiledWindow(content)
	m.root = newNode(vertical, root, 0, 0, width, height)
	root.node = m.root
	m.width = width
	m.height = height

	if err = m.root.Resize(width, height); err != nil {
		return
	}

	return
}

func (m *WindowManager) Draw(w fractal.Writer) error {
	return m.root.Draw(w)
}

func (m *WindowManager) Close(win *TiledWindow) error {
	panic("TODO: not implemented")
}

// note that the returned TiledWindow should be initialied by the caller
func (m *WindowManager) SplitVertical(tw *TiledWindow, content fractal.Component) (newtw *TiledWindow, err error) {
	return m.split(tw, vertical, content)
}

func (m *WindowManager) SplitHorizontal(tw *TiledWindow, content fractal.Component) (newtw *TiledWindow, err error) {
	return m.split(tw, horizontal, content)
}

func (m *WindowManager) split(tw *TiledWindow, direction splitdir, content fractal.Component) (newtw *TiledWindow, err error) {
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

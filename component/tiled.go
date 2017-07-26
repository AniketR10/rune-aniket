package component

import (
	"fmt"

	"github.com/ernestrc/fractal"
)

type splitdir uint8

const (
	vertical splitdir = iota
	horizontal
)

type linkedComponent interface {
	setParent(*tnode)
	fractal.Component
}

type tnode struct {
	fractal.Coordinates
	width     int
	height    int
	tiles     []linkedComponent
	direction splitdir
	parent    *tnode
}

type TiledWindow struct {
	content fractal.Component
	parent  *tnode
}

func newNode(direction splitdir, parent *tnode, w *TiledWindow, x, y, width, height int) (t *tnode) {
	t = new(tnode)
	t.X, t.Y, t.width, t.height = x, y, width, height
	t.tiles = []linkedComponent{w}
	t.direction = direction
	t.parent = parent
	return
}

func newTiledWindow(content fractal.Component) (t *TiledWindow) {
	t = new(TiledWindow)
	t.content = content
	return
}

func (t *TiledWindow) setParent(parent *tnode) {
	t.parent = parent
}

func (t *tnode) setParent(parent *tnode) {
	t.parent = parent
}

func (t *tnode) resizeHorizontal(len, width, height int) (err error) {
	cheight := height / len
	hspare := height - cheight*len

	useSpareIdx := len - hspare
	spareCell := 0

	for i, ti := range t.tiles {
		offset := ((i - useSpareIdx) * spareCell)
		if err = ti.Move(t.X, (t.Y+i*cheight)+offset); err != nil {
			return
		}

		if i == useSpareIdx {
			spareCell = 1
		}

		if err = ti.Resize(width, cheight+spareCell); err != nil {
			return
		}
	}
	return
}

func (t *tnode) resizeVertical(len, width, height int) (err error) {
	cwidth := width / len
	wspare := width - cwidth*len

	useSpareIdx := len - wspare
	spareCell := 0

	for i, ti := range t.tiles {
		offset := ((i - useSpareIdx) * spareCell)
		if err = ti.Move((t.X+i*cwidth)+offset, t.Y); err != nil {
			return
		}

		if i == useSpareIdx {
			spareCell = 1
		}

		if err = ti.Resize(cwidth+spareCell, height); err != nil {
			return
		}
	}
	return
}

func (t *tnode) Resize(width, height int) (err error) {
	t.height = height
	t.width = width

	len := len(t.tiles)

	if len == 0 {
		panic(fmt.Sprintf("can't resize a tnode with no tiles: %[1]p: %+[1]v", t))
	}

	if t.direction == vertical {
		return t.resizeVertical(len, width, height)
	}

	return t.resizeHorizontal(len, width, height)
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

type WindowManager struct {
	root          *tnode
	width, height int
	fractal.Coordinates
}

func (t *tnode) childIdx(comp linkedComponent) int {
	for i, t := range t.tiles {
		if t == comp {
			return i
		}
	}

	panic("corrupt node: window already closed or tile does not belong to this node")
}

func (m *WindowManager) split(tw *TiledWindow, direction splitdir, content fractal.Component) (newtw *TiledWindow, err error) {
	if tw == nil {
		panic("trying to split a nil tile")
	}

	node := tw.parent
	newtw = newTiledWindow(content)

	if node.direction == direction {
		newtw.parent = node
		node.tiles = append(node.tiles, newtw)
		err = node.Resize(node.width, node.height)
		return
	}

	x, y := tw.Position()
	// substitute window we are splitting over for a node
	// which will contain the current window and a new one
	nnode := newNode(direction, node, tw, x, y, tw.Width(), tw.Height())
	nnode.tiles = append(nnode.tiles, newtw)
	newtw.parent = nnode

	i := node.childIdx(tw)
	node.tiles[i] = nnode
	tw.parent = nnode
	err = nnode.Resize(nnode.width, nnode.height)

	return
}

func (m *WindowManager) Draw(w fractal.Writer) error {
	return m.root.Draw(w)
}

func (m *WindowManager) Close(tw *TiledWindow) (err error) {
	node := tw.parent

	if node.parent == nil && len(node.tiles) == 1 {
		panic("unsupported: trying to close last window: remove manager instead")
	}

	// remove window
	i := node.childIdx(tw)
	copy(node.tiles[i:], node.tiles[i+1:])
	node.tiles[len(node.tiles)-1] = nil
	node.tiles = node.tiles[:len(node.tiles)-1]

	// add last component to parent node and remove itself
	if node.parent != nil && len(node.tiles) == 1 {
		parent := node.parent
		child := node.tiles[0]

		j := parent.childIdx(node)
		parent.tiles[j] = child
		child.setParent(parent)

		// avoid memory leaks
		node.parent = nil

		return parent.Resize(parent.width, parent.height)
	}

	return node.Resize(node.width, node.height)
}

func (m *WindowManager) SplitVertical(tw *TiledWindow, content fractal.Component) (newtw *TiledWindow, err error) {
	return m.split(tw, vertical, content)
}

func (m *WindowManager) SplitHorizontal(tw *TiledWindow, content fractal.Component) (newtw *TiledWindow, err error) {
	return m.split(tw, horizontal, content)
}

func (m *WindowManager) Resize(width, height int) (err error) {
	m.width = width
	m.height = height

	if err = m.root.Resize(width, height); err != nil {
		return err
	}

	return
}

func (m *WindowManager) Move(x, y int) error {
	// TODO return t.content.Move(x, y)
}

func (m *WindowManager) Height() int {
	return m.height
}

func (m *WindowManager) Width() int {
	return m.width
}

func (m *WindowManager) Position() (int, int) {
	return m.X, m.Y
}

func NewTiledManager(width, height int, content fractal.Component) (m *WindowManager, root *TiledWindow, err error) {
	m = new(WindowManager)
	root = newTiledWindow(content)
	m.root = newNode(vertical, nil, root, 0, 0, width, height)
	root.parent = m.root
	m.width = width
	m.height = height

	if err = m.root.Resize(width, height); err != nil {
		return
	}

	return
}

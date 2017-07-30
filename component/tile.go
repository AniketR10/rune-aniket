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
	setParent(*TileManager)
	fractal.Component
}

type TileManager struct {
	pos       fractal.Coordinates
	width     int
	height    int
	children  []linkedComponent
	direction splitdir
	parent    *TileManager
}

type Tile struct {
	content fractal.Component
	parent  *TileManager
}

func newNode(direction splitdir, parent *TileManager, w *Tile, x, y, width, height int) (t *TileManager) {
	t = new(TileManager)
	t.pos.X, t.pos.Y, t.width, t.height = x, y, width, height
	t.children = []linkedComponent{w}
	t.direction = direction
	t.parent = parent
	return
}

func newTile(content fractal.Component) (t *Tile) {
	t = new(Tile)
	t.content = content
	return
}

func (t *Tile) setParent(parent *TileManager) {
	t.parent = parent
}

func (t *TileManager) setParent(parent *TileManager) {
	t.parent = parent
}

func (t *TileManager) resizeHorizontal(len, width, height int) (err error) {
	cheight := height / len
	hspare := height - cheight*len

	useSpareIdx := len - hspare
	spareCell := 0

	for i, ti := range t.children {
		offset := ((i - useSpareIdx) * spareCell)
		if err = ti.Move(t.pos.X, (t.pos.Y+i*cheight)+offset); err != nil {
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

func (t *TileManager) resizeVertical(len, width, height int) (err error) {
	cwidth := width / len
	wspare := width - cwidth*len

	useSpareIdx := len - wspare
	spareCell := 0

	for i, ti := range t.children {
		offset := ((i - useSpareIdx) * spareCell)
		if err = ti.Move((t.pos.X+i*cwidth)+offset, t.pos.Y); err != nil {
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

func (t *TileManager) Resize(width, height int) (err error) {
	t.height = height
	t.width = width

	len := len(t.children)

	if len == 0 {
		panic(fmt.Sprintf("can't resize a TileManager with no children: %[1]p: %+[1]v", t))
	}

	if t.direction == vertical {
		return t.resizeVertical(len, width, height)
	}

	return t.resizeHorizontal(len, width, height)
}

func (t *TileManager) Move(x, y int) error {
	t.pos.X, t.pos.Y = x, y
	return t.Resize(t.width, t.height)
}

func (t *TileManager) Draw(w fractal.Writer) (err error) {
	for _, ti := range t.children {
		if err = ti.Draw(w); err != nil {
			return
		}
	}

	return
}

func (t *TileManager) Height() int {
	return t.height
}

func (t *TileManager) Width() int {
	return t.width
}

func (t *TileManager) Position() (int, int) {
	return t.pos.X, t.pos.Y
}

func (t *Tile) Resize(width, height int) (err error) {
	return t.content.Resize(width, height)
}

func (t *Tile) Move(x, y int) error {
	return t.content.Move(x, y)
}

func (t *Tile) Draw(w fractal.Writer) (err error) {
	return t.content.Draw(w)
}

func (t *Tile) Height() int {
	return t.content.Height()
}

func (t *Tile) Width() int {
	return t.content.Width()
}

func (t *Tile) Position() (int, int) {
	return t.content.Position()
}

func (t *TileManager) childIdx(comp linkedComponent) int {
	for i, t := range t.children {
		if t == comp {
			return i
		}
	}

	panic("corrupt node: window already closed or tile does not belong to this node")
}

func (m *TileManager) split(tw *Tile, direction splitdir, content fractal.Component) (t *Tile, err error) {
	if tw == nil {
		panic("trying to split a nil tile")
	}

	node := tw.parent
	t = newTile(content)

	if node.direction == direction {
		t.parent = node
		node.children = append(node.children, t)
		err = node.Resize(node.width, node.height)
		return
	}

	x, y := tw.Position()
	// substitute tile we are splitting over for a node
	// which will contain the current tile and a new one
	nnode := newNode(direction, node, tw, x, y, tw.Width(), tw.Height())
	nnode.children = append(nnode.children, t)
	t.parent = nnode

	i := node.childIdx(tw)
	node.children[i] = nnode
	tw.parent = nnode
	err = nnode.Resize(nnode.width, nnode.height)

	return
}

func (tw *Tile) Close() (err error) {
	node := tw.parent

	if node.parent == nil && len(node.children) == 1 {
		panic("unsupported: trying to close last window: remove manager instead")
	}

	// remove window
	i := node.childIdx(tw)
	copy(node.children[i:], node.children[i+1:])
	node.children[len(node.children)-1] = nil
	node.children = node.children[:len(node.children)-1]

	// add last component to parent node and remove itself
	if node.parent != nil && len(node.children) == 1 {
		parent := node.parent
		child := node.children[0]

		j := parent.childIdx(node)
		parent.children[j] = child
		child.setParent(parent)

		// avoid memory leaks
		node.parent = nil

		return parent.Resize(parent.width, parent.height)
	}

	return node.Resize(node.width, node.height)
}

func (m *TileManager) SplitVertical(tw *Tile, content fractal.Component) (*Tile, error) {
	return m.split(tw, vertical, content)
}

func (m *TileManager) SplitHorizontal(tw *Tile, content fractal.Component) (*Tile, error) {
	return m.split(tw, horizontal, content)
}

func NewTileManager(width, height int, content fractal.Component) (m *TileManager, root *Tile, err error) {
	root = newTile(content)
	m = newNode(vertical, nil, root, 0, 0, width, height)
	root.parent = m
	m.width = width
	m.height = height

	if err = m.Resize(width, height); err != nil {
		return
	}

	return
}

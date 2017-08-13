package fractal

import (
	"fmt"
)

type splitdir uint8

const (
	vertical splitdir = iota
	horizontal
)

type linkedComponent interface {
	setParent(*TileNode)
	getParent() *TileNode
	Component
}

type TileNode struct {
	pos       Coordinates
	width     int
	height    int
	children  []linkedComponent
	direction splitdir
	parent    *TileNode
}

type Tile struct {
	content Component
	parent  *TileNode
}

func (t *TileNode) Init(content Component) *Tile {
	root := newTile(content)
	t.initNode(vertical, nil, root, 0, 0, 0, 0)
	root.parent = t
	return root
}

func NewTileNode(content Component) (*TileNode, *Tile) {
	t := new(TileNode)
	root := t.Init(content)
	return t, root
}

func (t *TileNode) initNode(direction splitdir, parent *TileNode, w *Tile, x, y, width, height int) {
	t.pos.X, t.pos.Y, t.width, t.height = x, y, width, height
	t.children = []linkedComponent{w}
	t.direction = direction
	t.parent = parent
}

func newTile(content Component) (t *Tile) {
	t = new(Tile)
	t.content = content
	return
}

func (t *Tile) setParent(parent *TileNode) {
	t.parent = parent
}

func (t *TileNode) setParent(parent *TileNode) {
	t.parent = parent
}

func (t *TileNode) resizeHorizontal(len, width, height int) (err error) {
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

func (t *TileNode) resizeVertical(len, width, height int) (err error) {
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

func (t *TileNode) Resize(width, height int) (err error) {
	t.height = height
	t.width = width

	len := len(t.children)

	if len == 0 {
		panic(fmt.Sprintf("can't resize a TileNode with no children: %[1]p: %+[1]v", t))
	}

	if t.direction == vertical {
		return t.resizeVertical(len, width, height)
	}

	return t.resizeHorizontal(len, width, height)
}

func (t *TileNode) Move(x, y int) error {
	t.pos.X, t.pos.Y = x, y
	return t.Resize(t.width, t.height)
}

func (t *TileNode) Draw(w Writer) (err error) {
	for _, ti := range t.children {
		if err = ti.Draw(w); err != nil {
			return
		}
	}

	return
}

func (t *TileNode) Height() int {
	return t.height
}

func (t *TileNode) Width() int {
	return t.width
}

func (t *TileNode) Position() (int, int) {
	return t.pos.X, t.pos.Y
}

func (t *Tile) Resize(width, height int) (err error) {
	return t.content.Resize(width, height)
}

func (t *Tile) Move(x, y int) error {
	return t.content.Move(x, y)
}

func (t *Tile) Draw(w Writer) (err error) {
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

func (t *TileNode) childIdx(comp linkedComponent) int {
	for i, t := range t.children {
		if t == comp {
			return i
		}
	}

	panic("corrupt node: window already closed or tile does not belong to this node")
}

func (m *TileNode) split(tw *Tile, direction splitdir, content Component) (t *Tile, err error) {
	if tw == nil {
		panic("trying to split a nil tile")
	}

	node := tw.parent
	t = newTile(content)
	i := node.childIdx(tw)

	if node.direction == direction {
		t.parent = node
		target := i + 1 // target position

		if target == len(node.children) {
			node.children = append(node.children, t)
		} else {
			node.children = append(node.children, nil)
			copy(node.children[target+1:], node.children[target:])
			node.children[target] = t
		}
		err = node.Resize(node.width, node.height)
		return
	}

	x, y := tw.Position()
	// substitute tile we are splitting over for a node
	// which will contain the current tile and a new one
	nnode := new(TileNode)
	nnode.initNode(direction, node, tw, x, y, tw.Width(), tw.Height())
	nnode.children = append(nnode.children, t)
	t.parent = nnode

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

func (m *TileNode) SplitVertical(tw *Tile, content Component) (*Tile, error) {
	return m.split(tw, vertical, content)
}

func (m *TileNode) SplitHorizontal(tw *Tile, content Component) (*Tile, error) {
	return m.split(tw, horizontal, content)
}

func (t *Tile) Content() Component {
	return t.content
}

// LeftMostTile will return the left-most tile in the node
// if node's split is horizontal, or the top-most tile if the node's split is vertical
func (t *TileNode) LeftMostTile() *Tile {
	if len(t.children) == 0 {
		return nil
	}

	if tile, ok := t.children[0].(*Tile); ok {
		return tile
	}

	return t.children[0].(*TileNode).LeftMostTile()
}

// RightMostTile will return the right-most tile in the node
// if node's split is horizontal, or the bottom-most tile if the node's split is vertical
func (t *TileNode) RightMostTile() *Tile {
	len := len(t.children)
	if len == 0 {
		return nil
	}

	idx := len - 1
	if tile, ok := t.children[idx].(*Tile); ok {
		return tile
	}

	return t.children[idx].(*TileNode).RightMostTile()
}

func (t *TileNode) getParent() *TileNode {
	return t.parent
}

func (t *Tile) getParent() *TileNode {
	return t.parent
}

func tileLeftDir(l linkedComponent, direction splitdir) *Tile {
	node := l.getParent()

	if node == nil {
		return nil
	}

	i := node.childIdx(l)
	if i == 0 || node.direction != direction {
		return tileLeftDir(node, direction)
	}

	link := node.children[i-1]

	if t, ok := link.(*Tile); ok {
		return t
	}

	return link.(*TileNode).RightMostTile()
}

func tileRightDir(l linkedComponent, direction splitdir) *Tile {
	node := l.getParent()
	if node == nil {
		return nil
	}

	i := node.childIdx(l)
	if i == len(node.children)-1 || node.direction != direction {
		return tileRightDir(node, direction)
	}

	link := node.children[i+1]

	if t, ok := link.(*Tile); ok {
		return t
	}

	return link.(*TileNode).LeftMostTile()
}

func (t *Tile) TileLeft() *Tile {
	return tileLeftDir(t, vertical)
}

func (t *Tile) TileRight() *Tile {
	return tileRightDir(t, vertical)
}

func (t *Tile) TileUp() *Tile {
	return tileLeftDir(t, horizontal)
}

func (t *Tile) TileDown() *Tile {
	return tileRightDir(t, horizontal)
}

func (t *TileNode) Len() (len int) {
	for _, c := range t.children {
		if _, ok := c.(*Tile); ok {
			len++
		} else {
			len += c.(*TileNode).Len()
		}
	}

	return
}

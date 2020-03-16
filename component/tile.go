package component

import (
	"fmt"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

type splitdir uint8

const (
	root splitdir = iota
	vertical
	horizontal
)

// TileTree represents the root node of a tree of TileNodes.
type TileTree struct {
	root TileNode
}

// TileNode represents a node in a tree of tiled components.
type TileNode struct {
	width, height int
	content       tui.Component
	children      []*Virtual
	direction     splitdir
	parent        *TileNode
}

// Init initializes a TileTree or resets it if already initialied.
func (t *TileTree) Init(content tui.Component) (n *TileNode) {
	n = new(TileNode)
	t.root.direction = root
	t.root.children = []*Virtual{&Virtual{C: n}}
	n.initNode(vertical, content, &t.root)
	return
}

// NewTileTree allocates storage for a new TileTree and initializes it.
// It also returns the TileNode allocated to store the given content.
func NewTileTree(content tui.Component) (t *TileTree, n *TileNode) {
	t = new(TileTree)
	n = t.Init(content)
	return
}

// Resize resizes the contents of this TileTree.
func (t *TileTree) Resize(width, height int) {
	t.root.Resize(width, height)
}

// Draw draws the contents of this TileTree.
func (t *TileTree) Draw(w tui.Writer) {
	t.root.Draw(w)
}

func (t *TileNode) initNode(
	direction splitdir, content tui.Component, parent *TileNode,
) {
	t.content = content
	t.children = []*Virtual{}
	t.direction = direction
	t.parent = parent
}

func (t *TileNode) resizeHorizontal(width, height int) (err error) {
	length := len(t.children)
	cheight := height / length
	hspare := height - cheight*length

	useSpareIdx := length - hspare
	spareCell := 0

	for i, ti := range t.children {
		offset := ((i - useSpareIdx) * spareCell)
		ti.Move(term.Coordinates{0, i*cheight + offset})

		if i == useSpareIdx {
			spareCell = 1
		}

		ti.Resize(width, cheight+spareCell)
	}
	return
}

func (t *TileNode) resizeVertical(width, height int) {
	length := len(t.children)
	cwidth := width / length
	wspare := width - cwidth*length

	useSpareIdx := length - wspare
	spareCell := 0

	for i, ti := range t.children {
		offset := ((i - useSpareIdx) * spareCell)
		ti.Move(term.Coordinates{i*cwidth + offset, 0})

		if i == useSpareIdx {
			spareCell = 1
		}

		ti.Resize(cwidth+spareCell, height)
	}
	return
}

// Resize : Component
func (t *TileNode) Resize(width, height int) {
	t.height = height
	t.width = width

	if len(t.children) == 0 && t.content == nil {
		panic("corrupted node: non-empty children and content")
	}

	if t.content != nil {
		t.content.Resize(width, height)
		return
	}

	if t.direction == vertical {
		t.resizeVertical(width, height)
		return

	}

	t.resizeHorizontal(width, height)
}

// Draw : Component
func (t *TileNode) Draw(w tui.Writer) {
	if len(t.children) == 0 && t.content == nil {
		panic("corrupted node: non-empty children and content")
	}

	if t.content != nil {
		t.content.Draw(w)
		return
	}

	for _, ti := range t.children {
		ti.Draw(w)
	}

	return
}

func (t *TileNode) childIdx(child *TileNode) int {
	for i, c := range t.children {
		if c.C == child {
			return i
		}
	}

	panic("tile is not a child of parent")
}

func (t *TileNode) addChildAtIdx(
	child *TileNode, content tui.Component, direction splitdir, idx int,
) {
	if idx > len(t.children) {
		panic(fmt.Errorf("trying to append child at index out of bounds: %d; len=%d",
			idx, len(t.children)))
	}

	v := &Virtual{C: child}

	// transfer content to child at index 0 but do it in a way such that it
	// maintains mapping of content to TileNode.
	// This is because instances of TileNode are leaked outside of the
	// tree through various APIs.
	if len(t.children) == 0 {
		proxyNode := new(TileNode)
		proxyNode.initNode(direction, nil, t.parent)
		proxyNode.children = append(proxyNode.children, &Virtual{C: t}, v)

		idx := t.parent.childIdx(t)
		t.parent.children[idx] = &Virtual{C: proxyNode}

		child.initNode(direction, content, proxyNode)
		t.initNode(direction, t.content, proxyNode)

		proxyNode.parent.Resize(proxyNode.parent.width, proxyNode.parent.height)
		return
	}

	child.initNode(direction, content, t)

	t.children = append(t.children, nil)
	copy(t.children[idx+1:], t.children[idx:])
	t.children[idx] = v

	t.Resize(t.width, t.height)
}

func split(over *TileNode, direction splitdir, content tui.Component) (
	n *TileNode,
) {
	if content == nil {
		panic("empty content for tile")
	}
	if over == nil {
		panic("trying to split over a nil tile")
	}

	n = new(TileNode)

	parent := over.parent

	if parent.direction == direction {
		i := parent.childIdx(over)
		parent.addChildAtIdx(n, content, direction, i+1)
		return
	}

	over.addChildAtIdx(n, content, direction, 0)
	return
}

func removeChild(parent, child *TileNode) {
	i := parent.childIdx(child)
	copy(parent.children[i:], parent.children[i+1:])
	parent.children = parent.children[:len(parent.children)-1]

	child.parent = nil
	child.children = nil

	// transfer last child to content but do it in a way such that it
	// maintains mapping of contents to TileNode.
	if len(parent.children) == 1 && parent.parent != nil {
		proxyNode := parent
		lastNode := parent.children[0]

		idx := proxyNode.parent.childIdx(proxyNode)
		proxyNode.parent.children[idx] = lastNode

		lastNode.C.(*TileNode).parent = proxyNode.parent

		proxyNode.parent.Resize(proxyNode.parent.width, proxyNode.parent.height)

		proxyNode.parent = nil
		proxyNode.children = nil
		return
	}

	parent.Resize(parent.width, parent.height)
}

// Close removes this node from the tree. It panics if node is last node on the tree.
func (t *TileNode) Close() {
	if t.parent == nil {
		return
	}
	if t.parent.parent == nil && len(t.parent.children) == 1 {
		panic("unsupported: trying to close last node")
	}

	if len(t.parent.children) != 0 {
		removeChild(t.parent, t)
		return
	}

	t.parent.Close()
}

// SplitVertical splits the given node to incorporate new content. If direction of the
// given node's split is vertical, a new node with content will be added as a sibling of node.
// Otherwise, a new node with content will become a child of the given node so
// width will be divided in half so new node can be drawn next to it.
// It panics if node is a child of this TileTree.
func (t *TileTree) SplitVertical(
	node *TileNode, content tui.Component,
) *TileNode {
	return split(node, vertical, content)
}

// SplitHorizontal splits the given node to incorporate new content. If direction of the
// given node's split is horizontal, a new node with content will be added as a sibling of node.
// Otherwise, a new node will become a child of the given node so
// height will be divided in half so new node can be drawn next to it.
// It panics if node is a child of this TileTree.
func (t *TileTree) SplitHorizontal(
	node *TileNode, content tui.Component,
) *TileNode {
	return split(node, horizontal, content)
}

// leftMostChild will return the left-most tile in the node
// if node's split is horizontal, or the top-most tile if the node's split is vertical
func (t *TileNode) leftMostChild() *TileNode {
	node := t.children[0].C.(*TileNode)
	if len(node.children) == 0 {
		return node
	}

	return node.leftMostChild()
}

// rightMostChild will return the right-most tile in the node
// if node's split is horizontal, or the bottom-most tile if the node's split is vertical
func (t *TileNode) rightMostChild() *TileNode {
	node := t.children[len(t.children)-1].C.(*TileNode)
	if len(node.children) == 0 {
		return node
	}

	return node.rightMostChild()
}

func getParentIdx(node *TileNode) (parent *TileNode, i int) {
	parent = node.parent
	if parent == nil {
		panic("corrupted tree: exposed root node")
	}
	i = parent.childIdx(node)
	return
}

func tileLeftDir(node *TileNode, direction splitdir) *TileNode {
	parent, i := getParentIdx(node)
	if parent.parent == nil {
		return nil
	}

	if i == 0 || parent.direction != direction {
		return tileLeftDir(parent, direction)
	}

	link := parent.children[i-1].C.(*TileNode)
	if len(link.children) == 0 {
		return link
	}

	return link.rightMostChild()
}

func tileRightDir(node *TileNode, direction splitdir) *TileNode {
	parent, i := getParentIdx(node)
	if parent.parent == nil {
		return nil
	}

	if i == len(parent.children)-1 || parent.direction != direction {
		return tileRightDir(parent, direction)
	}

	link := parent.children[i+1].C.(*TileNode)
	if len(link.children) == 0 {
		return link
	}

	return link.leftMostChild()
}

// TileLeft returns the tile left-adjacent to t or nil if t is the
// left-most tile in the tree.
func (t *TileNode) TileLeft() *TileNode {
	return tileLeftDir(t, vertical)
}

// TileRight returns the tile right-adjacent to t or nil if t is the
// right-most tile in the tree.
func (t *TileNode) TileRight() *TileNode {
	return tileRightDir(t, vertical)
}

// TileUp returns the tile on top of t or nil if t is the
// top-most tile in the tree.
func (t *TileNode) TileUp() *TileNode {
	return tileLeftDir(t, horizontal)
}

// TileDown returns the tile in the bottom of t or nil if t is the
// bottom-most tile in the tree.
func (t *TileNode) TileDown() *TileNode {
	return tileRightDir(t, horizontal)
}

// Size returns the total number of nodes under this TileNode.
func (t *TileNode) Size() (size int) {
	if len(t.children) == 0 {
		return 1
	}

	for _, c := range t.children {
		size += c.C.(*TileNode).Size()
	}

	return
}

// Size returns the total number of nodes in this tree.
func (t *TileTree) Size() (size int) {
	return t.root.Size()
}

// Content returns the Component held by this TileNode in the TileTree.
func (t *TileNode) Content() tui.Component {
	if t.content == nil {
		panic("corrupted node: leaked proxy node outside of tree")
	}
	return t.content
}

func (t *TileNode) tileAt(pos term.Coordinates) *TileNode {
	if t.direction == vertical {
		for _, child := range t.children {
			childPos := child.Position()
			if pos.X >= childPos.X && pos.X < childPos.X+child.Width() {
				return child.C.(*TileNode).tileAt(pos)
			}
		}
	} else {
		for _, child := range t.children {
			childPos := child.Position()
			if pos.Y >= childPos.Y && pos.Y < childPos.Y+child.Height() {
				return child.C.(*TileNode).tileAt(pos)
			}
		}
	}

	if len(t.children) != 0 {
		panic(fmt.Sprintf("could not finde tile at %+v", pos))
	}

	return t
}

func (t *TileNode) tilePosition(child *TileNode, currOffset term.Coordinates) (
	offset term.Coordinates, ok bool,
) {
	if t == child {
		panic("missed child on parent loop")
	}

	for _, c := range t.children {
		offset = c.Position()

		ok = (c.C == child)
		if ok {
			offset.X += currOffset.X
			offset.Y += currOffset.Y
			return
		}

		offset, ok = c.C.(*TileNode).tilePosition(child, offset)
		if ok {
			return
		}
	}
	return
}

// TilePosition returns the given tile's position offset inside this TileTree.
// It panics if tile is not a member of this tree.
func (t *TileTree) TilePosition(tile *TileNode) term.Coordinates {
	offset, ok := t.root.tilePosition(tile, term.Coordinates{})
	if !ok {
		panic("tile does not belong to this tree")
	}
	return offset
}

// TileAt returns the tile at pos term.Coordinates.
func (t *TileTree) TileAt(pos term.Coordinates) *TileNode {
	if pos.X < 0 || pos.Y < 0 || pos.X >= t.root.width || pos.Y >= t.root.height {
		panic("Coordinates out of bounds")
	}
	return t.root.tileAt(pos)
}

// SetContent sets the content of a TileNode to c.
func (t *TileNode) SetContent(c tui.Component) {
	t.content = c
	t.content.Resize(t.width, t.height)
}

func (t *TileNode) iterate(op func(*TileNode)) {
	if t.content != nil {
		op(t)
		return
	}

	for _, ti := range t.children {
		ti.C.(*TileNode).iterate(op)
	}
}

// Iterate applies op to the content of all nodes of this tree.
func (t *TileTree) Iterate(op func(*TileNode)) {
	t.root.iterate(op)
}

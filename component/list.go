package component

import (
	"container/list"
	"sort"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// ListNode is an element of a component List.
type ListNode struct {
	l  *List
	el *list.Element
}

// List represents a list of components which are drawn each one
// in series as a separate row.
type List struct {
	elementHeight int
	width, height int
	offset        int
	list          list.List // list of Virtual
}

// NewList allocates storage for a new List and initializes it.
func NewList(elementHeight int) (l *List) {
	l = new(List)
	l.Init(elementHeight)
	return l
}

func (l *List) listNode(el *list.Element) ListNode {
	return ListNode{
		l:  l,
		el: el,
	}
}

// Reset resets the contents of this List.
func (l *List) Reset() {
	l.list.Init()
}

// Init initializes this List.
func (l *List) Init(elementHeight int) {
	if elementHeight <= 0 {
		panic("element height cannot be smaller than or equal to 0")
	}
	l.elementHeight = elementHeight
	l.list.Init()
}

// SetElementHeight sets the height for each element of this list.
func (l *List) SetElementHeight(height int) {
	l.elementHeight = height
	l.Resize(l.width, l.height)
}

// ElementHeight returns the height for each element of this list.
func (l *List) ElementHeight() int {
	return l.elementHeight
}

// CanSeekUp returns whether SeekUp would seek one row up.
func (l *List) CanSeekUp() bool {
	return l.offset > 0
}

// CanSeekDown returns whether SeekUp would seek one row down.
func (l *List) CanSeekDown() bool {
	return l.offset < l.list.Len()-l.height/l.elementHeight
}

// SeekUp shifts the contents of this list one row up.
func (l *List) SeekUp() bool {
	ok := l.CanSeekUp()
	if ok {
		l.offset--
	}
	return ok
}

// SeekDown shifts the contents of this list one row down.
func (l *List) SeekDown() bool {
	ok := l.CanSeekDown()
	if ok {
		l.offset++
	}
	return ok
}

// SeekEnd shifts the contents of this list such that the last element
// is drawn at the top of the list.
func (l *List) SeekEnd() (ok bool) {
	for l.CanSeekDown() {
		l.offset++
		ok = true
	}
	return
}

// SeekStart shifts the contents of this list such that the first element
// is drawn at the top f the list.
func (l *List) SeekStart() (ok bool) {
	if l.offset != 0 {
		ok = true
	}
	l.offset = 0
	return
}

// Resize resizes this list to fit within width and height.
func (l *List) Resize(width, height int) {
	var ok bool
	l.width, l.height = width, height
	var comp *Virtual
	for i, el := 0, l.list.Front(); el != nil; el, i = el.Next(), i+1 {
		comp, ok = el.Value.(*Virtual)
		if !ok {
			panic("element of this list is not Virtual")
		}
		comp.Resize(l.width, l.elementHeight)
		ypos := (i - l.offset) * l.elementHeight
		comp.Move(term.Coordinates{X: 0, Y: ypos})
	}
}

// Draw draws this list's elements with the current seek offset.
func (l *List) Draw(w term.Writer) {
	// API exposes internal list so we need
	// to make sure that the elements are properly position and sized
	// before drawing
	l.Resize(l.width, l.height)
	lastVisible := l.height/l.elementHeight + l.offset

	for i, el := 0, l.list.Front(); i < lastVisible && el != nil; i, el = i+1, el.Next() {
		if i < l.offset {
			continue
		}
		el.Value.(*Virtual).Draw(w)
	}

	return
}

// Back returns the last node of list l or nil if the list is empty.
func (l *List) Back() (ListNode, bool) {
	b := l.list.Back()
	if b == nil {
		return ListNode{}, false
	}
	return l.listNode(b), true
}

// Front returns the first node of list l or nil if the list is empty.
func (l *List) Front() (ListNode, bool) {
	f := l.list.Front()
	if f == nil {
		return ListNode{}, false
	}
	return l.listNode(f), true
}

// Len returns the number of nodes of list l in O(1).
func (l *List) Len() int { return l.list.Len() }

// MoveAfter moves node e to its new position after mark. If e or mark is
// not an node of l, or e == mark, the list is not modified.
func (l *List) MoveAfter(e, mark ListNode) {
	l.list.MoveAfter(e.el, mark.el)
}

// MoveBefore moves node e to its new position before mark. If e or mark is
// not a node of l, or e == mark, the list is not modified.
func (l *List) MoveBefore(e, mark ListNode) {
	l.list.MoveBefore(e.el, mark.el)
}

// MoveToBack moves node e to the back of list l. If e is not an node of
// l, the list is not modified.
func (l *List) MoveToBack(e ListNode) { l.list.MoveToBack(e.el) }

// MoveToFront moves element e to the front of list l. If e is not an element
// of l, the list is not modified. The element must not be nil.
func (l *List) MoveToFront(e ListNode) { l.list.MoveToFront(e.el) }

// PushBackList inserts a copy of an other list at the back of list l. The
// lists l and other must NOT be the same or nil.
func (l *List) PushBackList(other *List) {
	if l == other {
		panic("other list cannot be self: components can't be deep cloned")
	}
	l.list.PushBackList(&other.list)
}

// PushFrontList inserts a copy of an other list at the front of list l. The
// lists l and other must NOT be the same or nil.
func (l *List) PushFrontList(other *List) {
	if l == other {
		panic("other list cannot be self: components can't be deep cloned")
	}
	l.list.PushFrontList(&other.list)
}

// InsertAfter inserts a new element c immediately after mark and
// returns the linked node. If mark is not an element of l, the list
// is not modified.
func (l *List) InsertAfter(c tui.Component, mark ListNode) ListNode {
	v := &Virtual{C: c}
	return l.listNode(l.list.InsertAfter(v, mark.el))
}

// InsertBefore inserts a new element c immediately before mark
// and returns the linked node. If mark is not an element of l,
// the list is not modified.
func (l *List) InsertBefore(c tui.Component, mark ListNode) ListNode {
	v := &Virtual{C: c}
	return l.listNode(l.list.InsertBefore(v, mark.el))
}

// PushBack inserts a new element c at the back of list l and
// returns the linked node.
func (l *List) PushBack(c tui.Component) ListNode {
	v := &Virtual{C: c}
	return l.listNode(l.list.PushBack(v))
}

// PushFront inserts a new element c at the front of list l and
// returns the linked node.
func (l *List) PushFront(c tui.Component) ListNode {
	v := &Virtual{C: c}
	return l.listNode(l.list.PushFront(v))
}

// Remove removes e from l if e is a node of list l. It returns the element
// value e.Value. The element must not be nil.
func (l *List) Remove(e ListNode) tui.Component {
	return l.list.Remove(e.el).(*Virtual).C
}

// SetValue sets the tui.Component value in ListNode.
func (e ListNode) SetValue(c tui.Component) {
	if e.el == nil || e.el.Value == nil {
		panic("trying to SetValue on an un-linked ListNode")
	}
	v := e.el.Value.(*Virtual)
	v.C = c
}

// Value gets the tui.Component value in ListNode.
func (e ListNode) Value() tui.Component {
	if e.el == nil || e.el.Value == nil {
		return nil
	}
	return e.el.Value.(*Virtual).C
}

// Prev gets the previous linked node in the list before e.
func (e ListNode) Prev() (ListNode, bool) {
	if e.el == nil || e.el.Prev() == nil {
		return ListNode{}, false
	}
	return e.l.listNode(e.el.Prev()), true
}

// Next gets the next linked node in the list after e.
func (e ListNode) Next() (ListNode, bool) {
	if e.el == nil || e.el.Next() == nil {
		return ListNode{}, false
	}
	return e.l.listNode(e.el.Next()), true
}

// ElementAt returns the element at pos Coordinates or panics if
// coordinates are out of the bounds of this List.
func (l *List) ElementAt(pos term.Coordinates) (ListNode, bool) {
	if pos.X < 0 || pos.Y < 0 {
		panic("negative coordinates")
	}

	el, ok := l.Front()
	if !ok {
		return ListNode{}, ok
	}

	for i := 0; i < l.offset; i++ {
		el, _ = el.Next()
	}

	i := 0
	y := pos.Y
	ok = true
	for ok {
		if i*l.elementHeight+l.elementHeight > y {
			return el, true
		}
		el, ok = el.Next()
		i++
	}

	return ListNode{}, false
}

// Sort returns a copy of this List sorted with the provided less function.
func (l *List) Sort(less func(a, b tui.Component) bool) {
	els := make([]ListNode, l.Len())
	i := 0
	for node, ok := l.Front(); ok; node, ok = node.Next() {
		els[i] = node
		i++
	}

	sort.Slice(els, func(i, j int) bool {
		a := els[i].Value()
		b := els[j].Value()
		return less(a, b)
	})

	for i := l.Len() - 1; i >= 0; i-- {
		l.MoveToFront(els[i])
	}
}

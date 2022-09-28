package component

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

const defaultElementHeight = 1

var (
	defaultTextAttr = term.Attributes{
		Bg: term.ColorDefault,
		Fg: term.ColorDefault,
	}
	highlightTextAttr = term.Attributes{
		Bg: term.ColorDefault,
		Fg: term.ColorDefault | term.AttrBold,
	}
)

// FocusList wraps a List to provide an element Focus. It takes WithAttributes
// components.
type FocusList struct {
	Inverted      bool
	list          *List
	focus         ListNode
	focusIdx      int
	height, width int
	textAttr      term.Attributes
	focusAttr     term.Attributes
}

// NewFocusList allocates storage for a new FocusList and initializes it.
func NewFocusList() *FocusList {
	ret := new(FocusList)
	ret.Init()
	return ret
}

// Init initializes this FocusList with the default element height of 1.
func (l *FocusList) Init() {
	l.InitWithAttr(defaultTextAttr, highlightTextAttr)
}

// InitWithAttr initializes this FocusList with the default element height of 1,
// and text as the text attributes and focus as the focus attributes.
func (l *FocusList) InitWithAttr(text, focus term.Attributes) {
	l.list = NewList(defaultElementHeight)
	l.focus = ListNode{}
	l.focusIdx = 0
	l.textAttr = text
	l.focusAttr = focus
}

func setAttr(n ListNode, attr term.Attributes) {
	n.Value().(WithAttributes).SetAttr(attr)
}

func (l *FocusList) switchFocus(newFocus ListNode) {
	if l.focus.Value() != nil {
		setAttr(l.focus, l.textAttr)
	}
	l.focus = newFocus
	if l.focus.Value() != nil {
		setAttr(l.focus, l.focusAttr)
	}
}

func (l *FocusList) trySetFirstFocus(node ListNode) bool {
	if l.focus.Value() == nil {
		l.switchFocus(node)
		return true
	}
	return false
}

// SetFocus sets the focus of this FocusList to node.
func (l *FocusList) SetFocus(node ListNode) {
	if node.l != l.list {
		panic("ListNode does not belong to FocusList")
	}

	defer l.switchFocus(node)

	// In order to find the new focusIdx, first attempt to scan
	// the current view to optimize SetFocus for calls with a node currently
	// rendered on screen.
	idx := l.focusIdx
	for n, ok := l.focus.Next(); ok; n, ok = n.Next() {
		idx++
		if n == node {
			l.focusIdx = idx
			return
		}
		// Ignore element height and use height as a vague representation
		// of the current view over this FocusList.
		if idx > l.focusIdx+l.height {
			break
		}
	}

	idx = l.focusIdx
	for n, ok := l.focus.Prev(); ok; n, ok = n.Prev() {
		idx--
		if n == node {
			l.focusIdx = idx
			return
		}
		if idx < l.focusIdx-l.height {
			break
		}
	}

	// fallback to iterating entire list
	l.focusIdx = 0
	for n, ok := l.list.Front(); ok; n, ok = n.Next() {
		if n == node {
			return
		}
		l.focusIdx++
	}

	panic("could not finde ListNode in FocusList")
}

// Reset resets the contents of this FocusList.
func (l *FocusList) Reset() {
	l.switchFocus(ListNode{})
	l.focusIdx = 0
	l.list.Reset()
}

// Back returns the last node of list l or nil if the list is empty.
func (l *FocusList) Back() (ListNode, bool) {
	n, ok := l.list.Back()
	return n, ok
}

// Front returns the first node of list l or nil if the list is empty.
func (l *FocusList) Front() (ListNode, bool) {
	n, ok := l.list.Front()
	return n, ok
}

// Draw satisfies tui.Component
func (l *FocusList) Draw(w term.Writer) {
	l.list.Draw(w)
}

// ElementAt returns the element at pos Coordinates or panics if
// coordinates are out of the bounds of this List.
func (l *FocusList) ElementAt(pos term.Coordinates) (ListNode, bool) {
	node, ok := l.list.ElementAt(pos)
	return node, ok
}

// ElementHeight returns the height for each element of this list.
func (l *FocusList) ElementHeight() int {
	return l.list.ElementHeight()
}

// Len returns the number of nodes of list l in O(1).
func (l *FocusList) Len() int {
	return l.list.Len()
}

// PushBack inserts a new element c at the back of list l and
// returns the linked node.
func (l *FocusList) PushBack(c WithAttributes) ListNode {
	n := l.list.PushBack(c)
	setAttr(n, l.textAttr)
	l.trySetFirstFocus(n)
	return n
}

// PushBackList inserts a copy of an other list at the back of list l. The
// lists l and other must NOT be the same or nil.
func (l *FocusList) PushBackList(other *FocusList) {
	focus, ok := other.Focus()
	if ok {
		setAttr(focus, l.textAttr)
	}
	other.Iterate(func(c WithAttributes) {
		l.PushBack(c)
	})
}

// PushFront inserts a new element c with value v at the front of list l and
// returns e.
func (l *FocusList) PushFront(c WithAttributes) ListNode {
	n := l.list.PushFront(c)
	setAttr(n, l.textAttr)
	if !l.trySetFirstFocus(n) {
		l.focusIdx++
	}
	return n
}

// PushFrontList inserts a copy of an other list at the front of list l. The
// lists l and other must NOT be the same or nil.
func (l *FocusList) PushFrontList(other *FocusList) {
	focus, ok := other.Focus()
	if ok {
		setAttr(focus, l.textAttr)
	}
	for node, ok := other.Back(); ok; node, ok = node.Prev() {
		l.PushFront(node.Value().(WithAttributes))
	}
}

// Iterate iterates over all elements in l.
func (l *FocusList) Iterate(fn func(WithAttributes)) {
	for node, ok := l.list.Front(); ok; node, ok = node.Next() {
		fn(node.Value().(WithAttributes))
	}
}

// Resize satisfies tui.Compontent
func (l *FocusList) Resize(width, height int) {
	l.height, l.width = height, width
	l.list.Resize(width, height)
}

// SetElementHeight sets the height for each element of this list.
func (l *FocusList) SetElementHeight(height int) {
	l.list.SetElementHeight(height)
}

// FocusDown sets the focus to the node after the current focus
// and returns true, or if the focus is already the last node,
// it does nothing and returns false.
func (l *FocusList) FocusDown() bool {
	next, ok := l.focus.Next()
	if ok {
		l.focusIdx++
		l.switchFocus(next)
		if l.focusIdx-l.list.Offset() >= l.height-1 {
			l.list.SeekDown()
		}
	}
	return ok
}

// CanFocusDown returns true if FocusDown would return true.
func (l *FocusList) CanFocusDown() bool {
	_, ok := l.focus.Next()
	return ok
}

// CanFocusUp returns true if FocusUp would return true.
func (l *FocusList) CanFocusUp() bool {
	_, ok := l.focus.Prev()
	return ok
}

// FocusUp sets the focus to the node before the current focus
// and returns true, or if the focus is already the first node,
// it does nothing and returns false.
func (l *FocusList) FocusUp() bool {
	prev, ok := l.focus.Prev()
	if ok {
		l.focusIdx--
		l.switchFocus(prev)
		if l.focusIdx-l.list.Offset() <= 0 {
			l.list.SeekUp()
		}
	}
	return ok
}

// FocusStart sets the focus to the first node of l.
func (l *FocusList) FocusStart() (ok bool) {
	front, ok := l.Front()
	if !ok {
		return ok
	}
	l.list.SeekStart()
	l.focusIdx = 0
	l.switchFocus(front)
	return ok
}

// FocusEnd sets the focus to the last node of l.
func (l *FocusList) FocusEnd() (ok bool) {
	back, ok := l.Back()
	if !ok {
		return ok
	}
	l.list.SeekEnd()
	l.focusIdx = l.list.Len() - 1
	l.switchFocus(back)
	return ok
}

// Focus returns the current node in focus.
func (l *FocusList) Focus() (ListNode, bool) {
	if l.focus.Value() == nil {
		return ListNode{}, false
	}
	return l.focus, true
}

// Offset returns this list's current seek offset.
func (l *FocusList) Offset() int {
	return l.list.Offset()
}

// FocusOffset returns this list's focus index in the underlying list.
func (l *FocusList) FocusOffset() int {
	return l.focusIdx
}

// Sort sorts the elements of this list with the provided less function.
// It also resets the current focus node, according to the Inverted
// property in FocusList.
func (l *FocusList) Sort(less func(a, b WithAttributes) bool) {
	l.list.Sort(func(a, b tui.Component) bool {
		return less(a.(WithAttributes), b.(WithAttributes))
	})

	if l.Inverted {
		l.FocusEnd()
	} else {
		l.FocusStart()
	}
}

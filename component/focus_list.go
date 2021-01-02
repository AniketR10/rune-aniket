package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
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

// WithAttributes represents a tui.Component that can be set attributes.
type WithAttributes interface {
	tui.Component
	SetAttr(attr term.Attributes)
}

// FocusList wraps a List to provide an element Focus. It takes WithAttributes
// components.
type FocusList struct {
	list          *List
	focus         ListNode
	focusOffset   int
	height, width int
}

// NewFocusList allocates storage for a new FocusList and initializes it.
func NewFocusList() *FocusList {
	ret := new(FocusList)
	ret.Init()
	return ret
}

// Init initializes this FocusList with the default element height of 1.
func (l *FocusList) Init() {
	l.list = NewList(defaultElementHeight)
	l.focus = ListNode{}
	l.focusOffset = 0
}

func setFocusFrameCharSet(focus ListNode, attr term.Attributes) {
	v := focus.Value()
	if v == nil {
		return
	}
	v.(WithAttributes).SetAttr(attr)
}

func (l *FocusList) switchFocus(newFocus ListNode) {
	setFocusFrameCharSet(l.focus, defaultTextAttr)
	l.focus = newFocus
	setFocusFrameCharSet(l.focus, highlightTextAttr)

	l.seekFocus()
}

func (l *FocusList) seekFocus() {
	ok := true
	for ok && l.focusOffset > 0 && l.focusOffset >= l.height {
		ok = l.SeekDown()
	}

	ok = true
	for ok && l.focusOffset < l.height && l.focusOffset < 0 {
		ok = l.SeekUp()
	}
}

func (l *FocusList) trySetFirstFocus(node ListNode) bool {
	if l.focus.Value() == nil {
		l.switchFocus(node)
		return true
	}
	return false
}

// Reset resets the contents of this FocusList.
func (l *FocusList) Reset() {
	l.list.Reset()
	l.focusOffset = 0
	l.focus = ListNode{}
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
	l.trySetFirstFocus(n)
	return n
}

// PushBackList inserts a copy of an other list at the back of list l. The
// lists l and other must NOT be the same or nil.
func (l *FocusList) PushBackList(other *FocusList) {
	focus, ok := other.Focus()
	if ok {
		setFocusFrameCharSet(focus, defaultTextAttr)
	}
	for node, ok := other.Front(); ok; node, ok = node.Next() {
		l.PushBack(node.Value().(WithAttributes))
	}
}

// PushFront inserts a new element c with value v at the front of list l and
// returns e.
func (l *FocusList) PushFront(c WithAttributes) ListNode {
	n := l.list.PushFront(c)
	if !l.trySetFirstFocus(n) {
		l.focusOffset++
		l.seekFocus()
	}
	return n
}

// PushFrontList inserts a copy of an other list at the front of list l. The
// lists l and other must NOT be the same or nil.
func (l *FocusList) PushFrontList(other *FocusList) {
	focus, ok := other.Focus()
	if ok {
		setFocusFrameCharSet(focus, defaultTextAttr)
	}
	for node, ok := other.Back(); ok; node, ok = node.Prev() {
		l.PushFront(node.Value().(WithAttributes))
	}
}

// Remove removes e from l if e is a node of list l. It returns the element
// value e.Value. If the removed node is the focus, the focus will be switched
// first to the element below, and if not possible, to the element above.
func (l *FocusList) Remove(e ListNode) WithAttributes {
	if l.focus == e {
		if !l.FocusDown() {
			l.FocusUp()
		}
	}
	return l.list.Remove(e).(WithAttributes)
}

// Resize satisfies tui.Compontent
func (l *FocusList) Resize(width, height int) {
	l.height, l.width = height, width
	l.list.Resize(width, height)
	l.seekFocus()
}

// SeekDown shifts the contents of this list one row down.
func (l *FocusList) SeekDown() (ok bool) {
	if l.list.SeekDown() {
		l.focusOffset--
		ok = true
	}
	return
}

// SeekUp shifts the contents of this list one row up.
func (l *FocusList) SeekUp() (ok bool) {
	if l.list.SeekUp() {
		l.focusOffset++
		ok = true
	}
	return
}

// SeekEnd shifts the contents of this list such that the last element
// is drawn at the top of the list.
func (l *FocusList) SeekEnd() (ok bool) {
	for l.SeekDown() {
		ok = true
	}
	return
}

// SeekStart shifts the contents of this list such that the first element
// is drawn at the top f the list.
func (l *FocusList) SeekStart() (ok bool) {
	for l.SeekUp() {
		ok = true
	}
	return
}

// CanSeekDown returns whether SeekUp would seek one row down.
func (l *FocusList) CanSeekDown() bool {
	return l.list.CanSeekDown()
}

// CanSeekUp returns whether SeekUp would seek one row up.
func (l *FocusList) CanSeekUp() bool {
	return l.list.CanSeekUp()
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
		l.focusOffset++
		l.switchFocus(next)
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
		l.focusOffset--
		l.switchFocus(prev)
	}
	return ok
}

// FocusStart sets the focus to the first node of l.
func (l *FocusList) FocusStart() (ok bool) {
	for l.FocusUp() {
		ok = true
	}
	return
}

// FocusEnd sets the focus to the last node of l.
func (l *FocusList) FocusEnd() (ok bool) {
	for l.FocusDown() {
		ok = true
	}
	return
}

// Focus returns the current node in focus.
func (l *FocusList) Focus() (ListNode, bool) {
	if l.focus.Value() == nil {
		return ListNode{}, false
	}
	return l.focus, true
}

// Sort sorts the elements of this list with the provided less function.
func (l *FocusList) Sort(less func(a, b WithAttributes) bool) {
	l.list.Sort(func(a, b tui.Component) bool {
		return less(a.(WithAttributes), b.(WithAttributes))
	})
	l.focusOffset = 0
	l.focus, _ = l.Front()
}

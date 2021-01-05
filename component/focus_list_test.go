package component

import (
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type compWithAttr struct {
	tui.Component
}

func (c compWithAttr) SetAttr(attr term.Attributes) {}

type focusListTestList struct {
	*FocusList
}

func (l *focusListTestList) PushBackList(other testList) {
	l.FocusList.PushBackList(other.(*focusListTestList).FocusList)
}
func (l *focusListTestList) PushFrontList(other testList) {
	l.FocusList.PushFrontList(other.(*focusListTestList).FocusList)
}

func (l *focusListTestList) PushBack(c tui.Component) ListNode {
	return l.FocusList.PushBack(compWithAttr{c})
}

func (l *focusListTestList) PushFront(c tui.Component) ListNode {
	return l.FocusList.PushFront(compWithAttr{c})
}

func (l *focusListTestList) Remove(e ListNode) tui.Component {
	return l.FocusList.Remove(e)
}

func (l *focusListTestList) Sort(less func(a, b tui.Component) bool) {
	l.FocusList.Sort(func(a, b WithAttributes) bool {
		return less(a.(compWithAttr).Component, b.(compWithAttr).Component)
	})
}

func newFocusTestList(i int) testList {
	ret := &focusListTestList{FocusList: NewFocusList()}
	ret.FocusList.SetElementHeight(i)
	return ret
}

func TestFocusListFocus(t *testing.T) {
	t.Run("Focus returns false if list is empty", func(t *testing.T) {
		l := NewFocusList()
		_, ok := l.Focus()
		assert.False(t, ok)
	})

	t.Run("Focus returns true and the focus of the list", func(t *testing.T) {
		el1 := compWithAttr{&TestComponent{Ch: 'c'}}
		el2 := compWithAttr{&TestComponent{Ch: 'a'}}
		l := NewFocusList()

		l.PushFront(el1)
		n, ok := l.Focus()
		assert.True(t, ok)
		assert.Equal(t, n.Value(), el1)

		l.PushFront(el2)
		n, ok = l.Focus()
		assert.True(t, ok)
		assert.Equal(t, n.Value(), el1)

		l.FocusUp()
		n, ok = l.Focus()
		assert.True(t, ok)
		assert.Equal(t, n.Value(), el2)
	})

	t.Run("focus sequence", func(t *testing.T) {
		l := NewFocusList()
		require.False(t, l.CanFocusDown())
		require.False(t, l.CanFocusUp())

		l.PushBack(compWithAttr{&TestComponent{}})
		require.False(t, l.CanFocusDown())
		require.False(t, l.CanFocusUp())

		l.PushBack(compWithAttr{&TestComponent{}})
		require.True(t, l.CanFocusDown())
		require.False(t, l.CanFocusUp())

		l.PushFront(compWithAttr{&TestComponent{}})
		require.True(t, l.CanFocusDown())
		require.True(t, l.CanFocusUp())

		l.FocusUp()
		require.True(t, l.CanFocusDown())
		require.False(t, l.CanFocusUp())

		l.FocusEnd()
		require.False(t, l.CanFocusDown())
		require.True(t, l.CanFocusUp())

		l.FocusStart()
		require.True(t, l.CanFocusDown())
		require.False(t, l.CanFocusUp())

		l.FocusDown()
		require.True(t, l.CanFocusDown())
		require.True(t, l.CanFocusUp())
	})
}

func TestFocusListDraw(t *testing.T) {
	testListDraw(t, newFocusTestList)
}

func TestFocusListSort(t *testing.T) {
	testListSort(t, newFocusTestList)

	t.Run("focus and seeks to start of list upon call Sort", func(t *testing.T) {
		l := NewFocusList()
		l.PushBack(compWithAttr{&TestComponent{Ch: 'z'}})
		l.PushBack(compWithAttr{&TestComponent{Ch: 'x'}})
		l.PushBack(compWithAttr{&TestComponent{Ch: 'a'}})
		l.Resize(1, 1)
		require.True(t, l.SeekEnd())
		require.True(t, l.CanSeekUp())
		require.False(t, l.CanSeekDown())

		l.Sort(func(a, b WithAttributes) bool {
			return a.(compWithAttr).Component.(*TestComponent).Ch <
				b.(compWithAttr).Component.(*TestComponent).Ch
		})

		assert.False(t, l.CanSeekUp())
		assert.False(t, l.CanFocusUp())
		assert.True(t, l.CanSeekDown())
		assert.True(t, l.CanFocusDown())
	})
}

func TestFocusListFrontBack(t *testing.T) {
	testFrontBack(t, newFocusTestList)
}

func TestFocusListListRemove(t *testing.T) {
	testListRemove(t, newFocusTestList)
}

func TestFocusListEmptyDraw(t *testing.T) {
	testEmptyListDraw(t, newFocusTestList)
}

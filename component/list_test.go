package component

import (
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewList(t *testing.T) {
	l := NewList(1)

	l.Resize(8, 4)

	if l.ElementHeight() != 1 || l.width != 8 || l.height != 4 {
		t.Errorf("sizes not initialized correctly: %+v", l)
	}

	var l2 List

	l2.Init(1)
	l.Resize(8, 4)

	if l.ElementHeight() != 1 || l.width != 8 || l.height != 4 {
		t.Errorf("sizes not initialized correctly: %+v", l)
	}
}

func testEmptyListDraw(t *testing.T, constructor func(int) testList) {
	l := constructor(1)
	w := term.NewStringWriter(8, 4)

	assert.NotPanics(t, func() {
		l.Draw(w)
		l.Resize(10, 10)
		l.Draw(w)
	})
}

type testList interface {
	Back() (ListNode, bool)
	CanSeekDown() bool
	CanSeekUp() bool
	Draw(w term.Writer)
	ElementAt(pos term.Coordinates) (ListNode, bool)
	ElementHeight() int
	Front() (ListNode, bool)
	Len() int
	PushBackList(other testList)
	PushFrontList(other testList)
	PushBack(c tui.Component) ListNode
	PushFront(c tui.Component) ListNode
	Remove(e ListNode) tui.Component
	Resize(width, height int)
	SeekDown() bool
	SeekEnd() (ok bool)
	SeekStart() (ok bool)
	SeekUp() bool
	SetElementHeight(height int)
	Sort(func(a, b tui.Component) bool)
	Reset()
}

type listTestList struct {
	*List
}

func (l *listTestList) PushBackList(other testList) {
	l.List.PushBackList(other.(*listTestList).List)
}
func (l *listTestList) PushFrontList(other testList) {
	l.List.PushFrontList(other.(*listTestList).List)
}
func (l *listTestList) Sort(less func(a, b tui.Component) bool) {
	l.List.Sort(less)
}

func newTestList(i int) testList {
	ret := &listTestList{List: NewList(i)}
	return ret
}

func makeListOfTwo() (*List, []*TestComponent) {
	l := NewList(1)
	el1 := &TestComponent{Ch: 'A'}
	el2 := &TestComponent{Ch: 'b'}

	l.PushBack(el1)
	l.PushBack(el2)
	return l, []*TestComponent{el1, el2}
}

func testListDraw(t *testing.T, constructor func(int) testList) {
	l := constructor(1)
	l2 := constructor(1)

	l.Resize(8, 4)

	l2.Resize(8, 4)

	w := term.NewStringWriter(8, 4)

	tests := []testCase{
		{
			nil, `
        
        
        
        `,
		}, {
			func() { l.PushBack(&TestComponent{Ch: 'X'}) }, `
XXXXXXXX
        
        
        `,
		}, {
			func() { l.PushBack(&TestComponent{Ch: 'Y'}) }, `
XXXXXXXX
YYYYYYYY
        
        `,
		}, {
			func() { l.SeekDown() }, `
XXXXXXXX
YYYYYYYY
        
        `,
		}, {
			func() { l.SeekUp() }, `
XXXXXXXX
YYYYYYYY
        
        `,
		}, {
			func() { l.PushFront(&TestComponent{Ch: 'Z'}) }, `
ZZZZZZZZ
XXXXXXXX
YYYYYYYY
        `,
		}, {
			func() {
				l2.PushFront(&TestComponent{Ch: '$'})
				l2.PushFront(&TestComponent{Ch: '#'})
				l.PushBackList(l2)
			}, `
ZZZZZZZZ
XXXXXXXX
YYYYYYYY
########`,
		}, {
			func() { l.SeekUp() }, `
ZZZZZZZZ
XXXXXXXX
YYYYYYYY
########`,
		}, {
			func() { l.SeekDown() }, `
XXXXXXXX
YYYYYYYY
########
$$$$$$$$`,
		}, {
			func() { l.SeekDown() }, `
XXXXXXXX
YYYYYYYY
########
$$$$$$$$`,
		}, {
			func() {
				l.Resize(4, 4)
				_, ok := l.ElementAt(term.Coordinates{Y: 0})
				require.True(t, ok)
				_, ok = l.ElementAt(term.Coordinates{Y: 1})
				require.True(t, ok)
				_, ok = l.ElementAt(term.Coordinates{Y: 2})
				require.True(t, ok)
				_, ok = l.ElementAt(term.Coordinates{Y: 3})
				require.True(t, ok)
				_, ok = l.ElementAt(term.Coordinates{Y: 4})
				assert.False(t, ok)
				assert.Panics(t, func() {
					l.ElementAt(term.Coordinates{Y: -1})
				})
			}, `
XXXX    
YYYY    
####    
$$$$    `,
		}, {
			func() { l.SetElementHeight(2) }, `
XXXX    
XXXX    
YYYY    
YYYY    `,
		}, {
			func() { l.Resize(8, 4) }, `
XXXXXXXX
XXXXXXXX
YYYYYYYY
YYYYYYYY`,
		}, {
			func() { l.SetElementHeight(3) }, `
XXXXXXXX
XXXXXXXX
XXXXXXXX
        `,
		}, {
			func() { l.SeekDown() }, `
YYYYYYYY
YYYYYYYY
YYYYYYYY
        `,
		}, {
			func() { l.SeekEnd() }, `
$$$$$$$$
$$$$$$$$
$$$$$$$$
        `,
		}, {
			func() { l.SeekStart() }, `
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ
        `,
		}, {
			func() {
				l.SetElementHeight(1)
				l.Sort(func(a, b tui.Component) bool {
					return a.(*TestComponent).Ch < b.(*TestComponent).Ch
				})
			}, `
########
$$$$$$$$
XXXXXXXX
YYYYYYYY`,
		},
	}

	testWorkflow(t, l, w, tests)
}

func TestListNode(t *testing.T) {
	l := NewList(1)
	c1, c2, c3, c4 := NewScroll(), NewScroll(), NewScroll(), NewScroll()
	el1 := l.PushFront(c1)
	el2 := l.PushBack(c2)
	el4 := l.InsertAfter(c4, el2)
	el3 := l.InsertBefore(c3, el4)

	t.Run("Value returns the component instance passed in PushFront", func(t *testing.T) {
		assert.Equal(t, c1, el1.Value())
	})

	t.Run("Value returns the component instance passed in PushBack", func(t *testing.T) {
		assert.Equal(t, c2, el2.Value())
	})

	t.Run("Value returns the component instance passed in PushFront", func(t *testing.T) {
		assert.Equal(t, c3, el3.Value())
	})

	t.Run("Value returns the component instance passed in PushBack", func(t *testing.T) {
		assert.Equal(t, c4, el4.Value())
	})

	t.Run("Value on non-linked element returns false", func(t *testing.T) {
		assert.Nil(t, ListNode{}.Value())
	})

	t.Run("Prev returns the previous node", func(t *testing.T) {
		prev, ok := el2.Prev()
		require.True(t, ok)
		assert.Equal(t, el1, prev)
	})

	t.Run("Prev returns false if first node", func(t *testing.T) {
		_, ok := el1.Prev()
		require.False(t, ok)
	})

	t.Run("Next returns the next node", func(t *testing.T) {
		next, ok := el3.Next()
		require.True(t, ok)
		assert.Equal(t, el4, next)
	})

	t.Run("Next returns false if last node", func(t *testing.T) {
		_, ok := el4.Next()
		require.False(t, ok)
	})
}

func testListRemove(t *testing.T, constructor func(int) testList) {
	t.Run("Remvoe returns element in list", func(t *testing.T) {
		el := &TestComponent{}
		l := constructor(1)
		n := l.PushBack(el)

		ret := l.Remove(n)
		assert.Equal(t, el, getTestComponent(ret))
	})

	t.Run("Remove is idempotent", func(t *testing.T) {
		el := &TestComponent{}
		l := constructor(1)
		n := l.PushBack(el)
		assert.Equal(t, 1, l.Len())

		ret := l.Remove(n)
		assert.Equal(t, 0, l.Len())
		assert.NotNil(t, ret)

		ret = l.Remove(n)
		assert.NotNil(t, ret)
		assert.Equal(t, 0, l.Len())
	})
}

func testFrontBack(t *testing.T, constructor func(int) testList) {
	t.Run("Front/Back returns ok=false if empty list", func(t *testing.T) {
		l := constructor(1)
		_, ok := l.Back()
		assert.False(t, ok)

		_, ok = l.Front()
		assert.False(t, ok)
	})

	t.Run("Front/Back returns same element if list is of size 1", func(t *testing.T) {
		el := &TestComponent{Ch: 'C'}
		l := constructor(1)
		l.PushBack(el)

		front, ok := l.Front()
		assert.True(t, ok)
		back, ok := l.Back()
		assert.True(t, ok)

		assert.Equal(t, front, back)
	})

	t.Run("Back returns last element in list", func(t *testing.T) {
		l, els := makeListOfTwo()

		ret, ok := l.Back()
		assert.True(t, ok)
		assert.Equal(t, els[1], ret.Value())
	})

	t.Run("Front returns last element in list", func(t *testing.T) {
		l, els := makeListOfTwo()

		ret, ok := l.Front()
		assert.True(t, ok)
		assert.Equal(t, els[0], ret.Value())
	})
}

// bridge between testList and focusTestList
func getTestComponent(v tui.Component) *TestComponent {
	if a, ok := v.(compWithAttr); ok {
		return a.Component.(*TestComponent)
	}
	return v.(*TestComponent)
}

func sortTestComponent(a, b tui.Component) bool {
	return getTestComponent(a).Ch < getTestComponent(b).Ch
}

func assertEqualNodes(t *testing.T, expected int, l testList) {
	var i int
	for node, ok := l.Front(); ok; node, ok = node.Next() {
		i++
	}
	assert.Equal(t, expected, i)
}

func testListSort(t *testing.T, constructor func(int) testList) {
	t.Run("returns a copy of the list with same element height", func(t *testing.T) {
		l := constructor(1)

		// make sure returned list is correct
		for i := 0; i < 3; i++ {
			l.Sort(func(a, b tui.Component) bool { return false })
			assert.Equal(t, 1, l.ElementHeight())
		}
	})

	t.Run("returns a copy of the list sorted", func(t *testing.T) {
		l := constructor(1)
		l.PushBack(&TestComponent{Ch: 'b'})
		l.PushBack(&TestComponent{Ch: 'a'})
		l.PushBack(&TestComponent{Ch: 'c'})
		// make sure returned list is correct
		for i := 0; i < 3; i++ {
			l.Sort(func(a, b tui.Component) bool {
				return getTestComponent(a).Ch < getTestComponent(b).Ch
			})
			node, ok := l.Front()
			require.True(t, ok)
			assert.Equal(t, 'a', getTestComponent(node.Value()).Ch)

			node, ok = node.Next()
			require.True(t, ok)
			assert.Equal(t, 'b', getTestComponent(node.Value()).Ch)

			node, ok = node.Next()
			require.True(t, ok)
			assert.Equal(t, 'c', getTestComponent(node.Value()).Ch)

			node, ok = node.Next()
			require.False(t, ok)
		}
	})

	t.Run("incremental sorting of pushed elements", func(t *testing.T) {
		l := constructor(1)

		node := l.PushBack(&TestComponent{Ch: 'z'})
		_, ok := node.Next()
		assert.False(t, ok)

		l.Sort(sortTestComponent)

		node, ok = l.Front()
		require.True(t, ok)
		assert.Equal(t, 'z', getTestComponent(node.Value()).Ch)
		assertEqualNodes(t, 1, l)

		node = l.PushBack(&TestComponent{Ch: 'y'})
		_, ok = node.Next()
		assert.False(t, ok)

		l.Sort(sortTestComponent)

		node, ok = l.Front()
		require.True(t, ok)
		assert.Equal(t, 'y', getTestComponent(node.Value()).Ch)

		node, ok = node.Next()
		require.True(t, ok)
		assert.Equal(t, 'z', getTestComponent(node.Value()).Ch)

		assertEqualNodes(t, 2, l)
	})

	t.Run("does not invalidate preaviously leaked nodes", func(t *testing.T) {
		l := constructor(1)

		z := l.PushBack(&TestComponent{Ch: 'z'})
		_, ok := z.Next()
		assert.False(t, ok)

		y := l.PushBack(&TestComponent{Ch: 'y'})
		_, ok = y.Next()
		assert.False(t, ok)

		_, ok = z.Next()
		assert.True(t, ok)

		l.Sort(sortTestComponent)

		_, ok = y.Next()
		assert.True(t, ok)

		_, ok = z.Next()
		assert.False(t, ok)
	})
}

func TestListDraw(t *testing.T) {
	testListDraw(t, newTestList)
}

func TestListSort(t *testing.T) {
	testListSort(t, newTestList)
}

func TestFrontBack(t *testing.T) {
	testFrontBack(t, newTestList)
}

func TestListRemove(t *testing.T) {
	testListRemove(t, newTestList)
}

func TestEmptyListDraw(t *testing.T) {
	testEmptyListDraw(t, newTestList)
}

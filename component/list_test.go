package component

import (
	"testing"

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

func TestEmptyListDraw(t *testing.T) {
	l := NewList(1)
	w := term.NewStringWriter(8, 4)

	assert.NotPanics(t, func() {
		l.Draw(w)
		l.Resize(10, 10)
		l.Draw(w)
	})
}

func TestListDraw(t *testing.T) {
	l := NewList(1)
	l2 := NewList(1)

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
			l.SeekDown, `
XXXXXXXX
YYYYYYYY
        
        `,
		}, {
			l.SeekUp, `
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
			l.SeekUp, `
ZZZZZZZZ
XXXXXXXX
YYYYYYYY
########`,
		}, {
			l.SeekDown, `
XXXXXXXX
YYYYYYYY
########
$$$$$$$$`,
		}, {
			l.SeekDown, `
XXXXXXXX
YYYYYYYY
########
$$$$$$$$`,
		}, {
			func() {
				l.Resize(4, 4)
				elNode, ok := l.ElementAt(term.Coordinates{Y: 0})
				el := elNode.Value().(*TestComponent)
				require.True(t, ok)
				el1Node, ok := l.ElementAt(term.Coordinates{Y: 1})
				el1 := el1Node.Value().(*TestComponent)
				require.True(t, ok)
				el2Node, ok := l.ElementAt(term.Coordinates{Y: 2})
				el2 := el2Node.Value().(*TestComponent)
				require.True(t, ok)
				el3Node, ok := l.ElementAt(term.Coordinates{Y: 3})
				el3 := el3Node.Value().(*TestComponent)
				require.True(t, ok)
				assert.Equal(t, el.Ch, 'X')
				assert.Equal(t, el1.Ch, 'Y')
				assert.Equal(t, el2.Ch, '#')
				assert.Equal(t, el3.Ch, '$')
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
			l.SeekDown, `
YYYYYYYY
YYYYYYYY
YYYYYYYY
        `,
		}, {
			l.SeekEnd, `
$$$$$$$$
$$$$$$$$
$$$$$$$$
        `,
		}, {
			l.SeekStart, `
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ
        `,
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

package search

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func assertNoLeaks(t *testing.T, l *List) {
	assert.NoError(t, l.Close())
	goleak.VerifyNone(t)
}

func assertFocusEqual(t *testing.T, l *List, el []byte) {
	ret, ok := l.Focus()
	require.True(t, ok)
	assert.Equal(t, el, ret)
}

func TestListCount(t *testing.T) {
	var wg sync.WaitGroup
	l := NewList(ListConfig{Interrupt: wg.Done})

	l.PushSync([]byte("capitol insurrection"))
	assert.Equal(t, 1, l.TotalCount())
	assert.Equal(t, 1, l.MatchCount())

	wg.Add(1)
	l.SearchQueryWrite('c')
	wg.Wait()

	assert.Equal(t, 1, l.TotalCount())
	assert.Equal(t, 1, l.MatchCount())

	wg.Add(1)
	l.SearchQueryDelete()
	wg.Wait()

	// we cannot do two at a time because there's a race between canceling
	// the original async search and calling interrupt.
	wg.Add(1)
	l.SearchQueryWrite('X')
	wg.Wait()

	assert.Equal(t, 1, l.TotalCount())
	assert.Equal(t, 0, l.MatchCount())

	l.PushSync([]byte("X Files"))
	assert.Equal(t, 2, l.TotalCount())
	assert.Equal(t, 1, l.MatchCount())

	assertNoLeaks(t, l)
}

func TestListFocus(t *testing.T) {
	l := NewList(ListConfig{})
	_, ok := l.Focus()
	assert.False(t, ok)

	el1 := []byte("angry pants")
	el2 := []byte("angry girls")
	el3 := []byte("pants :@")
	l.PushSync(el1)
	l.PushSync(el2)
	l.PushSync(el3)
	assertFocusEqual(t, l, el1)

	assert.True(t, l.FocusDown())
	assert.True(t, l.FocusDown())
	assertFocusEqual(t, l, el3)

	assert.True(t, l.FocusStart())
	assertFocusEqual(t, l, el1)

	assert.True(t, l.FocusEnd())
	assertFocusEqual(t, l, el3)

	assert.True(t, l.FocusUp())
	assertFocusEqual(t, l, el2)

	assertNoLeaks(t, l)
}

func pushTestData(l *List, n int) {
	// push exactly the number of elements equal to
	// this component's height, so only interrupt should
	// be called only once while processing data
	for i := 0; i < n; i++ {
		l.Push() <- []byte(strconv.Itoa(i))
	}
}

func TestListAsyncPush(t *testing.T) {
	t.Run("no search", func(t *testing.T) {
		var wg sync.WaitGroup

		l := NewList(ListConfig{Interrupt: wg.Done})
		height := 100
		l.Resize(100, height)

		wg.Add(3)
		go func() {
			pushTestData(l, height)
			close(l.Push())
			wg.Done()
		}()

		wg.Wait()

		assert.Equal(t, height, l.TotalCount())
		assert.Equal(t, height, l.MatchCount())

		assertNoLeaks(t, l)
	})

	t.Run("search query after items pushed", func(t *testing.T) {
		var wg sync.WaitGroup
		l := NewList(ListConfig{Interrupt: wg.Done})
		height := 100
		l.Resize(100, height)

		wg.Add(1)
		pushTestData(l, height)
		wg.Wait()
		wg.Add(2)

		l.SearchQueryWrite('9')
		l.Wait()
		l.SearchQueryWrite('9')
		l.Wait()

		assert.Equal(t, height, l.TotalCount())
		assert.Equal(t, 1, l.MatchCount())

		assertNoLeaks(t, l)
	})

	t.Run("search query before items pushed", func(t *testing.T) {
		var wg sync.WaitGroup
		l := NewList(ListConfig{Interrupt: wg.Done})
		n := 100
		l.Resize(n, n)

		wg.Add(3)
		l.SearchQueryWrite('9')
		l.Wait() // make next search query doesn't cancel prev
		l.SearchQueryWrite('9')
		l.Wait()
		pushTestData(l, n)
		wg.Wait()

		// make sure it doesn't block
		l.Wait()

		assert.Equal(t, n, l.TotalCount())
		assert.Equal(t, 1, l.MatchCount())

		assertNoLeaks(t, l)
	})

	t.Run("concurrent search query", func(t *testing.T) {
		var wg sync.WaitGroup
		l := NewList(ListConfig{Interrupt: wg.Done})
		n := 100
		l.Resize(n, n)

		wg.Add(3)
		go pushTestData(l, n)

		l.SearchQueryWrite('9')
		l.Wait()
		l.SearchQueryWrite('9')
		l.Wait()

		wg.Wait()
		l.Wait()

		assert.Equal(t, n, l.TotalCount())
		assert.Equal(t, 1, l.MatchCount())

		assertNoLeaks(t, l)
	})
}

func TestListDraw(t *testing.T) {
	l := NewList(ListConfig{SearchBase: ":"})
	l.Resize(8, 4)

	w := term.NewStringWriter(8, 4)

	tests := []struct {
		action   func()
		expected string
	}{{
		nil, `
:    0/0
        
        
        `,
	}, {
		func() { l.PushSync([]byte("Safe Changes - Talaboman")) }, `
:    1/1
Safe Cha
        
        `,
	}, {
		func() {
			for i := 0; i < 19; i++ {
				l.PushSync([]byte("For the Time Being - Phonique"))
			}
		}, `
:  20/20
Safe Cha
For the 
For the `,
	}, {
		func() {
			l.SearchQueryWrite('P')
			l.Wait()
		}, `
:P 19/20
For the 
For the 
For the `,
	},
	}

	for _, tcase := range tests {
		if err := w.Clear(term.Attributes{}); err != nil {
			t.Fatal(err)
		}

		if tcase.action != nil {
			tcase.action()
		}

		l.Draw(w)

		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}

		// for readability, we expected strings are written starting with \n
		expected := strings.TrimLeft(tcase.expected, "\n")
		assert.Equal(t, expected, w.String())
	}
}

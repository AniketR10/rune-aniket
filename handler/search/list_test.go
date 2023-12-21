package search

import (
	"bytes"
	"context"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

type listConstructor func(ListConfig) (listIfc, *cell.Buffer)

type listIfc interface {
	Close() error
	DataReset()
	Draw(w term.Writer)
	Focus() (Match, bool)
	FocusDown() bool
	FocusEnd() bool
	FocusStart() bool
	FocusUp() bool
	InputHeight() int
	MatchCount() int
	Push(context.Context) chan<- []byte
	PushSync(b []byte) (matched bool)
	Resize(width, height int)
	SetMinInputHeight(height int)
	TotalCount() int
	Wait()
}

func wgInterrupter(wg *sync.WaitGroup) term.Interrupter {
	return term.FuncInterrupter(func(context.Context) error {
		wg.Done()
		return nil
	})
}

func assertNoLeaks(t *testing.T, l listIfc) {
	assert.NoError(t, l.Close())
	ignoreOpenCensus := goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")
	goleak.VerifyNone(t, ignoreOpenCensus)
}

func assertFocusEqual(t *testing.T, l listIfc, el []byte) {
	ret, ok := l.Focus()
	require.True(t, ok)
	assert.Equal(t, string(el), string(ret.Data()))
}

func newSimpleList(cfg ListConfig) (listIfc, *cell.Buffer) {
	// make tests deterministic
	cfg.interruptEvery = 24 * time.Hour
	cfg.setFileCountEvery = 1
	l := NewList(cfg)
	return l, l.Buffer()
}

func newSimpleListBottomSearchBar(cfg ListConfig) (listIfc, *cell.Buffer) {
	cfg.BottomSearchBar = true
	cfg.interruptEvery = 24 * time.Hour
	cfg.setFileCountEvery = 1
	l := NewList(cfg)
	return l, l.Buffer()
}

func TestListCount(t *testing.T) {
	testListCount(t, newSimpleList)
}

func testListCount(t *testing.T, constructor listConstructor) {
	var wg sync.WaitGroup
	l, buf := constructor(ListConfig{Interrupter: wgInterrupter(&wg)})

	l.PushSync([]byte("capitol insurrection"))
	assert.Equal(t, 1, l.TotalCount())
	assert.Equal(t, 1, l.MatchCount())

	wg.Add(1)
	buf.WriteString("c")
	wg.Wait()

	assert.Equal(t, 1, l.TotalCount())
	assert.Equal(t, 1, l.MatchCount())

	wg.Add(1)
	buf.DeleteCell(term.Coordinates{X: 0})
	wg.Wait()

	// we cannot do two at a time because there's a race between canceling
	// the original async search and calling interrupt.
	wg.Add(1)
	buf.WriteString("X")
	wg.Wait()

	assert.Equal(t, 1, l.TotalCount())
	assert.Equal(t, 0, l.MatchCount())

	l.PushSync([]byte("X Files"))
	assert.Equal(t, 2, l.TotalCount())
	assert.Equal(t, 1, l.MatchCount())

	assertNoLeaks(t, l)
}

func TestListFocus(t *testing.T) {
	testListFocus(t, newSimpleList)
}

func testListFocus(t *testing.T, constructor listConstructor) {
	l, _ := constructor(ListConfig{})
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

func TestListFocusBottomSearchBar(t *testing.T) {
	testListFocusBottomSearchBar(t, newSimpleList)
}

func testListFocusBottomSearchBar(t *testing.T, constructor listConstructor) {
	l, _ := constructor(ListConfig{BottomSearchBar: true})
	_, ok := l.Focus()
	assert.False(t, ok)

	el1 := []byte("angry pants")
	el2 := []byte("angry girls")
	el3 := []byte("pants :@")
	l.PushSync(el1)
	l.PushSync(el2)
	l.PushSync(el3)
	assertFocusEqual(t, l, el3)

	assert.False(t, l.FocusDown())
	assert.True(t, l.FocusUp())
	assert.True(t, l.FocusUp())
	assertFocusEqual(t, l, el1)

	assert.True(t, l.FocusStart())
	assertFocusEqual(t, l, el1)

	assert.True(t, l.FocusEnd())
	assertFocusEqual(t, l, el3)

	assert.True(t, l.FocusUp())
	assertFocusEqual(t, l, el2)

	assertNoLeaks(t, l)
}

func pushTestData(l listIfc, n int) chan<- []byte {
	// push exactly the number of elements equal to
	// this component's height, so only interrupt should
	// be called only once while processing data
	ctx := context.Background()

	ch := l.Push(ctx)
	for i := 0; i < n; i++ {
		ch <- []byte(strconv.Itoa(i))
	}
	return ch
}

func TestListAsyncPush(t *testing.T) {
	testListAsyncPush(t, newSimpleList)
}

func TestListAsyncPushBottomSearchBar(t *testing.T) {
	testListAsyncPush(t, newSimpleListBottomSearchBar)
}

func testListAsyncPush(t *testing.T, constructor listConstructor) {
	t.Run("no search", func(t *testing.T) {
		var wg sync.WaitGroup

		l, _ := constructor(ListConfig{Interrupter: wgInterrupter(&wg)})
		height := 100
		l.Resize(100, height)

		wg.Add(2)
		go func() {
			defer wg.Done()
			ch := pushTestData(l, height)
			close(ch)
		}()

		wg.Wait()

		assert.Equal(t, height, l.TotalCount())
		assert.Equal(t, height, l.MatchCount())

		assertNoLeaks(t, l)
	})

	t.Run("search query after items pushed", func(t *testing.T) {
		var wg sync.WaitGroup
		l, buf := constructor(ListConfig{Interrupter: wgInterrupter(&wg)})
		height := 100
		l.Resize(100, height)

		wg.Add(1)
		ch := pushTestData(l, height)
		close(ch)
		wg.Wait()

		wg.Add(2)
		buf.WriteString("9")
		l.Wait()
		buf.WriteString("9")
		l.Wait()

		assert.Equal(t, height, l.TotalCount())
		assert.Equal(t, 1, l.MatchCount())

		assertNoLeaks(t, l)
	})

	t.Run("search query before items pushed", func(t *testing.T) {
		var wg sync.WaitGroup
		l, buf := constructor(ListConfig{Interrupter: wgInterrupter(&wg)})
		n := 100
		l.Resize(n, n)

		wg.Add(1)
		buf.WriteString("9")
		l.Wait() // make next search query doesn't cancel prev
		wg.Wait()

		wg.Add(1)
		buf.WriteString("9")
		l.Wait()
		wg.Wait()

		wg.Add(1)
		ch := pushTestData(l, n)
		close(ch)
		wg.Wait()

		// make sure it doesn't block
		l.Wait()

		assert.Equal(t, n, l.TotalCount())
		assert.Equal(t, 1, l.MatchCount())

		assertNoLeaks(t, l)
	})

	t.Run("concurrent search query", func(t *testing.T) {
		var wg sync.WaitGroup
		l, buf := constructor(ListConfig{Interrupter: wgInterrupter(&wg)})
		n := 100
		l.Resize(n, n)

		wg.Add(3)
		go func() {
			ch := pushTestData(l, n)
			close(ch)
		}()

		buf.WriteString("9")
		l.Wait()
		buf.WriteString("9")
		l.Wait()

		wg.Wait()
		l.Wait()

		assert.Equal(t, n, l.TotalCount())
		assert.Equal(t, 1, l.MatchCount())

		assertNoLeaks(t, l)
	})
}

func TestListDraw(t *testing.T) {
	testListDraw(t, newSimpleList)
}

func testListDraw(t *testing.T, constructor listConstructor) {
	l, buf := constructor(ListConfig{})
	l.Resize(8, 4)
	defer l.Close()

	w := term.NewStringWriter(8, 4)

	tests := []struct {
		action   func()
		expected string
	}{{
		nil, `
     0/0
        
        
        `,
	}, {
		func() { l.PushSync([]byte("Safe Changes - Talaboman")) }, `
     1/1
Safe Cha
        
        `,
	}, {
		func() {
			for i := 0; i < 19; i++ {
				l.PushSync([]byte("For the Time Being - Phonique"))
			}
		}, `
   20/20
Safe Cha
For the 
For the `,
	}, {
		func() {
			buf.WriteString("P")
			l.Wait()
		}, `
P  19/20
For the 
For the 
For the `,
	}, {
		func() {
			l.SetMinInputHeight(2)
		}, `
P       
   19/20
For the 
For the `,
	}, {
		func() {
			l.SetMinInputHeight(1)
			for i := 0; i < 8; i++ {
				buf.WriteString("P")
			}
			l.Wait()
		}, `
PPPPPPPP
P   0/20
        
        `,
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

func TestListDrawBottomSearchBar(t *testing.T) {
	testListDrawBottomSearchBar(t, newSimpleList)
}

func testListDrawBottomSearchBar(t *testing.T, constructor listConstructor) {
	l, buf := constructor(ListConfig{BottomSearchBar: true})
	l.Resize(8, 4)
	defer l.Close()

	w := term.NewStringWriter(8, 4)

	tests := []struct {
		action   func()
		expected string
	}{{
		nil, `
        
        
        
     0/0`,
	}, {
		func() { l.PushSync([]byte("Safe Changes - Talaboman")) }, `
Safe Cha
        
        
     1/1`,
	}, {
		func() {
			for i := 0; i < 19; i++ {
				l.PushSync([]byte("For the Time Being - Phonique"))
			}
		}, `
For the 
For the 
For the 
   20/20`,
	}, {
		func() {
			buf.WriteString("P")
			l.Wait()
		}, `
For the 
For the 
For the 
P  19/20`,
	}, {
		func() {
			l.SetMinInputHeight(2)
		}, `
For the 
For the 
P       
   19/20`,
	}, {
		func() {
			l.SetMinInputHeight(1)
			for i := 0; i < 8; i++ {
				buf.WriteString("P")
			}
			l.Wait()
		}, `
        
        
PPPPPPPP
P   0/20`,
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

func TestListWait(t *testing.T) {
	testListWait(t, newSimpleList)
}

func testListWait(t *testing.T, constructor listConstructor) {
	t.Run("does not panic a new list", func(t *testing.T) {
		l, _ := constructor(ListConfig{})
		require.NotPanics(t, l.Wait)
		assert.NoError(t, l.Close())
	})
}

func TestListBuffer(t *testing.T) {
	t.Run("inserts on shared buffer are materialized on search buffer", func(t *testing.T) {
		l := NewList(ListConfig{})
		shared := l.Buffer()
		defer l.Close()

		shared.WriteString("blah")
		assert.Equal(t, "blah", l.searchBar.internalRead.String())
	})

	t.Run("deletes on search buffer are materialized on shared buffer", func(t *testing.T) {
		l := NewList(ListConfig{})
		defer l.Close()

		shared := l.Buffer()
		shared.WriteString("blah")
		assert.True(t, shared.TruncateFrom(term.Coordinates{X: 1}))
		assert.Equal(t, "b", l.searchBar.internalRead.String())
	})
}

func TestListElementAt(t *testing.T) {
	l := NewList(ListConfig{BottomSearchBar: true})
	l.Resize(8, 4)
	defer l.Close()

	l.PushSync([]byte("JT"))
	l.PushSync([]byte("Gladiator"))

	match1, _, ok := l.ElementAt(term.Coordinates{})
	require.True(t, ok)

	assert.Equal(t, 0, match1.Index())
	assert.Equal(t, "JT", string(match1.Data()))

	match2, _, ok := l.ElementAt(term.Coordinates{Y: 1, X: 7})
	require.True(t, ok)

	assert.Equal(t, 1, match2.Index())
	assert.Equal(t, "Gladiator", string(match2.Data()))

	match2, _, ok = l.ElementAt(term.Coordinates{Y: 2})
	require.False(t, ok)
}

func TestListLeak(t *testing.T) {
	testListLeak(t, newSimpleList)
}

func testListLeak(t *testing.T, constructor listConstructor) {
	t.Run("does not leak when Init + Close", func(t *testing.T) {
		l, _ := constructor(ListConfig{})
		assertNoLeaks(t, l)
	})
}

func BenchmarkHandleSearch10(b *testing.B) {
	benchmarkHandleSearch(b, 10, false)
}
func BenchmarkHandleSearch100(b *testing.B) {
	benchmarkHandleSearch(b, 100, false)
}
func BenchmarkHandleSearch1000(b *testing.B) {
	benchmarkHandleSearch(b, 1000, false)
}
func BenchmarkHandleSearch1000000(b *testing.B) {
	benchmarkHandleSearch(b, 1000000, false)
}

func BenchmarkHandleSearchBottomSearchBar10(b *testing.B) {
	benchmarkHandleSearch(b, 10, true)
}
func BenchmarkHandleSearchBottomSearchBar100(b *testing.B) {
	benchmarkHandleSearch(b, 100, true)
}
func BenchmarkHandleSearchBottomSearchBar1000(b *testing.B) {
	benchmarkHandleSearch(b, 1000, true)
}
func BenchmarkHandleSearchBottomSearchBar1000000(b *testing.B) {
	benchmarkHandleSearch(b, 1000000, true)
}

func benchmarkHandleSearch(b *testing.B, n int, bottomSearchBar bool) {
	const sample = `2022-06-17 15:45:24.985	WARNING	[-]	-	msg: 3 errors occurred: Failed to load \"key_bindings.<m-k>\": invalid key: '<m-k>'; Failed to load \"key_bindings.<m-j>\": invalid key: '<m-j>'; Failed to load \"browser.frameunion_charset\": type is invalid
2022-06-17 15:45:24.985	WARNING	[-]	-	msg: 3 errors occurred: Failed to load \"key_bindings.<m-k>\": invalid key: '<m-k>'; Failed to load \"key_bindings.<m-j>\": invalid key: '<m-j>'; Failed to load \"browser.frameunion_charset\": type is invalid
2022-06-17 15:45:24.985	DEBUG	[-]	-	msg: starting extension
2022-06-17 15:45:24.986	DEBUG	[-]	-	msg: extension started
2022-06-17 15:45:24.986	DEBUG	[-]	-	msg: waiting for RPC address
2022-06-17 15:45:25.003	DEBUG	[-]	extension_fuzzy_file	msg: extension address
2022-06-17 15:45:25.003	DEBUG	[-]	-	msg: using extension
2022-06-17 15:45:25.003	INFO	[-]	extension_fuzzy_file	msg: listen tcp 127.0.0.1:6061: bind: address already in use
2022-06-17 15:45:25.004	TRACE	[-]	stdio	msg: waiting for stdio data
2022-06-17 15:45:25.004	DEBUG	[-]	-	msg: starting extension`
	lines := bytes.Split([]byte(sample), []byte{'\n'})
	data := make([][]byte, n)
	for i := 1; i < int(math.Max(1, float64(n/len(lines)))); i++ {
		for _, line := range lines {
			data = append(data, line)
		}
	}

	l := NewList(ListConfig{BottomSearchBar: bottomSearchBar})
	w := term.NoopWriter{}
	l.Resize(300, 200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// cannot call asyncSearch for benchmark
		// in order to measure handleSearch perf, we need to
		// run all the logic synchronously
		l.mu.Lock()
		if l.cancelSearch != nil {
			l.cancelSearch()
		}
		ctx := context.Background()
		l.searchCtx, l.cancelSearch = context.WithCancel(ctx)
		l.list.Reset()
		l.mu.Unlock()
		l.handleSearch(context.Background(), func() {}, data, "D")
		l.Draw(w)
	}
}

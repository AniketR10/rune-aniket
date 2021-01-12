package editor

import (
	"sync"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
	testutil "github.com/ernestrc/go-tui/util/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type funcEventHandler func(term.Event) bool

func (f funcEventHandler) Handle(ev term.Event) bool {
	return f(ev)
}

type browserInternal interface {
	browser.Browser
	tui.Handler
}

type browserConstructor func(ed Editor, opts ...browser.Option) (browserInternal, error)

type testEditor struct {
	name string
	buf  *cell.Buffer
}

func (e *testEditor) Edit(name string, buf *cell.Buffer) (Handler, error) {
	e.name = name
	e.buf = buf
	return handler.NewTestHandler(), nil
}

type testFileBuffer struct {
	flushErr error
	closeErr error
}

func (t *testFileBuffer) Flush() error {
	return t.flushErr
}

func (t *testFileBuffer) Close() error {
	return t.closeErr
}

func openTestFile(filePath string, buf *cell.Buffer, swapDir string) (
	browser.FlusherCloser, error,
) {
	return &testFileBuffer{}, nil
}

func recoverTestFile(filePath, swapFilePath string, buf *cell.Buffer) (
	browser.FlusherCloser, error,
) {
	return openTestFile(filePath, buf, "")
}

func newTestBrowserHandler() *Ex {
	ret := new(Ex)
	ret.openFileFn = openTestFile
	ret.recoverFileFn = recoverTestFile
	return ret
}

func TestBrowserHandlerDraw(t *testing.T) {
	testBrowserHandlerDraw(t, func(ed Editor, opts ...browser.Option) (browserInternal, error) {
		b := newTestBrowserHandler()
		err := b.Init(ed, opts...)
		if err != nil {
			return nil, err
		}
		return b, nil
	})
}

func testBrowserHandlerDraw(t *testing.T, constructor browserConstructor) {
	cases := []testutil.HandlerSequenceTestCase{
		{"asdf",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`},
		{":",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│                  │
│                  │
│▐                 │
└──────────────────┘`},
		{"e cabin.go>",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"a",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{":e other.go>",
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"#", // simulates ctrl-h
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{"#", // simulates ctrl-h
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"$", // simulates ctrl-l
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{"$", // simulates ctrl-l
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{":bclose>",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{":wq!^",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│wq▐               │
└──────────────────┘`},
		{"^^^^",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{":<",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{":e other.go>1111",
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
		{"$$##",
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
	}

	// testutil.TestHandlerSequence maps ':' characters to the following event
	// this is to work around ex's assumptions on underlying handler.
	commandEvent := term.Event{Type: term.EventKey, Key: term.KeyCtrlBackslash}
	browser, err := constructor(&testEditor{}, browser.WithCommandEvent(commandEvent))
	require.NoError(t, err)

	testutil.TestHandlerSequence(t, browser, 20, 10, cases)

	win, err := browser.SplitVerticalLeft(handler.NewTestHandler())
	require.NoError(t, err)

	// test window ifc
	focus, err := browser.Focus()
	require.NoError(t, err)

	h := handler.NewTestHandler()
	h.Ch = 'Z' // helps identify in tests

	err = browser.Subscribe(term.Event{Type: term.EventKey, Ch: ']'},
		funcEventHandler(func(ev term.Event) bool {
			browser.SplitHorizontalBelow(h)
			return false
		}))
	require.NoError(t, err)

	newMappings := map[term.Event]term.Event{
		term.Event{Type: term.EventKey, Ch: ')'}: term.Event{Type: term.EventKey, Key: term.KeyCtrlL},
		term.Event{Type: term.EventKey, Ch: '('}: term.Event{Type: term.EventKey, Key: term.KeyCtrlH},
	}
	require.NoError(t, browser.MergeKeyMap(newMappings))

	cases = []testutil.HandlerSequenceTestCase{
		{"]__",
			`┌──────────────────┐
│cabin.go  other.go│
├────────┐┌────────┤
│AAAAAAAA││EEEEEEEE│
│AAAAAAAA││EEEEEEEE│
└────────┘│EEEEEEEE│
┌────────┐│EEEEEEEE│
│ZZZZZZZZ││EEEEEEEE│
│ZZZZZZZZ││EEEEEEEE│
└────────┘└────────┘`},
		{":<111111111_",
			`┌──────────────────┐
│cabin.go  other.go│
├────────┐┌────────┤
│AAAAAAAA││EEEEEEEE│
│AAAAAAAA││EEEEEEEE│
└────────┘│EEEEEEEE│
┌────────┐│EEEEEEEE│
│cccccccc││EEEEEEEE│
│cccccccc││EEEEEEEE│
└────────┘└────────┘`},
	}

	testutil.TestHandlerSequence(t, browser, 20, 10, cases)

	var unmounted int
	hx := handler.NewTestHandler()
	hx.Ch = '$'
	hx.OnUnmountCallback = func() error { unmounted++; return nil }
	require.NoError(t, focus.SetContent(hx))

	cases = []testutil.HandlerSequenceTestCase{
		{"___",
			`┌──────────────────┐
│cabin.go  other.go│
├────────┐┌────────┤
│$$$$$$$$││EEEEEEEE│
│$$$$$$$$││EEEEEEEE│
└────────┘│EEEEEEEE│
┌────────┐│EEEEEEEE│
│cccccccc││EEEEEEEE│
│cccccccc││EEEEEEEE│
└────────┘└────────┘`},
	}

	testutil.TestHandlerSequence(t, browser, 20, 10, cases)

	require.NoError(t, win.Close())
	require.NoError(t, focus.Close())

	cases = []testutil.HandlerSequenceTestCase{
		{":close>)))",
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{":bcloseAll>:e other.go>bcde((((",
			`┌──────────────────┐
│other.go          │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
	}
	testutil.TestHandlerSequence(t, browser, 20, 10, cases)

	cases = []testutil.HandlerSequenceTestCase{
		{"", `┌──┐
│..│
├EE┤
EEEE`},
	}
	testutil.TestHandlerSequence(t, browser, 4, 4, cases)

	require.NoError(t, browser.SetMessage("wasup: %s", "Z"))
	cases = []testutil.HandlerSequenceTestCase{
		{"",
			`┌──────────────────┐
│other.go          │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│wasup: Z          │
└──────────────────┘`},
	}
	testutil.TestHandlerSequence(t, browser, 20, 10, cases)

	nh, err := browser.Open("bugz")
	require.NoError(t, err)
	focus, err = browser.Focus()
	require.NoError(t, err)
	err = focus.SetContent(nh)
	require.NoError(t, err)

	cases = []testutil.HandlerSequenceTestCase{
		{"b___",
			`┌──────────────────┐
│other.go  bugz    │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
	}
	testutil.TestHandlerSequence(t, browser, 20, 10, cases)

	assert.NoError(t, browser.Close())
	assert.Equal(t, 1, unmounted)
}

func assertHandled(
	t *testing.T, h *handler.TestHandler, startingRune rune, exit, handled bool,
) {
	// test handler increments the character that it displays next
	// upon handling a new event
	require.False(t, exit)
	require.True(t, handled)
	assert.NotEqual(t, startingRune, h.Ch)
}

func newBrowserForSubscribeTest(t *testing.T, ev term.Event) (
	*Ex, *handler.TestHandler, rune,
) {
	b := newTestBrowserHandler()
	require.NoError(t, b.Init(&testEditor{}))

	h := handler.NewTestHandler()
	err := b.Subscribe(ev, browser.HandlerEventHandler(h))
	require.NoError(t, err)

	return b, h, h.Ch
}

func TestBrowserHandlerSubscribe(t *testing.T) {
	t.Run("proxies event to subscribed EventHandler", func(t *testing.T) {
		ev := term.Event{Type: term.EventKey, Ch: '*'}
		b, h, startingRune := newBrowserForSubscribeTest(t, ev)

		exit, handled := b.Handle(ev)
		assertHandled(t, h, startingRune, exit, handled)
	})

	t.Run("returns error on second event Subscribe", func(t *testing.T) {
		ev := term.Event{Type: term.EventError}
		b, h, startingRune := newBrowserForSubscribeTest(t, ev)

		h2 := handler.NewTestHandler()
		err := b.Subscribe(ev, browser.HandlerEventHandler(h2))
		assert.Error(t, err)

		exit, handled := b.Handle(ev)
		assertHandled(t, h, startingRune, exit, handled)
	})

	t.Run("upon handler exit, it unsubscribes EventHandler", func(t *testing.T) {
		ev := term.Event{Type: term.EventKey, Ch: '*'}
		b, h, startingRune := newBrowserForSubscribeTest(t, ev)
		h.Exit = true

		exit, handled := b.Handle(ev)
		assertHandled(t, h, startingRune, exit, handled)

		nextRune := h.Ch
		exit, _ = b.Handle(ev)
		assert.False(t, exit)
		assert.Equal(t, nextRune, h.Ch)
	})
}

func TestBrowserHandlerPublishInterrupt(t *testing.T) {
	t.Run("calls interrupt handle asynchronously", func(t *testing.T) {
		var wg sync.WaitGroup
		browser := newTestBrowserHandler()
		require.NoError(t, browser.Init(&testEditor{}))
		browser.interruptDraw = wg.Done

		wg.Add(1)
		browser.PublishInterrupt()

		wg.Wait()
	})
}

func TestMultipleFilesStartup(t *testing.T) {
	cases := []testutil.HandlerSequenceTestCase{
		{"",
			`┌──────────────────┐
│cabin.go  wi.go   │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"a",
			`┌──────────────────┐
│cabin.go  wi.go   │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{"#", // simulates ctrl-h
			`┌──────────────────┐
│cabin.go  wi.go   │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
	}

	b := newTestBrowserHandler()
	err := b.Init(&testEditor{},
		browser.WithFilepath("cabin.go"),
		browser.WithFilepath("wi.go"),
	)
	require.NoError(t, err)

	testutil.TestHandlerSequence(t, b, 20, 10, cases)
}

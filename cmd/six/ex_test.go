package main

import (
	"io"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
	testutil "github.com/ernestrc/go-tui/util/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testutil.TestHandlerSequence maps ':' characters to the following event
// this is to work around ex's assumptions on underlying handler.
var testCommandEvent = term.Event{Type: term.EventKey, Key: term.KeyCtrlBackslash}

type browserConstructor func(ed editor.Editor, opts ...editor.Option) (tui.Handler, browser.Browser, error)

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

func openTestFile(filePath string, buf *cell.Buffer, swapDir string, readOnly bool) (
	editor.FlusherCloser, error,
) {
	return &testFileBuffer{}, nil
}

func recoverTestFile(filePath, swapFilePath string, buf *cell.Buffer) (
	editor.FlusherCloser, error,
) {
	return openTestFile(filePath, buf, "", false)
}

func withOpenFileStubs(opts ...editor.Option) []editor.Option {
	opts = append(opts, editor.WithOpenFileFn(openTestFile))
	opts = append(opts, editor.WithRecoverFileFn(recoverTestFile))
	return opts
}

func TestBrowserHandlerDraw(t *testing.T) {
	testBrowserHandlerDraw(t, func(ed editor.Editor, opts ...editor.Option) (tui.Handler, browser.Browser, error) {
		b := new(Ex)
		err := b.Init(ed, withOpenFileStubs(opts...)...)
		if err != nil {
			return nil, nil, err
		}
		return b, b.Browser(), nil
	})
}

func testBrowserHandlerDraw(t *testing.T, constructor browserConstructor) {
	cases := []testutil.HandlerSequenceTestCase{
		{"a",
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
		{":e cabin.go>",
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
		{":e /tmp/other.go>",
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
		{":wq!^^^^^",
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
	bh, b, err := constructor(editor.Mock(),
		editor.WithCommandEvent(testCommandEvent),
		editor.WithCommandKeyBinding(term.Event{Type: term.EventKey, Ch: '4'}, "close"),
	)
	require.NoError(t, err)

	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	win, err := b.Split(browser.OrientationLeft, browser.NewTestHandler())
	require.NoError(t, err)

	// test window ifc
	focus, err := b.Focus()
	require.NoError(t, err)

	h := browser.NewTestHandler()
	h.Ch = 'Z' // helps identify in tests

	_, err = b.Split(browser.OrientationBottom, h)
	require.NoError(t, err)

	cases = []testutil.HandlerSequenceTestCase{
		{"_",
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

	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	var unmounted int
	hx := browser.NewTestHandler()
	hx.Ch = '$'
	hx.OnUnmountCallback = func() error { unmounted++; return nil }
	require.NoError(t, focus.SetContent(hx))

	for i := 0; i < 10; i++ {
		content, err := focus.Content()
		require.NoError(t, err)
		require.NoError(t, focus.SetContent(content))
	}

	cases = []testutil.HandlerSequenceTestCase{
		{"_",
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

	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	require.NoError(t, win.Close())
	require.NoError(t, focus.Close())

	cases = []testutil.HandlerSequenceTestCase{
		// test CommandKeyBindings
		{"4$$$",
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
		{":bcloseAll>:e other.go>bcde####__",
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
	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	cases = []testutil.HandlerSequenceTestCase{
		{"", `┌──┐
│..│
├EE┤
EEEE`},
	}
	testutil.TestHandlerSequence(t, bh, 4, 4, cases)

	require.NoError(t, b.SetMessage("wasup: %s", "Z"))
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
	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	nh, err := b.Open("bugz")
	require.NoError(t, err)
	focus, err = b.Focus()
	require.NoError(t, err)
	err = focus.SetContent(nh)
	require.NoError(t, err)

	cases = []testutil.HandlerSequenceTestCase{
		{"b__",
			`┌──────────────────┐
│other.go  bugz    │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│wasup: Z          │
└──────────────────┘`},
		{":3>",
			`┌──────────────────┐
│other.go  bugz    │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│▐BBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│wasup: Z          │
└──────────────────┘`},
		{":0>",
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
	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	floating1, err := b.Floating(browser.NewTestHandler(), term.Coordinates{X: 1, Y: 1}, 6, 4)
	require.NoError(t, err)

	// should not be able to split over a floating window, which is currently in focus
	_, err = b.Split(browser.OrientationTop, browser.NewTestHandler())
	require.Error(t, err)
	cases = []testutil.HandlerSequenceTestCase{
		{"__",
			`┌──────────────────┐
│other.go  bugz    │
├──────────────────┤
│┌────┐BBBBBBBBBBBB│
││AAAA│BBBBBBBBBBBB│
││AAAA│BBBBBBBBBBBB│
│└────┘BBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
	}

	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	require.NoError(t, floating1.Close())

	cases = []testutil.HandlerSequenceTestCase{
		{"__",
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
	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	var o browser.Orientation
	for i := 0; i < 4; i++ {
		b1 := browser.NewTestHandler()
		b1.Ch = rune(strconv.Itoa(i)[0])
		err = b.Bar(o, b1)
		require.NoError(t, err)
		o++
	}

	// test case for issue #27
	cases = []testutil.HandlerSequenceTestCase{
		{":e ait^^^aix^^^^d airsoft.map____>",
			`┌────────────────────────────────────────────────┐
│other.go  bugz  airsoft.map                     │
├────────────────────────────────────────────────┤
│000000000000000000000000000000000000000000000000│
├─┬────────────────────────────────────────────┬─┤
│2│AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA│3│
│2│AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA│3│
├─┴────────────────────────────────────────────┴─┤
│111111111111111111111111111111111111111111111111│
└────────────────────────────────────────────────┘`},
	}
	testutil.TestHandlerSequence(t, bh, 50, 10, cases)

	cases = []testutil.HandlerSequenceTestCase{
		{"____",
			`┌────────────────────────────────────────────────┐
│other.go  bugz  airsoft.map                     │
├────────────────────────────────────────────────┤
│000000000000000000000000000000000000000000000000│
├─┬────────────────────────────────────────────┬─┤
│2│AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA│3│
│2│AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA│3│
├─┴────────────────────────────────────────────┴─┤
│111111111111111111111111111111111111111111111111│
└────────────────────────────────────────────────┘`},
	}
	testutil.TestHandlerSequence(t, bh, 50, 10, cases)

	assert.NoError(t, bh.(io.Closer).Close())
	assert.NoError(t, b.Close())
	assert.Equal(t, 11, unmounted)
}

func assertHandled(
	t *testing.T, h *browser.TestHandler, startingRune rune, exit, handled bool,
) {
	// test handler increments the character that it displays next
	// upon handling a new event
	require.False(t, exit)
	require.True(t, handled)
	assert.NotEqual(t, startingRune, h.Ch)
}

func TestBrowserHandlerPublishInterrupt(t *testing.T) {
	t.Run("calls interrupt handle asynchronously", func(t *testing.T) {
		var wg sync.WaitGroup
		browser := new(Ex)
		opts := withOpenFileStubs()
		opts = append(opts, editor.WithInterrupt(wg.Done))
		require.NoError(t, browser.Init(editor.Mock(), opts...))
		defer browser.Close()

		wg.Add(1)
		browser.Browser().PublishInterrupt()

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

	b := new(Ex)
	opts := withOpenFileStubs(
		editor.WithFilepath("cabin.go"),
		editor.WithFilepath("wi.go"),
	)
	err := b.Init(editor.Mock(),
		opts...,
	)
	require.NoError(t, err)
	defer b.Close()

	testutil.TestHandlerSequence(t, b, 20, 10, cases)
}

func TestExCommandResponsive(t *testing.T) {
	cases := []testutil.HandlerSequenceTestCase{
		{":edit",
			`                    
                    
     ┌────────┐     
     │edit▐   │     
     │edit    │     
     │        │     
     └────────┘     
                    
                    
                    `},
		{":eeeeeeeeeeeeeeeeeeeeeeeeee",
			`                    
                    
     ┌────────┐     
     │eeeeeeee│     
     │eeeeeeee│     
     │eeeeeee▐│     
     └────────┘     
                    
                    
                    `},
		{":e eeeeeeeeeeeeeeeeeeeeeeeee",
			`                    
                    
     ┌────────┐     
     │e eeeeee│     
     │eeeeeeee│     
     │eeeeeee▐│     
     └────────┘     
                    
                    
                    `},
	}

	var closeFns []func() error
	fn := func(t *testing.T) tui.Handler {
		b := new(Ex)
		opts := withOpenFileStubs(
			editor.WithCommandEvent(testCommandEvent),
			editor.WithWindowManagerConfig(component.WindowManagerConfig{
				Frame: false,
			}),
			editor.WithCommandOverlayConfig(editor.CommandOverlayConfig{
				Width:  10,
				Height: 5,
				Frame:  true,
			}),
		)
		err := b.Init(editor.Mock(), opts...)
		require.NoError(t, err)
		closeFns = append(closeFns, b.Close)
		return b
	}
	testutil.TestHandlerIsolated(t, fn, 20, 10, cases)
	for _, close := range closeFns {
		close()
	}
}

func TestExKeySequence(t *testing.T) {
	cases := []testutil.HandlerSequenceTestCase{
		{"gl",
			`┌──────────────────┐
│10k.go  button.go │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{"gg",
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
	}

	var closeFns []func() error
	fn := func(t *testing.T) tui.Handler {
		b := new(Ex)
		opts := withOpenFileStubs(
			editor.WithFilepath("10k.go"),
			editor.WithFilepath("button.go"),
			editor.WithCommandEvent(testCommandEvent),
			editor.WithCommandSequenceBinding(handler.Sequence{
				First: term.Event{Type: term.EventKey, Ch: 'g'},
				Last:  term.Event{Type: term.EventKey, Ch: 'l'},
			}, "bufferNext"),
			editor.WithCommandSequenceBinding(handler.Sequence{
				First: term.Event{Type: term.EventKey, Ch: 'g'},
				Last:  term.Event{Type: term.EventKey, Ch: 'g'},
			}, "bufferCloseAll"),
			editor.WithSequencerTimeout(10*time.Second),
		)
		err := b.Init(editor.Mock(), opts...)
		require.NoError(t, err)
		closeFns = append(closeFns, b.Close)
		return b
	}
	testutil.TestHandlerIsolated(t, fn, 20, 10, cases)
	for _, close := range closeFns {
		close()
	}
}

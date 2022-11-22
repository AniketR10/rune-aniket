package main

import (
	"fmt"
	"io"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	testutil "unstable.build/go-tui/util/test"
	"unstable.build/go-tui/workspace"
)

// testutil.TestHandlerSequence maps ':' characters to the following event
// this is to work around ex's assumptions on underlying handler.
var testCommandKey = term.KeyComb{Key: term.KeyCtrlBackslash}

type browserConstructor func(ed text.Editor, opts ...text.Option) (tui.Handler, browser.Browser, error)

type testFileBuffer struct {
	flushErr error
	closeErr error
	closed   bool
}

func (t *testFileBuffer) Flush() error {
	return t.flushErr
}

func (t *testFileBuffer) Close() error {
	t.closed = true
	return t.closeErr
}

type testLoader struct {
	buf *testFileBuffer
}

func (w *testLoader) Load(filePath workspace.URI, buf *cell.Buffer, swapDir workspace.URI, readOnly bool) (
	workspace.FlusherCloser, error,
) {
	if w.buf != nil {
		return w.buf, nil
	}
	return &testFileBuffer{}, nil
}

func (w *testLoader) Recover(
	filePath, swapFilePath workspace.URI,
	buf *cell.Buffer, force bool,
) (
	workspace.FlusherCloser, error,
) {
	return w.Load(filePath, buf, swapFilePath, false)
}

func (w *testLoader) URI(path string) (workspace.URI, error) {
	return workspace.CurrentUserHostURI(path)
}

func TestBrowserHandlerDraw(t *testing.T) {
	testBrowserHandlerDraw(t, func(ed text.Editor, opts ...text.Option) (tui.Handler, browser.Browser, error) {
		b := newExForTesting(t, ed, opts...)
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
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`},
		{"$", // simulates ctrl-l
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`},
		{":bclose>",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘`},
		{":wq!^^^^^",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
		{":<",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
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
│GGGGGGGGGGGGGGGGGG│
│GGGGGGGGGGGGGGGGGG│
│GGGGGGGGGGGGGGGGGG│
│GGGGGGGGGGGGGGGGGG│
│GGGGGGGGGGGGGGGGGG│
│GGGGGGGGGGGGGGGGGG│
└──────────────────┘`},
	}
	bh, b, err := constructor(text.NopEditor(),
		text.WithCommandKey(testCommandKey),
		text.WithCommandKeyBinding(term.KeyComb{Ch: '4'}, []string{"close"}),
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
		{"",
			`┌──────────────────┐
│cabin.go  other.go│
├────────┐┌────────┤
│AAAAAAAA││GGGGGGGG│
│AAAAAAAA││GGGGGGGG│
└────────┘│GGGGGGGG│
┌────────┐│GGGGGGGG│
│ZZZZZZZZ││GGGGGGGG│
│ZZZZZZZZ││GGGGGGGG│
└────────┘└────────┘`},
		{":<111111111",
			`┌──────────────────┐
│cabin.go  other.go│
├────────┐┌────────┤
│AAAAAAAA││GGGGGGGG│
│AAAAAAAA││GGGGGGGG│
└────────┘│GGGGGGGG│
┌────────┐│GGGGGGGG│
│cccccccc││GGGGGGGG│
│cccccccc││GGGGGGGG│
└────────┘└────────┘`},
	}

	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	var closed int
	hx := browser.NewTestHandler()
	hx.Ch = '$'
	hx.CloseCallback = func() error { closed++; return nil }
	require.NoError(t, focus.SetContent(hx))

	cases = []testutil.HandlerSequenceTestCase{
		{"",
			`┌──────────────────┐
│cabin.go  other.go│
├────────┐┌────────┤
│$$$$$$$$││GGGGGGGG│
│$$$$$$$$││GGGGGGGG│
└────────┘│GGGGGGGG│
┌────────┐│GGGGGGGG│
│cccccccc││GGGGGGGG│
│cccccccc││GGGGGGGG│
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
│HHHHHHHHHHHHHHHHHH│
│HHHHHHHHHHHHHHHHHH│
│HHHHHHHHHHHHHHHHHH│
│HHHHHHHHHHHHHHHHHH│
│HHHHHHHHHHHHHHHHHH│
│HHHHHHHHHHHHHHHHHH│
└──────────────────┘`},
		{":bcloseAll>:e other.go>bcde####",
			`┌──────────────────┐
│other.go          │
├──────────────────┤
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
└──────────────────┘`},
	}
	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	cases = []testutil.HandlerSequenceTestCase{
		{"", `┌──┐
│..│
├II┤
IIII`},
	}
	testutil.TestHandlerSequence(t, bh, 4, 4, cases)

	require.NoError(t, b.SetMessage("wasup: %s", "Z"))
	cases = []testutil.HandlerSequenceTestCase{
		{"",
			`┌──────────────────┐
│other.go          │
├──────────────────┤
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│wasup: Z          │
└──────────────────┘`},
	}
	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	uri, err := workspace.ParseURI("file:///bugz")
	require.NoError(t, err)
	nh, err := b.Open(uri)
	require.NoError(t, err)
	focus, err = b.Focus()
	require.NoError(t, err)
	err = focus.SetContent(nh)
	require.NoError(t, err)

	cases = []testutil.HandlerSequenceTestCase{
		{"b",
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
│wasup: Z          │
└──────────────────┘`},
	}
	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	floating1, err := b.Floating(browser.NewTestHandler(), term.Coordinates{X: 1, Y: 1}, 6, 4)
	require.NoError(t, err)

	// should not be able to split over a floating window, which is currently in focus
	_, err = b.Split(browser.OrientationTop, browser.NewTestHandler())
	require.Error(t, err)
	cases = []testutil.HandlerSequenceTestCase{
		{"",
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
		{":reload>", // test reload non file
			`┌──────────────────┐
│other.go  bugz    │
├──────────────────┤
│┌────┐BBBBBBBBBBBB│
││AAAA│BBBBBBBBBBBB│
││AAAA│BBBBBBBBBBBB│
│└────┘BBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│not a file        │
└──────────────────┘`},
	}

	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	require.NoError(t, floating1.Close())

	cases = []testutil.HandlerSequenceTestCase{
		{"",
			`┌──────────────────┐
│other.go  bugz    │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│not a file        │
└──────────────────┘`},
	}
	testutil.TestHandlerSequence(t, bh, 20, 10, cases)

	o := browser.OrientationTop
	for i := 0; i < 4; i++ {
		b1 := browser.NewTestHandler()
		b1.Ch = rune(strconv.Itoa(i)[0])
		err = b.Bar(o, b1)
		require.NoError(t, err)
		o++
	}

	// test case for issue #27
	cases = []testutil.HandlerSequenceTestCase{
		{":e ait^^^aix^^^^ airsoft.map>",
			`┌────────────────────────────────────────────────┐
│other.go  bugz  airsoft.map                     │
├────────────────────────────────────────────────┤
│000000000000000000000000000000000000000000000000│
├─┬────────────────────────────────────────────┬─┤
│2│AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA│3│
│2│AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA│3│
├─┴────────────────────────────────────────────┴─┤
│not a file                                      │
└────────────────────────────────────────────────┘`},
	}
	testutil.TestHandlerSequence(t, bh, 50, 10, cases)

	assert.NoError(t, bh.(io.Closer).Close())
	assert.NoError(t, b.Close())
	assert.Equal(t, 1, closed)
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

func TestBrowserHandlerInterrupts(t *testing.T) {
	t.Run("Interrupt calls interrupt handle", func(t *testing.T) {
		var wg sync.WaitGroup
		opts := []text.Option{text.WithInterrupt(wg.Done)}
		browser := newExForTesting(t, text.NopEditor(), opts...)
		defer browser.Close()

		wg.Add(1)
		browser.Browser().PublishInterrupt()

		wg.Wait()
	})
	t.Run("SendEventNone calls interrupt handle", func(t *testing.T) {
		var wg sync.WaitGroup
		opts := []text.Option{text.WithSendNone(wg.Done)}
		browser := newExForTesting(t, text.NopEditor(), opts...)
		defer browser.Close()

		wg.Add(1)
		browser.Browser().PublishEventNone()

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
		{"aa",
			`┌──────────────────┐
│cabin.go  wi.go   │
├──────────────────┤
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
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
		{"#:reload>",
			`┌──────────────────┐
│wi.go  cabin.go   │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
	}

	file1, err := workspace.ParseURI("file:///cabin.go")
	require.NoError(t, err)
	file2, err := workspace.ParseURI("file:///wi.go")
	require.NoError(t, err)
	opts := []text.Option{
		text.WithFile(file1),
		text.WithFile(file2),
		text.WithCommandKey(testCommandKey),
	}
	mockBuf := testFileBuffer{}
	workspace := testLoader{buf: &mockBuf}
	b := newExForTestingWithWorkspace(t, &workspace, text.NopEditor(), nopPublishEvent, opts...)
	defer b.Close()

	testutil.TestHandlerSequence(t, b, 20, 10, cases)

	assert.True(t, mockBuf.closed)
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
     │edit eee│     
     │eeeeeeee│     
     │eeeeeee▐│     
     └────────┘     
                    
                    
                    `},
	}

	var closeFns []func() error
	fn := func(t *testing.T) tui.Handler {
		opts := []text.Option{
			text.WithCommandKey(testCommandKey),
			text.WithWindowManagerConfig(handler.WindowManagerConfig{
				WindowManagerConfig: component.WindowManagerConfig{Frame: false}}),
			text.WithCommandOverlayConfig(text.CommandOverlayConfig{
				FrameCharSet: component.FrameCharSetDefault(),
				Width:        10,
				Height:       5,
				Frame:        true,
			}),
		}
		b := newExForTesting(t, text.NopEditor(), opts...)
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
		{"zgl",
			`┌──────────────────┐
│10k.go  button.go │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"g",
			`┌──────────────────┐
│10k.go  button.go │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"go",
			`┌──────────────────┐
│10k.go  button.go │
├──────────────────┤
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`},
		// 2 seconds of wait should be plenty for sequencer to deem 'g' sequence
		// stale and re-issue event.
		{"g____________________",
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
		var mu sync.Mutex
		file1, err := workspace.ParseURI("file:///10k.go")
		require.NoError(t, err)
		file2, err := workspace.ParseURI("file:///button.go")
		require.NoError(t, err)
		opts := []text.Option{
			text.WithFile(file1),
			text.WithFile(file2),
			text.WithCommandKey(testCommandKey),
			text.WithCommandSequenceBinding(handler.Sequence{
				First: term.KeyComb{Ch: 'g'},
				Last:  term.KeyComb{Ch: 'l'},
			}, []string{"bufferNext"}),
			text.WithCommandSequenceBinding(handler.Sequence{
				First: term.KeyComb{Ch: 'g'},
				Last:  term.KeyComb{Ch: 'g'},
			}, []string{"bufferCloseAll"}),
			text.WithSequencerTimeout(1 * time.Second),
		}
		ex := new(ex)
		require.NoError(t, ex.init(text.NopEditor(), &testLoader{}, document.NewInMemoryService(),
			func(ev term.Event) bool {
				// do not confuse interrupt from list with sequence re-issue commands
				if ev.Type == term.EventInterrupt {
					return true
				}
				mu.Lock()
				defer mu.Unlock()
				ex.Handle(ev)
				return true
			}, opts...))
		ex.subscribeCommands()
		b := testEx{ex}
		closeFns = append(closeFns, func() error {
			mu.Lock()
			defer mu.Unlock()
			return b.Close()
		})
		return handler.Sync(&mu, b)
	}
	testutil.TestHandlerIsolated(t, fn, 20, 10, cases)
	for _, close := range closeFns {
		close()
	}
}

func TestExTabIntegration(t *testing.T) {
	cases := []testutil.HandlerSequenceTestCase{
		{"",
			`┌──────────────────┐
│Fieshta  Pahty    │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{":bufferCloseAll>",
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
		opts := []text.Option{
			text.WithCommandKey(testCommandKey),
		}
		b := newExForTesting(t, text.NopEditor(), opts...)
		uri1, err := workspace.ParseURI("file:///Fieshta")
		require.NoError(t, err)
		uri2, err := workspace.ParseURI("file:///Pahty")
		require.NoError(t, err)
		tab, err := b.comp.Tab(uri1, "Fieshta", browser.NewTestHandler())
		require.NoError(t, err)
		_, err = b.comp.Tab(uri2, "Pahty", browser.NewTestHandler())
		require.NoError(t, err)
		focus, err := b.comp.Focus()
		require.NoError(t, err)
		require.NoError(t, focus.SetContent(tab))
		closeFns = append(closeFns, b.Close)
		return b
	}
	testutil.TestHandlerIsolated(t, fn, 20, 10, cases)
	for _, close := range closeFns {
		close()
	}
}

func TestExExit(t *testing.T) {
	commands := []string{
		"writeQuit",
		"writeForceQuit!",
		"forceQuit!",
		"quit",
	}

	for _, cmd := range commands {
		t.Run(fmt.Sprintf("ex exits %s command is issued", cmd), func(t *testing.T) {
			b := newExForTesting(t, text.NopEditor(),
				text.WithCommandKey(testCommandKey),
			)
			defer b.Close()

			// start command prompt
			ev := term.Event{
				Type: term.EventKey,
				Ch:   testCommandKey.Ch,
				Mod:  testCommandKey.Mod,
				Key:  testCommandKey.Key,
			}
			exit, handled := b.Handle(ev)
			assert.True(t, handled)
			require.False(t, exit)

			for _, ch := range cmd {
				exit, handled := b.Handle(term.Event{Ch: ch, Type: term.EventKey})
				assert.True(t, handled)
				require.False(t, exit)
			}

			exit, handled = b.Handle(
				term.Event{Key: term.KeyEnter, Type: term.EventKey})
			assert.True(t, handled)
			require.True(t, exit)

		})
	}

	t.Run("ex does not exit when inner handler returns exit=true", func(t *testing.T) {
		b := newExForTesting(t, text.NopEditor())
		defer b.Close()

		h := browser.NewTestHandler()
		h.Exit = true
		h.Handled = true

		uri, err := workspace.ParseURI("file:///bols")
		require.NoError(t, err)

		tab, err := b.comp.Tab(uri, "bleh", h)
		require.NoError(t, err)

		w, err := b.comp.Focus()
		require.NoError(t, err)

		err = w.SetContent(tab)
		require.NoError(t, err)

		exit, handled := b.Handle(term.Event{Ch: 'a', Type: term.EventKey})
		assert.True(t, handled)
		assert.False(t, exit)
	})
}

// remove non-determinism of search.List async search
type testEx struct {
	*ex
}

func (t testEx) Handle(ev term.Event) (bool, bool) {
	quit, handle := t.ex.Handle(ev)
	t.ex.cmd.list.Wait()
	return quit, handle
}

func newExForTestingWithWorkspace(
	t *testing.T, workspace *testLoader,
	ed text.Editor,
	publishEvent func(term.Event) bool,
	opts ...text.Option,
) testEx {
	ex := new(ex)
	require.NoError(t, ex.init(ed, workspace, document.NewInMemoryService(),
		publishEvent, opts...))
	ex.subscribeCommands()
	return testEx{ex}
}

func newExForTesting(t *testing.T, ed text.Editor, opts ...text.Option) testEx {
	return newExForTestingWithWorkspace(t, &testLoader{}, ed, nopPublishEvent, opts...)
}

func TestNewWindow(t *testing.T) {
	cases := []testutil.HandlerSequenceTestCase{
		{":newWindow>:changeSplitOrientation h>:newWindow>",
			`┌──────────────────┐
│                  │
├────────┐┌────────┤
│        ││        │
│        ││        │
│        │└────────┘
│changed split dire┐
│ction to horizonta│
│l                 │
└────────┘└────────┘`},
		{":close>:close>aaaaaaa",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│changed split dire│
│ction to horizonta│
│l                 │
└──────────────────┘`},
	}

	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
	}
	b := newExForTesting(t, text.NopEditor(), opts...)
	defer b.Close()

	testutil.TestHandlerSequence(t, b, 20, 10, cases)
}

func TestCommandHistory(t *testing.T) {
	cases := []testutil.HandlerSequenceTestCase{
		{":e hello.go>:e wi.go>1234",
			`┌──────────────────┐
│hello.go  wi.go   │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
		{":::>",
			`┌──────────────────┐
│hello.go  wi.go   │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
	}

	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
	}
	b := newExForTesting(t, text.NopEditor(), opts...)
	defer b.Close()

	testutil.TestHandlerSequence(t, b, 20, 10, cases)
}

func TestCommandAliases(t *testing.T) {
	cases := []testutil.HandlerSequenceTestCase{
		{":todo>1234",
			`┌──────────────────┐
│hello.go  wi.go   │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
		{":bp>",
			`┌──────────────────┐
│hello.go  wi.go   │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
	}

	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
		text.WithCommandAliases(map[string][]string{
			"todo": []string{"edit hello.go", "edit wi.go"},
			"bp":   []string{"bufferNext"},
		}),
	}
	b := newExForTesting(t, text.NopEditor(), opts...)
	defer b.Close()

	testutil.TestHandlerSequence(t, b, 20, 10, cases)
}

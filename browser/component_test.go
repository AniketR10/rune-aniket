package browser

import (
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
	testutil "github.com/ernestrc/go-tui/util/test"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var splitSuite = []struct {
	method string
	split  func(c *Component, h Handler) Window
}{
	{"SplitVerticalLeft:", (*Component).SplitVerticalLeft},
	{"SplitVerticalRight:", (*Component).SplitVerticalRight},
	{"SplitHorizontalAbove:", (*Component).SplitHorizontalAbove},
	{"SplitHorizontalBelow:", (*Component).SplitHorizontalBelow},
}

func TestComponentCloseWindow(t *testing.T) {

	t.Run("closing the last window returns error", func(t *testing.T) {
		c := NewComponent(Config{})
		assert.Error(t, c.Focus().Close())
	})

	for _, _tcase := range splitSuite {
		tcase := _tcase
		t.Run(tcase.method+"closes window correctly", func(t *testing.T) {
			c := NewComponent(Config{})

			var h Handler
			h = NewTestHandler()
			h = c.NewBuffer("OAK", h, nil)

			win := tcase.split(c, h)
			require.Equal(t, 2, c.wm.Size())

			assert.NoError(t, win.Close())
			require.Equal(t, 1, c.wm.Size())

			assert.Len(t, c.freeBuffers(), 1)
		})

		t.Run(tcase.method+"Close is idempotent", func(t *testing.T) {
			c := NewComponent(Config{})
			win := tcase.split(c, NewTestHandler())
			require.NoError(t, win.Close())
			assert.NoError(t, win.Close())
		})

		t.Run(tcase.method+"close window on handler exit", func(t *testing.T) {
			c := NewComponent(Config{})
			var h Handler
			h = NewTestHandler()
			h.(*TestHandler).Exit = true
			h.(*TestHandler).Handled = true
			win := tcase.split(c, h)

			assert.Equal(t, 0, c.tabs.Size())
			assert.Equal(t, 2, c.wm.Size())

			exit, handled := c.Handle(term.Event{})
			require.True(t, handled)
			require.False(t, exit)

			assert.Equal(t, 0, c.tabs.Size())
			assert.Equal(t, 1, c.wm.Size())
			require.NoError(t, win.Close())
			assert.Equal(t, 1, c.wm.Size())
		})

		t.Run(tcase.method+"Close calls onWindowClosed callback", func(t *testing.T) {
			var i int
			c := NewComponent(Config{})
			win := tcase.split(c, NewTestHandler())
			win.onWindowClosed(func() {
				i++
			})

			require.NoError(t, win.Close())
			assert.Equal(t, 1, i)
		})
	}
}

func TestComponentRemoveAllBuffers(t *testing.T) {
	w1 := NewComponent(Config{})
	w2 := NewComponent(Config{})
	w2.SplitHorizontalAbove(NewTestHandler())

	tsuite := []struct {
		description string
		c           *Component
		before      int
		after       int
	}{
		{"removes all buffers with one window", w1, 3, 0},
		{"removes all buffers with multiple windows", w2, 3, 0},
	}

	for _, tcase := range tsuite {
		c := tcase.c
		before := tcase.before
		after := tcase.after
		t.Run(tcase.description, func(t *testing.T) {
			handlers := [3]testFlushCloser{}
			c.NewBuffer("a", &handlers[0], &handlers[0])
			c.NewBuffer("b", &handlers[1], &handlers[1])
			c.NewBuffer("c", &handlers[2], &handlers[2])
			c.UpdateWindowBufferNextFree(c.Focus())
			c.ShiftFocus()
			c.UpdateWindowBufferNextFree(c.Focus())

			assert.Equal(t, before, c.tabs.Size())
			c.RemoveAllBuffers()

			assert.Equal(t, after, c.tabs.Size())
			for _, h := range handlers {
				assert.Equal(t, 1, h.closed)
				// buffer does not get called unmount
				// because that's just for internal use
				assert.Equal(t, 0, h.unmounted)
			}
		})
	}
}

type testFlushCloser struct {
	handler.TestHandler
	flushed, closed, unmounted int
}

func (t *testFlushCloser) Flush() error {
	t.flushed++
	return nil
}

func (t *testFlushCloser) Close() error {
	t.closed++
	return nil
}
func (t *testFlushCloser) OnUnmount() error {
	t.unmounted++
	return nil
}

func TestComponentFlushBuffer(t *testing.T) {
	c := NewComponent(Config{})
	mock := testFlushCloser{}
	c.NewBuffer("a", &mock, &mock)
	c.UpdateWindowBufferNextFree(c.Focus())

	c.FlushBuffer(c.Focus())
	assert.Equal(t, 1, mock.flushed)
	assert.Equal(t, 0, mock.closed)
	assert.Equal(t, 0, mock.unmounted)
}

func assertFreeBuffer(t *testing.T, h tui.Handler, free bool) {
	assert.Equal(t, free, h.(*buffer).free)
}

func assertWindowContent(t *testing.T, win Window, expected Handler) {
	content, err := win.Content()
	require.NoError(t, err)
	assert.Equal(t, expected, content)
}

func TestComponentSetContent(t *testing.T) {
	for _, _tcase := range splitSuite {
		tcase := _tcase
		t.Run(tcase.method, func(t *testing.T) {
			c := NewComponent(Config{})
			win0 := c.Focus()
			christmasBuffer := c.NewBuffer("Merry Christmas", NewTestHandler(), nil)

			assertFreeBuffer(t, christmasBuffer, true)
			require.NoError(t, win0.SetContent(christmasBuffer))
			assertFreeBuffer(t, christmasBuffer, false)
			assertWindowContent(t, win0, christmasBuffer)
			assert.False(t, c.UpdateWindowBufferNext(win0))

			h2 := NopHandler(&handler.TestHandler{})
			win1 := tcase.split(c, h2)
			assertWindowContent(t, win1, h2)
			require.Error(t, win1.SetContent(christmasBuffer))
			assertWindowContent(t, win1, h2)
			assertFreeBuffer(t, christmasBuffer, false)

			h3 := NopHandler(&handler.TestHandler{})
			require.NoError(t, win1.SetContent(h3))
			assertWindowContent(t, win1, h3)
			assertFreeBuffer(t, christmasBuffer, false)

			h4 := NopHandler(&handler.TestHandler{})
			require.NoError(t, win0.SetContent(h4))
			assertWindowContent(t, win0, h4)
			assertFreeBuffer(t, christmasBuffer, true)
		})
	}
}

func TestComponentUpdateWindowBuffer(t *testing.T) {
	for _, _tcase := range splitSuite {
		tcase := _tcase
		t.Run(tcase.method, func(t *testing.T) {
			c := NewComponent(Config{})
			win0 := c.Focus()
			amzn := c.NewBuffer("AMZN", NewTestHandler(), nil)
			c.UpdateWindowBufferNextFree(win0)
			tsla := c.NewBuffer("TSLA", NewTestHandler(), nil)
			goog := c.NewBuffer("GOOG", NewTestHandler(), nil)
			win := tcase.split(c, goog)

			assertFreeBuffer(t, amzn, false)
			assertFreeBuffer(t, tsla, true)
			assertFreeBuffer(t, goog, false)

			for i := 0; i < 3; i++ {
				assert.True(t, c.UpdateWindowBufferNext(win))
			}

			assertFreeBuffer(t, amzn, false)
			assertFreeBuffer(t, tsla, false)
			assertFreeBuffer(t, goog, true)

			for i := 0; i < 3; i++ {
				assert.True(t, c.UpdateWindowBufferPrev(win0))
			}

			assertFreeBuffer(t, amzn, true)
			assertFreeBuffer(t, tsla, false)
			assertFreeBuffer(t, goog, false)

			assert.True(t, c.UpdateWindowBufferNextFree(win0))

			assertFreeBuffer(t, amzn, false)
			assertFreeBuffer(t, tsla, false)
			assertFreeBuffer(t, goog, true)

			assert.True(t, c.UpdateWindowBufferLastFree(win0))

			assertFreeBuffer(t, amzn, true)
			assertFreeBuffer(t, tsla, false)
			assertFreeBuffer(t, goog, false)

			assert.Error(t, win0.SetContent(tsla))
			assert.NoError(t, win0.SetContent(amzn))

			assertFreeBuffer(t, amzn, false)
			assertFreeBuffer(t, tsla, false)
			assertFreeBuffer(t, goog, true)

			assertWindowContent(t, win0, amzn)
			assertWindowContent(t, win, tsla)
		})
	}
}

func TestComponentSetContentUnmount(t *testing.T) {
	c := NewComponent(Config{})
	win0 := c.Focus()
	h1 := NewTestHandler()
	h2 := NewTestHandler()
	var unmounted int
	h1.OnUnmountCallback = func() error {
		unmounted++
		return nil
	}
	assert.NoError(t, win0.SetContent(h1))
	assertWindowContent(t, win0, h1)
	assert.NoError(t, win0.SetContent(h2))
	assertWindowContent(t, win0, h2)
	assert.Equal(t, 1, unmounted)

	assert.NoError(t, win0.SetContent(h1))
	assertWindowContent(t, win0, h1)
	assert.Equal(t, 1, unmounted)
	content1, err := win0.Content()
	require.NoError(t, err)

	assert.NoError(t, win0.SetContent(content1))
	assertWindowContent(t, win0, h1)
	// unmounted and mounted again
	assert.Equal(t, 2, unmounted)

	win1 := c.SplitHorizontalBelow(h2)
	assert.Equal(t, 2, unmounted)

	assert.NoError(t, win0.Close())
	assert.Equal(t, 3, unmounted)

	assert.Equal(t, c.Focus(), win1)
}

func TestComponentHandlerUnmount(t *testing.T) {
	cfg := DefaultConfig()

	for _, _tcase := range splitSuite {
		split := _tcase.split
		t.Run(_tcase.method, func(t *testing.T) {
			tsuite := []struct {
				name string
				fn   func(*testing.T, *Component, Window, *testFlushCloser)
			}{
				{
					"UpdateWindowBufferNextFree",
					func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
						assert.True(t, c.UpdateWindowBufferNextFree(win))
					},
				}, {
					"UpdateWindowBufferNext",
					func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
						assert.True(t, c.UpdateWindowBufferNext(win))
					},
				}, {
					"UpdateWindowBufferPrev",
					func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
						assert.True(t, c.UpdateWindowBufferPrev(win))
					},
				}, {
					"Window.SetContent",
					func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
						assert.NoError(t, win.SetContent(NewTestHandler()))
					},
				}, {
					"Window.Close",
					func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
						assert.NoError(t, win.Close())
					},
				}, {
					"RemoveWindowBuffer",
					func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
						assert.True(t, c.RemoveWindowBuffer(win))
					},
				}, {
					"Handle(exit=true)",
					func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
						h.Exit = true
						h.Handled = true
						exit, handled := c.Handle(term.Event{})
						assert.False(t, exit)
						assert.True(t, handled)
					},
				},
			}

			for _, tcase := range tsuite {
				t.Run(tcase.name, func(t *testing.T) {
					mock := &testFlushCloser{}
					c := NewComponent(cfg)
					c.NewBuffer("Robinhood", NewTestHandler(), nil)
					c.NewBuffer("Stash", NewTestHandler(), nil)
					win := split(c, mock)

					tcase.fn(t, c, win, mock)
					assert.Equal(t, 1, mock.unmounted)
				})
			}
		})
	}
}

func TestComponentMultipleWindow(t *testing.T) {
	w := term.NewStringWriter(20, 8)

	cfg := DefaultConfig()
	cfg.Logger = log.New()
	cfg.Logger.SetLevel(log.TraceLevel)
	c := NewComponent(cfg)
	c.Resize(20, 8)

	// w1 := c.Focus()
	var w2 Window
	// var w3 Window
	h2 := NewTestHandler()
	h3 := NewTestHandler()
	h3.Ch = 'C'

	tests := []testutil.ComponentTestCase{
		{
			nil, `
┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
		}, {func() {
			w2 = c.SplitHorizontalBelow(h2)
		}, `
┌──────────────────┐
│                  │
├──────────────────┤
│                  │
└──────────────────┘
┌──────────────────┐
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`,
		}, {func() {
			/*w3 =*/ c.SplitVerticalRight(h3)
		}, `
┌──────────────────┐
│                  │
├──────────────────┤
│                  │
└──────────────────┘
┌────────┐┌────────┐
│AAAAAAAA││CCCCCCCC│
└────────┘└────────┘`,
		}, {func() {
			assert.True(t, c.FocusLeft())

			var unmounted int
			var closed int
			h2.Exit = true
			w2.onWindowClosed(func() {
				closed++
			})
			h2.OnUnmountCallback = func() error {
				assert.NoError(t, w2.Close())
				unmounted++
				return nil
			}
			h2.Handled = true
			_, handled := c.Handle(term.Event{})
			assert.True(t, handled)
			assert.Equal(t, 1, unmounted)
			assert.Equal(t, 1, closed)
		}, `
┌──────────────────┐
│                  │
├──────────────────┤
│                  │
└──────────────────┘
┌──────────────────┐
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		},
	}

	testutil.TestComponent(t, c, w, tests)
}

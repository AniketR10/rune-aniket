package browser

import (
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
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
			h = handler.NewTestHandler()
			h = c.NewBuffer("OAK", h, nil)

			win := tcase.split(c, h)
			require.Equal(t, 2, c.wm.Size())

			assert.NoError(t, win.Close())
			require.Equal(t, 1, c.wm.Size())

			assert.Len(t, c.freeBuffers(), 1)
		})

		t.Run(tcase.method+"Close is idempotent", func(t *testing.T) {
			c := NewComponent(Config{})
			win := tcase.split(c, handler.NewTestHandler())
			require.NoError(t, win.Close())
			assert.NoError(t, win.Close())
		})

		t.Run(tcase.method+"close window on handler exit", func(t *testing.T) {
			c := NewComponent(Config{})
			var h Handler
			h = handler.NewTestHandler()
			h.(*handler.TestHandler).Exit = true
			h = c.NewBuffer("bla", h, nil)
			win := tcase.split(c, h)

			assert.Equal(t, 1, c.tabs.Size())
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
			win := tcase.split(c, handler.NewTestHandler())
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
	w2.SplitHorizontalAbove(handler.NewTestHandler())

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

func TestComponentSetContent(t *testing.T) {
	for _, _tcase := range splitSuite {
		tcase := _tcase
		t.Run(tcase.method, func(t *testing.T) {
			c := NewComponent(Config{})
			win0 := c.Focus()
			christmasBuffer := c.NewBuffer("Merry Christmas", handler.NewTestHandler(), nil)

			assertFreeBuffer(t, christmasBuffer, true)
			require.NoError(t, win0.SetContent(christmasBuffer))
			assertFreeBuffer(t, christmasBuffer, false)
			assert.False(t, c.UpdateWindowBufferNext(win0))

			win1 := tcase.split(c, NopHandler(&handler.TestHandler{}))
			require.Error(t, win1.SetContent(christmasBuffer))
			assertFreeBuffer(t, christmasBuffer, false)

			require.NoError(t, win1.SetContent(NopHandler(&handler.TestHandler{})))
			assertFreeBuffer(t, christmasBuffer, false)

			require.NoError(t, win0.SetContent(NopHandler(&handler.TestHandler{})))
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
			amzn := c.NewBuffer("AMZN", handler.NewTestHandler(), nil)
			c.UpdateWindowBufferNextFree(win0)
			tsla := c.NewBuffer("TSLA", handler.NewTestHandler(), nil)
			goog := c.NewBuffer("GOOG", handler.NewTestHandler(), nil)
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
		})
	}
}

func TestComponentHandlerUnmount(t *testing.T) {
	cfg := DefaultConfig()

	for _, _tcase := range splitSuite {
		split := _tcase.split
		t.Run(_tcase.method, func(t *testing.T) {
			tsuite := []func(*testing.T, *Component, Window, *testFlushCloser){
				func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
					assert.True(t, c.UpdateWindowBufferNextFree(win))
				},
				func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
					assert.True(t, c.UpdateWindowBufferNext(win))
				},
				func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
					assert.True(t, c.UpdateWindowBufferPrev(win))
				},
				func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
					assert.NoError(t, win.SetContent(handler.NewTestHandler()))
				},
				func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
					assert.NoError(t, win.Close())
				},
				func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
					assert.True(t, c.RemoveWindowBuffer(win))
				},
				func(t *testing.T, c *Component, win Window, h *testFlushCloser) {
					h.Exit = true
					exit, handled := c.Handle(term.Event{})
					assert.True(t, handled)
					assert.False(t, exit)
				},
			}

			for _, tcase := range tsuite {
				mock := &testFlushCloser{}
				c := NewComponent(cfg)
				c.NewBuffer("Robinhood", handler.NewTestHandler(), nil)
				c.NewBuffer("Stash", handler.NewTestHandler(), nil)
				win := split(c, mock)

				tcase(t, c, win, mock)
				assert.Equal(t, 1, mock.unmounted)
			}
		})
	}
}

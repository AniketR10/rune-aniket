package browser

import (
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloseWindow(t *testing.T) {

	t.Run("closing the last window returns error", func(t *testing.T) {
		c := NewComponent(Config{})
		assert.Error(t, c.Focus().Close())
	})

	t.Run("closes window correctly", func(t *testing.T) {
		c := NewComponent(Config{})

		var h tui.Handler
		h = handler.NewTestHandler()
		h, _ = c.newBuffer("OAK", h, nil)

		win := c.SplitVerticalLeft(h)
		require.Equal(t, 2, c.wm.Size())

		assert.NoError(t, win.Close())
		require.Equal(t, 1, c.wm.Size())

		assert.Len(t, c.freeBuffers(), 1)
	})

	t.Run("Close is idempotent", func(t *testing.T) {
		c := NewComponent(Config{})
		win := c.SplitVerticalLeft(handler.NewTestHandler())
		require.NoError(t, win.Close())
		assert.NoError(t, win.Close())
	})

	t.Run("close window on handler exit", func(t *testing.T) {
		c := NewComponent(Config{})
		var h tui.Handler
		h = handler.NewTestHandler()
		h.(*handler.TestHandler).Exit = true
		h, _ = c.newBuffer("bla", h, nil)
		win := c.SplitVerticalLeft(h)

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
}

func TestRemoveAllBuffers(t *testing.T) {
	w1 := NewComponent(Config{})
	w2 := NewComponent(Config{})
	w2.SplitHorizontalAbove(handler.NewTestHandler())

	tsuite := []struct {
		description string
		c           *Component
		before      int
		after       int
	}{
		{"removes all buffers with one window", w1, 3, 1},
		{"removes all buffers with multiple windows", w2, 3, 2},
	}

	for _, tcase := range tsuite {
		c := tcase.c
		before := tcase.before
		after := tcase.after
		t.Run(tcase.description, func(t *testing.T) {
			c.NewBuffer("a", handler.NewTestHandler(), nil)
			c.NewBuffer("b", handler.NewTestHandler(), nil)
			c.NewBuffer("c", handler.NewTestHandler(), nil)

			assert.Equal(t, before, c.tabs.Size())
			c.RemoveAllBuffers()

			assert.Equal(t, after, c.tabs.Size())
		})
	}
}

type testFlushCloser struct {
	handler.TestHandler
	flushed, closed int
}

func (t *testFlushCloser) Flush() error {
	t.flushed++
	return nil
}

func (t *testFlushCloser) Close() error {
	t.closed++
	return nil
}

func TestFlushBuffer(t *testing.T) {
	c := NewComponent(Config{})
	mock := testFlushCloser{}
	c.NewBuffer("a", &mock, &mock)

	c.FlushBuffer(c.Focus())
	assert.Equal(t, 1, mock.flushed)
	assert.Equal(t, 0, mock.closed)
}

func assertFreeBuffer(t *testing.T, h tui.Handler, free bool) {
	assert.Equal(t, free, h.(*buffer).free)
}

func TestUpdateWindowBuffer(t *testing.T) {
	// FIXME once API has been refactored
	t.SkipNow()

	c := NewComponent(Config{})
	win0 := c.Focus()
	amzn := c.NewBuffer("AMZN", handler.NewTestHandler(), nil)
	tsla := c.NewBuffer("TSLA", handler.NewTestHandler(), nil)
	goog := c.NewBuffer("GOOG", handler.NewTestHandler(), nil)
	win := c.SplitHorizontalBelow(goog)

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
	assertFreeBuffer(t, tsla, true)
	assertFreeBuffer(t, goog, false)
}

func TestComponentHandlerCloser(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Logger = log.StandardLogger()
	log.SetLevel(log.TraceLevel)

	t.Run("call io.Closer.Close on handler if content is ephemeral and is removed", func(t *testing.T) {
		tsuite := []func(*testing.T, *Component, Window){
			func(t *testing.T, c *Component, win Window) {
				assert.True(t, c.UpdateWindowBufferNextFree(win))
			},
			func(t *testing.T, c *Component, win Window) {
				assert.True(t, c.UpdateWindowBufferNext(win))
			},
			func(t *testing.T, c *Component, win Window) {
				assert.True(t, c.UpdateWindowBufferPrev(win))
			},
			func(t *testing.T, c *Component, win Window) {
				require.NoError(t, win.Close())
			},
		}

		for _, tcase := range tsuite {
			mock := &testFlushCloser{}
			c := NewComponent(cfg)
			c.NewBuffer("Robinhood", handler.NewTestHandler(), nil)
			c.NewBuffer("Stash", handler.NewTestHandler(), nil)
			win := c.SplitHorizontalBelow(mock)

			tcase(t, c, win)
			assert.Equal(t, 1, mock.closed)
		}
	})
}

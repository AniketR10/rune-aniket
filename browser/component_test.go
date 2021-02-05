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

		// makes sure that window list is not corrupted
		c.RemoveAllTabs()
	})

	for _, _tcase := range splitSuite {
		tcase := _tcase
		t.Run(tcase.method+"closes window correctly", func(t *testing.T) {
			c := NewComponent(Config{})

			var h Handler
			h = NewTestHandler()
			h = c.NewTab("OAK", "OAK", h, nil)

			win := tcase.split(c, h)
			require.Equal(t, 2, c.wm.Size())

			assert.NoError(t, win.Close())
			require.Equal(t, 1, c.wm.Size())

			assert.Len(t, c.freeTabs(), 1)
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

func TestComponentRemoveAllTabs(t *testing.T) {
	w1 := NewComponent(Config{})
	w2 := NewComponent(Config{})
	w2.SplitHorizontalAbove(NewTestHandler())

	tsuite := []struct {
		description string
		c           *Component
		before      int
		after       int
	}{
		{"removes all tabs with one window", w1, 3, 0},
		{"removes all tabs with multiple windows", w2, 3, 0},
	}

	for _, tcase := range tsuite {
		c := tcase.c
		before := tcase.before
		after := tcase.after
		t.Run(tcase.description, func(t *testing.T) {
			handlers := [3]testCloser{}
			c.NewTab("a", "a", &handlers[0], &handlers[0])
			c.NewTab("b", "b", &handlers[1], &handlers[1])
			c.NewTab("c", "c", &handlers[2], &handlers[2])
			c.UpdateWindowTabNextFree(c.Focus())
			c.ShiftFocus()
			c.UpdateWindowTabNextFree(c.Focus())

			assert.Equal(t, before, c.tabs.Size())
			c.RemoveAllTabs()

			assert.Equal(t, after, c.tabs.Size())
			for _, h := range handlers {
				assert.Equal(t, 1, h.closed)
				// tab does not get called unmount
				// because that's just for internal use
				assert.Equal(t, 0, h.unmounted)
			}
		})
	}
}

type testCloser struct {
	handler.TestHandler
	closed, unmounted int
}

func (t *testCloser) Close() error {
	t.closed++
	return nil
}
func (t *testCloser) OnUnmount() error {
	t.unmounted++
	return nil
}

func assertFreeTab(t *testing.T, h tui.Handler, free bool) {
	assert.Equal(t, free, h.(*Tab).free)
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
			christmasTab := c.NewTab("Merry Christmas", "Merry Christmas", NewTestHandler(), nil)

			// if component is rendering start text, then first call
			// to NewTab should set the content to the new tab
			assertFreeTab(t, christmasTab, false)
			require.Error(t, win0.SetContent(christmasTab))
			assertFreeTab(t, christmasTab, false)
			assertWindowContent(t, win0, christmasTab)
			assert.False(t, c.UpdateWindowTabNext(win0))

			h2 := NopHandler(&handler.TestHandler{})
			win1 := tcase.split(c, h2)
			assertWindowContent(t, win1, h2)
			require.Error(t, win1.SetContent(christmasTab))
			assertWindowContent(t, win1, h2)
			assertFreeTab(t, christmasTab, false)

			h3 := NopHandler(&handler.TestHandler{})
			require.NoError(t, win1.SetContent(h3))
			assertWindowContent(t, win1, h3)
			assertFreeTab(t, christmasTab, false)

			h4 := NopHandler(&handler.TestHandler{})
			require.NoError(t, win0.SetContent(h4))
			assertWindowContent(t, win0, h4)
			assertFreeTab(t, christmasTab, true)
		})
	}
}

func TestComponentUpdateWindowTab(t *testing.T) {
	for _, _tcase := range splitSuite {
		tcase := _tcase
		t.Run(tcase.method, func(t *testing.T) {
			c := NewComponent(Config{})
			win0 := c.Focus()
			amzn := c.NewTab("AMZN", "AMZN", NewTestHandler(), nil)
			c.UpdateWindowTabNextFree(win0)
			tsla := c.NewTab("TSLA", "TSLA", NewTestHandler(), nil)
			goog := c.NewTab("GOOG", "GOOG", NewTestHandler(), nil)
			win := tcase.split(c, goog)

			assertFreeTab(t, amzn, false)
			assertFreeTab(t, tsla, true)
			assertFreeTab(t, goog, false)

			for i := 0; i < 3; i++ {
				assert.True(t, c.UpdateWindowTabNext(win))
			}

			assertFreeTab(t, amzn, false)
			assertFreeTab(t, tsla, false)
			assertFreeTab(t, goog, true)

			for i := 0; i < 3; i++ {
				assert.True(t, c.UpdateWindowTabPrev(win0))
			}

			assertFreeTab(t, amzn, true)
			assertFreeTab(t, tsla, false)
			assertFreeTab(t, goog, false)

			assert.True(t, c.UpdateWindowTabNextFree(win0))

			assertFreeTab(t, amzn, false)
			assertFreeTab(t, tsla, false)
			assertFreeTab(t, goog, true)

			assert.True(t, c.UpdateWindowTabLastFree(win0))

			assertFreeTab(t, amzn, true)
			assertFreeTab(t, tsla, false)
			assertFreeTab(t, goog, false)

			assert.Error(t, win0.SetContent(tsla))
			assert.NoError(t, win0.SetContent(amzn))

			assertFreeTab(t, amzn, false)
			assertFreeTab(t, tsla, false)
			assertFreeTab(t, goog, true)

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
				fn   func(*testing.T, *Component, Window, *testCloser)
			}{
				{
					"UpdateWindowTabNextFree",
					func(t *testing.T, c *Component, win Window, h *testCloser) {
						assert.True(t, c.UpdateWindowTabNextFree(win))
					},
				}, {
					"UpdateWindowTabNext",
					func(t *testing.T, c *Component, win Window, h *testCloser) {
						assert.True(t, c.UpdateWindowTabNext(win))
					},
				}, {
					"UpdateWindowTabPrev",
					func(t *testing.T, c *Component, win Window, h *testCloser) {
						assert.True(t, c.UpdateWindowTabPrev(win))
					},
				}, {
					"Window.SetContent",
					func(t *testing.T, c *Component, win Window, h *testCloser) {
						assert.NoError(t, win.SetContent(NewTestHandler()))
					},
				}, {
					"Window.Close",
					func(t *testing.T, c *Component, win Window, h *testCloser) {
						assert.NoError(t, win.Close())
					},
				}, {
					"RemoveWindowContent",
					func(t *testing.T, c *Component, win Window, h *testCloser) {
						assert.True(t, c.RemoveWindowContent(win))
					},
				}, {
					"Handle(exit=true)",
					func(t *testing.T, c *Component, win Window, h *testCloser) {
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
					mock := &testCloser{}
					c := NewComponent(cfg)
					c.NewTab("Robinhood", "Robinhood", NewTestHandler(), nil)
					c.NewTab("Stash", "Stash", NewTestHandler(), nil)
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

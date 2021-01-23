package editor

import (
	"errors"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	keya        = term.Event{Type: term.EventKey, Ch: 'a'}
	keyb        = term.Event{Type: term.EventKey, Ch: 'b'}
	evInterrupt = term.Event{Type: term.EventInterrupt}
)

type testFlusherCloser struct{}

func (t testFlusherCloser) Close() error { return nil }
func (t testFlusherCloser) Flush() error { return nil }

func newTestComponent(ed Editor, opts ...browser.Option) (*Component, error) {
	c, err := NewComponent(ed, opts...)
	if err != nil {
		return nil, err
	}

	c.openFileFn = func(filePath string,
		buf *cell.Buffer, swapDir string) (flusherCloser, error) {
		return testFlusherCloser{}, nil
	}
	c.recoverFileFn = func(filePath, swapFilePath string,
		buf *cell.Buffer) (flusherCloser, error) {
		return testFlusherCloser{}, nil
	}
	return c, nil
}

func TestComponentInterfaces(t *testing.T) {
	// this test is just a compile-time test
	c, err := NewComponent(&testEditor{})
	require.NoError(t, err)

	var ed Editor
	ed = c

	var b browser.Browser
	b = c

	var comp tui.Component
	comp = c

	// use so compiler does not complain
	ed.Edit("", cell.NewBuffer())
	_, _ = b.Focus()
	comp.Resize(0, 0)
}

func TestComponentKeyMapper(t *testing.T) {
	t.Run("KeyMapping on a non-mapped event returns false", func(t *testing.T) {
		c, err := newTestComponent(&testEditor{})
		require.NoError(t, err)

		ev, ok := c.KeyMapping(keya)
		assert.False(t, ok)
		assert.Equal(t, keya, ev)
	})

	t.Run("only allows Key event mappings", func(t *testing.T) {
		c, err := newTestComponent(&testEditor{})
		require.NoError(t, err)

		m1 := map[term.Event]term.Event{
			evInterrupt: keya,
		}
		m2 := map[term.Event]term.Event{
			keya: evInterrupt,
		}
		for _, m := range []map[term.Event]term.Event{m1, m2} {
			err := c.MergeKeyMap(m)
			require.Error(t, err)
		}
	})

	t.Run("KeyMapping returns mapped event", func(t *testing.T) {
		c, err := newTestComponent(&testEditor{})
		require.NoError(t, err)

		m := map[term.Event]term.Event{
			keya: keyb,
		}
		err = c.MergeKeyMap(m)
		require.NoError(t, err)

		ev, ok := c.KeyMapping(keya)
		assert.True(t, ok)
		assert.Equal(t, keyb, ev)
	})
}

func TestComponentTermSubscriber(t *testing.T) {
	t.Run("Publish on a non-mapped event returns false", func(t *testing.T) {
		c, err := newTestComponent(&testEditor{})
		require.NoError(t, err)

		ok := c.Publish(keya)
		assert.False(t, ok)
	})

	t.Run("only allows Key event subscriptions", func(t *testing.T) {
		c, err := newTestComponent(&testEditor{})
		require.NoError(t, err)

		err = c.Subscribe(evInterrupt, browser.FuncEventHandler(func(term.Event) bool { return false }))
		require.Error(t, err)
	})

	t.Run("only allows ONE subscription. Second attempt returns error", func(t *testing.T) {
		c, err := newTestComponent(&testEditor{})
		require.NoError(t, err)

		err = c.Subscribe(keya, browser.FuncEventHandler(func(term.Event) bool { return false }))
		require.NoError(t, err)
		err = c.Subscribe(keya, browser.FuncEventHandler(func(term.Event) bool { return false }))
		require.Error(t, err)
	})

	t.Run("Publish publishes event to ONE subscribed handler", func(t *testing.T) {
		c, err := newTestComponent(&testEditor{})
		require.NoError(t, err)

		var called int
		err = c.Subscribe(keya, browser.FuncEventHandler(func(term.Event) bool {
			called++
			return false
		}))
		err = c.Subscribe(keya, browser.FuncEventHandler(func(term.Event) bool {
			called++
			return false
		}))

		handled := c.Publish(keya)
		assert.True(t, handled)
		assert.Equal(t, 1, called)
	})
}

func newTestComponentWithFile(
	t *testing.T, filename string,
) (*Component, browser.Handler) {
	c, err := newTestComponent(&testEditor{})
	require.NoError(t, err)
	h, err := c.Open(filename)
	require.NoError(t, err)
	return c, h
}

func TestComponentOpen(t *testing.T) {
	t.Run("opens a new tab", func(t *testing.T) {
		myName := "It's_1am_and_I'm_very_tired.go"
		c, h := newTestComponentWithFile(t, myName)

		h2, ok := c.Browser().Tab(myName)
		assert.True(t, ok)
		assert.Equal(t, h, h2)
	})

	t.Run("it's idempotent", func(t *testing.T) {
		myName := "La_Rosalia.mp3"

		c, h := newTestComponentWithFile(t, myName)
		_, ok := c.Browser().Tab(myName)
		assert.True(t, ok)

		c.openFileFn = func(filePath string,
			buf *cell.Buffer, swapDir string) (flusherCloser, error) {
			return nil, ErrFileAlreadyOpen
		}

		h2, err := c.Open(myName)
		require.NoError(t, err)
		assert.Equal(t, h, h2)
	})

	t.Run("bubbles up open file error", func(t *testing.T) {
		myName := "lmao"
		c, _ := newTestComponentWithFile(t, myName)
		myErr := errors.New("oopsie daisy")
		c.openFileFn = func(filePath string,
			buf *cell.Buffer, swapDir string) (flusherCloser, error) {
			return nil, myErr
		}

		_, err := c.Open(myName)
		require.Error(t, err)
	})
}

func TestComponentEditorSubscriber(t *testing.T) {
	tsuite := []struct {
		name    string
		evType  EventType
		trigger func(*testing.T, *Component, string)
	}{
		{
			"Edit->EventTypeOpen",
			EventTypeOpen,
			func(t *testing.T, c *Component, resourceName string) {
				_, err := c.Edit(resourceName, cell.NewBuffer())
				assert.NoError(t, err)
			},
		},
		{
			"Flush->EventTypeFlush",
			EventTypeFlush,
			func(t *testing.T, c *Component, resourceName string) {
				_, err := c.OpenFileTab(resourceName, "")
				require.NoError(t, err)

				win, err := c.Focus()
				require.NoError(t, err)

				assert.NoError(t, c.Flush(win))
			},
		},
		{
			"Browser.RemoveWindowContent->EventTypeClose",
			EventTypeClose,
			func(t *testing.T, c *Component, resourceName string) {
				_, err := c.OpenFileTab(resourceName, "")
				require.NoError(t, err)

				win, err := c.Focus()
				require.NoError(t, err)

				c.Browser().RemoveWindowContent(win)
			},
		},
	}

	for _, _tcase := range tsuite {
		tcase := _tcase
		t.Run(tcase.name+" SubscribeEditor subscribes an event handler", func(t *testing.T) {
			c, err := newTestComponent(&testEditor{})
			require.NoError(t, err)

			filename := "Joe_Biden.txt"
			var fired int
			c.SubscribeEditor(tcase.evType, FuncEventHandler(func(ev Event) bool {
				fired++
				assert.Equal(t, ev.ResourceName, filename)
				return false
			}))

			tcase.trigger(t, c, filename)
			assert.Equal(t, 1, fired)
		})

		t.Run(tcase.name+" unsubscribes if handler returns exit=true", func(t *testing.T) {
			c, err := newTestComponent(&testEditor{})
			require.NoError(t, err)

			filename := "Jill_Biden.txt"
			var fired int
			c.SubscribeEditor(tcase.evType, FuncEventHandler(func(ev Event) bool {
				fired++
				return true
			}))

			tcase.trigger(t, c, filename)
			tcase.trigger(t, c, filename)
			assert.Equal(t, 1, fired)
		})
	}
}

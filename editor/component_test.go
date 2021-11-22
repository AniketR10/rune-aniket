package editor

import (
	"errors"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	keya        = term.Event{Type: term.EventKey, Ch: 'a'}
	keyb        = term.Event{Type: term.EventKey, Ch: 'b'}
	evInterrupt = term.Event{Type: term.EventInterrupt}
)

type testFlusherCloser struct {
	closeFn func() error
	flushFn func() error
}

func (t *testFlusherCloser) Close() error {
	if t.closeFn != nil {
		return t.closeFn()
	}
	return nil
}
func (t *testFlusherCloser) Flush() error {
	if t.flushFn != nil {
		return t.flushFn()
	}
	return nil
}

func newTestComponentErr(ed Editor) (*Component, error) {
	c, err := NewComponent(ed, DefaultConfig())
	if err != nil {
		return nil, err
	}

	c.openFileFn = func(filePath string,
		buf *cell.Buffer, swapDir string, readOnly bool) (flusherCloser, error) {
		return &testFlusherCloser{}, nil
	}
	c.recoverFileFn = func(filePath, swapFilePath string,
		buf *cell.Buffer) (flusherCloser, error) {
		return &testFlusherCloser{}, nil
	}
	return c, nil
}

func newTestComponent(t *testing.T, ed Editor) *Component {
	c, err := newTestComponentErr(ed)
	require.NoError(t, err)
	return c
}

func TestComponentInterfaces(t *testing.T) {
	// this test is just a compile-time test
	c, err := NewComponent(&testEditor{}, DefaultConfig())
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
		c := newTestComponent(t, &testEditor{})

		ev, _, ok := c.KeyMapping(keya)
		assert.False(t, ok)
		assert.Equal(t, keya, ev)
	})

	t.Run("only allows Key event mappings", func(t *testing.T) {
		c := newTestComponent(t, &testEditor{})

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
		c := newTestComponent(t, &testEditor{})

		m := map[term.Event]term.Event{
			keya: keyb,
		}
		err := c.MergeKeyMap(m)
		require.NoError(t, err)

		ev, _, ok := c.KeyMapping(keya)
		assert.True(t, ok)
		assert.Equal(t, keyb, ev)
	})

	t.Run("KeyMapping returns mapped command", func(t *testing.T) {
		c := newTestComponent(t, &testEditor{})
		c.config.CommandKeyBindings[keyb] = "myCmd"

		m := map[term.Event]term.Event{
			keya: keyb,
		}
		err := c.MergeKeyMap(m)
		require.NoError(t, err)

		ev, cmd, ok := c.KeyMapping(keya)
		assert.True(t, ok)
		assert.Equal(t, keyb, ev)
		assert.Equal(t, "myCmd", cmd)
	})
}

func TestComponentTermSubscriber(t *testing.T) {
	t.Run("Publish on a non-mapped event returns false", func(t *testing.T) {
		c := newTestComponent(t, &testEditor{})

		ok := c.Publish(keya)
		assert.False(t, ok)
	})

	t.Run("only allows Key event subscriptions", func(t *testing.T) {
		c := newTestComponent(t, &testEditor{})

		err := c.SubscribeTermEvents(evInterrupt, browser.FuncEventHandler(func(term.Event) bool { return false }))
		require.Error(t, err)
	})

	t.Run("only allows ONE subscription. Second attempt returns error", func(t *testing.T) {
		c := newTestComponent(t, &testEditor{})

		err := c.SubscribeTermEvents(keya, browser.FuncEventHandler(func(term.Event) bool { return false }))
		require.NoError(t, err)
		err = c.SubscribeTermEvents(keya, browser.FuncEventHandler(func(term.Event) bool { return false }))
		require.Error(t, err)
	})

	t.Run("Publish publishes event to ONE subscribed handler", func(t *testing.T) {
		c := newTestComponent(t, &testEditor{})

		var called int
		err := c.SubscribeTermEvents(keya, browser.FuncEventHandler(func(term.Event) bool {
			called++
			return false
		}))
		require.NoError(t, err)
		err = c.SubscribeTermEvents(keya, browser.FuncEventHandler(func(term.Event) bool {
			called++
			return false
		}))
		require.Error(t, err)

		handled := c.Publish(keya)
		assert.True(t, handled)
		assert.Equal(t, 1, called)
	})
}

func newTestComponentWithFile(
	t *testing.T, filename string,
) (*Component, browser.Handler) {
	c := newTestComponent(t, &testEditor{})
	h, err := c.Open(filename)
	require.NoError(t, err)
	return c, h
}

func TestComponentOpen(t *testing.T) {
	t.Run("opens a new tab", func(t *testing.T) {
		myName := "/tmp/It's_1am_and_I'm_very_tired.go"
		c, h := newTestComponentWithFile(t, myName)

		h2, ok := c.Browser().Tab(myName)
		assert.True(t, ok)
		assert.Equal(t, h, h2)
	})

	t.Run("it's idempotent", func(t *testing.T) {
		myName := "/var/music/La_Rosalia.mp3"

		c, h := newTestComponentWithFile(t, myName)
		_, ok := c.Browser().Tab(myName)
		assert.True(t, ok)

		c.openFileFn = func(filePath string,
			buf *cell.Buffer, swapDir string, readOnly bool) (flusherCloser, error) {
			return nil, ErrFileAlreadyOpen
		}

		h2, err := c.Open(myName)
		require.NoError(t, err)
		assert.Equal(t, h, h2)
	})

	t.Run("it's idempotent 2", func(t *testing.T) {
		myName := "/tmp/It's_1am_and_I'm_very_tired.go"
		c, _ := newTestComponentWithFile(t, myName)

		t2, ok := c.Browser().Tab(myName)
		require.True(t, ok)

		b3, err := c.Open(myName)
		require.NoError(t, err)
		assert.Equal(t, t2, b3)
	})

	t.Run("bubbles up open file error", func(t *testing.T) {
		c, _ := newTestComponentWithFile(t, "lmao")
		myErr := errors.New("oopsie daisy")
		c.openFileFn = func(filePath string,
			buf *cell.Buffer, swapDir string, readOnly bool) (flusherCloser, error) {
			return nil, myErr
		}

		_, err := c.Open("smtg_else")
		require.Error(t, err)
	})

	t.Run("if file is already open it returns its handler", func(t *testing.T) {
		filename := "wasup"
		c, h1 := newTestComponentWithFile(t, filename)
		c.openFileFn = func(filePath string,
			buf *cell.Buffer, swapDir string, readOnly bool) (flusherCloser, error) {
			t.Log("should not call openFileFn")
			t.Fail()
			return nil, nil
		}

		h2, err := c.Open(filename)
		require.NoError(t, err)
		assert.Equal(t, h1, h2)
	})
}

func TestComponentEditorSubscriber(t *testing.T) {
	content := "Mr. Patoto"
	tsuite := []struct {
		name       string
		evType     EventType
		trigger    func(*testing.T, *Component, string)
		preTrigger func(*testing.T, *Component, string)
	}{
		{
			"Edit->EventTypeOpen",
			EventTypeOpen,
			func(t *testing.T, c *Component, resourceName string) {
				buf := cell.NewBuffer()
				buf.WriteString(content)
				_, err := c.Edit(resourceName, buf)
				assert.NoError(t, err)
			},
			nil,
		},
		{
			"OpenFileTab->EventTypeOpen",
			EventTypeOpen,
			func(t *testing.T, c *Component, resourceName string) {
				_, err := c.OpenFileTab(resourceName, "", false)
				assert.NoError(t, err)
			},
			nil,
		},
		{
			"Open->EventTypeOpen",
			EventTypeOpen,
			func(t *testing.T, c *Component, resourceName string) {
				_, err := c.Open(resourceName)
				assert.NoError(t, err)
			},
			nil,
		},
		{
			"Flush->EventTypeFlush",
			EventTypeFlush,
			func(t *testing.T, c *Component, resourceName string) {
				h, err := c.OpenFileTab(resourceName, "", false)
				require.NoError(t, err)

				win, err := c.Focus()
				require.NoError(t, err)

				require.NoError(t, win.SetContent(h))

				assert.NoError(t, c.Flush(win))
			},
			nil,
		},
		{
			"Browser.RemoveWindowContent->EventTypeClose",
			EventTypeClose,
			func(t *testing.T, c *Component, resourceName string) {
				h, err := c.OpenFileTab(resourceName, "", false)
				require.NoError(t, err)

				win, err := c.Focus()
				require.NoError(t, err)

				require.NoError(t, win.SetContent(h))

				c.Browser().RemoveWindowContent(win)
			},
			nil,
		},
		{
			"buf.WriteString->EventTypeInsert",
			EventTypeInsert,
			func(t *testing.T, c *Component, resourceName string) {
				buf := cell.NewBuffer()
				_, err := c.Edit(resourceName, buf)
				assert.NoError(t, err)

				buf.WriteString("wasup")
			},
			nil,
		},
		{
			"buf.DeleteRow->EventTypeDelete",
			EventTypeDelete,
			func(t *testing.T, c *Component, resourceName string) {
				buf := cell.NewBuffer()
				buf.WriteString("wasup")
				_, err := c.Edit(resourceName, buf)
				assert.NoError(t, err)

				buf.DeleteRow(0)
			},
			nil,
		},
		{
			"Window.SetContent->EventTypeFocus",
			EventTypeFocus,
			func(t *testing.T, c *Component, resourceName string) {
				h, err := c.Open(resourceName)
				assert.NoError(t, err)

				win, err := c.Focus()
				require.NoError(t, err)
				require.NoError(t, win.SetContent(h))
			},
			nil,
		},
		{
			"Split->EventTypeFocus",
			EventTypeFocus,
			func(t *testing.T, c *Component, resourceName string) {
				h, err := c.OpenFileTab(resourceName, "", false)
				require.NoError(t, err)

				_, err = c.Split(browser.OrientationBottom, h)
				require.NoError(t, err)
			},
			func(t *testing.T, c *Component, resourceName string) {
				_, err := c.Open("blah")
				require.NoError(t, err)
			},
		},
		{
			"SetContent->EventTypeFocus",
			EventTypeFocus,
			func(t *testing.T, c *Component, resourceName string) {
				// SubscribeEditor should trigger it
			},
			func(t *testing.T, c *Component, resourceName string) {
				h, err := c.Open(resourceName)
				require.NoError(t, err)

				win, err := c.Focus()
				require.NoError(t, err)

				require.NoError(t, win.SetContent(h))
			},
		},
		{
			"SubscribeOpen->EventTypeOpen",
			EventTypeOpen,
			func(t *testing.T, c *Component, resourceName string) {
				// SubscribeEditor should trigger it
			},
			func(t *testing.T, c *Component, resourceName string) {
				_, err := c.Open(resourceName)
				require.NoError(t, err)
			},
		},
		{
			"Handle>EventTypeCursor",
			EventTypeCursor,
			func(t *testing.T, c *Component, resourceName string) {
				buf := cell.NewBuffer()
				buf.WriteString(content)
				h, err := c.Edit(resourceName, buf)
				assert.NoError(t, err)
				h.Handle(term.Event{Ch: 'l'})
			},
			nil,
		},
	}

	for _, _tcase := range tsuite {
		tcase := _tcase
		t.Run(tcase.name+" SubscribeEditor subscribes an event handler", func(t *testing.T) {
			c := newTestComponent(t, &testEditor{})

			filename := "~/Joe_Biden.txt"
			if tcase.preTrigger != nil {
				tcase.preTrigger(t, c, filename)
			}

			var fired int
			usr, _ := user.Current()
			dir := usr.HomeDir
			c.SubscribeEditorEvents(tcase.evType, FuncEventHandler(func(ev Event) bool {
				// if preTrigger, then only assert relevant file event
				if tcase.preTrigger == nil {
					assert.Equal(t, filepath.Base(ev.ResourceName), "Joe_Biden.txt")
					fired++
				} else if filepath.Base(ev.ResourceName) == "Joe_Biden.txt" {
					fired++
				}
				// Edit skip Edit as it takes the resource name as is.
				if tcase.evType == EventTypeOpen && tcase.name != "Edit->EventTypeOpen" {
					assert.True(t, strings.Contains(ev.ResourceName, dir))
				}
				return false
			}))

			tcase.trigger(t, c, filename)
			assert.Equal(t, 1, fired)
		})

		t.Run(tcase.name+" unsubscribes if handler returns exit=true", func(t *testing.T) {
			c := newTestComponent(t, &testEditor{})

			filename := "Jill_Biden.txt"
			if tcase.preTrigger != nil {
				tcase.preTrigger(t, c, filename)
			}

			var fired int
			c.SubscribeEditorEvents(tcase.evType, FuncEventHandler(func(ev Event) bool {
				if tcase.preTrigger == nil {
					fired++
				} else if filepath.Base(ev.ResourceName) == "Jill_Biden.txt" {
					fired++
				}
				return true
			}))

			tcase.trigger(t, c, filename)
			assert.Equal(t, 1, fired)
		})
	}

	t.Run("no events are dispatched after Close is called", func(t *testing.T) {
		c := newTestComponent(t, &testEditor{})

		filename := "Jill_Biden.txt"
		ev := Event{
			Type:         EventTypeClose,
			ResourceName: filename,
		}
		fc := testFlusherCloser{closeFn: func() error {
			c.dispatchEvent(ev)
			return nil
		}}
		c.openFileFn = func(filePath string,
			buf *cell.Buffer, swapDir string, readOnly bool) (flusherCloser, error) {
			return &fc, nil
		}

		var fired int
		c.SubscribeEditorEvents(EventTypeClose, FuncEventHandler(func(ev Event) bool {
			fired++
			return false
		}))

		_, err := c.Open(filename)
		require.NoError(t, err)

		assert.NoError(t, c.Close())
		assert.Equal(t, 0, fired)
	})

	for _, _tcase := range tsuite {
		switch _tcase.evType {
		case EventTypeFlush, EventTypeOpen:
		default:
			return
		}

		tcase := _tcase
		t.Run(tcase.name+" events are dispatched with content", func(t *testing.T) {
			c := newTestComponent(t, &testEditor{})

			filename := "Toy Rory"

			c.openFileFn = func(filePath string,
				buf *cell.Buffer, swapDir string, readOnly bool) (flusherCloser, error) {
				buf.WriteString(content)
				return &testFlusherCloser{}, nil
			}

			var dispatched string
			c.SubscribeEditorEvents(tcase.evType, FuncEventHandler(func(ev Event) bool {
				dispatched = ev.Content
				return false
			}))

			tcase.trigger(t, c, filename)
			assert.Equal(t, content, dispatched)
		})
	}

	t.Run("one open event is dispatched per open tab upon subscribe to open", func(t *testing.T) {
		c := newTestComponent(t, &testEditor{})

		content := "how bout that"
		filename1 := "JJ.txt"
		filename2 := "J2.txt"

		c.openFileFn = func(filePath string,
			buf *cell.Buffer, swapDir string, readOnly bool) (flusherCloser, error) {
			buf.WriteString(content)
			return &testFlusherCloser{}, nil
		}

		_, err := c.Open(filename1)
		require.NoError(t, err)
		_, err = c.Open(filename2)
		require.NoError(t, err)

		var fired int
		c.SubscribeEditorEvents(EventTypeOpen, FuncEventHandler(func(ev Event) bool {
			fired++
			assert.Equal(t, content, ev.Content)
			return false
		}))

		assert.Equal(t, 2, fired)
	})
}

func TestDispatchCommand(t *testing.T) {
	t.Run("DispatchCommand returns false if there's no registered handler", func(t *testing.T) {
		c := newTestComponent(t, &testEditor{})

		assert.False(t, c.DispatchCommand(handler.NewTestHandler(), "jklfwe", "SELL"))
	})
}

func TestComponentEditor(t *testing.T) {
	t.Run("returns tab with name as Handler", func(t *testing.T) {
		myName := "/tmp/Ennio_Morricone.go"
		c, h1 := newTestComponentWithFile(t, myName)

		h2, err := c.Editor(myName)
		assert.NoError(t, err)
		assert.Equal(t, h1.(*browser.Tab).Handler(), h2)
	})

	t.Run("returns error if no handler is found with name", func(t *testing.T) {
		c := newTestComponent(t, &testEditor{})

		h, err := c.Editor("The sundown")
		assert.Error(t, err)
		assert.Nil(t, h)
	})
}

func TestComponentCommands(t *testing.T) {
	t.Run("returns empty slice if no commands have been registered", func(t *testing.T) {
		c := newTestComponent(t, &testEditor{})
		assert.Len(t, c.Commands(), 0)
	})

	t.Run("returns registered commands", func(t *testing.T) {
		c := newTestComponent(t, &testEditor{})
		c.SubscribeCommand("myCmd", FuncCommandHandler(func(Command) bool {
			return true
		}))
		cmds := c.Commands()
		require.Len(t, cmds, 1)
		assert.Equal(t, "myCmd", cmds[0])
	})
}

func testRegister(t *testing.T,
	constructor func(ed Editor, mu *sync.Mutex, resourceName string) (*Component, Editor, error)) {
	t.Run("Registered handler is unsubscribed upon returning exit=true", func(t *testing.T) {
		var mu sync.Mutex
		name1 := "HERS"
		myArgs := []string{"a", "bbbbbbbbbbbbbbbbbbbbb"}
		myCmd := "BUY"
		c, sut, err := constructor(&testEditor{}, &mu, name1)
		require.NoError(t, err)

		mu.Lock()
		h1, err := c.Edit(name1, cell.NewBuffer())
		mu.Unlock()
		require.NoError(t, err)

		var called int
		var wg sync.WaitGroup
		sut.SubscribeCommand(myCmd, FuncCommandHandler(func(cmd Command) bool {
			defer wg.Done()
			assert.Equal(t, myCmd, cmd.Name)
			assert.Equal(t, myArgs, cmd.Args)
			called++
			return true
		}))

		wg.Add(1)
		mu.Lock()
		assert.True(t, c.DispatchCommand(h1, name1, myCmd, myArgs...))
		mu.Unlock()

		wg.Wait()

		for i := 0; i < 20; i++ {
			mu.Lock()
			c.DispatchCommand(h1, name1, myCmd, myArgs...)
			mu.Unlock()
		}

		assert.Equal(t, 1, called)
	})
}

func TestComponentRegister(t *testing.T) {
	testRegister(t, func(ed Editor, mu *sync.Mutex, resName string) (*Component, Editor, error) {
		c, err := newTestComponentErr(ed)
		return c, c, err
	})
}

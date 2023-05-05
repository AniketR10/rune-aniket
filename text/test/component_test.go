package test

import (
	"context"
	"errors"
	"os/user"
	"strings"
	"sync"
	"testing"

	"github.com/ernestrc/blue/iterator"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	browsertest "unstable.build/go-tui/browser/test"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	testutil "unstable.build/go-tui/util/test"
	"unstable.build/go-tui/workspace"
	workspacetest "unstable.build/go-tui/workspace/test"
)

var (
	keya = term.KeyComb{Ch: 'a'}
	keyb = term.KeyComb{Ch: 'b'}
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

type testLoader struct {
	content       string
	flusherCloser *testFlusherCloser
	expectError   error
}

func (t *testLoader) Load(
	file workspaceapi.URI, buf *cell.Buffer, swapDir workspaceapi.URI, readOnly bool,
) (workspace.FlusherCloser, error) {
	if t.expectError != nil {
		return nil, t.expectError
	}
	if t.flusherCloser != nil {
		return t.flusherCloser, nil
	}
	if t.content != "" {
		buf.WriteString(t.content)
	}
	return &testFlusherCloser{}, nil
}

func (t *testLoader) Recover(
	file, swapFilePath workspaceapi.URI, buf *cell.Buffer, force bool,
) (workspace.FlusherCloser, error) {
	return t.Load(file, buf, workspaceapi.URI{}, false)
}

func (t *testLoader) URI(path string) (workspaceapi.URI, error) {
	panic("unused")
}

func newTestComponentErr(ed text.Editor, cfg text.Config) (*text.Component, *testLoader, error) {
	loader := &testLoader{}
	c, err := text.NewComponent(ed, loader, cfg)
	if err != nil {
		return nil, nil, err
	}
	return c, loader, nil
}

func newTestComponent(t *testing.T, ed text.Editor) (*text.Component, *testLoader) {
	return newTestComponentConfig(t, ed, text.DefaultConfig())
}

func newTestComponentConfig(t *testing.T, ed text.Editor, cfg text.Config) (
	*text.Component, *testLoader,
) {
	c, loader, err := newTestComponentErr(ed, cfg)
	require.NoError(t, err)
	return c, loader
}

func TestComponentInterfaces(t *testing.T) {
	// this test is just a compile-time test
	c, err := text.NewComponent(NopEditor(), &testLoader{}, text.DefaultConfig())
	require.NoError(t, err)

	var ed text.Editor
	ed = c

	var b browser.Browser
	b = c

	var comp tui.Component
	comp = c

	// use so compiler does not complain
	ed.Edit(workspaceapi.URI{}, cell.NewBuffer())
	_, _ = b.Focus()
	comp.Resize(0, 0)
}

func TestComponentKeyMapper(t *testing.T) {
	t.Run("KeyMapping on a non-mapped event returns false", func(t *testing.T) {
		c, _ := newTestComponent(t, NopEditor())
		_, ok := c.KeyMapping(keya)
		assert.False(t, ok)
	})

	t.Run("KeyMapping returns mapped command", func(t *testing.T) {
		config := text.DefaultConfig()
		config.CommandKeyBindings[keya] = []string{"myCmd"}
		c, _ := newTestComponentConfig(t, NopEditor(), config)
		cmd, ok := c.KeyMapping(keya)
		assert.True(t, ok)
		assert.Equal(t, []string{"myCmd"}, cmd)
	})

	t.Run("KeyMapping returns mapped command and args", func(t *testing.T) {
		config := text.DefaultConfig()
		config.CommandKeyBindings[keya] = []string{"myCmd", "1"}
		c, _ := newTestComponentConfig(t, NopEditor(), config)
		cmd, ok := c.KeyMapping(keya)
		assert.True(t, ok)
		assert.Equal(t, []string{"myCmd", "1"}, cmd)
	})
}

func newTestComponentWithFile(
	t *testing.T, filename string,
) (*text.Component, *testLoader, browserapi.Handler, workspaceapi.URI) {
	uri, err := workspaceapi.ParseURI(filename)
	require.NoError(t, err)
	c, loader := newTestComponent(t, NopEditor())
	h, err := c.Open(uri)
	require.NoError(t, err)
	return c, loader, h, uri
}

func TestComponentOpen(t *testing.T) {
	t.Run("opens a new tab", func(t *testing.T) {
		myName := "file:///tmp/Its_1am_and_Im_very_tired.go"
		c, _, h, uri := newTestComponentWithFile(t, myName)

		h2, ok := c.Browser().Tab(uri)
		assert.True(t, ok)
		assert.Equal(t, h, h2)
	})

	t.Run("it's idempotent", func(t *testing.T) {
		myName := "file:///var/music/La_Rosalia.mp3"

		c, loader, h, uri := newTestComponentWithFile(t, myName)
		_, ok := c.Browser().Tab(uri)
		assert.True(t, ok)

		loader.expectError = workspaceapi.ErrFileAlreadyOpen

		h2, err := c.Open(uri)
		require.NoError(t, err)
		assert.Equal(t, h, h2)
	})

	t.Run("it's idempotent 2", func(t *testing.T) {
		myName := "file:///tmp/Its_1am_and_Im_very_tired.go"
		c, _, _, uri := newTestComponentWithFile(t, myName)

		t2, ok := c.Browser().Tab(uri)
		require.True(t, ok)

		b3, err := c.Open(uri)
		require.NoError(t, err)
		assert.Equal(t, t2, b3)
	})

	t.Run("bubbles up open file error", func(t *testing.T) {
		c, loader, _, _ := newTestComponentWithFile(t, "file:///tmp/lmao")
		myErr := errors.New("oopsie daisy")
		loader.expectError = myErr

		uri, err := workspaceapi.ParseURI("file:///Holmes.xd")
		require.NoError(t, err)
		_, err = c.Open(uri)
		require.Error(t, err)
	})

	t.Run("if file is already open it returns its handler", func(t *testing.T) {
		c, loader, h1, uri := newTestComponentWithFile(t, "file:///tmp/wasup")
		loader.expectError = errors.New("should not be called")

		h2, err := c.Open(uri)
		require.NoError(t, err)
		assert.Equal(t, h1, h2)
	})
}

func TestComponentEditorSubscriber(t *testing.T) {
	content := "Mr. Patoto"
	tsuite := []struct {
		name       string
		evType     textapi.EventType
		trigger    func(*testing.T, *text.Component, workspaceapi.URI)
		preTrigger func(*testing.T, *text.Component, workspaceapi.URI)
	}{
		{
			"Edit->EventTypeOpen",
			textapi.EventTypeOpen,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				buf := cell.NewBuffer()
				buf.WriteString(content)
				_, err := c.Edit(resource, buf)
				assert.NoError(t, err)
			},
			nil,
		},
		{
			"OpenFileTab->EventTypeOpen",
			textapi.EventTypeOpen,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				_, err := c.OpenFileTab(resource, false)
				assert.NoError(t, err)
			},
			nil,
		},
		{
			"Open->EventTypeOpen",
			textapi.EventTypeOpen,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				_, err := c.Open(resource)
				assert.NoError(t, err)
			},
			nil,
		},
		{
			"Flush->EventTypeFlush",
			textapi.EventTypeFlush,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				h, err := c.OpenFileTab(resource, false)
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
			textapi.EventTypeClose,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				h, err := c.OpenFileTab(resource, false)
				require.NoError(t, err)

				win, err := c.Focus()
				require.NoError(t, err)

				require.NoError(t, win.SetContent(h))

				c.Browser().RemoveWindowContent(win)
			},
			nil,
		},
		{
			"buf.WriteString->EventTypeEdit",
			textapi.EventTypeEdit,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				buf := cell.NewBuffer()
				_, err := c.Edit(resource, buf)
				assert.NoError(t, err)

				buf.WriteString("wasup")
			},
			nil,
		},
		{
			"buf.DeleteRow->EventTypeEdit",
			textapi.EventTypeEdit,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				buf := cell.NewBuffer()
				buf.WriteString("wasup")
				_, err := c.Edit(resource, buf)
				assert.NoError(t, err)

				buf.DeleteRow(0)
			},
			nil,
		},
		{
			"Window.SetContent->EventTypeUnfocus",
			textapi.EventTypeUnfocus,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				uri2, err := workspaceapi.ParseURI("file:///tmp/bleh")
				require.NoError(t, err)

				win, err := c.Focus()
				require.NoError(t, err)

				h, err := c.Open(uri2)
				require.NoError(t, err)
				require.NoError(t, win.SetContent(h))
			},
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				h, err := c.Open(resource)
				require.NoError(t, err)

				win, err := c.Focus()
				require.NoError(t, err)
				require.NoError(t, win.SetContent(h))
			},
		},
		{
			"Window.SetContent->EventTypeFocus",
			textapi.EventTypeFocus,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				h, err := c.Open(resource)
				require.NoError(t, err)

				win, err := c.Focus()
				require.NoError(t, err)
				require.NoError(t, win.SetContent(h))
			},
			nil,
		},
		{
			"Split->EventTypeFocus",
			textapi.EventTypeFocus,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				h, err := c.OpenFileTab(resource, false)
				require.NoError(t, err)

				focus, err := c.Focus()
				require.NoError(t, err)
				_, err = c.Split(browserapi.OrientationBottom, focus, h)
				require.NoError(t, err)
			},
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				uri2, err := workspaceapi.ParseURI("file:///tmp/blah")
				require.NoError(t, err)
				_, err = c.Open(uri2)
				require.NoError(t, err)
			},
		},
		{
			"SetContent->EventTypeFocus",
			textapi.EventTypeFocus,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				// SubscribeEditor should trigger it
			},
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				h, err := c.Open(resource)
				require.NoError(t, err)

				win, err := c.Focus()
				require.NoError(t, err)

				require.NoError(t, win.SetContent(h))
			},
		},
		{
			"SubscribeOpen->EventTypeOpen",
			textapi.EventTypeOpen,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				// SubscribeEditor should trigger it
			},
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				_, err := c.Open(resource)
				require.NoError(t, err)
			},
		},
		{
			"Handle>EventTypeCursor",
			textapi.EventTypeCursor,
			func(t *testing.T, c *text.Component, resource workspaceapi.URI) {
				buf := cell.NewBuffer()
				buf.WriteString(content)
				h, err := c.Edit(resource, buf)
				assert.NoError(t, err)
				h.Handle(term.Event{Ch: 'l'})
			},
			nil,
		},
	}

	for _, _tcase := range tsuite {
		tcase := _tcase
		t.Run(tcase.name+" SubscribeEditor subscribes an event handler", func(t *testing.T) {
			c, _ := newTestComponent(t, NopEditor())

			filename := "~/Joe_Biden.txt"
			uri, err := workspaceapi.CurrentUserHostURI(filename)
			require.NoError(t, err)
			if tcase.preTrigger != nil {
				tcase.preTrigger(t, c, uri)
			}

			var fired int
			usr, _ := user.Current()
			dir := usr.HomeDir
			evs := []textapi.EventType{tcase.evType}
			h := text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
				// if preTrigger, then only assert relevant file event
				if tcase.preTrigger == nil {
					assert.True(t, strings.Contains(ev.URI.String(), "Joe_Biden.txt"))
					fired++
				} else if strings.Contains(ev.URI.String(), "Joe_Biden.txt") {
					fired++
				}
				// Edit skip Edit as it takes the resource name as is.
				if tcase.evType == textapi.EventTypeOpen && tcase.name != "Edit->EventTypeOpen" {
					assert.True(t, strings.Contains(ev.URI.String(), dir))
				}
				return false
			})
			c.SubscribeEvents(evs, h)

			tcase.trigger(t, c, uri)
			assert.Equal(t, 1, fired)
		})

		t.Run(tcase.name+" unsubscribes if handler returns exit=true", func(t *testing.T) {
			c, _ := newTestComponent(t, NopEditor())

			filename := "file:///Jill_Biden.txt"
			uri, err := workspaceapi.ParseURI(filename)
			require.NoError(t, err)
			if tcase.preTrigger != nil {
				tcase.preTrigger(t, c, uri)
			}

			var fired int
			evs := []textapi.EventType{tcase.evType}
			h := text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
				if tcase.preTrigger == nil {
					assert.True(t, strings.Contains(ev.URI.String(), "Jill_Biden.txt"))
					fired++
					return true
				}
				if strings.Contains(ev.URI.String(), "Jill_Biden.txt") {
					fired++
					return true
				}
				return false
			})
			c.SubscribeEvents(evs, h)

			tcase.trigger(t, c, uri)
			assert.Equal(t, 1, fired)
		})
	}

	t.Run("no events are dispatched after Close is called", func(t *testing.T) {
		c, loader := newTestComponent(t, NopEditor())

		filename := "file:///Jill_Biden.txt"
		uri, err := workspaceapi.ParseURI(filename)
		require.NoError(t, err)
		fc := testFlusherCloser{closeFn: func() error {
			return nil
		}}

		loader.flusherCloser = &fc

		var fired int
		evs := []textapi.EventType{textapi.EventTypeClose}
		h := text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			fired++
			return false
		})
		c.SubscribeEvents(evs, h)

		_, err = c.Open(uri)
		require.NoError(t, err)

		assert.NoError(t, c.Close())
		assert.Equal(t, 0, fired)
	})

	t.Run("one open event is dispatched per open tab upon subscribe to open", func(t *testing.T) {
		c, loader := newTestComponent(t, NopEditor())
		uri1, err := workspaceapi.ParseURI("file:///Jill_Biden.txt")
		require.NoError(t, err)
		uri2, err := workspaceapi.ParseURI("file:///Joe_Biden.txt")
		require.NoError(t, err)

		content := "how bout that"
		loader.content = content

		_, err = c.Open(uri1)
		require.NoError(t, err)
		_, err = c.Open(uri2)
		require.NoError(t, err)

		var fired int
		ev := []textapi.EventType{textapi.EventTypeOpen}
		h := text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			fired++
			assert.Equal(t, content, ev.Content)
			return false
		})
		c.SubscribeEvents(ev, h)

		assert.Equal(t, 2, fired)
	})

	t.Run("EventTypeUnfocus is dispatched before EventTypeFocus on content update", func(t *testing.T) {
		c, _ := newTestComponent(t, NopEditor())
		uri1, err := workspaceapi.ParseURI("file:///Jill_Biden.txt")
		require.NoError(t, err)
		uri2, err := workspaceapi.ParseURI("file:///Joe_Biden.txt")
		require.NoError(t, err)

		a, err := c.Open(uri1)
		require.NoError(t, err)

		b, err := c.Open(uri2)
		require.NoError(t, err)

		win, err := c.Focus()
		require.NoError(t, err)

		require.NoError(t, win.SetContent(a))

		var i int
		evs := []textapi.EventType{textapi.EventTypeFocus, textapi.EventTypeUnfocus}
		h := text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			if i == 1 {
				assert.Equal(t, textapi.EventTypeUnfocus, ev.Type)
			} else {
				assert.Equal(t, textapi.EventTypeFocus, ev.Type)
			}
			i++
			return false
		})
		c.SubscribeEvents(evs, h)

		// upon SubscribeEvents, we dispatch first Focus
		assert.Equal(t, 1, i)
		require.NoError(t, win.SetContent(b))
		assert.Equal(t, 3, i)
	})
}

func expectEvent(
	t *testing.T, mock *MockEventHandler, uri workspaceapi.URI, tpe textapi.EventType,
) {
	mock.EXPECT().Handle(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, ev textapi.Event) bool {
			if tpe == ev.Type {
				assert.Equal(t, uri.String(), ev.URI.String())
			}
			return false
		})
}

func TestEventTypeFocusIntegration(t *testing.T) {
	// TODO for some reason when upgrading o mock 1.6.0
	// and testify v1.8.1 this started failing
	t.Skip()

	ctrl := gomock.NewController(t)
	c, _ := newTestComponent(t, NopEditor())
	mock := NewMockEventHandler(ctrl)
	c.SubscribeEvents([]textapi.EventType{textapi.EventTypeFocus, textapi.EventTypeUnfocus}, mock)

	uri1, err := workspaceapi.ParseURI("file:///Elon.txt")
	require.NoError(t, err)

	uri2, err := workspaceapi.ParseURI("file:///Jeffrey.txt")
	require.NoError(t, err)

	h1, err := c.OpenFileTab(uri1, false)
	require.NoError(t, err)

	h2, err := c.OpenFileTab(uri2, false)
	require.NoError(t, err)

	w1, err := c.Focus()
	require.NoError(t, err)

	// sut
	expectEvent(t, mock, uri2, textapi.EventTypeUnfocus)
	expectEvent(t, mock, uri1, textapi.EventTypeFocus)
	require.NoError(t, c.Browser().Focus().SetContent(h1))

	expectEvent(t, mock, uri1, textapi.EventTypeUnfocus)
	expectEvent(t, mock, uri2, textapi.EventTypeFocus)
	require.NoError(t, c.Browser().Focus().SetContent(h2))

	expectEvent(t, mock, uri2, textapi.EventTypeUnfocus)
	expectEvent(t, mock, uri1, textapi.EventTypeFocus)
	focus, err := c.Focus()
	require.NoError(t, err)
	w2, err := c.Split(browserapi.OrientationRight, focus, h1)
	require.NoError(t, err)

	expectEvent(t, mock, uri1, textapi.EventTypeUnfocus)
	expectEvent(t, mock, uri2, textapi.EventTypeFocus)
	_, err = c.SetFocus(w1)
	require.NoError(t, err)

	expectEvent(t, mock, uri2, textapi.EventTypeUnfocus)
	expectEvent(t, mock, uri1, textapi.EventTypeFocus)
	_, err = c.SetFocus(w2)
	require.NoError(t, err)

	ok := c.Browser().NextTab(w2)
	require.False(t, ok)

	expectEvent(t, mock, uri1, textapi.EventTypeUnfocus)
	expectEvent(t, mock, uri2, textapi.EventTypeFocus)
	ok = c.Browser().ShiftFocus()
	require.True(t, ok)

	expectEvent(t, mock, uri2, textapi.EventTypeUnfocus)
	expectEvent(t, mock, uri1, textapi.EventTypeFocus)
	ok = c.Browser().ShiftFocus()
	require.True(t, ok)

	expectEvent(t, mock, uri1, textapi.EventTypeUnfocus)
	expectEvent(t, mock, uri2, textapi.EventTypeFocus)
	require.NoError(t, w2.Close())

	ok = c.Browser().NextTab(w2)
	require.False(t, ok)

	expectEvent(t, mock, uri2, textapi.EventTypeUnfocus)
	expectEvent(t, mock, uri1, textapi.EventTypeFocus)
	ok = c.Browser().NextTab(w1)
	require.True(t, ok)
}

func TestDispatchCommand(t *testing.T) {
	uri, err := workspaceapi.ParseURI("file:///MacMecMic")
	require.NoError(t, err)

	t.Run("returns false if there's no registered handler", func(t *testing.T) {
		c, _ := newTestComponent(t, NopEditor())

		win, _ := c.Focus()
		cmd := textapi.Command{
			Resource: NewTestHandler(),
			URI:      uri,
			Name:     "SELL",
			Window:   win,
		}
		ok, err := c.DispatchCommand(cmd)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("uses aliases from config to dispatch", func(t *testing.T) {
		config := text.DefaultConfig()
		config.CommandAliases = map[string][]string{
			"workstation_layout": []string{
				"newWindow",
				"edit /tmp/todo.md",
			},
		}
		c, _ := newTestComponentConfig(t, NopEditor(), config)
		win, _ := c.Focus()

		var newWindowCalled, editCalled bool

		c.SubscribeCommand("newWindow", text.FuncCommandHandler(func(ctx context.Context, cmd textapi.Command) (bool, error) {
			newWindowCalled = true
			assert.Equal(t, cmd.Name, "newWindow")
			assert.Equal(t, cmd.Args, []string{})
			return false, nil
		}))

		c.SubscribeCommand("edit", text.FuncCommandHandler(func(ctx context.Context, cmd textapi.Command) (bool, error) {
			editCalled = true
			assert.Equal(t, cmd.Name, "edit")
			assert.Equal(t, cmd.Args, []string{"/tmp/todo.md"})
			return false, nil
		}))

		cmd := textapi.Command{
			Resource: NewTestHandler(),
			URI:      uri,
			Name:     "workstation_layout",
			Window:   win,
		}
		ok, err := c.DispatchCommand(cmd)
		assert.True(t, ok)
		require.NoError(t, err)
		assert.True(t, newWindowCalled)
		assert.True(t, editCalled)
	})

	t.Run("bubbles up HandleCommand errors", func(t *testing.T) {
		c, _ := newTestComponent(t, NopEditor())
		win, _ := c.Focus()
		c.SubscribeCommand("bla", text.FuncCommandHandler(func(ctx context.Context, cmd textapi.Command) (bool, error) {
			return false, errors.New("boom")
		}))

		cmd := textapi.Command{
			Resource: NewTestHandler(),
			URI:      uri,
			Name:     "bla",
			Window:   win,
		}
		ok, err := c.DispatchCommand(cmd)
		assert.True(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})

	t.Run("handles bad aliases", func(t *testing.T) {
		config := text.DefaultConfig()
		config.CommandAliases = map[string][]string{
			"bad1": {""},
			"bad2": {},
			"bad3": nil,
		}
		c, _ := newTestComponentConfig(t, NopEditor(), config)
		win, _ := c.Focus()

		for _, cmd := range []string{"bad1", "bad2", "bad3"} {
			cmd := textapi.Command{
				Resource: NewTestHandler(),
				URI:      uri,
				Name:     cmd,
				Window:   win,
			}
			ok, err := c.DispatchCommand(cmd)
			assert.False(t, ok)
			require.NoError(t, err)
		}
	})

	t.Run("handles aliases missing from config", func(t *testing.T) {
		config := text.DefaultConfig()
		config.CommandAliases = nil
		c, _ := newTestComponentConfig(t, NopEditor(), config)
		win, _ := c.Focus()

		cmd := textapi.Command{
			Resource: NewTestHandler(),
			URI:      uri,
			Name:     "kaboom",
			Window:   win,
		}
		ok, err := c.DispatchCommand(cmd)
		assert.False(t, ok)
		require.NoError(t, err)
	})

	t.Run("NewComponent returns error if aliases is recursive", func(t *testing.T) {
		cfg := text.DefaultConfig()
		cfg.CommandAliases = map[string][]string{
			"blah": {"bleh"},
			"bleh": {"blah"},
		}
		_, err := text.NewComponent(NopEditor(), &testLoader{}, cfg)
		require.Error(t, err)
	})
}
func TestCompleteCommand(t *testing.T) {
	t.Run("returns empty iterator if there's no registered handler", func(t *testing.T) {
		c, _ := newTestComponent(t, NopEditor())

		it, _, err := c.CompleteCommand(context.Background(), "blabla")
		require.NoError(t, err)
		assertIteratorLen(t, 0, it)
	})

	t.Run("returns empty iterator if attempting to complete alias", func(t *testing.T) {
		cfg := text.DefaultConfig()
		cfg.CommandAliases = map[string][]string{
			"workstation_layout": {
				"newWindow",
				"edit /tmp/todo.md",
			},
		}
		c, _ := newTestComponentConfig(t, NopEditor(), cfg)

		it, _, err := c.CompleteCommand(context.Background(), "workstation_layout")
		require.NoError(t, err)
		assertIteratorLen(t, 0, it)
	})

	t.Run("calls command handler Complete", func(t *testing.T) {
		c, _ := newTestComponent(t, NopEditor())

		c.SubscribeCommand("edit", text.FuncCommandCompleter(func(ctx context.Context, cmd textapi.Command) (bool, error) {
			return false, nil
		}, func(ctx context.Context, args []string) (iterator.Iterator[string], string, error) {
			assert.Equal(t, []string{"letter", "number"}, args)
			return iterator.FromSlice([]string{"one", "two"}), "2", nil
		}))

		it, newLastArg, err := c.CompleteCommand(context.Background(), "edit", "letter", "number")
		require.NoError(t, err)
		options := assertIteratorLen(t, 2, it)
		assert.Equal(t, []string{"one", "two"}, options)
		assert.Equal(t, "2", newLastArg)
	})
}

func assertIteratorLen(t *testing.T, n int, it iterator.Iterator[string]) []string {
	var ret []string
	var i int
	for ; ; i++ {
		next, ok := it.Next()
		if !ok {
			break
		}
		ret = append(ret, next)
	}
	assert.Equal(t, n, i)
	return ret
}

func TestComponentEditor(t *testing.T) {
	t.Run("returns tab with name as Handler", func(t *testing.T) {
		myName := "file:///tmp/Ennio_Morricone.go"
		c, _, h1, uri := newTestComponentWithFile(t, myName)

		h2, err := c.Editor(uri)
		assert.NoError(t, err)
		assert.Equal(t, h1.(*browser.Tab).Handler(), h2)
	})

	t.Run("returns error if no handler is found with name", func(t *testing.T) {
		c, _ := newTestComponent(t, NopEditor())

		h, err := c.Editor(workspaceapi.URI{})
		assert.Error(t, err)
		assert.Nil(t, h)
	})
}

func TestComponentCommands(t *testing.T) {
	t.Run("returns empty slice if no commands have been registered", func(t *testing.T) {
		c, _ := newTestComponent(t, NopEditor())
		assert.Len(t, c.Commands(), 0)
	})

	t.Run("returns registered commands", func(t *testing.T) {
		c, _ := newTestComponent(t, NopEditor())
		c.SubscribeCommand("myCmd", text.FuncCommandHandler(func(context.Context, textapi.Command) (bool, error) {
			return true, nil
		}))
		cmds := c.Commands()
		require.Len(t, cmds, 1)
		assert.Equal(t, "myCmd", cmds[0])
	})
	t.Run("returns configured aliases", func(t *testing.T) {
		cfg := text.DefaultConfig()
		cfg.CommandAliases = map[string][]string{
			"blah": []string{},
		}
		c, err := text.NewComponent(NopEditor(), &testLoader{}, cfg)
		require.NoError(t, err)
		cmds := c.Commands()
		require.Len(t, cmds, 1)
		assert.Equal(t, "blah", cmds[0])
	})
}

func testRegister(t *testing.T,
	constructor func(ed text.Editor, mu *sync.Mutex, resource workspaceapi.URI) (*text.Component, text.Editor, error)) {
	t.Run("Registered handler is unsubscribed upon returning exit=true", func(t *testing.T) {
		var mu sync.Mutex
		resource1, err := workspaceapi.ParseURI("file:///HERS")
		require.NoError(t, err)
		myArgs := []string{"a", "bbbbbbbbbbbbbbbbbbbbb"}
		myCmd := "BUY"
		c, sut, err := constructor(NopEditor(), &mu, resource1)
		require.NoError(t, err)

		mu.Lock()
		win, _ := c.Focus()
		h1, err := c.Edit(resource1, cell.NewBuffer())
		mu.Unlock()
		require.NoError(t, err)

		var called int
		var wg sync.WaitGroup
		sut.SubscribeCommand(myCmd, text.FuncCommandHandler(func(ctx context.Context, cmd textapi.Command) (bool, error) {
			defer wg.Done()
			assert.Equal(t, myCmd, cmd.Name)
			assert.Equal(t, myArgs, cmd.Args)
			called++
			return true, nil
		}))

		wg.Add(1)
		mu.Lock()
		cmd := textapi.Command{Resource: h1, URI: resource1, Name: myCmd, Args: myArgs,
			Window: win}
		ok, err := c.DispatchCommand(cmd)
		assert.True(t, ok)
		require.NoError(t, err)
		mu.Unlock()

		wg.Wait()

		for i := 0; i < 20; i++ {
			mu.Lock()
			c.DispatchCommand(cmd)
			mu.Unlock()
		}

		assert.Equal(t, 1, called)
	})
}

func TestComponentRegister(t *testing.T) {
	testRegister(t, func(ed text.Editor, mu *sync.Mutex, res workspaceapi.URI) (*text.Component, text.Editor, error) {
		c, _, err := newTestComponentErr(ed, text.DefaultConfig())
		return c, c, err
	})
}
func TestUnregisterCommand(t *testing.T) {
	resource1, err := workspaceapi.ParseURI("file:///HERS")
	require.NoError(t, err)
	myArgs := []string{"a", "bbbbbbbbbbbbbbbbbbbbb"}
	myCmd := "BUY"
	c, err := text.NewComponent(NopEditor(), &testLoader{}, text.DefaultConfig())
	require.NoError(t, err)

	win, _ := c.Focus()
	h1, err := c.Edit(resource1, cell.NewBuffer())
	require.NoError(t, err)

	var called int
	c.SubscribeCommand(myCmd, text.FuncCommandHandler(func(ctx context.Context, cmd textapi.Command) (bool, error) {
		called++
		return false, nil
	}))

	cmd := textapi.Command{
		Resource: h1,
		URI:      resource1,
		Name:     myCmd,
		Args:     myArgs,
		Window:   win,
	}
	ok, err := c.DispatchCommand(cmd)
	assert.True(t, ok)
	require.NoError(t, err)

	assert.Equal(t, 1, called)

	err = c.UnsubscribeCommand(myCmd)
	require.NoError(t, err)

	ok, err = c.DispatchCommand(cmd)
	assert.False(t, ok)
	require.NoError(t, err)
	assert.Equal(t, 1, called)
}

func TestTabIntegration(t *testing.T) {
	testTabIntegration(t, func(ed text.Editor, mu *sync.Mutex) (*text.Component, browser.WindowManager, error) {
		c, _, err := newTestComponentErr(ed, text.DefaultConfig())
		return c, c, err
	})
}

func testTabIntegration(t *testing.T,
	constructor func(ed text.Editor, mu *sync.Mutex) (*text.Component, browser.WindowManager, error)) {
	t.Run("switches to a tab upon call to SetContent", func(t *testing.T) {
		cases := []testutil.HandlerSequenceTestCase{
			{"",
				`┌──────────────────┐
│$$  ##            │
├──────────────────┤
│##################│
│##################│
│##################│
│##################│
│##################│
│##################│
└──────────────────┘`},
		}

		fn := func(t *testing.T) tui.Handler {
			var mu sync.Mutex
			c, wm, err := constructor(NopEditor(), &mu)
			require.NoError(t, err)

			resource1, err := workspaceapi.ParseURI("file:///a")
			require.NoError(t, err)
			resource2, err := workspaceapi.ParseURI("file:///b")
			require.NoError(t, err)
			b1 := browsertest.NewTestHandler()
			b1.Ch = '$'
			_, err = wm.Tab(resource1, "$$", b1)
			require.NoError(t, err)

			b2 := browsertest.NewTestHandler()
			b2.Ch = '#'
			t2, err := wm.Tab(resource2, "##", b2)
			require.NoError(t, err)

			win, err := wm.Focus()
			require.NoError(t, err)

			require.NoError(t, win.SetContent(t2))
			return handler.Sync(&mu, handler.Nop(c))
		}
		testutil.TestHandlerIsolated(t, fn, 20, 10, cases)
	})
}

func TestFlush(t *testing.T) {
	resource1, err := workspaceapi.ParseURI("file:///a")
	require.NoError(t, err)

	t.Run("calls underlying closer Flush", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mock := NewMockHandler(ctrl)
		mockEditor := NewMockEditor(ctrl)
		mockWorkspace := workspacetest.NewMockLoader(ctrl)
		mockFlusherCloser := workspacetest.NewMockFlusherCloser(ctrl)

		c, err := text.NewComponent(mockEditor, mockWorkspace, text.DefaultConfig())
		require.NoError(t, err)

		win, err := c.Focus()
		require.NoError(t, err)

		mockWorkspace.EXPECT().Load(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(mockFlusherCloser, nil).Times(1)
		mock.EXPECT().Resize(gomock.Any(), gomock.Any()).Times(1)
		mockEditor.EXPECT().CellView(gomock.Any()).
			Return(text.NewCellView(cell.NewBuffer().View())).Times(1)
		mockEditor.EXPECT().Edit(gomock.Any(), gomock.Any()).Return(mock, nil)

		h, err := c.OpenFileTab(resource1, true)
		require.NoError(t, win.SetContent(h))

		mockFlusherCloser.EXPECT().Flush().Times(1)
		require.NoError(t, c.Flush(win))
	})

	t.Run("returns ErrInvalidSave if called on tab with nil closer handle", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mock := NewMockHandler(ctrl)
		c, _ := newTestComponent(t, nil)
		win, err := c.Focus()
		require.NoError(t, err)

		mock.EXPECT().Resize(gomock.Any(), gomock.Any()).Times(1)
		h, err := c.Tab(resource1, "Rupi Kaur", mock)
		require.NoError(t, win.SetContent(h))

		require.Equal(t, textapi.ErrInvalidSave, c.Flush(win))
	})

	t.Run("returns ErrInvalidSave if called on non-tab", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mock := NewMockHandler(ctrl)
		c, _ := newTestComponent(t, nil)
		win, err := c.Focus()
		require.NoError(t, err)

		mock.EXPECT().Resize(gomock.Any(), gomock.Any()).AnyTimes()
		_, err = c.Split(browserapi.OrientationTop, win, mock)
		require.NoError(t, err)

		require.Equal(t, textapi.ErrInvalidSave, c.Flush(win))
	})
}

// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package textrpc

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/tcell/v3"
	gomock "go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/browserapi"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/browser/browserrpc"
	"unstable.build/go-tui/browser/browsertest"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/handler/handlertest"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/texttest"
	"unstable.build/go-tui/workspace"
)

func doSetupIntTest(
	t *testing.T, broker rpc.MuxBroker, register func(*grpc.Server),
) (conn *grpc.ClientConn, closeFn func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	register(grpcServer)

	go grpcServer.Serve(lis)

	conn, err = grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	closeFn = func() {
		grpcServer.Stop()
		lis.Close()
	}
	return
}

func setupIntTest(
	t *testing.T, broker rpc.MuxBroker, s *Server,
) (*Client, func()) {
	conn, closeFn := doSetupIntTest(t, broker, func(grpcServer *grpc.Server) {
		RegisterEditorServer(grpcServer, s)
	})
	client := NewClient(context.Background(), broker, conn)
	return client, func() {
		client.Close()
		closeFn()
	}
}

func setupWmIntTest(
	t *testing.T, broker rpc.MuxBroker, s *browserrpc.Server,
) (*browserrpc.Client, func()) {
	conn, closeFn := doSetupIntTest(t, broker, func(grpcServer *grpc.Server) {
		browserrpc.RegisterWindowManagerServer(grpcServer, s)
	})
	client := browserrpc.NewClient(context.Background(), broker, conn)
	return client, func() {
		client.Close()
		closeFn()
	}
}

func TestClientServerIntegration(t *testing.T) {
	uri, err := workspaceapi.ParseURI("file:///test")
	require.NoError(t, err)
	t.Run("client through server calls underlying editor Edit", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := rpc.NewUnixGRPCBroker("", "", "")
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, nopLocker{})

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		expectEdit(t, ed, uri, "hero")
		buf := cell.NewBuffer()
		buf.WriteString("hero")

		_, err := client.Edit(uri, buf)
		require.NoError(t, err)
	})

	t.Run("client through server calls underlying Editor", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := rpc.NewUnixGRPCBroker("", "", "")
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, nopLocker{})

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		ed.EXPECT().Editor(gomock.Any()).Times(1).
			DoAndReturn(func(_uri workspaceapi.URI) (tui.Handler, error) {
				assert.Equal(t, _uri, uri)
				return handler.NewTestHandler(), nil
			})
		_, err := client.Editor(uri)
		require.NoError(t, err)
	})

	t.Run("underlying editor Edito errors bubble up to client", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := rpc.NewUnixGRPCBroker("", "", "")
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, nopLocker{})

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		ed.EXPECT().Edit(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("The Upsetter")).
			Times(1)

		_, err := client.Edit(uri, cell.NewBuffer())
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "The Upsetter"))
	})

	t.Run("client through server calls underlying editor Subscribe", func(t *testing.T) {
		str1 := "Granola Lola"
		tsuite := []struct {
			name       string
			evType     textapi.EventType
			trigger    func(t *testing.T, resourceName string, ed text.Editor, buf *cell.Buffer)
			start, end *term.Coordinates
			content    *string
		}{
			{
				"Edit->EventTypeOpen",
				textapi.EventTypeOpen,
				func(t *testing.T, resourceName string, ed text.Editor, buf *cell.Buffer) {
					ed.Edit(uri, buf)
				}, nil, nil, nil,
			},
			{
				"Edit->EventTypeEdit",
				textapi.EventTypeEdit,
				func(t *testing.T, resourceName string, ed text.Editor, buf *cell.Buffer) {
					ed.Edit(uri, buf)
					buf.WriteString(str1)
				}, &term.Coordinates{}, &term.Coordinates{}, &str1,
			},
			{
				"Edit->EventTypeEdit",
				textapi.EventTypeEdit,
				func(t *testing.T, resourceName string, ed text.Editor, buf *cell.Buffer) {
					buf.WriteString(str1)
					ed.Edit(uri, buf)
					buf.DeleteRow(0)
				}, &term.Coordinates{}, &term.Coordinates{Y: 1}, nil,
			},
			{
				"Handle->EventTypeCursor",
				textapi.EventTypeCursor,
				func(t *testing.T, resourceName string, ed text.Editor, buf *cell.Buffer) {
					buf.WriteString(str1)
					h, err := ed.Edit(uri, buf)
					assert.NoError(t, err)
					h.Handle(term.Event{Ch: 'l'})
				}, &term.Coordinates{}, &term.Coordinates{}, nil,
			},
			{
				"Handle->EventTypeSelection",
				textapi.EventTypeSelection,
				func(t *testing.T, resourceName string, ed text.Editor, buf *cell.Buffer) {
					buf.WriteString(str1)
					h, err := ed.Edit(uri, buf)
					assert.NoError(t, err)
					h.Handle(term.Event{Ch: 'v'})
				}, &term.Coordinates{}, &term.Coordinates{}, nil,
			},
		}

		for i, _tcase := range tsuite {
			tcase := _tcase
			t.Run(tcase.name, func(t *testing.T) {
				var wg sync.WaitGroup
				b := rpc.NewUnixGRPCBroker("", "", "")
				ed := texttest.NopEditorWithCallback(wg.Done)
				s := NewServer(b, ed, new(sync.Mutex))

				client, closeFn := setupIntTest(t, b, s)
				defer closeFn()

				wg.Add(1)
				err := client.SubscribeEvents([]textapi.EventType{tcase.evType},
					text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
						defer wg.Done()
						if tcase.start != nil {
							assert.Equal(t, *tcase.start, ev.Start)
						}
						if tcase.end != nil {
							assert.Equal(t, *tcase.end, ev.End)
						}
						if tcase.content != nil {
							assert.Equal(t, *tcase.content, ev.Content)
						}
						return false
					}))
				require.NoError(t, err)

				// wait for subscribe callback
				wg.Wait()

				// proceed to trigger
				wg.Add(1)

				buf := cell.NewBuffer()
				tcase.trigger(t, strconv.Itoa(i), ed, buf)
				wg.Wait()

				s.editor.Lock()
				defer s.editor.Unlock()

				assert.NoError(t, s.Close())
			})
		}
	})

	t.Run("calls unsubscribe if event stream completes", func(t *testing.T) {
		var wg sync.WaitGroup
		b := rpc.NewUnixGRPCBroker("", "", "")
		ed := texttest.NopEditorWithCallback(wg.Done)
		s := NewServer(b, ed, new(sync.Mutex))

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		wg.Add(1)
		err := client.SubscribeEvents([]textapi.EventType{textapi.EventTypeOpen},
			text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
				defer wg.Done()
				return true
			}))
		require.NoError(t, err)

		// wait for subscribe callback
		wg.Wait()

		// proceed to trigger
		ed.Edit(uri, cell.NewBuffer())

		// wg panics if Done called but not added
		wg.Wait()

		// close
		s.editor.Lock()
		defer s.editor.Unlock()

		assert.NoError(t, s.Close())
	})

	t.Run("event handler drops messages if event handler server is not processing events", func(t *testing.T) {
		var wg sync.WaitGroup
		b := rpc.NewUnixGRPCBroker("", "", "")
		ed := texttest.NopEditorWithCallback(wg.Done)
		s := NewServer(b, ed, new(sync.Mutex))

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		var mu sync.Mutex
		evs := make(map[textapi.EventType]textapi.Event)

		wg.Add(1)
		err := client.SubscribeEvents([]textapi.EventType{
			textapi.EventTypeOpen,
			textapi.EventTypeClose,
			textapi.EventTypeFlush,
			textapi.EventTypeEdit,
			textapi.EventTypeScroll,
			textapi.EventTypeFocus,
			textapi.EventTypeUnfocus,
			textapi.EventTypeCursor,
			textapi.EventTypeSelection,
		}, text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			mu.Lock()
			defer mu.Unlock()
			evs[ev.Type] = ev
			return false
		}))
		require.NoError(t, err)

		// wait for subscribe callback, which also uses wg
		wg.Wait()

		subs := ed.Subscribers()
		require.Len(t, subs, 9)
		require.Len(t, subs[textapi.EventTypeOpen], 1)
		handler := subs[textapi.EventTypeOpen][0]

		// proceed to trigger, should not deadlock
		mu.Lock() // try to deadlock
		for i := 0; i < 10000; i++ {
			tpe := textapi.EventType(i % 9)
			handler.Handle(context.Background(), textapi.Event{URI: uri, Type: tpe})
		}

		mu.Unlock()
		// there's no deterministic way to know how many msgs will
		// be buffered by the underlying transport, so this is the only way
		time.Sleep(5 * time.Second)
		mu.Lock()

		require.Len(t, evs, 9)
		s.editor.Lock()
		defer s.editor.Unlock()

		assert.NoError(t, s.Close())
	})

	t.Run("SetLocationList sets the location list of the remote editor", func(t *testing.T) {
		var wg sync.WaitGroup
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := rpc.NewUnixGRPCBroker("", "", "")
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, new(sync.Mutex))

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		expectEdit(t, ed, uri, "")
		h, err := client.Edit(uri, cell.NewBuffer())
		require.NoError(t, err)

		l := text.LocationSlice([]textapi.Location{loc2})

		expectEditor(t, ed, uri)
		ed.EXPECT().SetLocationList(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(h text.Handler, pri textapi.LocationPriority, id string, ll text.LocationList) error {
				defer wg.Done()
				assertLocation(t, ll, 0, loc2)
				assert.Equal(t, locID, id)
				assertLocationListLen(t, ll, 1)
				assert.Equal(t, textapi.LocationPriorityError, pri)
				return nil
			}).Times(1)

		wg.Add(1)
		err = client.SetLocationList(h, textapi.LocationPriorityError, locID, l)
		require.NoError(t, err)

		wg.Wait()
	})

	t.Run("SetDefaultAttributes sets the default attrs of the remote editor", func(t *testing.T) {
		var wg sync.WaitGroup
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := rpc.NewUnixGRPCBroker("", "", "")
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, new(sync.Mutex))

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		expectEdit(t, ed, uri, "")
		h, err := client.Edit(uri, cell.NewBuffer())
		require.NoError(t, err)

		expectedAttrs := term.Attributes{
			Attrs: tcell.AttrUnderline | tcell.AttrBold,
			Fg:    tcell.ColorWhite,
			Bg:    tcell.ColorNavy,
		}

		expectEditor(t, ed, uri)
		ed.EXPECT().SetDefaultAttributes(gomock.Any(), gomock.Any()).
			DoAndReturn(func(h text.Handler, attrs term.Attributes) error {
				defer wg.Done()
				assert.Equal(t, expectedAttrs, attrs)
				return nil
			}).Times(1)

		wg.Add(1)
		err = client.SetDefaultAttributes(h, expectedAttrs)
		require.NoError(t, err)

		wg.Wait()
	})

	t.Run("Writer returns a Writer that is able to modify underlying buffer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := rpc.NewUnixGRPCBroker("", "", "")
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, nopLocker{})

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		buf := cell.NewBuffer()
		expectEdit(t, ed, uri, "")
		h, err := client.Edit(uri, buf)
		require.NoError(t, err)

		w := client.CellEditor(h)
		at := term.Coordinates{X: 1}

		expectEditor(t, ed, uri)
		ed.EXPECT().CellEditor(gomock.Any()).Return(text.NewCellEditor(buf.Editor())).Times(1)
		from, to, _, err := w.Edit(context.Background(), at, at, "el\nAridio")

		require.NoError(t, err)
		require.Equal(t, term.Coordinates{}, from)
		require.Equal(t, term.Coordinates{X: 6, Y: 1}, to)
		require.Equal(t, " el\nAridio", buf.String())

		expectEditor(t, ed, uri)
		ed.EXPECT().CellEditor(gomock.Any()).Return(text.NewCellEditor(buf.Editor())).Times(1)
		start, end, str, err := w.Edit(
			context.Background(), term.Coordinates{}, term.Coordinates{Y: 1}, "")

		require.NoError(t, err)
		assert.Equal(t, term.Coordinates{}, start)
		assert.Equal(t, term.Coordinates{}, end)
		assert.Equal(t, " el\n", str)
		assert.Equal(t, "Aridio", buf.String())
	})

	t.Run("Reader returns a Reader that is able to read underlying buffer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := rpc.NewUnixGRPCBroker("", "", "")
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, nopLocker{})

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		buf := cell.NewBuffer()
		buf.WriteString("guacamole")
		expectEdit(t, ed, uri, "guacamole")
		h, err := client.Edit(uri, buf)
		require.NoError(t, err)

		expectEditor(t, ed, uri)
		ed.EXPECT().CellView(gomock.Any()).Return(text.NewCellView(buf.View())).Times(2)

		r := client.CellView(h)
		cells, err := r.RawCells()
		require.NoError(t, err)
		assert.Equal(t, "guacamole", cell.CellsToString(cells))

		buf.WriteString("\npollos hermanos")

		cells, err = r.RawCells()
		require.NoError(t, err)
		assert.Equal(t, "guacamole\npollos hermanos", cell.CellsToString(cells))
	})
}

func TestRPCTab(t *testing.T) {
	testTabIntegration(t, func(ed text.Editor, mu *sync.Mutex) (*text.Component, browserapi.WindowManager, error) {
		c, err := newTestComponentErr(ed)
		if err != nil {
			return nil, nil, err
		}

		b := rpc.NewUnixGRPCBroker("", "", "")
		s := browserrpc.NewServer(b, c, mu)
		s.SetSyncMode()

		client, closeFn := setupWmIntTest(t, b, s)
		t.Cleanup(func() {
			mu.Lock()
			s.Stop()
			mu.Unlock()
			closeFn()
		})

		return c, client, err
	})
}

func TestRPCRegister(t *testing.T) {
	testRegister(t, func(ed text.Editor, mu *sync.Mutex, res workspaceapi.URI) (*text.Component, text.Editor, error) {
		c, err := newTestComponentErr(ed)
		if err != nil {
			return nil, nil, err
		}

		b := rpc.NewUnixGRPCBroker("", "", "")
		s := NewServer(b, c, mu)

		client, closeFn := setupIntTest(t, b, s)
		t.Cleanup(func() {
			mu.Lock()
			s.Close()
			mu.Unlock()
			closeFn()
		})

		return c, texttest.EditorFromAPIEditor{Ed: client}, err
	})
}

func TestRPCComplete(t *testing.T) {
}

func assertLocation(t *testing.T, l text.LocationList, idx int, loca textapi.Location) {
	resetLocationList(l)
	var i int
	for loc, ok := l.Current(); ok; loc, ok = l.Next() {
		if idx == i {
			assert.Equal(t, loca, loc)
			return
		}
		i++
	}
}
func assertLocationListLen(t *testing.T, l text.LocationList, length int) {
	resetLocationList(l)
	var i int
	for _, ok := l.Current(); ok; _, ok = l.Next() {
		i++
	}
	assert.Equal(t, length, i)
}

func resetLocationList(l text.LocationList) {
	for {
		_, ok := l.Prev()
		if !ok {
			break
		}
	}
}

func testTabIntegration(t *testing.T,
	constructor func(ed text.Editor, mu *sync.Mutex) (*text.Component, browserapi.WindowManager, error)) {
	t.Run("switches to a tab upon call to SetContent", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{"",
				`┌──────────────────┐
│x $$  x ##        │
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
			c, wm, err := constructor(texttest.NopEditor(), &mu)
			require.NoError(t, err)

			resource1, err := workspaceapi.ParseURI("file:///a")
			require.NoError(t, err)
			resource2, err := workspaceapi.ParseURI("file:///b")
			require.NoError(t, err)
			b1 := browsertest.NewTestHandler()
			b1.Ch = '$'
			_, err = wm.Tab(resource1, 'x', "$$", b1)
			require.NoError(t, err)

			b2 := browsertest.NewTestHandler()
			b2.Ch = '#'
			t2, err := wm.Tab(resource2, 'x', "##", b2)
			require.NoError(t, err)

			win, err := wm.Focus()
			require.NoError(t, err)

			require.NoError(t, wm.SetWindowContent(win, t2))
			return handler.Sync(&mu, handler.Nop(c.Browser()))
		}
		handlertest.TestHandlerIsolated(t, fn, 20, 10, cases)
	})
}

type testLoader struct {
	content       string
	flusherCloser *testFlusherCloser
	expectError   error
}

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

func (t *testLoader) Remove(string) error {
	return nil
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

func (t *testLoader) Open(path string, flag int, perm os.FileMode) (
	workspaceapi.File, *workspaceapi.Error,
) {
	panic("unused")
}

func (t *testLoader) Stat(path string) (os.FileInfo, error) {
	panic("unused")
}

func (t *testLoader) ReadDir(name string) ([]os.DirEntry, error) {
	panic("unused")
}

func newTestComponentErr(ed text.Editor) (*text.Component, error) {
	cfg := text.DefaultConfig()
	c, err := text.NewComponent(ed, document.NewInMemoryService(), &testLoader{}, cfg)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func testRegister(t *testing.T,
	constructor func(ed text.Editor, mu *sync.Mutex, resource workspaceapi.URI) (*text.Component, text.Editor, error)) {
	t.Run("Subscribed handler is called", func(t *testing.T) {
		var mu sync.Mutex
		resource1, err := workspaceapi.ParseURI("file:///HERS")
		require.NoError(t, err)
		myArgs := []string{"a", "bbbbbbbbbbbbbbbbbbbbb"}
		myCmd := textapi.CommandManual{
			Name:     "BUY",
			Synopsis: "what",
			Summary:  "It's prime day!",
			Commands: []textapi.CommandManual{
				{
					Name:     "applycoupon",
					Synopsis: "howmuch",
					Summary:  "do it",
				},
			},
		}
		c, sut, err := constructor(texttest.NopEditor(), &mu, resource1)
		require.NoError(t, err)

		mu.Lock()
		h1, err := c.Edit(resource1, cell.NewBuffer())
		mu.Unlock()
		require.NoError(t, err)

		var called int
		var wg sync.WaitGroup
		sut.SubscribeCommand(myCmd,
			text.FuncCommandHandler(func(ctx context.Context, cmd textapi.Command) error {
				defer wg.Done()
				assert.Equal(t, myCmd.Name, cmd.Name)
				assert.Equal(t, myArgs, cmd.Args)
				assert.NotNil(t, cmd.Window)
				called++
				return nil
			}, nil))

		wg.Add(1)
		mu.Lock()
		win, err := c.Focus()
		require.NoError(t, err)
		cmd := textapi.Command{
			Resource: h1,
			URI:      resource1,
			Name:     myCmd.Name,
			Args:     myArgs,
			Window:   win,
		}
		ok, err := c.DispatchCommand(cmd)
		require.NoError(t, err)
		assert.True(t, ok)
		mu.Unlock()

		wg.Wait()

		assert.Equal(t, 1, called)
	})

	t.Run("Complete is called", func(t *testing.T) {
		var mu sync.Mutex
		resource1, err := workspaceapi.ParseURI("file:///HERS")
		require.NoError(t, err)
		myCmd := textapi.CommandManual{
			Name: "HODL",
		}
		c, sut, err := constructor(texttest.NopEditor(), &mu, resource1)
		require.NoError(t, err)

		sut.SubscribeCommand(myCmd,
			text.FuncCommandHandler(func(ctx context.Context, cmd textapi.Command) error {
				return nil
			}, func(ctx context.Context, cmd string, args []string) (iterator.Iterator[string], string, error) {
				assert.Equal(t, []string{"1", "2"}, args)
				return iterator.FromSlice([]string{"4EVER"}), "sub", nil
			}))

		mu.Lock()
		defer mu.Unlock()

		it, str, err := c.CompleteCommand(context.Background(), "HODL", []string{"1", "2"}...)
		require.NoError(t, err)

		sl, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)

		assert.Equal(t, []string{"4EVER"}, sl)
		clientsShouldNeverBeAllowedSubstitution := ""
		assert.Equal(t, clientsShouldNeverBeAllowedSubstitution, str)
	})
}

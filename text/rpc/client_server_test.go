package rpc

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	browserpb "unstable.build/go-tui/browser/rpc"
	browsertest "unstable.build/go-tui/browser/test"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	texttest "unstable.build/go-tui/text/test"
	testutil "unstable.build/go-tui/util/test"
	"unstable.build/go-tui/workspace"
)

func doSetupIntTest(
	t *testing.T, broker proto.MuxBroker, register func(*grpc.Server),
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
	t *testing.T, broker proto.MuxBroker, s *Server,
) (*Client, func()) {
	conn, closeFn := doSetupIntTest(t, broker, func(grpcServer *grpc.Server) {
		RegisterEditorServer(grpcServer, s)
	})
	client := NewClient(broker, conn)
	return client, func() {
		client.Close()
		closeFn()
	}
}

func setupWmIntTest(
	t *testing.T, broker proto.MuxBroker, s *browserpb.Server,
) (*browserpb.Client, func()) {
	conn, closeFn := doSetupIntTest(t, broker, func(grpcServer *grpc.Server) {
		browserpb.RegisterWindowManagerServer(grpcServer, s)
	})
	client := browserpb.NewClient(broker, conn)
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
		b := proto.NewDialBroker()
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, nopLocker{}, testBrowserServer{})

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
		b := proto.NewDialBroker()
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, nopLocker{}, testBrowserServer{})

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
		b := proto.NewDialBroker()
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, nopLocker{}, testBrowserServer{})

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
		}

		for i, _tcase := range tsuite {
			tcase := _tcase
			t.Run(tcase.name, func(t *testing.T) {
				var wg sync.WaitGroup
				var mu sync.Mutex
				b := proto.NewDialBroker()
				ed := texttest.NopEditorWithCallback(wg.Done)
				s := NewServer(b, ed, &mu, testBrowserServer{})

				client, closeFn := setupIntTest(t, b, s)
				defer closeFn()

				wg.Add(1)
				err := client.SubscribeEditorEvents([]textapi.EventType{tcase.evType},
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
				// simulate runtime mutex
				mu.Lock()
				tcase.trigger(t, strconv.Itoa(i), ed, buf)
				mu.Unlock()
				wg.Wait()

				mu.Lock()
				assert.NoError(t, s.Close())
				mu.Unlock()
			})
		}
	})

	t.Run("SetLocationList sets the location list of the remote editor", func(t *testing.T) {
		var wg sync.WaitGroup
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := proto.NewDialBroker()
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, new(sync.Mutex), testBrowserServer{})

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
		b := proto.NewDialBroker()
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, new(sync.Mutex), testBrowserServer{})

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		expectEdit(t, ed, uri, "")
		h, err := client.Edit(uri, cell.NewBuffer())
		require.NoError(t, err)

		expectedAttrs := term.Attributes{
			Fg: term.ColorWhite | term.AttrUnderline,
			Bg: term.ColorCyan | term.AttrBold,
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
		b := proto.NewDialBroker()
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, nopLocker{}, testBrowserServer{})

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
		from, to, _, err := w.Edit(at, at, "el\nAridio")

		require.NoError(t, err)
		require.Equal(t, term.Coordinates{}, from)
		require.Equal(t, term.Coordinates{X: 6, Y: 1}, to)
		require.Equal(t, " el\nAridio", buf.String())

		expectEditor(t, ed, uri)
		ed.EXPECT().CellEditor(gomock.Any()).Return(text.NewCellEditor(buf.Editor())).Times(1)
		start, end, str, err := w.Edit(term.Coordinates{}, term.Coordinates{Y: 1}, "")

		require.NoError(t, err)
		assert.Equal(t, term.Coordinates{}, start)
		assert.Equal(t, term.Coordinates{}, end)
		assert.Equal(t, " el\n", str)
		assert.Equal(t, "Aridio", buf.String())
	})

	t.Run("Reader returns a Reader that is able to read underlying buffer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := proto.NewDialBroker()
		ed := texttest.NewMockEditor(ctrl)
		s := NewServer(b, ed, nopLocker{}, testBrowserServer{})

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
	var closeFns []func()
	testTabIntegration(t, func(ed text.Editor, mu *sync.Mutex) (*text.Component, browserapi.WindowManager, error) {
		c, err := newTestComponentErr(ed)
		if err != nil {
			return nil, nil, err
		}

		b := proto.NewDialBroker()
		s := browserpb.NewServer(b, c, mu)

		client, closeFn := setupWmIntTest(t, b, s)
		closeFns = append(closeFns, func() {
			mu.Lock()
			s.Close()
			mu.Unlock()
			closeFn()
		})

		return c, client, err
	})
	for _, closeFn := range closeFns {
		closeFn()
	}
}

func TestRPCRegister(t *testing.T) {
	var closeFns []func()

	testRegister(t, func(ed text.Editor, mu *sync.Mutex, res workspaceapi.URI) (*text.Component, text.Editor, error) {
		c, err := newTestComponentErr(ed)
		if err != nil {
			return nil, nil, err
		}

		b := proto.NewDialBroker()
		s := NewServer(b, c, mu, browserpb.NewServer(b, c, mu))

		client, closeFn := setupIntTest(t, b, s)
		closeFns = append(closeFns, func() {
			mu.Lock()
			s.Close()
			mu.Unlock()
			closeFn()
		})

		return c, texttest.EditorFromAPIEditor{Ed: client}, err
	})

	for _, closeFn := range closeFns {
		closeFn()
	}
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
			c, wm, err := constructor(texttest.NopEditor(), &mu)
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
			return handler.Sync(&mu, handler.Nop(c.Browser()))
		}
		testutil.TestHandlerIsolated(t, fn, 20, 10, cases)
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

func newTestComponentErr(ed text.Editor) (*text.Component, error) {
	cfg := text.DefaultConfig()
	c, err := text.NewComponent(ed, &testLoader{}, cfg)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func testRegister(t *testing.T,
	constructor func(ed text.Editor, mu *sync.Mutex, resource workspaceapi.URI) (*text.Component, text.Editor, error)) {
	t.Run("Registered handler is unsubscribed upon returning exit=true", func(t *testing.T) {
		var mu sync.Mutex
		resource1, err := workspaceapi.ParseURI("file:///HERS")
		require.NoError(t, err)
		myArgs := []string{"a", "bbbbbbbbbbbbbbbbbbbbb"}
		myCmd := "BUY"
		c, sut, err := constructor(texttest.NopEditor(), &mu, resource1)
		require.NoError(t, err)

		mu.Lock()
		h1, err := c.Edit(resource1, cell.NewBuffer())
		mu.Unlock()
		require.NoError(t, err)

		var called int
		var wg sync.WaitGroup
		sut.SubscribeCommand(myCmd,
			text.FuncCommandHandler(func(ctx context.Context, cmd textapi.Command) (bool, error) {
				defer wg.Done()
				assert.Equal(t, myCmd, cmd.Name)
				assert.Equal(t, myArgs, cmd.Args)
				assert.NotNil(t, cmd.Window)
				called++
				return true, nil
			}))

		wg.Add(1)
		mu.Lock()
		win, err := c.Focus()
		require.NoError(t, err)
		cmd := textapi.Command{
			Resource: h1,
			URI:      resource1,
			Name:     myCmd,
			Args:     myArgs,
			Window:   browser.WindowToAPIWindow{Win: win},
		}
		ok, err := c.DispatchCommand(cmd)
		require.NoError(t, err)
		assert.True(t, ok)
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

type testBrowserServer struct {
	windowChannelID string
}

func (t testBrowserServer) ServeWindow(win browser.Window) (string, error) {
	return t.windowChannelID, nil
}

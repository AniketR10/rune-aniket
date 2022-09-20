package text

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	browserpb "github.com/ernestrc/go-tui/browser/rpc"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	textpb "github.com/ernestrc/go-tui/text/rpc"
	"github.com/ernestrc/go-tui/workspace"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
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
		textpb.RegisterEditorServer(grpcServer, s)
	})
	client := NewClient(broker, conn)
	return client, func() {
		client.Close()
		closeFn()
	}
}

func setupWmIntTest(
	t *testing.T, broker proto.MuxBroker, s *browser.Server,
) (*browser.Client, func()) {
	conn, closeFn := doSetupIntTest(t, broker, func(grpcServer *grpc.Server) {
		browserpb.RegisterWindowManagerServer(grpcServer, s)
	})
	client := browser.NewClient(broker, conn)
	return client, func() {
		client.Close()
		closeFn()
	}
}

func expectInitialServerSubscribe(t *testing.T, mock *MockEditor) {
	mock.EXPECT().SubscribeEditorEvents(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(1)
}

func TestClientServerIntegration(t *testing.T) {
	uri, err := workspace.ParseURI("file:///test")
	require.NoError(t, err)
	t.Run("client through server calls underlying editor Edit", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := proto.NewDialBroker()
		ed := NewMockEditor(ctrl)
		expectInitialServerSubscribe(t, ed)
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
		b := proto.NewDialBroker()
		ed := NewMockEditor(ctrl)
		expectInitialServerSubscribe(t, ed)
		s := NewServer(b, ed, nopLocker{})

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		ed.EXPECT().Editor(gomock.Any()).Times(1).
			DoAndReturn(func(_uri workspace.URI) (tui.Handler, error) {
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
		ed := NewMockEditor(ctrl)
		expectInitialServerSubscribe(t, ed)
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
			evType     EventType
			trigger    func(t *testing.T, resourceName string, ed Editor, buf *cell.Buffer)
			start, end *term.Coordinates
			content    *string
		}{
			{
				"Edit->EventTypeOpen",
				EventTypeOpen,
				func(t *testing.T, resourceName string, ed Editor, buf *cell.Buffer) {
					ed.Edit(uri, buf)
				}, nil, nil, nil,
			},
			{
				"Edit->EventTypeEdit",
				EventTypeEdit,
				func(t *testing.T, resourceName string, ed Editor, buf *cell.Buffer) {
					ed.Edit(uri, buf)
					buf.WriteString(str1)
				}, &term.Coordinates{}, &term.Coordinates{}, &str1,
			},
			{
				"Edit->EventTypeEdit",
				EventTypeEdit,
				func(t *testing.T, resourceName string, ed Editor, buf *cell.Buffer) {
					buf.WriteString(str1)
					ed.Edit(uri, buf)
					buf.DeleteRow(0)
				}, &term.Coordinates{}, &term.Coordinates{Y: 1}, nil,
			},
			{
				"Handle->EventTypeCursor",
				EventTypeCursor,
				func(t *testing.T, resourceName string, ed Editor, buf *cell.Buffer) {
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
				ed := &testEditor{}
				s := NewServer(b, ed, &mu)

				client, closeFn := setupIntTest(t, b, s)
				defer closeFn()

				wg.Add(1)
				err := client.SubscribeEditorEvents([]EventType{tcase.evType},
					FuncEventHandler(func(ctx context.Context, ev Event) bool {
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

				buf := cell.NewBuffer()
				// simulate runtime mutex
				mu.Lock()
				tcase.trigger(t, strconv.Itoa(i), ed, buf)
				mu.Unlock()
				wg.Wait()

				assert.NoError(t, s.Close())
			})
		}
	})

	t.Run("SetLocationList sets the location list of the remote editor", func(t *testing.T) {
		var wg sync.WaitGroup
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := proto.NewDialBroker()
		ed := NewMockEditor(ctrl)
		expectInitialServerSubscribe(t, ed)
		s := NewServer(b, ed, new(sync.Mutex))

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		expectEdit(t, ed, uri, "")
		h, err := client.Edit(uri, cell.NewBuffer())
		require.NoError(t, err)

		l := LocationSlice([]Location{loc2})

		ed.EXPECT().SetLocationList(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(h Handler, id string, ll LocationList) error {
				defer wg.Done()
				assertLocation(t, ll, 0, loc2)
				assert.Equal(t, locID, id)
				assertLocationListLen(t, ll, 1)
				return nil
			}).Times(1)

		wg.Add(1)
		err = client.SetLocationList(h, locID, l)
		require.NoError(t, err)

		wg.Wait()
	})

	t.Run("Writer returns a Writer that is able to modify underlying buffer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := proto.NewDialBroker()
		ed := NewMockEditor(ctrl)
		expectInitialServerSubscribe(t, ed)
		s := NewServer(b, ed, nopLocker{})

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		buf := cell.NewBuffer()
		expectEdit(t, ed, uri, "")
		h, err := client.Edit(uri, buf)
		require.NoError(t, err)

		w := client.CellEditor(h)
		at := term.Coordinates{X: 1}

		ed.EXPECT().CellEditor(gomock.Any()).Return(NewCellEditor(buf.Editor())).Times(1)
		from, to, _, err := w.Edit(at, at, "el\nAridio")

		require.NoError(t, err)
		require.Equal(t, term.Coordinates{}, from)
		require.Equal(t, term.Coordinates{X: 6, Y: 1}, to)
		require.Equal(t, " el\nAridio", buf.String())

		ed.EXPECT().CellEditor(gomock.Any()).Return(NewCellEditor(buf.Editor())).Times(1)
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
		ed := NewMockEditor(ctrl)
		expectInitialServerSubscribe(t, ed)
		s := NewServer(b, ed, nopLocker{})

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		buf := cell.NewBuffer()
		buf.WriteString("guacamole")
		expectEdit(t, ed, uri, "guacamole")
		h, err := client.Edit(uri, buf)
		require.NoError(t, err)

		ed.EXPECT().CellView(gomock.Any()).Return(NewCellView(buf.View())).Times(2)

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
	testTabIntegration(t, func(ed Editor, mu *sync.Mutex) (*Component, browser.WindowManager, error) {
		c, err := newTestComponentErr(ed)
		if err != nil {
			return nil, nil, err
		}

		b := proto.NewDialBroker()
		s := browser.NewServer(b, c, mu)

		client, closeFn := setupWmIntTest(t, b, s)
		closeFns = append(closeFns, func() {
			s.Close()
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

	testRegister(t, func(ed Editor, mu *sync.Mutex, res workspace.URI) (*Component, Editor, error) {
		c, err := newTestComponentErr(ed)
		if err != nil {
			return nil, nil, err
		}

		if m, ok := ed.(*MockEditor); ok {
			expectInitialServerSubscribe(t, m)
		}

		b := proto.NewDialBroker()
		s := NewServer(b, c, mu)

		client, closeFn := setupIntTest(t, b, s)
		closeFns = append(closeFns, func() {
			s.Close()
			closeFn()
		})

		return c, client, err
	})

	for _, closeFn := range closeFns {
		closeFn()
	}
}
